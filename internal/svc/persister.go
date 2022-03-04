package svc

import (
	"context"
	"fmt"

	"github.com/airbusgeo/geocube/interface/database"
	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/geocube/internal/utils"
)

// saveJob persists the job in the database, the lock status and its tasks
// saveJob must be done inside a transaction. If txn=nil, saveJob calls itself inside a unitOfWork.
// The action depends on the persistent state (IsNew/IsToDelete/IsDirty => Create/Delete/Update)
func (svc *Service) saveJob(ctx context.Context, txn database.GeocubeTxBackend, job *geocube.Job) error {
	if txn == nil {
		return svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error { return svc.saveJob(ctx, txn, job) })
	}

	var toUpdateTasks, toCreateTasks []*geocube.Task
	var toDeleteTasksID []string
	for _, task := range job.Tasks {
		if task.IsToDelete() {
			toDeleteTasksID = append(toDeleteTasksID, task.ID)
		} else if task.IsNew() {
			toCreateTasks = append(toCreateTasks, task)
		} else if task.IsDirty() {
			toUpdateTasks = append(toUpdateTasks, task)
		}
	}

	// Deletion
	{
		for i := range job.LockedDatasets {
			if job.LockedDatasets[i].IsToDelete() {
				if err := txn.ReleaseDatasets(ctx, job.ID, i); err != nil {
					return fmt.Errorf("savejob.%d.%w", i, err)
				}
			}
		}

		for _, taskID := range toDeleteTasksID {
			if err := txn.DeleteTask(ctx, taskID); err != nil {
				return fmt.Errorf("savejob.%w", err)
			}
		}

		if job.Params != nil && job.Params.IsToDelete() {
			switch job.Type {
			case geocube.JobTypeCONSOLIDATION:
				if err := txn.DeleteConsolidationParams(ctx, job.Payload.ParamsID); err != nil {
					return fmt.Errorf("savejob.%w", err)
				}
			case geocube.JobTypeDELETION:
				//TODO persist a deletion job
			case geocube.JobTypeINGESTION:
				//TODO persist an ingestion job
			}
		}

		if job.IsToDelete() {
			if err := txn.DeleteJob(ctx, job.ID); err != nil {
				return fmt.Errorf("savejob.%w", err)
			}
		}
	}

	// Creation
	{
		if job.IsNew() {
			if err := txn.CreateJob(ctx, job); err != nil {
				return fmt.Errorf("savejob.%w", err)
			}
		}

		if job.Params != nil && job.Params.IsNew() {
			switch job.Type {
			case geocube.JobTypeCONSOLIDATION:
				if err := txn.CreateConsolidationParams(ctx, job.Payload.ParamsID, *job.Params.(*geocube.ConsolidationParams)); err != nil {
					return fmt.Errorf("savejob.%w", err)
				}
			case geocube.JobTypeDELETION:
				//TODO create deletion params
			case geocube.JobTypeINGESTION:
				//TODO create ingestion params
			}
		}

		for i, l := range job.LockedDatasets {
			if l.IsNew() || l.IsDirty() {
				if err := txn.LockDatasets(ctx, job.ID, l.NewIDs(), i); err != nil {
					return fmt.Errorf("savejob.%d.%w", i, err)
				}
			}
		}

		if len(toCreateTasks) > 0 {
			if err := txn.CreateTasks(ctx, job.ID, toCreateTasks); err != nil {
				return fmt.Errorf("savejob.%w", err)
			}
		}
	}

	// Update
	{
		for _, task := range toUpdateTasks {
			if err := txn.UpdateTask(ctx, task); err != nil {
				return fmt.Errorf("savejob.%w", err)
			}
		}

		if job.Params != nil && job.Params.IsDirty() {
			panic("Params cannot be dirty")
		}

		if job.IsDirty() || job.IsClean() {
			if err := txn.UpdateJob(ctx, job); err != nil {
				if geocube.IsError(err, geocube.EntityNotFound) {
					return utils.MakeTemporary(fmt.Errorf("savejob.%w (temporary)", err)) // OCC error
				}
				return fmt.Errorf("savejob.%w", err)
			}
		}
	}

	// Update persistent state
	for i := len(job.Tasks) - 1; i >= 0; i-- {
		if job.Tasks[i].IsToDelete() {
			job.Tasks[i].Deleted()
			job.Tasks[i] = job.Tasks[len(job.Tasks)-1]
			job.Tasks = job.Tasks[:len(job.Tasks)-1]
		}
	}
	if job.Params != nil && job.Params.IsToDelete() {
		job.Params.Deleted()
		job.Params = nil
	}
	for i, l := range job.LockedDatasets {
		if l.IsToDelete() {
			job.LockedDatasets[i].Deleted()
			job.LockedDatasets[i] = geocube.LockedDatasets{}
		}
	}

	if job.IsToDelete() {
		job.Deleted()
	} else {
		job.Clean(true)
	}
	return nil
}

// saveContainer persists the container and its dataset in the database.
// saveContainer must be done inside a transaction. If txn=nil, saveContainer calls itself inside a unitOfWork.
// The action depends on the persistent state (IsNew/IsToDelete/IsDirty => Create/Delete/Update) and the one of each dataset
func (svc *Service) saveContainer(ctx context.Context, txn database.GeocubeTxBackend, container *geocube.Container) error {
	if txn == nil {
		return svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error { return svc.saveContainer(ctx, txn, container) })
	}
	var newDatasets []*geocube.Dataset
	var todeleteDatasetsID []string
	for _, dataset := range container.Datasets {
		if dataset.IsToDelete() {
			todeleteDatasetsID = append(todeleteDatasetsID, dataset.ID)
		} else if dataset.IsNew() {
			newDatasets = append(newDatasets, dataset)
		} /*} else if dataset.IsDirty() { // A dataset cannot be dirty*/
	}

	if container.IsNew() {
		if err := txn.CreateContainer(ctx, container); err != nil {
			return fmt.Errorf("saveContainer.%w", err)
		}
	}

	if len(todeleteDatasetsID) > 0 {
		if err := txn.DeleteDatasets(ctx, todeleteDatasetsID); err != nil {
			return fmt.Errorf("saveContainer.%w", err)
		}
	}

	if len(newDatasets) > 0 {
		if err := txn.CreateDatasets(ctx, newDatasets); err != nil {
			return fmt.Errorf("saveContainer.%w", err)
		}
	}

	if container.IsDirty() {
		if err := txn.UpdateContainer(ctx, container); err != nil {
			return fmt.Errorf("saveContainer.%w", err)
		}
	}

	if container.IsToDelete() {
		if container.Managed {
			return geocube.NewDependencyStillExists("Container", "", "uri", container.URI, "Attempt to delete a managed container from the database")
		}
		if err := txn.DeleteContainerLayout(ctx, container.URI); err != nil {
			if !geocube.IsError(err, geocube.EntityNotFound) {
				return fmt.Errorf("saveContainer.%w", err)
			}
		}
		if err := txn.DeleteContainer(ctx, container.URI); err != nil {
			return fmt.Errorf("saveContainer.%w", err)
		}
	}

	// Update the persistent status at the end
	for i := len(container.Datasets) - 1; i >= 0; i-- {
		d := container.Datasets[i]
		if d.IsToDelete() {
			d.Deleted()
			container.Datasets[i] = container.Datasets[len(container.Datasets)-1]
			container.Datasets = container.Datasets[:len(container.Datasets)-1]
		}
	}
	if container.IsToDelete() {
		container.Deleted()
	} else {
		container.Clean(true)
	}

	return nil
}

// saveVariable persists the variable and its instances in the database.
// saveVariable must be done inside a transaction. If txn=nil, saveVariable calls itself inside a unitOfWork.
// The action depends on the persistent state (IsNew/IsToDelete/IsDirty => Create/Delete/Update) and the one of each instance
func (svc *Service) saveVariable(ctx context.Context, txn database.GeocubeTxBackend, variable *geocube.Variable) error {
	if txn == nil {
		return svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error { return svc.saveVariable(ctx, txn, variable) })
	}
	var toDeleteInstances, toUpdateInstances, toCreateInstances []*geocube.VariableInstance
	for _, instance := range variable.Instances {
		if instance.IsToDelete() {
			toDeleteInstances = append(toDeleteInstances, instance)
		} else if instance.IsDirty() {
			toUpdateInstances = append(toUpdateInstances, instance)
		} else if instance.IsNew() {
			toCreateInstances = append(toCreateInstances, instance)
		}
	}

	// Deletion
	{
		for _, instance := range toDeleteInstances {
			if err := txn.DeleteInstance(ctx, instance.ID); err != nil {
				return fmt.Errorf("saveVariable.%w", err)
			}
		}

		if variable.ConsolidationParams.IsToDelete() {
			if err := txn.DeleteConsolidationParams(ctx, variable.ID); err != nil {
				return fmt.Errorf("saveVariable.%w", err)
			}
		}

		if variable.IsToDelete() {
			if err := txn.DeleteVariable(ctx, variable.ID); err != nil {
				return fmt.Errorf("saveVariable.%w", err)
			}
		}
	}

	if variable.IsDirty() {
		if err := txn.UpdateVariable(ctx, variable); err != nil {
			return fmt.Errorf("saveVariable.%w", err)
		}
	}

	if variable.IsNew() {
		if err := txn.CreateVariable(ctx, variable); err != nil {
			return fmt.Errorf("saveVariable.%w", err)
		}
	}

	if variable.ConsolidationParams.IsNew() || variable.ConsolidationParams.IsDirty() {
		if err := txn.CreateConsolidationParams(ctx, variable.ID, variable.ConsolidationParams); err != nil {
			return fmt.Errorf("saveVariable.%w", err)
		}
	}

	for _, instance := range toUpdateInstances {
		if err := txn.UpdateInstance(ctx, instance); err != nil {
			return fmt.Errorf("saveVariable.%w", err)
		}
	}

	for _, instance := range toCreateInstances {
		if err := txn.CreateInstance(ctx, variable.ID, instance); err != nil {
			return fmt.Errorf("saveVariable.%w", err)
		}
	}

	// Update persistent state
	for _, instance := range toDeleteInstances {
		instance.Deleted()
		delete(variable.Instances, instance.ID)
	}
	if variable.ConsolidationParams.IsToDelete() {
		variable.ConsolidationParams.Deleted()
		variable.ConsolidationParams = geocube.ConsolidationParams{}
	}

	if variable.IsToDelete() {
		variable.Deleted()
	} else {
		variable.Clean(true)
	}

	return nil
}

// prepareIndexation prepares the fully defined container to be persisted in database with the fully defined datasets.
// The remote container must exist and be reachable (no validation).
// Can be in or out a unitofwork
func (svc *Service) prepareIndexation(ctx context.Context, db database.GeocubeBackend, container *geocube.Container, datasets []*geocube.Dataset) error {
	// Fetch container
	dbcontainers, err := db.ReadContainers(ctx, []string{container.URI})
	if err != nil {
		if !geocube.IsError(err, geocube.EntityNotFound) {
			return fmt.Errorf("prepareIndexation.%w", err)
		}
	} else {
		dbcontainers[0].Clean(true)
		*container = *dbcontainers[0]
	}

	// Add each dataset to the container
	for _, dataset := range datasets {
		if err = container.AddDataset(dataset); err != nil {
			return fmt.Errorf("prepareIndexation.%w", err)
		}
	}

	return nil
}
