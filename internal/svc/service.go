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
	"github.com/airbusgeo/geocube/internal/utils/grid"
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
		return fmt.Errorf("failed to parse container URI : %w", err)
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
	ds, err := godal.Open(dataset.GDALOpenName(), image.ErrLoger)
	if err != nil {
		return geocube.NewValidationError("%s is not reachable", dataset.GDALOpenName())
	}
	defer ds.Close()

	// Validate bands
	nbbands := int64(ds.Structure().NBands)
	for _, b := range dataset.Bands {
		if b <= 0 || b > nbbands {
			return geocube.NewValidationError("%s has no band: %d", dataset.ContainerURI, b)
		}
	}

	// Set shape
	extent, err := ds.Bounds()
	if err != nil {
		return geocube.NewValidationError("failed to get dataset's bounds : %s", err.Error())
	}
	bounds := geom.NewBounds(geom.XY)
	bounds.SetCoords([]float64{extent[0], extent[1]}, []float64{extent[2], extent[3]})
	if err := dataset.SetShape(bounds.Polygon(), ds.Projection()); err != nil {
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
			return geocube.NewValidationError("%s : all bands must have the same data type (found %s and %s)", dataset.ContainerURI, gdaldtype.String(), bstruct.DataType.String())
		}
	}
	dtype := geocube.DTypeFromGDal(gdaldtype)
	if dtype == geocube.DTypeUNDEFINED {
		return geocube.NewValidationError("%s : datatype not found or not supported: %s", dataset.ContainerURI, gdaldtype.String())
	}
	if dataset.DataMapping.DType != geocube.DTypeUNDEFINED && dtype != dataset.DataMapping.DType {
		fmt.Printf("Warning: overwrite dtype (%s->%s) of %s\n", dataset.DataMapping.DType, dtype, dataset.ContainerURI)
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

	// Validate container
	if err = svc.validateRemoteContainer(ctx, newcontainer); err != nil {
		return err
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
			return err
		}
		// Validate using remote dataset
		if err := svc.validateAndSetRemoteDataset(ctx, dataset); err != nil {
			return err
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
	start := time.Now()
	datasetsID, err := svc.db.ListActiveDatasetsID(ctx, job.Payload.InstanceID, recordsID, nil, time.Time{}, time.Time{})
	if err != nil {
		return fmt.Errorf("ConsolidateFromRecords.%w", err)
	}
	fmt.Printf("ListActiveDatasetsID: %v\n", time.Since(start))

	return svc.consolidate(ctx, job, datasetsID)
}

// ConsolidateFromFilters implements GeocubeService
func (svc *Service) ConsolidateFromFilters(ctx context.Context, job *geocube.Job, tags map[string]string, fromTime, toTime time.Time) error {
	// Get the list of datasets for the instanceID and the filters provided
	// TODO check that ListActiveDatasetsID does not take too long
	start := time.Now()
	datasetsID, err := svc.db.ListActiveDatasetsID(ctx, job.Payload.InstanceID, nil, tags, fromTime, toTime)
	if err != nil {
		return fmt.Errorf("ConsolidateFromFilters.%w", err)
	}
	fmt.Printf("ListActiveDatasetsID: %v\n", time.Since(start))

	return svc.consolidate(ctx, job, datasetsID)
}

func (svc Service) consolidate(ctx context.Context, job *geocube.Job, datasetsID []string) error {
	if len(datasetsID) == 0 {
		return geocube.NewEntityNotFound("", "", "", "No dataset found for theses records and instances")
	}

	return svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
		// Check and get consolidation parameters
		var params *geocube.ConsolidationParams
		{
			variable, err := txn.ReadVariableFromInstanceID(ctx, job.Payload.InstanceID)
			if err != nil {
				return fmt.Errorf("consolidate.%w", err)
			}
			params, err = txn.ReadConsolidationParams(ctx, variable.ID)
			if err != nil {
				return fmt.Errorf("consolidate.%w", err)
			}
			params.Clean()
			if err := job.SetParams(*params); err != nil {
				return fmt.Errorf("consolidate.%w", err)
			}
		}

		// Lock datasets
		job.LockDatasets(datasetsID, geocube.LockFlagINIT)

		// Persist the job
		start := time.Now()
		if err := svc.saveJob(ctx, txn, job); err != nil {
			return fmt.Errorf("consolidate.%w", err)
		}
		fmt.Printf("SaveJob: %v\n", time.Since(start))
		start = time.Now()

		// Start the job
		if err := svc.csldOnEnterNewState(ctx, job); err != nil {
			return fmt.Errorf("consolidate.%w", err)
		}
		return nil
	})
}

// CreateLayout implements GeocubeService
func (svc *Service) CreateLayout(ctx context.Context, layout *geocube.Layout) error {
	return svc.unitOfWork(ctx, func(txn database.GeocubeTxBackend) error {
		return txn.CreateLayout(ctx, layout)
	})
}

// ListLayouts implements GeocubeService
func (svc *Service) ListLayouts(ctx context.Context, nameLike string) ([]*geocube.Layout, error) {
	return svc.db.FindLayouts(ctx, nameLike)
}

// TileAOI implements GeocubeService
func (svc *Service) TileAOI(ctx context.Context, aoi *geocube.AOI, crsS string, resolution float32, width, height int32) (<-chan *grid.Cell, error) {
	// Create Layout with a regular grid
	layout := geocube.Layout{
		GridParameters: geocube.Metadata{
			"grid":         "regular",
			"crs":          crsS,
			"cell_x_size":  fmt.Sprintf("%d", width),
			"cell_y_size":  fmt.Sprintf("%d", height),
			"resolution":   fmt.Sprintf("%f", resolution),
			"ox":           "0",
			"oy":           "0",
			"memory_limit": fmt.Sprintf("%d", ramSize/10),
		},
	}

	// Tile AOI
	return layout.Covers(ctx, (*geom.MultiPolygon)(aoi.Geometry.MultiPolygon))
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
	if forceAnyState {
		return svc.handleJobEvt(ctx, *geocube.NewJobEvent(jobID, geocube.RetryForced, ""))
	}
	return svc.handleJobEvt(ctx, *geocube.NewJobEvent(jobID, geocube.ConsolidationRetried, ""))
}

// CancelJob implements GeocubeService
func (svc *Service) CancelJob(ctx context.Context, jobID string) error {
	return svc.handleJobEvt(ctx, *geocube.NewJobEvent(jobID, geocube.CancelledByUser, ""))
}

// ContinueJob implements GeocubeService
func (svc *Service) ContinueJob(ctx context.Context, jobID string) error {
	return svc.handleJobEvt(ctx, *geocube.NewJobEvent(jobID, geocube.Continue, ""))
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
