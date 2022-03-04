package svc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/airbusgeo/geocube/interface/database"
	"github.com/airbusgeo/geocube/interface/storage/uri"
	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/geocube/internal/log"
	"github.com/airbusgeo/geocube/internal/utils"
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
	if !job.Waiting {
		start := time.Now()
		switch job.Type {
		case geocube.JobTypeCONSOLIDATION:
			err = svc.csldOnEnterNewState(ctx, job)
		case geocube.JobTypeDELETION:
			err = svc.delOnEnterNewState(ctx, job)
		}
		job.LogMsgf(geocube.DEBUG, "  ... in %v", time.Since(start))
	}
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

		if job.State == geocube.JobStateABORTED || job.State == geocube.JobStateFAILED || job.State == geocube.JobStateROLLBACKFAILED {
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

	job.LogMsgf(geocube.DEBUG, "TaskEvt received with status %s (id:%s, err:%s)", evt.Status.String(), evt.TaskID, evt.Error)

	if err = svc.saveJob(ctx, nil, job); err != nil {
		return fmt.Errorf("handleTaskEvt(%s).%w", evt.TaskID, err)
	}

	if job.ActiveTasks == 0 {
		if job.State == geocube.JobStateCONSOLIDATIONCANCELLING {
			job.LogMsg(geocube.INFO, "Job has been canceled")
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

// delOnEnterNewState should only returns publishing error
// All the other errors must be handle by the state machine
func (svc *Service) delOnEnterNewState(ctx context.Context, job *geocube.Job) error {
	switch job.State {
	case geocube.JobStateNEW:
		return svc.publishEvent(ctx, geocube.JobCreated, job, "")

	case geocube.JobStateCREATED:
		if err := svc.delSetToDelete(ctx, job); err != nil {
			return svc.publishEvent(ctx, geocube.DeletionNotReady, job, err.Error())
		}
		return svc.publishEvent(ctx, geocube.DeletionReady, job, "")

	case geocube.JobStateDELETIONINPROGRESS:
		if err := svc.delRemoveDatasets(ctx, job); err != nil {
			return svc.publishEvent(ctx, geocube.RemovalFailed, job, err.Error())
		}
		return svc.publishEvent(ctx, geocube.RemovalDone, job, "")

	case geocube.JobStateDELETIONEFFECTIVE:
		if err := svc.delDeleteContainers(ctx, job); err != nil {
			return svc.publishEvent(ctx, geocube.DeletionFailed, job, err.Error())
		}
		return svc.publishEvent(ctx, geocube.DeletionDone, job, "")

	case geocube.JobStateDONE:
		// Finished !
		log.Logger(ctx).Sugar().Debug("deletion successfully completed")
		return nil

	case geocube.JobStateDONEBUTUNTIDY:
		return svc.opContactAdmin(ctx, job)

	case geocube.JobStateDELETIONFAILED:
		job.LogErr("Deletion failed")
		job.LogMsg(geocube.INFO, "Wait for user command...")
		return nil

	case geocube.JobStateABORTED:
		if err := svc.delRollback(ctx, job); err != nil {
			return svc.publishEvent(ctx, geocube.RollbackFailed, job, err.Error())
		}
		return svc.publishEvent(ctx, geocube.RollbackDone, job, "")

	case geocube.JobStateROLLBACKFAILED:
		job.LogErr("Rollback failed")
		job.LogMsg(geocube.INFO, "Wait for user command...")
		// Finished but...
		return nil

	case geocube.JobStateFAILED:
		job.LogErr("Job failed")
		// Finished but...
		return nil
	}

	return nil
}

func (svc *Service) delInit(ctx context.Context, job *geocube.Job, instancesID, recordsID []string) error {
	if err := svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) (err error) {
		datasets, err := txn.FindDatasets(ctx, geocube.DatasetStatusACTIVE, "", "", instancesID, recordsID, geocube.Metadata{}, time.Time{}, time.Time{}, nil, nil, 0, 0, false)
		if err != nil {
			return err
		}
		if len(datasets) == 0 {
			return geocube.NewEntityNotFound("", "", "", "No dataset found for theses records and instances")
		}
		datasetsID := make([]string, len(datasets))
		for i, dataset := range datasets {
			job.LogMsgf(geocube.DEBUG, "Lock %s%v %s (record:%s, instance:%s)", dataset.GDALURI(), dataset.Bands, dataset.ID, dataset.RecordID, dataset.InstanceID)
			datasetsID[i] = dataset.ID
		}

		// Lock datasets
		job.LockDatasets(datasetsID, geocube.LockFlagTODELETE)

		// Persist the job
		start := time.Now()
		if err := svc.saveJob(ctx, txn, job); err != nil {
			return err
		}
		log.Logger(ctx).Sugar().Debugf("SaveJob: %v\n", time.Since(start))

		return nil
	}); err != nil {
		return fmt.Errorf("delInit.%w", err)
	}

	// Start the job
	log.Logger(ctx).Sugar().Debug("new deletion job started")
	if err := svc.delOnEnterNewState(ctx, job); err != nil {
		return fmt.Errorf("delInit.%w", err)
	}
	return nil
}

func (svc *Service) delSetToDelete(ctx context.Context, job *geocube.Job) error {
	job.LogMsg(geocube.INFO, "Set datasets to delete...")

	return svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
		// Active datasets are tagged to_delete
		err := txn.ChangeDatasetsStatus(ctx, job.ID, geocube.DatasetStatusACTIVE, geocube.DatasetStatusTODELETE)
		if err != nil {
			return err
		}

		// Persist changes in db
		return svc.saveJob(ctx, txn, job)
	})
}

func (svc *Service) delRemoveDatasets(ctx context.Context, job *geocube.Job) error {
	job.LogMsg(geocube.INFO, "Remove datasets...")

	return svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
		// Remove datasets and empty containers
		containersURI, err := svc.opSubFncRemoveDatasetsAndContainers(ctx, txn, job, geocube.LockFlagTODELETE, geocube.DatasetStatusTODELETE)
		if err != nil {
			return err
		}
		// Create deletion tasks
		for _, uri := range containersURI {
			job.LogMsgf(geocube.DEBUG, "Create task to delete: %s", uri)
			if err := job.CreateDeletionTask(uri); err != nil {
				return err
			}
		}
		// Persist changes in db
		return svc.saveJob(ctx, txn, job)
	})
}

// opRemoveJobDatasetsAndContainers removes all the datasets locked by the job with the given status and the containers (if empty)
// Return a list of remote containers to be deleted
func (svc *Service) opSubFncRemoveDatasetsAndContainers(ctx context.Context, txn database.GeocubeTxBackend, job *geocube.Job, lockFlag geocube.LockFlag, datasetStatus geocube.DatasetStatus) ([]string, error) {
	if txn == nil {
		var uris []string
		return uris, svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) (err error) {
			uris, err = svc.opSubFncRemoveDatasetsAndContainers(ctx, txn, job, lockFlag, datasetStatus)
			return err
		})
	}

	// Find the datasets of the job with the given status
	datasets, err := txn.FindDatasets(ctx, datasetStatus, "", job.ID, nil, nil, geocube.Metadata{}, time.Time{}, time.Time{}, nil, nil, 0, 0, true)
	if err != nil {
		return nil, fmt.Errorf("opRemoveJobDatasetsAndContainers.%w", err)
	}
	job.LogMsgf(geocube.DEBUG, "%d datasets to delete", len(datasets))
	// First, release the datasets (otherwise the database cannot delete them)
	job.ReleaseDatasets(lockFlag)
	if err = svc.saveJob(ctx, txn, job); err != nil {
		return nil, fmt.Errorf("opRemoveJobDatasetsAndContainers.%w", err)
	}

	// Then, remove them and get a list of containers URI to be deleted (empty and managed)
	return svc.opSubFncRemoveDatasets(ctx, txn, datasets)
}

// opSubFncRemoveDatasets removes all the datasets and the containers (if empty)
// Returns a list of empty and managed containers to be physically deleted
func (svc *Service) opSubFncRemoveDatasets(ctx context.Context, txn database.GeocubeTxBackend, datasets []*geocube.Dataset) (emptyManagedContainers []string, err error) {
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
		return nil, fmt.Errorf("opSubFncRemoveDatasets.%w", err)
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
				return nil, fmt.Errorf("opSubFncRemoveDatasets.%w", err)
			}
			if container.Managed {
				// Unmanaged the container to flag it "toDelete"
				container.Managed = false
				emptyManagedContainers = append(emptyManagedContainers, container.URI)
			}
			container.Delete()
		}
	}

	// Save containers
	for _, container := range containers {
		if err = svc.saveContainer(ctx, txn, container); err != nil {
			return nil, fmt.Errorf("opSubFncRemoveDatasets.%w", err)
		}
	}

	return emptyManagedContainers, nil
}

func (svc *Service) delDeleteContainers(ctx context.Context, job *geocube.Job) error {
	job.LogMsg(geocube.INFO, "Delete containers...")

	return svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
		// Read pending tasks
		var err error
		if job.Tasks, err = svc.db.ReadTasks(ctx, job.ID, []geocube.TaskState{geocube.TaskStatePENDING, geocube.TaskStateFAILED}); err != nil {
			return err
		}
		if len(job.Tasks) == 0 {
			job.LogMsg(geocube.DEBUG, "Nothing to delete")
			return nil
		}

		// Delete containers
		workers := utils.MinI(20, len(job.Tasks))
		tasks := make(chan *geocube.Task)
		wg := sync.WaitGroup{}
		mutex := sync.Mutex{}

		wg.Add(workers)
		for w := 0; w < workers; w++ {
			go func() error {
				defer wg.Done()
				for task := range tasks {
					containerURI, err := task.DeletionPayload()
					if err != nil {
						return err
					}
					status := geocube.TaskSuccessful
					if err = svc.opSubFncDeleteContainer(ctx, containerURI); err != nil {
						status = geocube.TaskFailed
					}
					mutex.Lock()
					job.UpdateTask(*geocube.NewTaskEvent(job.ID, task.ID, status, err))
					mutex.Unlock()
				}
				return nil
			}()
		}
		for _, task := range job.Tasks {
			tasks <- task
		}
		close(tasks)
		wg.Wait()

		// Delete done tasks
		for i := range job.Tasks {
			if job.Tasks[i].State == geocube.TaskStateDONE {
				job.DeleteTask(i)
			}
		}

		// Persist job
		return svc.saveJob(ctx, txn, job)
	})
}

func (svc *Service) delRollback(ctx context.Context, job *geocube.Job) error {
	job.LogMsg(geocube.INFO, "Rollback...")

	// Rollback from JobStateDELETIONINPROGRESS: nothing to do (unit of work)

	// Rollback from JobStateCREATED: datasets ToActive
	if err := svc.db.ChangeDatasetsStatus(ctx, job.ID, geocube.DatasetStatusTODELETE, geocube.DatasetStatusACTIVE); err != nil {
		return fmt.Errorf("Rollback.%w", err)
	}

	// Rollback from JobStateNEW: release old datasets
	job.ReleaseDatasets(geocube.LockFlagTODELETE)

	// Persist DeleteTasks and ReleaseDatasets
	if err := svc.saveJob(ctx, nil, job); err != nil {
		return fmt.Errorf("Rollback.%w", err)
	}

	return nil
}

func (svc *Service) opSubFncDeleteContainer(ctx context.Context, containerURI string) error {
	URI, err := uri.ParseUri(containerURI)
	if err != nil {
		return fmt.Errorf("opSubFncDeleteContainer.%w", err)
	}
	if err := URI.Delete(ctx); err != nil {
		return fmt.Errorf("opSubFncDeleteContainer.%w", err)
	}
	return nil
}

func (svc *Service) opContactAdmin(ctx context.Context, job *geocube.Job) error {
	job.LogMsg(geocube.WARN, "Contact admin...")
	//TODO Contact Admin
	return nil
}

func (svc *Service) publishEvent(ctx context.Context, status geocube.JobStatus, job *geocube.Job, serr string) error {
	job.LogMsgf(geocube.DEBUG, "  Event %s %s...", status.String(), serr)

	evt := geocube.NewJobEvent(job.ID, status, serr)

	if job.ExecutionLevel == geocube.ExecutionSynchronous {
		return svc.handleJobEvt(ctx, *evt)
	}

	data, err := geocube.MarshalEvent(evt)
	if err != nil {
		panic("Unable to marshal event")
	}

	return svc.eventPublisher.Publish(ctx, data)
}
