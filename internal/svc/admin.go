package svc

import (
	"context"
	"errors"
	"fmt"

	"github.com/airbusgeo/geocube/interface/database"
	"github.com/google/uuid"

	"github.com/airbusgeo/geocube/internal/geocube"
)

var errSimulationEnded = errors.New("simulation ended")

// TidyPending implements ServiceAdmin
func (svc *Service) TidyPending(ctx context.Context, aois, records, variables, instances, containers, params bool, simulate bool) ([]int64, error) {
	var nbs [6]int64

	err := svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) (err error) {
		if records {
			if nbs[1], err = txn.DeletePendingRecords(ctx, nil); err != nil {
				return fmt.Errorf("tidy records: %w", err)
			}
		}
		if aois {
			if nbs[0], err = txn.DeletePendingAOIs(ctx); err != nil {
				return fmt.Errorf("tidy aois: %w", err)
			}
		}

		if instances {
			if nbs[2], err = txn.DeletePendingInstances(ctx); err != nil {
				return fmt.Errorf("tidy instances: %w", err)
			}
		}

		if variables {
			if nbs[3], err = txn.DeletePendingVariables(ctx); err != nil {
				return fmt.Errorf("tidy variables: %w", err)
			}
		}

		if containers {
			if nbs[4], err = txn.DeletePendingContainers(ctx); err != nil {
				return fmt.Errorf("tidy containers: %w", err)
			}
		}

		if params {
			if nbs[5], err = txn.DeletePendingConsolidationParams(ctx); err != nil {
				return fmt.Errorf("tidy params: %w", err)
			}
		}

		if simulate {
			return errSimulationEnded
		}
		return nil
	})

	if errors.Is(err, errSimulationEnded) {
		return nbs[:], nil
	}
	if err != nil {
		return nbs[:], fmt.Errorf("TidyPending: %w", err)
	}

	return nbs[:], nil
}

// UpdateDatasets implements ServiceAdmin
func (svc *Service) UpdateDatasets(ctx context.Context, simulate bool, instanceID string, recordIds []string, dmapping geocube.DataMapping) (map[string]int64, error) {
	var results map[string]int64

	err := svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) (err error) {
		if results, err = txn.UpdateDatasets(ctx, instanceID, recordIds, dmapping); err != nil {
			return fmt.Errorf("UpdateDatasets.%w", err)
		}

		if simulate {
			return errSimulationEnded
		}
		return nil
	})

	if errors.Is(err, errSimulationEnded) {
		return results, nil
	}
	if err != nil {
		return results, fmt.Errorf("UpdateDatasets.%w", err)
	}

	return results, nil
}

// DeleteDatasets implements ServiceAdmin
func (svc *Service) DeleteDatasets(ctx context.Context, jobName string, instanceIDs, recordIDs, datasetPatterns []string, executionLevel geocube.ExecutionLevel) (*geocube.Job, error) {
	// Create the job
	if jobName == "" {
		jobName = uuid.New().String()
	}
	job := geocube.NewDeletionJob(jobName, executionLevel)

	if err := svc.delInit(ctx, job, instanceIDs, recordIDs, datasetPatterns); err != nil {
		return nil, fmt.Errorf("DeleteDatasets.%w", err)
	}
	return job, nil
}
