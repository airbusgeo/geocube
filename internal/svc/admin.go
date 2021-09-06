package svc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/airbusgeo/geocube/interface/database"

	"github.com/airbusgeo/geocube/internal/geocube"
)

var errSimulationEnded = errors.New("simulation ended")

// TidyPending implements ServiceAdmin
func (svc *Service) TidyPending(ctx context.Context, aois, records, variables, instances, containers, params bool, simulate bool) ([]int64, error) {
	var nbs [6]int64

	err := svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) (err error) {
		if records {
			if nbs[1], err = txn.DeletePendingRecords(ctx); err != nil {
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
func (svc *Service) UpdateDatasets(ctx context.Context, simulate bool, instanceID string, dmapping geocube.DataMapping) (map[string]int64, error) {
	var results map[string]int64

	err := svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) (err error) {
		if results, err = txn.UpdateDatasets(ctx, instanceID, dmapping); err != nil {
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
func (svc *Service) DeleteDatasets(ctx context.Context, simulate bool, instancesID []string, recordsID []string) ([]string, error) {
	var results []string

	err := svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) (err error) {
		datasets, err := txn.FindDatasets(ctx, geocube.DatasetStatusACTIVE, "", "", instancesID, recordsID, geocube.Metadata{}, time.Time{}, time.Time{}, nil, nil, 0, 0, false)
		if err != nil {
			return fmt.Errorf("DeleteDatasets.%w", err)
		}
		var datasetsID []string
		for _, dataset := range datasets {
			results = append(results, fmt.Sprintf("%s[%v] %s (record:%s, instance:%s)", dataset.GDALOpenName(), dataset.Bands, dataset.ID, dataset.RecordID, dataset.InstanceID))
			datasetsID = append(datasetsID, dataset.ID)
		}

		if !simulate {
			if err = txn.DeleteDatasets(ctx, datasetsID); err != nil {
				return fmt.Errorf("DeleteDatasets.%w", err)
			}
		}
		return nil
	})

	if err != nil {
		return results, fmt.Errorf("DeleteDatasets.%w", err)
	}

	return results, nil
}
