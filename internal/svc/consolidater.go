package svc

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/airbusgeo/geocube/interface/database"
	"github.com/airbusgeo/geocube/interface/storage/uri"
	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/geocube/internal/log"
	"github.com/airbusgeo/geocube/internal/utils"
	"github.com/google/uuid"
)

// csldOnEnterNewState should only returns publishing error
// All the other errors must be handle by the state machine
func (svc *Service) csldOnEnterNewState(ctx context.Context, j *geocube.Job) error {
	switch j.State {
	case geocube.JobStateNEW:
		return svc.publishEvent(ctx, geocube.JobCreated, j, "")

	case geocube.JobStateCREATED:
		if err := svc.csldPrepareOrders(ctx, j); err != nil {
			return svc.publishEvent(ctx, geocube.PrepareOrdersFailed, j, err.Error())
		}
		return svc.publishEvent(ctx, geocube.OrdersPrepared, j, "")

	case geocube.JobStateCONSOLIDATIONINPROGRESS:
		if err := svc.csldSendOrders(ctx, j); err != nil {
			return svc.publishEvent(ctx, geocube.SendOrdersFailed, j, err.Error())
		}
		return nil

	case geocube.JobStateCONSOLIDATIONDONE:
		if err := svc.csldIndex(ctx, j); err != nil {
			return svc.publishEvent(ctx, geocube.ConsolidationIndexingFailed, j, err.Error())
		}
		return svc.publishEvent(ctx, geocube.ConsolidationIndexed, j, "")

	case geocube.JobStateCONSOLIDATIONINDEXED:
		if err := svc.csldSwapDatasets(ctx, j); err != nil {
			return svc.publishEvent(ctx, geocube.SwapDatasetsFailed, j, err.Error())
		}
		return svc.publishEvent(ctx, geocube.DatasetsSwapped, j, "")

	case geocube.JobStateCONSOLIDATIONEFFECTIVE:
		if err := svc.csldDeleteDatasets(ctx, j); err != nil {
			return svc.publishEvent(ctx, geocube.StartDeletionFailed, j, err.Error())
		}
		return svc.publishEvent(ctx, geocube.DeletionStarted, j, "")

	case geocube.JobStateDONE:
		// Finished !
		log.Logger(ctx).Sugar().Debug("consolidation successfully completed")
		return nil

	case geocube.JobStateDONEBUTUNTIDY:
		return svc.opContactAdmin(ctx, j)

	case geocube.JobStateCONSOLIDATIONCANCELLING:
		if err := svc.csldCancel(ctx, j); err != nil {
			return svc.publishEvent(ctx, geocube.CancellationFailed, j, err.Error())
		}
		return svc.publishEvent(ctx, geocube.CancellationDone, j, "")

	case geocube.JobStateCONSOLIDATIONFAILED, geocube.JobStateINITIALISATIONFAILED, geocube.JobStateCANCELLATIONFAILED:
		j.LogErr("Consolidation failed")
		j.LogMsg(geocube.INFO, "Wait for user command...")
		return nil

	case geocube.JobStateCONSOLIDATIONRETRYING:
		if err := svc.csldConsolidationRetry(ctx, j); err != nil {
			return svc.publishEvent(ctx, geocube.ConsolidationRetryFailed, j, err.Error())
		}
		return svc.publishEvent(ctx, geocube.OrdersPrepared, j, "")

	case geocube.JobStateABORTED:
		if err := svc.csldRollback(ctx, j); err != nil {
			return svc.publishEvent(ctx, geocube.RollbackFailed, j, err.Error())
		}
		return svc.publishEvent(ctx, geocube.RollbackDone, j, "")

	case geocube.JobStateROLLBACKFAILED:
		j.LogErr("Rollback failed")
		j.LogMsg(geocube.INFO, "Wait for user command...")
		// Finished but...
		return nil

	case geocube.JobStateFAILED:
		j.LogErr("Job failed")
		// Finished but...
		return nil
	}

	return nil
}

func (svc Service) csldInit(ctx context.Context, job *geocube.Job, datasetsID []string) error {
	job.LogMsgf(geocube.DEBUG, "Init with %d datasets", len(datasetsID))
	if len(datasetsID) == 0 {
		return geocube.NewEntityNotFound("", "", "", "No dataset found for theses records and instances")
	}

	if err := svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
		// Check and get consolidation parameters
		var params *geocube.ConsolidationParams
		{
			variable, err := txn.ReadVariableFromInstanceID(ctx, job.Payload.InstanceID)
			if err != nil {
				return err
			}
			if params, err = txn.ReadConsolidationParams(ctx, variable.ID); err != nil {
				return err
			}
			params.Clean()
			if err := job.SetParams(*params); err != nil {
				return err
			}
		}

		// Lock datasets
		job.LockDatasets(datasetsID, geocube.LockFlagINIT)
		// Persist the job
		start := time.Now()
		if err := svc.saveJob(ctx, txn, job); err != nil {
			return err
		}
		log.Logger(ctx).Sugar().Debugf("SaveJob: %v\n", time.Since(start))

		return nil
	}); err != nil {
		return fmt.Errorf("csldInit.%w", err)
	}

	// Start the job
	log.Logger(ctx).Sugar().Debug("new consolidation job started")
	if err := svc.csldOnEnterNewState(ctx, job); err != nil {
		return fmt.Errorf("csldInit.%w", err)
	}
	return nil
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
	job.LogMsg(geocube.INFO, "Prepare consolidation orders...")
	logger := log.Logger(ctx).Sugar()

	return svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
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
		job.LogMsgf(geocube.DEBUG, "%d record(s) found", len(recordsTime))
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
		var cells <-chan geocube.StreamedCell
		var layout *geocube.Layout
		{
			// Get the union of geometries of all the datasets locked by the job
			aoi, err := txn.GetDatasetsGeometryUnion(ctx, job.ID)
			if err != nil {
				return fmt.Errorf("csldPrepareOrders.%w", err)
			}
			logger.Debugf("GetUnionGeom:%v\n", time.Since(start))
			start = time.Now()

			// Get the layout
			layout, err = txn.ReadLayout(ctx, job.Payload.Layout)
			if err != nil {
				return fmt.Errorf("csldPrepareOrders.%w", err)
			}

			// Create grid
			if err := layout.InitGrid(ctx, svc.db); err != nil {
				return fmt.Errorf("csldPrepareOrders.%w", err)
			}

			// Get all the cells covering the AOI in the layout
			cells, err = layout.Covers(ctx, aoi, true)
			if err != nil {
				return fmt.Errorf("csldPrepareOrders.%w", err)
			}
			logger.Debugf("ReadAndCoverLayout:%v\n", time.Since(start))
		}

		start = time.Now()

		// Create one task per cell
		datasetsToBeConsolidated := utils.StringSet{}
		for cell := range cells {
			if cell.Error != nil {
				return fmt.Errorf("csldPrepareOrders.%w", cell.Error)
			}
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
			containerBaseName := utils.URLJoin(svc.ingestionStoragePath, layout.Name, cell.URI, job.Payload.InstanceID)
			containerBase, err := geocube.NewConsolidationContainer(containerBaseName, variable, params, layout, cell.Cell)
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
			job.LogMsg(geocube.DEBUG, "Sorting datasets by datetime...")
			if err := csldPrepareOrdersSortDatasets(ctx, txn, datasets, recordsTime); err != nil {
				return fmt.Errorf("csldPrepareOrders.%w", err)
			}

			// Exclude full containers
			datasets = csldPrepareOrdersExcludeFullContainers(datasets, layout.MaxRecords)

			// Check that datasets are available
			checkAvailability := false
			if checkAvailability {
				var err error
				for _, dataset := range datasets {
					uri, e := uri.ParseUri(dataset.Event.URI)
					if e != nil {
						err = utils.MergeErrors(true, err, fmt.Errorf(dataset.Event.URI+": %w", e))
						continue
					}
					if exist, e := uri.Exist(ctx); e != nil {
						err = utils.MergeErrors(true, err, fmt.Errorf(dataset.Event.URI+": %w", e))
					} else if !exist {
						err = utils.MergeErrors(true, err, fmt.Errorf(dataset.Event.URI+" does not exists"))
					}
				}
				if err != nil {
					return err
				}
			}

			// Group datasets by records
			job.LogMsg(geocube.DEBUG, "Grouping datasets by records...")
			records := make([]geocube.ConsolidationRecord, 0, len(datasets))
			for i := 0; i < len(datasets); {
				var datasetIDS []string
				record := geocube.ConsolidationRecord{ID: datasets[i].RecordID, DateTime: recordsTime[datasets[i].RecordID]}
				for ; i < len(datasets) && record.ID == datasets[i].RecordID; i++ {
					record.Datasets = append(record.Datasets, datasets[i].Event)
					datasetIDS = append(datasetIDS, datasets[i].ID)
				}
				if record.ValidShape, err = svc.db.ComputeValidShapeFromCell(ctx, datasetIDS, cell.Cell); err != nil {
					if geocube.IsError(err, geocube.EntityNotFound) {
						log.Logger(ctx).Sugar().Debugf("csldPrepareOrders: skip record %v: %v", record.DateTime, err)
						continue
					}
					return fmt.Errorf("csldPrepareOrders: failed to compute valid shape from cell (%v): %w", cell.Ring.Coords(), err)
				}
				for _, datasetID := range datasetIDS {
					datasetsToBeConsolidated.Push(datasetID)
				}
				records = append(records, record)
			}
			if len(records) == 0 {
				continue
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
			job.LogMsgf(geocube.DEBUG, "Prepare %d container(s) with %d record(s) and %d dataset(s) (geographic: %v)", nbOfContainers, len(records), len(datasets), cell.GeographicRing.Coords())
		}
		if len(job.Tasks) != 0 {
			job.LogMsgf(geocube.INFO, "%d tasks are created", len(job.Tasks))
			logger.Debugf("Create %d Tasks (mean): %v\n", len(job.Tasks), time.Since(start)/time.Duration(len(job.Tasks)))
		}

		// Lock the datasets that will be deleted after the consolidation
		job.LockDatasets(datasetsToBeConsolidated.Slice(), geocube.LockFlagTODELETE)

		// Release the datasets used for initialisation
		job.ReleaseDatasets(geocube.LockFlagINIT)

		job.LogMsgf(geocube.INFO, "Consolidation orders prepared (%d task(s))", len(job.Tasks))

		// Save job
		return svc.saveJob(ctx, txn, job)
	})
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
	job.LogMsg(geocube.INFO, "Send consolidation orders...")

	// Retrieves tasks
	tasks, err := svc.db.ReadTasks(ctx, job.ID, []geocube.TaskState{geocube.TaskStatePENDING})
	if err != nil {
		return err
	}

	var consolidationOrders [][]byte
	for _, task := range tasks {
		consolidationOrders = append(consolidationOrders, task.Payload)
	}

	// If there is no consolidation orders to send, consolidation is done
	if len(consolidationOrders) == 0 {
		job.LogMsg(geocube.INFO, "No consolidation orders found !")
		return svc.publishEvent(ctx, geocube.ConsolidationDone, job, "")
	}

	// Publish
	return svc.consolidationPublisher.Publish(ctx, consolidationOrders...)
}

func (svc *Service) csldIndex(ctx context.Context, job *geocube.Job) (err error) {
	job.LogMsg(geocube.INFO, "Indexing new datasets...")

	// Retrieves tasks
	if job.Tasks, err = svc.db.ReadTasks(ctx, job.ID, []geocube.TaskState{geocube.TaskStateDONE}); err != nil {
		return err
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
				newDataset, err := geocube.NewDatasetFromIncomplete(*incompleteDataset, r, "GTIFF_DIR:"+strconv.Itoa(i+1))
				if err != nil {
					return fmt.Errorf("csldIndex.%w", err)
				}
				newDatasets = append(newDatasets, newDataset)
			}

			log.Logger(ctx).Sugar().Debugf("Index consolidated container %s containing %d datasets", newContainer.URI, len(newDatasets))
			job.LogMsgf(geocube.DEBUG, "Preparing indexation of %d new datasets", len(newDatasets))
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

			// Create containerLayout
			layout := job.Payload.Layout
			if err = txn.SaveContainerLayout(ctx, newContainer.URI, layout); err != nil {
				return fmt.Errorf("csldIndex.%w", err)
			}

			// Delete task
			job.DeleteTask(0)

			job.LogMsg(geocube.DEBUG, "Datasets indexed")

			// Save job
			return svc.saveJob(ctx, txn, job)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (svc *Service) csldSwapDatasets(ctx context.Context, job *geocube.Job) error {
	job.LogMsg(geocube.INFO, "Swap datasets...")

	return svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
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
		job.LogMsg(geocube.INFO, "Datasets swapped")
		// Persist changes in db
		return svc.saveJob(ctx, txn, job)
	})
}

func (svc *Service) csldDeleteDatasets(ctx context.Context, job *geocube.Job) error {
	// Persist the jobs
	return svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
		// Get Dataset to delete
		datasets, err := txn.FindDatasets(ctx, geocube.DatasetStatusTODELETE, "", job.ID, nil, nil, geocube.Metadata{}, time.Time{}, time.Time{}, nil, nil, 0, 0, true)
		if err != nil {
			return fmt.Errorf("DeleteDatasets.%w", err)
		}
		if len(datasets) == 0 {
			return nil
		}
		ids := make([]string, len(datasets))
		for i, dataset := range datasets {
			ids[i] = dataset.ID
		}

		// Create a deletion job
		deletionJob := geocube.NewDeletionJob(job.Name+"_deletion_"+uuid.New().String(), geocube.ExecutionAsynchronous)

		// Lock datasets for deletion
		deletionJob.LockDatasets(ids, geocube.LockFlagTODELETE)

		// Release dataset
		job.ReleaseDatasets(geocube.LockFlagTODELETE)

		job.LogMsgf(geocube.INFO, "Create a deletion job to delete %d dataset(s): %s", len(ids), deletionJob.Name)

		if err := svc.saveJob(ctx, txn, job); err != nil {
			return fmt.Errorf("DeleteDatasets.%w", err)
		}
		if err := svc.saveJob(ctx, txn, deletionJob); err != nil {
			return fmt.Errorf("DeleteDatasets.%w", err)
		}
		if err := svc.delOnEnterNewState(ctx, deletionJob); err != nil {
			return fmt.Errorf("DeleteDatasets.%w", err)
		}
		return nil
	})
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
				job.LogMsg(geocube.WARN, "Rollback: RemoteContainer "+container.URI+" is not pending and cannot be deleted.")
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

func (svc *Service) csldCancel(ctx context.Context, job *geocube.Job) error {
	job.LogMsg(geocube.INFO, "Cancel all tasks...")

	if job.ActiveTasks == 0 {
		return nil
	}

	var err error
	if job.Tasks, err = svc.db.ReadTasks(ctx, job.ID, []geocube.TaskState{geocube.TaskStatePENDING}); err != nil {
		return err
	}

	for taskIndex, task := range job.Tasks {
		job.CancelTask(taskIndex)

		// Create cancelled file
		path := svc.cancelledConsolidationPath + "/" + fmt.Sprintf("%s_%s", job.ID, task.ID)
		cancelledJobsURI, err := uri.ParseUri(path)
		if err != nil {
			return fmt.Errorf("%w: %s", err, path)
		}

		if err := cancelledJobsURI.Upload(ctx, nil); err != nil {
			return err
		}

		job.LogMsg(geocube.INFO, "Job and associated tasks are cancelled")
		if err = svc.saveJob(ctx, nil, job); err != nil {
			return err
		}
	}
	return nil
}

func (svc *Service) csldConsolidationRetry(ctx context.Context, job *geocube.Job) error {
	job.LogMsg(geocube.INFO, "Retry consolidation...")

	var err error
	// Load tasks
	if job.Tasks, err = svc.db.ReadTasks(ctx, job.ID, []geocube.TaskState{geocube.TaskStateFAILED}); err != nil {
		return err
	}
	// Reset and save task status
	job.ResetAllTasks()
	return svc.saveJob(ctx, nil, job)
}

func (svc *Service) csldRollback(ctx context.Context, job *geocube.Job) error {
	var err error
	job.LogMsg(geocube.INFO, "Rollback...")

	// Load tasks
	if job.Tasks, err = svc.db.ReadTasks(ctx, job.ID, nil); err != nil {
		return fmt.Errorf("Rollback.%w", err)
	}

	// Rollback from JobStateCONSOLIDATIONINDEXED: Unswap datasets
	// Nothing to do as swapDataset is a unitOfWork

	// Rollback from JobStateCONSOLIDATIONDONE: delete the inactive datasets
	{
		containersURI, err := svc.opSubFncRemoveDatasetsAndContainers(ctx, nil, job, geocube.LockFlagNEW, geocube.DatasetStatusINACTIVE)
		if err != nil {
			return fmt.Errorf("Rollback.%w", err)
		}

		if err := svc.csldSubFncDeleteContainers(ctx, containersURI); err != nil {
			return fmt.Errorf("Rollback.%w", err)
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

func (svc *Service) csldSubFncDeleteContainers(ctx context.Context, containersURI []string) error {
	// Delete containers
	workers := 20
	tasks := make(chan string)
	wg := sync.WaitGroup{}

	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() error {
			defer wg.Done()
			for task := range tasks {
				svc.opSubFncDeleteContainer(ctx, task)
			}
			return nil
		}()
	}
	for _, task := range containersURI {
		tasks <- task
	}
	close(tasks)
	wg.Wait()
	return nil
}
