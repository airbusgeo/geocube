package svc

// #include <unistd.h>
import "C"
import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/airbusgeo/geocube/interface/messaging"
	"github.com/airbusgeo/geocube/interface/storage/uri"

	"github.com/airbusgeo/geocube/interface/database"
	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/geocube/internal/image"
	"github.com/airbusgeo/geocube/internal/log"
	"github.com/airbusgeo/godal"
	"github.com/twpayne/go-geom"
)

var ramSize int

// Service implements GeocubeService
type Service struct {
	db                         database.GeocubeDBBackend
	eventPublisher             messaging.Publisher
	consolidationPublisher     messaging.Publisher
	catalogWorkers             int
	ingestionStoragePath       string
	cancelledConsolidationPath string
}

// New returns a new business service
func New(ctx context.Context, db database.GeocubeDBBackend, eventPublisher messaging.Publisher, consolidationPublisher messaging.Publisher, ingestionStoragePath, cancelledConsolidationPath string, catalogWorkers int) (*Service, error) {
	// Check parameters
	if (ingestionStoragePath == "") != (consolidationPublisher == nil) {
		return nil, fmt.Errorf("invalid arguments: to define the service to be able to handle consolidation, ingestionStoragePath and consolidationPublisher must be defined")
	}

	ramSize = int(C.sysconf(C._SC_PHYS_PAGES) * C.sysconf(C._SC_PAGE_SIZE))

	if catalogWorkers <= 0 {
		catalogWorkers = 1
	}
	return &Service{db: db, eventPublisher: eventPublisher, consolidationPublisher: consolidationPublisher, catalogWorkers: catalogWorkers, ingestionStoragePath: ingestionStoragePath, cancelledConsolidationPath: cancelledConsolidationPath}, nil
}

// CreateAOI implements GeocubeService
func (svc *Service) CreateAOI(ctx context.Context, aoi *geocube.AOI) error {
	return svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
		return txn.CreateAOI(ctx, aoi)
	})
}

// GetAOI implements GeocubeService
func (svc *Service) GetAOI(ctx context.Context, aoiID string) (*geocube.AOI, error) {
	return svc.db.ReadAOI(ctx, aoiID)
}

// CreateRecords implements GeocubeService
func (svc *Service) CreateRecords(ctx context.Context, records []*geocube.Record) error {
	return svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
		return txn.CreateRecords(ctx, records)
	})
}

// DeleteRecords implements GeocubeService
func (svc *Service) DeleteRecords(ctx context.Context, ids []string) (int64, error) {
	var nb int64
	err := svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) (err error) {
		nb, err = txn.DeleteRecords(ctx, ids)
		return err
	})
	return nb, err
}

// ListRecords implements GeocubeService
func (svc *Service) ListRecords(ctx context.Context, name string, tags geocube.Metadata, fromTime, toTime time.Time, aoi *geocube.AOI, page, limit int, loadAOI bool) ([]*geocube.Record, error) {
	return svc.db.FindRecords(ctx, name, tags, fromTime, toTime, "", aoi, page, limit, true, loadAOI)
}

// AddRecordsTags add tags on list of records
func (svc *Service) AddRecordsTags(ctx context.Context, ids []string, tags geocube.Metadata) (int64, error) {
	var nb int64
	err := svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) (err error) {
		nb, err = txn.AddRecordsTags(ctx, ids, tags)
		return err
	})

	return nb, err
}

// RemoveRecordsTags remove tags on list of records
func (svc *Service) RemoveRecordsTags(ctx context.Context, ids []string, tagsKey []string) (int64, error) {
	var nb int64
	err := svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) (err error) {
		nb, err = txn.RemoveRecordsTags(ctx, ids, tagsKey)
		return err
	})

	return nb, err
}

// CreateVariable implements GeocubeService
func (svc *Service) CreateVariable(ctx context.Context, variable *geocube.Variable) error {
	if !variable.IsNew() {
		return geocube.NewValidationError("wrong persistent status")
	}
	return svc.saveVariable(ctx, nil, variable)
}

// UpdateVariable implements GeocubeService
func (svc *Service) UpdateVariable(ctx context.Context, variableID string, name, unit, description, palette *string, resampling *geocube.Resampling) error {
	return svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
		variable, err := txn.ReadVariable(ctx, variableID)
		if err != nil {
			return fmt.Errorf("UpdateVariable.%w", err)
		}
		variable.Clean(true)
		if err = variable.Update(name, unit, description, palette, resampling); err != nil {
			return err
		}
		return svc.saveVariable(ctx, txn, variable)
	})
}

// GetVariable implements GeocubeService
// Retrieves variable with the first not-empty parameter
func (svc *Service) GetVariable(ctx context.Context, variableID, instanceID, variableName string) (*geocube.Variable, error) {
	var v *geocube.Variable
	var err error

	if variableID != "" {
		v, err = svc.db.ReadVariable(ctx, variableID)
	} else if instanceID != "" {
		v, err = svc.db.ReadVariableFromInstanceID(ctx, instanceID)
	} else if variableName != "" {
		v, err = svc.db.ReadVariableFromName(ctx, variableName)
	} else {
		return nil, geocube.NewValidationError("GetVariable: all parameters are empty")
	}
	if err != nil {
		return nil, fmt.Errorf("GetVariable.%w", err)
	}

	v.Clean(true)
	return v, nil
}

// InstantiateVariable implements GeocubeService
func (svc *Service) InstantiateVariable(ctx context.Context, variableID string, instance *geocube.VariableInstance) error {
	return svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
		variable, err := txn.ReadVariable(ctx, variableID)
		if err != nil {
			return fmt.Errorf("InstantiateVariable.%w", err)
		}
		variable.Clean(true)
		if err := variable.AddInstance(instance); err != nil {
			return fmt.Errorf("InstantiateVariable.%w", err)
		}
		return svc.saveVariable(ctx, txn, variable)
	})
}

// ListVariables implements GeocubeService
func (svc *Service) ListVariables(ctx context.Context, namelike string, page, limit int) ([]*geocube.Variable, error) {
	return svc.db.FindVariables(ctx, namelike, page, limit)
}

// DeleteVariable implements GeocubeService
func (svc *Service) DeleteVariable(ctx context.Context, id string) error {
	return svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
		variable, err := txn.ReadVariable(ctx, id)
		if err != nil {
			return err
		}
		variable.ToDelete("")
		return svc.saveVariable(ctx, txn, variable)
	})
}

// UpdateInstance implements GeocubeService
func (svc *Service) UpdateInstance(ctx context.Context, id string, name *string, newMetadata map[string]string, delMetadataKeys []string) error {
	return svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
		variable, err := txn.ReadVariableFromInstanceID(ctx, id)
		if err != nil {
			return err
		}
		variable.Clean(true)
		if err = variable.UpdateInstance(id, name, newMetadata, delMetadataKeys); err != nil {
			return err
		}
		return svc.saveVariable(ctx, txn, variable)
	})
}

// DeleteInstance implements GeocubeService
func (svc *Service) DeleteInstance(ctx context.Context, id string) error {
	return svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
		variable, err := txn.ReadVariableFromInstanceID(ctx, id)
		if err != nil {
			return err
		}
		variable.ToDelete(id)
		return svc.saveVariable(ctx, txn, variable)
	})
}

// CreatePalette implements GeocubeService
func (svc *Service) CreatePalette(ctx context.Context, palette *geocube.Palette, replaceIfExists bool) error {
	return svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
		err := txn.CreatePalette(ctx, palette)
		if replaceIfExists && geocube.IsError(err, geocube.EntityAlreadyExists) {
			err = txn.UpdatePalette(ctx, palette)
		}
		return err
	})
}

// Raise ValidationError
func (svc *Service) validateRemoteContainer(ctx context.Context, container *geocube.Container) error {
	containerURI, err := uri.ParseUri(container.URI)
	if err != nil {
		return fmt.Errorf("failed to parse container URI [%s]: %w", container.URI, err)
	}

	storageClass := geocube.StorageClassSTANDARD
	if strings.EqualFold(containerURI.Protocol(), "gs") {
		attrs, err := containerURI.GetAttrs(ctx)
		if err != nil {
			return geocube.NewValidationError(container.URI + " is not reachable.")
		}

		// Validate storage class
		storageClass, err = geocube.ToGcStorageClass(attrs.StorageClass)
		if err != nil {
			return geocube.NewValidationError(container.URI + ": " + err.Error())
		}
	}

	// Validate or set storageClass
	if container.StorageClass == geocube.StorageClassUNDEFINED {
		container.SetStorageClass(storageClass)
	} else if container.StorageClass != storageClass {
		return geocube.NewValidationError(container.URI + " has wrong storage class")
	}

	return nil
}

// validateAndSetRemoteDataset validates and completes Dataset
func (svc *Service) validateAndSetRemoteDataset(ctx context.Context, dataset *geocube.Dataset) error {
	datasetURI := dataset.GDALURI()
	ds, err := godal.Open(datasetURI, image.ErrLoger)
	if err != nil {
		return geocube.NewValidationError("%s is not reachable", datasetURI)
	}
	defer ds.Close()

	// Validate bands
	nbbands := int64(ds.Structure().NBands)
	for _, b := range dataset.Bands {
		if b <= 0 || b > nbbands {
			return geocube.NewValidationError("%s has no band: %d", datasetURI, b)
		}
	}

	// Set shape
	extent, err := ds.Bounds()
	if err != nil {
		return geocube.NewValidationError("failed to get dataset's bounds : %s", err.Error())
	}
	bounds := geom.NewBounds(geom.XY)
	bounds.SetCoords([]float64{extent[0], extent[1]}, []float64{extent[2], extent[3]})
	mp := geom.NewMultiPolygon(geom.XY)
	mp.Push(bounds.Polygon())
	if err := dataset.SetShape(mp, ds.Projection()); err != nil {
		return err
	}

	// Set format
	gdaldtype := godal.Unknown
	bands := ds.Bands()
	for _, b := range dataset.Bands {
		bstruct := bands[int(b)-1].Structure()
		if gdaldtype == godal.Unknown {
			gdaldtype = bstruct.DataType
		} else if gdaldtype != bstruct.DataType {
			return geocube.NewValidationError("%s : all bands must have the same data type (found %s and %s)", datasetURI, gdaldtype.String(), bstruct.DataType.String())
		}
	}
	dtype := geocube.DTypeFromGDal(gdaldtype)
	if dtype == geocube.DTypeUNDEFINED {
		return geocube.NewValidationError("%s : datatype not found or not supported: %s", datasetURI, gdaldtype.String())
	}
	if dataset.DataMapping.DType != geocube.DTypeUNDEFINED && dtype != dataset.DataMapping.DType {
		fmt.Printf("Warning: overwrite dtype (%s->%s) of %s\n", dataset.DataMapping.DType, dtype, datasetURI)
	}
	if err := dataset.SetDataType(dtype); err != nil {
		return err
	}

	// Set overviews
	hasOverviews := true
	for _, b := range dataset.Bands {
		if len(bands[int(b)-1].Overviews()) == 0 {
			hasOverviews = false
			break
		}
	}
	dataset.SetOverviews(hasOverviews)

	return nil
}

// IndexExternalDatasets implements GeocubeService
// Index datasets that are not fully known. Checks that the container is reachable and get some missing informations.
func (svc *Service) IndexExternalDatasets(ctx context.Context, newcontainer *geocube.Container, datasets []*geocube.Dataset) error {
	var err error
	log.Logger(ctx).Sugar().Debugf("Index external container %s containing %d datasets", newcontainer.URI, len(datasets))

	// Validate container
	if err = svc.validateRemoteContainer(ctx, newcontainer); err != nil {
		return fmt.Errorf("IndexExternalDatasets.%w", err)
	}

	// Validate datasets
	variables := make(map[string]*geocube.Variable)
	for _, dataset := range datasets {
		// Validate using variable
		v, ok := variables[dataset.InstanceID]
		if !ok {
			if v, err = svc.db.ReadVariableFromInstanceID(ctx, dataset.InstanceID); err != nil {
				return fmt.Errorf("IndexExternalDatasets.%w", err)
			}
			variables[dataset.InstanceID] = v
		}
		if err := dataset.ValidateWithVariable(v); err != nil {
			return fmt.Errorf("IndexExternalDatasets.%w", err)
		}
		// Validate using remote dataset
		if err := svc.validateAndSetRemoteDataset(ctx, dataset); err != nil {
			return fmt.Errorf("IndexExternalDatasets.%w", err)
		}
	}
	return svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
		if err := svc.prepareIndexation(ctx, txn, newcontainer, datasets); err != nil {
			return err
		}
		return svc.saveContainer(ctx, txn, newcontainer)
	})
}

// GetConsolidationParams implements GeocubeService
func (svc *Service) GetConsolidationParams(ctx context.Context, ID string) (*geocube.ConsolidationParams, error) {
	params, err := svc.db.ReadConsolidationParams(ctx, ID)
	if err != nil {
		return nil, fmt.Errorf("GetVariable.%w", err)
	}
	params.Clean()
	return params, nil
}

// ConfigConsolidation implements GeocubeService
func (svc *Service) ConfigConsolidation(ctx context.Context, variableID string, params geocube.ConsolidationParams) error {
	return svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
		// Get the variable
		variable, err := txn.ReadVariable(ctx, variableID)
		if err != nil {
			return fmt.Errorf("ConfigConsolidation.%w", err)
		}
		variable.Clean(true)

		// Update the configuration
		if err := variable.SetConsolidationParams(params); err != nil {
			return err // ValidationError
		}

		// Persist the variable
		err = svc.saveVariable(ctx, txn, variable)
		if err != nil {
			return fmt.Errorf("ConfigConsolidation.%w", err)
		}
		return nil
	})
}

// ConsolidateFromRecords implements GeocubeService
func (svc *Service) ConsolidateFromRecords(ctx context.Context, job *geocube.Job, recordsID []string) error {
	// Get the list of datasets for the instanceID and the records provided
	// TODO check that ListActiveDatasetsID does not take too long
	job.LogMsg(geocube.DEBUG, "Listing active datasets...")
	start := time.Now()
	datasetsID, err := svc.db.ListActiveDatasetsID(ctx, job.Payload.InstanceID, recordsID, nil, time.Time{}, time.Time{})
	if err != nil {
		return fmt.Errorf("ConsolidateFromRecords.%w", err)
	}
	log.Logger(ctx).Sugar().Debugf("ListActiveDatasetsID: %v\n", time.Since(start))
	if err := svc.csldInit(ctx, job, datasetsID); err != nil {
		return fmt.Errorf("ConsolidateFromRecords.%w", err)
	}
	return nil
}

// ConsolidateFromFilters implements GeocubeService
func (svc *Service) ConsolidateFromFilters(ctx context.Context, job *geocube.Job, tags map[string]string, fromTime, toTime time.Time) error {
	// Get the list of datasets for the instanceID and the filters provided
	// TODO check that ListActiveDatasetsID does not take too long
	start := time.Now()
	job.LogMsg(geocube.DEBUG, "Listing active datasets...")
	datasetsID, err := svc.db.ListActiveDatasetsID(ctx, job.Payload.InstanceID, nil, tags, fromTime, toTime)
	if err != nil {
		return fmt.Errorf("ConsolidateFromFilters.%w", err)
	}
	log.Logger(ctx).Sugar().Debugf("ListActiveDatasetsID: %v\n", time.Since(start))
	if err := svc.csldInit(ctx, job, datasetsID); err != nil {
		return fmt.Errorf("ConsolidateFromFilters.%w", err)
	}
	return nil
}

// CreateLayout implements GeocubeService
func (svc *Service) CreateLayout(ctx context.Context, layout *geocube.Layout) error {
	return svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
		return txn.CreateLayout(ctx, layout)
	})
}

// DeleteLayout implements GeocubeService
func (svc *Service) DeleteLayout(ctx context.Context, name string) error {
	return svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
		return txn.DeleteLayout(ctx, name)
	})
}

// CreateGrid implements GeocubeService
func (svc *Service) CreateGrid(ctx context.Context, grid *geocube.Grid) error {
	return svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
		return txn.CreateGrid(ctx, grid)
	})
}

// DeleteGrid implements GeocubeService
func (svc *Service) DeleteGrid(ctx context.Context, name string) error {
	return svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
		return txn.DeleteGrid(ctx, name)
	})
}

// ListGrids implements GeocubeService
func (svc *Service) ListGrids(ctx context.Context, nameLike string) ([]*geocube.Grid, error) {
	return svc.db.FindGrids(ctx, nameLike)
}

// ListLayouts implements GeocubeService
func (svc *Service) ListLayouts(ctx context.Context, nameLike string) ([]*geocube.Layout, error) {
	return svc.db.FindLayouts(ctx, nameLike)
}

// TileAOI implements GeocubeService
func (svc *Service) TileAOI(ctx context.Context, aoi *geocube.AOI, layoutName string, layout *geocube.Layout) (<-chan geocube.StreamedCell, error) {
	if layout == nil {
		var err error
		if layout, err = svc.db.ReadLayout(ctx, layoutName); err != nil {
			return nil, fmt.Errorf("TileAOI.%w", err)
		}
	}

	// Add a memory limit
	layout.GridParameters["memory_limit"] = fmt.Sprintf("%d", ramSize/10)

	// Create grid
	if err := layout.InitGrid(ctx, svc.db); err != nil {
		return nil, fmt.Errorf("TileAOI.%w", err)
	}

	// Tile AOI
	return layout.Covers(ctx, (*geom.MultiPolygon)(aoi.Geometry.MultiPolygon), true)
}

// ListJobs implements GeocubeService
// ListJobs retrieves only the Job but not the tasks
func (svc *Service) ListJobs(ctx context.Context, nameLike string) ([]*geocube.Job, error) {
	jobs, err := svc.db.FindJobs(ctx, nameLike)
	if err != nil {
		return nil, fmt.Errorf("ListJobs.%w", err)
	}
	// Job is clean
	for i := range jobs {
		jobs[i].Clean(true)
	}
	return jobs, nil
}

// GetJob implements GeocubeService
// GetJob retrieves only the Job but not the tasks
func (svc *Service) GetJob(ctx context.Context, jobID string) (*geocube.Job, error) {
	job, err := svc.db.ReadJob(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("GetJob.%w", err)
	}
	// Job is clean
	job.Clean(true)
	return job, nil
}

// RetryJob implements GeocubeService
func (svc *Service) RetryJob(ctx context.Context, jobID string, forceAnyState bool) error {
	job, err := svc.GetJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("RetryJob.%w", err)
	}
	event := geocube.Retried
	if forceAnyState {
		event = geocube.RetryForced
	}
	// Check that the event can be triggered (job is not persisted)
	if err := job.Trigger(*geocube.NewJobEvent(jobID, event, "")); err != nil {
		return fmt.Errorf("RetryJob.%w", err)
	}
	return svc.publishEvent(ctx, event, job, "")
}

// CancelJob implements GeocubeService
func (svc *Service) CancelJob(ctx context.Context, jobID string, forceAnyState bool) error {
	job, err := svc.GetJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("CancelJob.%w", err)
	}
	event := geocube.CancelledByUser
	if forceAnyState {
		event = geocube.CancelledByUserForced
	}
	// Check that the event can be triggered (job is not persisted)
	if err := job.Trigger(*geocube.NewJobEvent(jobID, event, "")); err != nil {
		return fmt.Errorf("CancelJob.%w", err)
	}
	return svc.publishEvent(ctx, event, job, "")
}

// ContinueJob implements GeocubeService
func (svc *Service) ContinueJob(ctx context.Context, jobID string) error {
	job, err := svc.GetJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("ContinueJob.%w", err)
	}
	// Check that the event can be triggered (job is not persisted)
	if err := job.Trigger(*geocube.NewJobEvent(jobID, geocube.Continue, "")); err != nil {
		return fmt.Errorf("ContinueJob.%w", err)
	}

	return svc.publishEvent(ctx, geocube.Continue, job, "")
}

// CleanJobs implements GeocubeService
func (svc *Service) CleanJobs(ctx context.Context, nameLike string, state *geocube.JobState) (int, error) {
	count := 0

	// Get the list of jobs that can be safely deleted
	states := []geocube.JobState{geocube.JobStateDONE, geocube.JobStateFAILED}
	if state != nil {
		states = []geocube.JobState{*state}
	}
	jobsID, err := svc.db.ListJobsID(ctx, nameLike, states)
	if err != nil {
		return 0, fmt.Errorf("CleanJobs.%w", err)
	}

	for _, jobID := range jobsID {
		err := svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
			// Get the job
			job, err := svc.db.ReadJob(ctx, jobID)
			if err != nil {
				return fmt.Errorf("CleanJobs.%w", err)
			}
			// Get the tasks
			job.Tasks, err = svc.db.ReadTasks(ctx, jobID, nil)
			if err != nil {
				return fmt.Errorf("CleanJobs.%w", err)
			}
			job.Clean(true)

			// Flag the job as ToDelete
			if ok := job.ToDelete(false); !ok {
				// If it cannot be safely deleted, we force the deletion
				log.Logger(ctx).Debug("Force deletion of job: " + jobID)
				job.ToDelete(true)
			}

			// Persist the changes
			err = svc.saveJob(ctx, txn, job)
			if err != nil {
				return fmt.Errorf("CleanJobs.%w", err)
			}
			return nil
		})
		if err != nil {
			// TODO handle error
			log.Logger(ctx).Sugar().Debugf("Unable to delete job %s: %v", jobID, err)
		} else {
			count++
		}
	}

	if err != nil {
		return 0, err
	}
	return count, nil
}

func (svc *Service) unitOfWork(ctx context.Context, f func(txn database.GeocubeTxBackend) error) (err error) {
	// Start transaction
	txn, err := svc.db.StartTransaction(ctx)
	if err != nil {
		return fmt.Errorf("uow.starttransaction: %w", err)
	}

	// Rollback if not successful
	defer func() {
		if e := txn.Rollback(); err == nil {
			err = e
		}
	}()

	// Execute function
	if err = f(txn); err != nil {
		return fmt.Errorf("uow.%w", err)
	}

	// Commit
	return txn.Commit()
}
