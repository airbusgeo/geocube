package svc

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/airbusgeo/geocube/interface/database"
	"github.com/airbusgeo/geocube/interface/storage/uri"
	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/geocube/internal/log"
	"github.com/airbusgeo/geocube/internal/utils"
	"github.com/airbusgeo/geocube/internal/utils/grid"
)

// HandleEvent handles TaskEvent and JobEvent for a job
func (svc *Service) HandleEvent(ctx context.Context, evt geocube.Event) error {
	if taskevt, ok := evt.(geocube.TaskEvent); ok {
		if err := svc.handleTaskEvt(ctx, taskevt); err != nil {
			if !utils.Temporary(err) {
				// TODO handle this case : it may result in a storage leak !!!
				return geocube.NewUnhandledEvent("FATAL %v!!! Job %s Task %s Status %s", err, taskevt.JobID, taskevt.TaskID, taskevt.Status.String())
			}
			return err
		}
		return nil
	}
	return svc.handleJobEvt(ctx, evt.(geocube.JobEvent))
}

func (svc *Service) handleJobEvt(ctx context.Context, evt geocube.JobEvent) error {
	// Get Job
	job, err := svc.GetJob(ctx, evt.JobID)
	if err != nil {
		return fmt.Errorf("handleJobEvt.%w", err)
	}

	// Trigger the event
	if err = job.Trigger(evt); err != nil {
		return fmt.Errorf("handleJobEvt.%w", err)
	}

	// Save the Job
	if err = svc.saveJob(ctx, nil, job); err != nil {
		return fmt.Errorf("handleJobEvt.%w", err)
	}

	// Launch the commands associated to the state
	start := time.Now()
	err = svc.csldOnEnterNewState(ctx, job)
	job.Log.Printf("  ... in %v", time.Since(start))
	return err
}

func (svc *Service) handleTaskEvt(ctx context.Context, evt geocube.TaskEvent) error {
	switch evt.Status {
	case geocube.TaskCancelled:
		// manage task cancelled
		return nil
	case geocube.TaskSuccessful:
		// manage tasks succeeded but job is cancelled (tasks are deleted)
		job, err := svc.db.ReadJob(ctx, evt.JobID)
		if err != nil {
			return fmt.Errorf("handleTaskEvt(%s).%w", evt.TaskID, err)
		}

		if job.State == geocube.JobStateABORTED || job.State == geocube.JobStateFAILED {
			return nil
		}
	default:
	}

	// Get Job with the task of the event
	job, err := svc.db.ReadJobWithTask(ctx, evt.JobID, evt.TaskID)
	if err != nil {
		return fmt.Errorf("handleTaskEvt(%s).%w", evt.TaskID, err)
	}
	job.Clean(true)

	if err = job.UpdateTask(evt); err != nil {
		return fmt.Errorf("handleTaskEvt(%s).%w", evt.TaskID, err)
	}

	if err = svc.saveJob(ctx, nil, job); err != nil {
		return fmt.Errorf("handleTaskEvt(%s).%w", evt.TaskID, err)
	}

	job.Log.Printf("TaskEvt received with status %s (id:%s, err:%s)", evt.Status.String(), evt.TaskID, evt.Error)

	if job.ActiveTasks == 0 {
		if job.State == geocube.JobStateCONSOLIDATIONCANCELLING {
			job.Log.Println("Job has been canceled")
			return svc.publishEvent(ctx, geocube.CancellationDone, job, "")
		}
		if job.FailedTasks > 0 {
			return svc.publishEvent(ctx, geocube.ConsolidationFailed, job,
				fmt.Sprintf("Job failed: %d tasks failed\n", job.FailedTasks))
		}
		return svc.publishEvent(ctx, geocube.ConsolidationDone, job, "")
	}

	return nil
}

// csldOnEnterNewState should only returns publishing error
// All the other errors must be handle by the state machine
func (svc *Service) csldOnEnterNewState(ctx context.Context, j *geocube.Job) error {
	switch j.State {
	case geocube.JobStateNEW:
		return svc.csldSendJobCreated(ctx, j)

	case geocube.JobStateCREATED:
		return svc.csldPrepareOrders(ctx, j)

	case geocube.JobStateCONSOLIDATIONINPROGRESS:
		return svc.csldSendOrders(ctx, j)

	case geocube.JobStateCONSOLIDATIONDONE:
		return svc.csldIndex(ctx, j)

	case geocube.JobStateCONSOLIDATIONINDEXED:
		return svc.csldSwapDatasets(ctx, j)

	case geocube.JobStateCONSOLIDATIONEFFECTIVE:
		return svc.csldDeleteDatasets(ctx, j)

	case geocube.JobStateDONE:
		// Finished !
		return nil

	case geocube.JobStateDONEBUTUNTIDY:
		j.Log.Printf(j.Payload.Err)
		return svc.csldContactAdmin(ctx, j)

	case geocube.JobStateCONSOLIDATIONCANCELLING:
		return svc.csldCancel(ctx, j)

	case geocube.JobStateCONSOLIDATIONFAILED, geocube.JobStateINITIALISATIONFAILED, geocube.JobStateCANCELLATIONFAILED:
		j.Log.Printf(j.Payload.Err)
		j.Log.Printf("Wait for user command...")
		return nil

	case geocube.JobStateCONSOLIDATIONRETRYING:
		return svc.csldRetry(ctx, j)

	case geocube.JobStateABORTED:
		return svc.csldRollback(ctx, j)

	case geocube.JobStateFAILED:
		j.Log.Printf(j.Payload.Err)
		// Finished but...
		return nil
	}

	return nil
}

func (svc *Service) csldSendJobCreated(ctx context.Context, job *geocube.Job) error {
	return svc.publishEvent(ctx, geocube.JobCreated, job, "")
}

func fillRecordsTime(recordsTime map[string]string, records []*geocube.Record) {
	for _, record := range records {
		recordsTime[record.ID] = record.Time.Format("2006-01-02 15:04:05")
	}
}

type CsldDataset struct {
	ID            string
	Event         geocube.ConsolidationDataset
	Consolidation bool
	RecordID      string
}

func (svc *Service) csldPrepareOrders(ctx context.Context, job *geocube.Job) error {
	job.Log.Printf("Prepare consolidation orders...")
	logger := log.Logger(ctx).Sugar()

	err := svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
		start := time.Now()

		// Get all the records id and datetime of the job
		recordsTime := make(map[string]string)
		{
			records, err := txn.FindRecords(ctx, "", nil, time.Time{}, time.Time{}, job.ID, nil, 0, 0, false, false)
			if err != nil {
				return fmt.Errorf("csldPrepareOrders.%w", err)
			}
			fillRecordsTime(recordsTime, records)
		}
		logger.Debugf("FindRecords (%d):%v\n", len(recordsTime), time.Since(start))
		start = time.Now()

		// Get Variable
		variable, err := txn.ReadVariableFromInstanceID(ctx, job.Payload.InstanceID)
		if err != nil {
			return fmt.Errorf("csldPrepareOrders.%w", err)
		}
		variable.Clean(true)

		// Get Consolidation parameters
		params, err := txn.ReadConsolidationParams(ctx, job.Payload.ParamsID)
		if err != nil {
			return fmt.Errorf("csldPrepareOrders.%w", err)
		}
		params.Clean()

		// Get all the cells in the layout covering all the datasets locked by the job
		var cells <-chan *grid.Cell
		var layout *geocube.Layout
		{
			// Get the union of geometries of all the datasets locked by the job
			aoi, err := txn.GetGeomUnionLockedDataset(ctx, job.ID)
			if err != nil {
				return fmt.Errorf("csldPrepareOrders.%w", err)
			}
			logger.Debugf("GetUnionGeom:%v\n", time.Since(start))
			start = time.Now()

			// Get the layout
			layout, err = txn.ReadLayout(ctx, job.Payload.LayoutID)
			if err != nil {
				return fmt.Errorf("csldPrepareOrders.%w", err)
			}

			// Get all the cells covering the AOI in the layout
			cells, err = layout.Covers(ctx, aoi)
			if err != nil {
				return fmt.Errorf("csldPrepareOrders.%w", err)
			}
			logger.Debugf("ReadAndCoverLayout:%v\n", time.Since(start))
		}

		start = time.Now()

		// Create one task per cell
		datasetsToBeConsolidated := utils.StringSet{}
		for cell := range cells {
			// Retrieve the datasets to be consolidated
			var datasets []*CsldDataset
			uniqueDatasetsID := utils.StringSet{}
			{
				// Retrieve all the datasets covering the cell
				ds, err := txn.FindDatasets(ctx, geocube.DatasetStatusACTIVE, "", job.ID, nil, nil, geocube.Metadata{},
					time.Time{}, time.Time{}, &cell.GeographicRing, &cell.Ring, 0, 0, true)
				if err != nil {
					return fmt.Errorf("csldPrepareOrders.%w", err)
				}
				// No datasets on this cell, skip it
				if len(ds) == 0 {
					continue
				}
				// Create InputDatasets
				datasets = make([]*CsldDataset, 0, len(ds))
				for _, dataset := range ds {
					datasets = append(datasets, &CsldDataset{
						ID:       dataset.ID,
						Event:    *geocube.NewConsolidationDataset(dataset),
						RecordID: dataset.RecordID,
					})
					uniqueDatasetsID.Push(dataset.ID)
				}
			}

			// Create a basic ConsolidationContainer
			containerBaseName := utils.URLJoin(svc.ingestionStoragePath, layout.Name+layout.ID, cell.URI, job.Payload.InstanceID)
			containerBase, err := geocube.NewConsolidationContainer(containerBaseName, variable, params, layout, cell)
			if err != nil {
				return fmt.Errorf("csldPrepareOrders.%w", err)
			}

			// Check if a consolidation is needed and handle reconsolidation
			if need, err := csldPrepareOrdersNeedConsolidation(ctx, txn, &datasets, uniqueDatasetsID, containerBase); err != nil || !need {
				if err != nil {
					return fmt.Errorf("csldPrepareOrders.%w", err)
				}
				// Consolidation is not needed
				continue
			}

			// Sort the dataset by datetime
			if err := csldPrepareOrdersSortDatasets(ctx, txn, datasets, recordsTime); err != nil {
				return fmt.Errorf("csldPrepareOrders.%w", err)
			}

			// Exclude full containers
			datasets = csldPrepareOrdersExcludeFullContainers(datasets, layout.MaxRecords)

			// Check that datasets are available
			checkAvailability := false
			if checkAvailability {
				var errs []string
				for _, dataset := range datasets {
					uri, err := uri.ParseUri(dataset.Event.URI)
					if err != nil {
						errs = append(errs, dataset.Event.URI+": "+err.Error())
						continue
					}
					strategy, err := uri.NewStorageStrategy(ctx)
					if err != nil {
						errs = append(errs, dataset.Event.URI+": "+err.Error())
						continue
					}
					exist, err := strategy.Exist(ctx, dataset.Event.URI)
					if err != nil {
						errs = append(errs, dataset.Event.URI+": "+err.Error())
						continue
					}
					if !exist {
						errs = append(errs, dataset.Event.URI+" does not exists")
					}
				}
				if len(errs) > 0 {
					return fmt.Errorf(strings.Join(errs, "\n"))
				}

			}

			// Group datasets by records
			records := make([]geocube.ConsolidationRecord, 0, len(datasets))
			for i := 0; i < len(datasets); {
				record := geocube.ConsolidationRecord{ID: datasets[i].RecordID, DateTime: recordsTime[datasets[i].RecordID]}
				for ; i < len(datasets) && record.ID == datasets[i].RecordID; i++ {
					record.Datasets = append(record.Datasets, datasets[i].Event)
					datasetsToBeConsolidated.Push(datasets[i].ID)
				}
				records = append(records, record)
			}

			// Create all the consolidationContainer
			nbOfContainers := (len(records)-1)/layout.MaxRecords + 1
			idx := 0
			for i := 0; i < nbOfContainers; i++ {
				// Search for an available URI
				for {
					idx++
					containerBase.URI = fmt.Sprintf("%s/%s", containerBaseName, strconv.Itoa(idx)+".tif")
					if _, err := txn.ReadContainers(ctx, []string{containerBase.URI}); err != nil {
						if geocube.IsError(err, geocube.EntityNotFound) {
							break
						}
						return fmt.Errorf("clsdPrepareOrders.%w", err)
					}
				}

				// Create a consolidation event
				evt := geocube.ConsolidationEvent{
					JobID:     job.ID,
					Container: *containerBase,
					Records:   records[i*layout.MaxRecords : utils.MinI(len(records), (i+1)*layout.MaxRecords)],
				}

				// Create a consolidation task
				if err = job.CreateConsolidationTask(evt); err != nil {
					return fmt.Errorf("csldPrepareOrders.%w", err)
				}
			}

		}
		if len(job.Tasks) != 0 {
			logger.Debugf("Create %d Tasks (mean): %v\n", len(job.Tasks), time.Since(start)/time.Duration(len(job.Tasks)))
		}

		// Lock the datasets that will be deleted after the consolidation
		job.LockDatasets(datasetsToBeConsolidated.Slice(), geocube.LockFlagTODELETE)

		// Release the datasets used for initialisation
		job.ReleaseDatasets(geocube.LockFlagINIT)

		// Save job
		return svc.saveJob(ctx, txn, job)
	})

	if err != nil {
		return svc.publishEvent(ctx, geocube.PrepareConsolidationOrdersFailed, job, err.Error())
	}

	return svc.publishEvent(ctx, geocube.ConsolidationOrdersPrepared, job, "")
}

// csldPrepareOrdersSortDatasets is a subtask of csldPrepareOrders
// fetching the records time and sorting the dataset by recordDateTime
func csldPrepareOrdersSortDatasets(ctx context.Context, txn database.GeocubeTxBackend, datasets []*CsldDataset, recordsTime map[string]string) error {
	// Fetch the records of datasets that are still unknown
	inputRecordsID := make([]string, 0, len(datasets))
	for i := range datasets {
		if _, ok := recordsTime[datasets[i].RecordID]; !ok {
			inputRecordsID = append(inputRecordsID, datasets[i].RecordID)
		}
	}
	rs, err := txn.ReadRecords(ctx, inputRecordsID)
	if err != nil {
		return fmt.Errorf("csldPrepareOrdersSortDatasets.%w", err)
	}
	// Fill the recordsTime struct
	fillRecordsTime(recordsTime, rs)
	// Sort the datasets
	sort.Slice(datasets, func(i, j int) bool {
		return recordsTime[datasets[i].RecordID] < recordsTime[datasets[j].RecordID]
	})
	return nil
}

// csldPrepareOrdersExcludeFullContainers is a subtask of csldPrepareOrders
// excluding the datasets belonging to a full container that does not need to be reconsolidated
func csldPrepareOrdersExcludeFullContainers(datasets []*CsldDataset, maxRecords int) []*CsldDataset {
	for ii, i := 0, 0; i < len(datasets); {
		n := 0
		// Find the contiguous datasets requiring consolidation and belonging to the same container
		for ; i < len(datasets) && !datasets[i].Consolidation; i++ {
			if datasets[i].Event.URI != datasets[ii].Event.URI {
				break
			}
			n++
		}
		// if the container is full and do not require reconsolidation => remove the datasets
		if n == maxRecords {
			datasets = append(datasets[:ii], datasets[i:]...)
			i = ii
		} else {
			ii = i
			i++
		}
	}
	return datasets
}

// csldPrepareOrdersNeedConsolidation is a subtask of csldPrepareOrders
// testing if a consolidation is needed (handle the reconsolidation case)
func csldPrepareOrdersNeedConsolidation(ctx context.Context, txn database.GeocubeTxBackend, datasets *[]*CsldDataset, uniqueDatasetsID utils.StringSet, containerBase *geocube.ConsolidationContainer) (bool, error) {
	need, noNeedReconsolidation := csldPrepareOrdersNeedReconsolidation(datasets, containerBase)

	if !need {
		return false, nil
	}

	// Browse the containers that do not need reconsolidation and append their datasets to the list
	if len(noNeedReconsolidation) > 0 {
		// Retrieve the containers
		containers, err := txn.ReadContainers(ctx, noNeedReconsolidation)
		if err != nil {
			return false, fmt.Errorf("csldPrepareOrders.%w", err)
		}
		for _, container := range containers {
			// Add all the datasets of the container
			for _, dataset := range container.Datasets {
				if !uniqueDatasetsID.Exists(dataset.ID) {
					uniqueDatasetsID.Push(dataset.ID)
					*datasets = append(*datasets, &CsldDataset{
						ID:       dataset.ID,
						Event:    *geocube.NewConsolidationDataset(dataset),
						RecordID: dataset.RecordID,
					})
				}
			}
		}
	}
	return true, nil
}

// csldPrepareOrdersNeedReconsolidation is a subtask of csldPrepareOrders
// testing if a consolidation is needed (handle the reconsolidation case)
func csldPrepareOrdersNeedReconsolidation(datasets *[]*CsldDataset, containerBase *geocube.ConsolidationContainer) (bool, []string) {
	var noNeedReconsolidation []string       // store the uri of the containers that are already consolidated, but may be reconsolidated
	needReconsolidation := map[string]bool{} // store the uri of the containers that are already consolidated and need to be reconsolidated
	consolidation := false
	for _, dataset := range *datasets {
		if !dataset.Event.InGroupOfContainers(containerBase) {
			// Consolidation is needed if the dataset is not in a consolidated container
			dataset.Consolidation = true
			consolidation = true
		} else if need, ok := needReconsolidation[dataset.Event.URI]; ok {
			// ... or if the consolidated container needs reconsolidation
			dataset.Consolidation = need
		} else if dataset.Event.NeedsReconsolidation(containerBase) {
			// ... or if the consolidationParams changed
			dataset.Consolidation = true
			consolidation = true
			needReconsolidation[dataset.Event.URI] = true
		} else {
			// The dataset is already in a consolidated container and nothing changed.
			// The whole container may eventually be reconsolidated if not already full.
			needReconsolidation[dataset.Event.URI] = false
			noNeedReconsolidation = append(noNeedReconsolidation, dataset.Event.URI)
		}
	}
	return consolidation, noNeedReconsolidation
}

func (svc *Service) csldSendOrders(ctx context.Context, job *geocube.Job) error {
	job.Log.Print("Send consolidation orders...")

	// Retrieves tasks
	tasks, err := svc.db.ReadTasks(ctx, job.ID, []geocube.TaskState{geocube.TaskStatePENDING})
	if err != nil {
		return svc.publishEvent(ctx, geocube.SendConsolidationOrdersFailed, job, "Unable to retrieve tasks")
	}

	var consolidationOrders [][]byte
	for _, task := range tasks {
		consolidationOrders = append(consolidationOrders, task.Payload)
	}

	// If there is no consolidation orders to send, consolidation is done
	if len(consolidationOrders) == 0 {
		job.Log.Print("No consolidation orders found !")
		return svc.publishEvent(ctx, geocube.ConsolidationDone, job, "")
	}

	// Publish
	if err := svc.consolidationPublisher.Publish(ctx, consolidationOrders...); err != nil {
		return svc.publishEvent(ctx, geocube.SendConsolidationOrdersFailed, job, err.Error())
	}
	return nil
}

func (svc *Service) csldIndex(ctx context.Context, job *geocube.Job) (err error) {
	job.Log.Print("Indexing new datasets...")

	// Retrieves tasks
	if job.Tasks, err = svc.db.ReadTasks(ctx, job.ID, []geocube.TaskState{geocube.TaskStateDONE}); err != nil {
		return svc.publishEvent(ctx, geocube.ConsolidationIndexingFailed, job, err.Error())
	}

	// Create new datasets
	for len(job.Tasks) > 0 {
		err := svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
			container, records, err := job.Tasks[0].ConsolidationOutput()
			if err != nil {
				return fmt.Errorf("csldIndex.%w", err)
			}
			newContainer, err := geocube.NewContainerFromConsolidation(container)
			if err != nil {
				return fmt.Errorf("csldIndex.%w", err)
			}

			// Datasets are all pretty much the same
			incompleteDataset, err := geocube.IncompleteDatasetFromConsolidation(container, job.Payload.InstanceID)
			if err != nil {
				return fmt.Errorf("csldIndex.%w", err)
			}
			// Specialize incompleteDataset
			newDatasets := make([]*geocube.Dataset, 0, len(records))
			for i, r := range records {
				newDataset, err := geocube.NewDatasetFromIncomplete(*incompleteDataset, r.ID, "GTIFF_DIR:"+strconv.Itoa(i+1))
				if err != nil {
					return fmt.Errorf("csldIndex.%w", err)
				}
				newDatasets = append(newDatasets, newDataset)
			}
			// Prepare the container for indexation
			if err = svc.prepareIndexation(ctx, txn, newContainer, newDatasets); err != nil {
				return fmt.Errorf("csldIndex.%w", err)
			}

			// Lock the new datasets
			{
				datasetsID := make([]string, 0, len(newContainer.Datasets))
				for _, dataset := range newContainer.Datasets {
					if dataset.IsNew() {
						datasetsID = append(datasetsID, dataset.ID)
					}
				}
				job.LockDatasets(datasetsID, geocube.LockFlagNEW)
			}

			// Save the container
			if err = svc.saveContainer(ctx, txn, newContainer); err != nil {
				return fmt.Errorf("csldIndex.%w", err)
			}

			// Delete task
			// TODO Consolidation delete tasks one by one ? Or all in a row (in the last case, it's more difficult to handle a Retry)
			job.DeleteTask(0)

			// Save job
			return svc.saveJob(ctx, txn, job)
		})
		if err != nil {
			return svc.publishEvent(ctx, geocube.ConsolidationIndexingFailed, job, err.Error())
		}
	}

	return svc.publishEvent(ctx, geocube.ConsolidationIndexed, job, "")
}

func (svc *Service) csldSwapDatasets(ctx context.Context, job *geocube.Job) error {
	job.Log.Print("Swap datasets...")

	err := svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
		// Active datasets are tagged to_delete
		err := txn.ChangeDatasetsStatus(ctx, job.ID, geocube.DatasetStatusACTIVE, geocube.DatasetStatusTODELETE)
		if err != nil {
			return err
		}

		// Inactive datasets are tagged active
		err = txn.ChangeDatasetsStatus(ctx, job.ID, geocube.DatasetStatusINACTIVE, geocube.DatasetStatusACTIVE)
		if err != nil {
			return err
		}
		// Release all the new datasets
		job.ReleaseDatasets(geocube.LockFlagNEW)

		// Persist changes in db
		return svc.saveJob(ctx, txn, job)
	})

	if err != nil {
		return svc.publishEvent(ctx, geocube.SwapDatasetsFailed, job, err.Error())
	}
	return svc.publishEvent(ctx, geocube.DatasetsSwapped, job, "")
}

func (svc *Service) csldDeleteDatasets(ctx context.Context, job *geocube.Job) error {
	job.Log.Print("Tidy job...")

	errors, err := svc.csldSubFncDeleteJobDatasetsAndContainers(ctx, job, geocube.LockFlagTODELETE, geocube.DatasetStatusTODELETE)

	if err = utils.MergeErrors(true, err, errors...); err != nil {
		job.Log.Println(err.Error())
		return svc.publishEvent(ctx, geocube.DeletionFailed, job, err.Error())
	}

	return svc.publishEvent(ctx, geocube.DeletionDone, job, "")
}

// csldSubFncDeleteJobDatasetsAndContainers deletes all the datasets locked by the job with the given status and the containers (if empty)
// Returns a list of errors for the deletion of remote containers
func (svc *Service) csldSubFncDeleteJobDatasetsAndContainers(ctx context.Context, job *geocube.Job, lockFlag geocube.LockFlag, datasetStatus geocube.DatasetStatus) ([]error, error) {
	var emptyContainers []*geocube.Container
	if err := svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
		// Find the datasets of the job with the given status
		datasets, err := txn.FindDatasets(ctx, datasetStatus, "", job.ID, nil, nil, geocube.Metadata{}, time.Time{}, time.Time{}, nil, nil, 0, 0, true)
		if err != nil {
			return fmt.Errorf("csldSubFncDeleteJobDatasetsAndContainers.%w", err)
		}
		// First, release the datasets (otherwise the database cannot delete them)
		job.ReleaseDatasets(lockFlag)
		if err = svc.saveJob(ctx, txn, job); err != nil {
			return fmt.Errorf("csldSubFncDeleteJobDatasetsAndContainers.%w", err)
		}

		// Then, delete them
		if emptyContainers, err = svc.csldSubFncDeleteDatasetsAndUnmanagedContainers(ctx, txn, datasets); err != nil {
			return fmt.Errorf("csldSubFncDeleteJobDatasetsAndContainers.%w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// It's not mandatory to delete the empty containers (it can be done latter), but we will do what we can
	// As there is a lot of chance of failure, we do it outside the previous unitofwork
	return svc.deleteEmptyContainers(ctx, emptyContainers)
}

// csldSubFncDeleteDatasetsAndUnmanagedContainers deletes all the datasets and the containers (if empty and not managed)
// Returns a list of empty and managed containers to be deleted
func (svc *Service) csldSubFncDeleteDatasetsAndUnmanagedContainers(ctx context.Context, txn database.GeocubeTxBackend, datasets []*geocube.Dataset) (emptyManagedContainers []*geocube.Container, err error) {
	containers := map[string]*geocube.Container{}
	var containersURI []string

	// Get all the containersURI
	for _, dataset := range datasets {
		if _, ok := containers[dataset.ContainerURI]; !ok {
			containers[dataset.ContainerURI] = nil
			containersURI = append(containersURI, dataset.ContainerURI)
		}
	}
	// Fetch containers
	cs, err := txn.ReadContainers(ctx, containersURI)
	if err != nil {
		return nil, fmt.Errorf("csldSubFncDeleteDatasetsAndUnmanagedContainers.%w", err)
	}
	for _, c := range cs {
		c.Clean(true)
		containers[c.URI] = c
	}

	// Delete datasets
	for _, dataset := range datasets {
		container := containers[dataset.ContainerURI]
		if empty, err := container.RemoveDataset(dataset.ID); empty || err != nil {
			if err != nil {
				return nil, fmt.Errorf("csldSubFncDeleteDatasetsAndUnmanagedContainers.%w", err)
			}
			if container.Managed {
				emptyManagedContainers = append(emptyManagedContainers, container)
			} else {
				container.Delete()
			}
		}
	}

	// Save containers
	for _, container := range containers {
		if err = svc.saveContainer(ctx, txn, container); err != nil {
			return nil, fmt.Errorf("csldSubFncDeleteDatasetsAndUnmanagedContainers.%w", err)
		}
	}

	return emptyManagedContainers, nil
}

// csldSubFncDeletePendingRemoteContainers physically delete remote containers that are not indexed
// It scans the tasks done and delete the container if not indexed.
// job.Tasks must contain the tasks done at least.
func (svc *Service) csldSubFncDeletePendingRemoteContainers(ctx context.Context, job *geocube.Job) error {
	for i, task := range job.Tasks {
		if task.State == geocube.TaskStateDONE {
			container, _, err := task.ConsolidationOutput()
			if err != nil {
				return fmt.Errorf("DeletePendingRemoteContainers.%w", err)
			}
			// If the container does not exist in the database
			if _, err = svc.db.ReadContainers(ctx, []string{container.URI}); !geocube.IsError(err, geocube.EntityNotFound) {
				if err != nil {
					return fmt.Errorf("DeletePendingRemoteContainers.%w", err)
				}
				// The container exists in the database. It is not pending. Issue a warning
				job.Log.Println("Rollback: RemoteContainer " + container.URI + " is not pending and cannot be deleted.")
				continue
			}
			// Physically delete the container
			containerURI, err := uri.ParseUri(container.URI)
			if err != nil {
				return fmt.Errorf("DeletePendingRemoteContainers.%w", err)
			}
			if err := containerURI.Delete(ctx); err != nil {
				return fmt.Errorf("DeletePendingRemoteContainers.%w", err)
			}
			// Cancel the task so that the deletion is not done twice and save
			job.CancelTask(i)
			if err = svc.saveJob(ctx, nil, job); err != nil {
				return fmt.Errorf("DeletePendingRemoteContainers.%w", err)
			}
		}
	}
	return nil
}

func (svc *Service) csldContactAdmin(ctx context.Context, job *geocube.Job) error {
	job.Log.Print("Contact admin...")
	//TODO Contact Admin
	return nil
}

func (svc *Service) csldCancel(ctx context.Context, job *geocube.Job) error {
	job.Log.Print("Cancel all tasks...")

	if job.ActiveTasks == 0 {
		return svc.publishEvent(ctx, geocube.CancellationDone, job, "")
	}

	var err error
	if job.Tasks, err = svc.db.ReadTasks(ctx, job.ID, []geocube.TaskState{geocube.TaskStatePENDING}); err != nil {
		return svc.publishEvent(ctx, geocube.CancellationFailed, job, err.Error())
	}

	for taskIndex, task := range job.Tasks {
		job.CancelTask(taskIndex)

		// Create cancelled file
		path := svc.cancelledConsolidationPath + "/" + fmt.Sprintf("%s_%s", job.ID, task.ID)
		cancelledJobsURI, err := uri.ParseUri(path)
		if err != nil {
			return svc.publishEvent(ctx, geocube.CancellationFailed, job, fmt.Sprintf("%s: %s", err.Error(), path))
		}

		if err := cancelledJobsURI.Upload(ctx, nil); err != nil {
			return svc.publishEvent(ctx, geocube.CancellationFailed, job, err.Error())
		}

		if err = svc.saveJob(ctx, nil, job); err != nil {
			return svc.publishEvent(ctx, geocube.CancellationFailed, job, err.Error())
		}
	}
	return svc.publishEvent(ctx, geocube.CancellationDone, job, "")
}

func (svc *Service) csldRetry(ctx context.Context, job *geocube.Job) error {
	job.Log.Print("Retry...")

	var err error
	// Load tasks
	if job.Tasks, err = svc.db.ReadTasks(ctx, job.ID, []geocube.TaskState{geocube.TaskStateFAILED}); err != nil {
		return svc.publishEvent(ctx, geocube.ConsolidationRetryFailed, job, err.Error())
	}
	// Reset and save task status
	job.ResetAllTasks()
	if err = svc.saveJob(ctx, nil, job); err != nil {
		return svc.publishEvent(ctx, geocube.ConsolidationRetryFailed, job, err.Error())
	}
	return svc.publishEvent(ctx, geocube.ConsolidationOrdersPrepared, job, "")
}

// csldRollback encapsulates csldSubFncRollback
func (svc *Service) csldRollback(ctx context.Context, job *geocube.Job) error {
	job.Log.Print("Rollback...")
	if err := svc.csldSubFncRollback(ctx, job); err != nil {
		return svc.publishEvent(ctx, geocube.RollbackFailed, job, err.Error())
	}
	return svc.publishEvent(ctx, geocube.RollbackDone, job, "")
}

func (svc *Service) csldSubFncRollback(ctx context.Context, job *geocube.Job) error {
	var err error

	// Load tasks
	if job.Tasks, err = svc.db.ReadTasks(ctx, job.ID, nil); err != nil {
		return fmt.Errorf("Rollback.%w", err)
	}

	// Rollback from JobStateCONSOLIDATIONINDEXED: Unswap datasets
	// Nothing to do as swapDataset is a unitOfWork

	// Rollback from JobStateCONSOLIDATIONDONE: delete the inactive datasets
	{
		errors, err := svc.csldSubFncDeleteJobDatasetsAndContainers(ctx, job, geocube.LockFlagNEW, geocube.DatasetStatusINACTIVE)
		if err != nil {
			return fmt.Errorf("Rollback.%w", err)
		}
		if errors != nil {
			errs := make([]string, len(errors))
			for i, err := range errors {
				errs[i] = err.Error()
			}
			return fmt.Errorf("Rollback:\n%s", strings.Join(errs, "\n"))
		}
	}

	// Rollback from JobStateCONSOLIDATIONINPROGRESS: delete the unindexed containers
	if err = svc.csldSubFncDeletePendingRemoteContainers(ctx, job); err != nil {
		return fmt.Errorf("Rollback.%w", err)
	}

	// Rollback from JobStateCREATED: delete consolidation orders and datasets ToDelete
	job.DeleteAllTasks()
	job.ReleaseDatasets(geocube.LockFlagTODELETE)

	// Rollback from JobStateNEW: release old datasets
	job.ReleaseDatasets(geocube.LockFlagINIT)

	// Persist DeleteTasks and ReleaseDatasets
	if err = svc.saveJob(ctx, nil, job); err != nil {
		return fmt.Errorf("Rollback.%w", err)
	}

	return nil
}

func (svc *Service) publishEvent(ctx context.Context, status geocube.JobStatus, job *geocube.Job, serr string) error {
	job.Log.Printf("  Event %s %s...", status.String(), serr)

	evt := geocube.NewJobEvent(job.ID, status, serr)

	data, err := geocube.MarshalEvent(evt)
	if err != nil {
		panic("Unable to marshal event")
	}

	return svc.eventPublisher.Publish(ctx, data)
}
