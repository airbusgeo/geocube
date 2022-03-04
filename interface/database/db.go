package database

import (
	"context"
	"time"

	"github.com/airbusgeo/geocube/internal/utils/grid"

	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/geocube/internal/utils/proj"
	"github.com/twpayne/go-geom"
)

type GeocubeTxBackend interface {
	GeocubeBackend
	// Must be call to apply transaction
	Commit() error
	// Might be called to cancel the transaction (no effect if commit has already be done)
	Rollback() error
}

type GeocubeDBBackend interface {
	GeocubeBackend
	StartTransaction(ctx context.Context) (GeocubeTxBackend, error)
}

type GeocubeBackend interface {
	/******************** Records *************************/
	// Create a batch of records in database
	CreateRecords(ctx context.Context, records []*geocube.Record) error
	// Delete a batch of records from the database iif no dataset has reference on.
	// Returns the number of deleted records
	DeleteRecords(ctx context.Context, ids []string) (int64, error)
	// FindRecords fetchs all the records that match the criterias
	// [Optional] namelike: filter by name (support "*?" and "(?i)" suffix for case insensitivity)
	// [Optional] fromTime, toTime: filter by datetime
	// [Optional] jobID: filter the records whom some datasets are locked by the job
	// [Optional] aoi: filter the records that intersects the AOI
	// [Optional] page, limit : limits the number of results
	// [Optional] order : order results by date
	// [Optional] loadAOI : load AOI of records
	FindRecords(ctx context.Context, namelike string, tags geocube.Metadata, fromTime, toTime time.Time, jobID string, aoi *geocube.AOI, page, limit int, order, loadAOI bool) ([]*geocube.Record, error)
	// AddRecordsTags add tags on list of records
	AddRecordsTags(ctx context.Context, ids []string, tags geocube.Metadata) (int64, error)
	// RemoveRecordsTags remove tags on list of records
	RemoveRecordsTags(ctx context.Context, ids []string, tagsKey []string) (int64, error)
	// DeletePendingRecords deletes records that are not linked to any datasets
	DeletePendingRecords(ctx context.Context) (int64, error)
	// ReadRecords with the given ids
	// Preserve order, removing duplicates
	// Raise EntityNotFound
	ReadRecords(ctx context.Context, ids []string) ([]*geocube.Record, error)
	// CreateAOI creates the aoi in database
	// Raise EntityAlreadyExists
	CreateAOI(ctx context.Context, aoi *geocube.AOI) error
	// ReadAOI retrieves the aoi from database
	// Raise EntityAlreadyNotFound
	ReadAOI(ctx context.Context, aoiID string) (*geocube.AOI, error)
	// GetUnionAOI returns the union of AOI of all the provided records
	// TODO delete ? GetUnionAOI(ctx context.Context, recordsID []string) (*geom.MultiPolygon, error)
	// DeletePendingAOIs deletes aois that are not linked to any records
	DeletePendingAOIs(ctx context.Context) (int64, error)

	/******************** Variables *************************/
	// CreateVariable creates the variable in database
	// Raise EntityAlreadyExists
	CreateVariable(ctx context.Context, variable *geocube.Variable) error
	// UpdateVariable updates the field (name, unit, description, palette, resampling) in database
	// Raise geocube.EntityNotFound, geocube.EntityAlreadyExists
	UpdateVariable(ctx context.Context, variable *geocube.Variable) error
	// DeleteVariable deletes the variable in the database
	DeleteVariable(ctx context.Context, variableID string) error
	// DeletePendingVariables deletes instances that are not linked to any variable
	DeletePendingVariables(ctx context.Context) (int64, error)
	// CreateInstance creates the instance in database
	CreateInstance(ctx context.Context, variableID string, instance *geocube.VariableInstance) (err error)
	// UpdateInstance updates the name and the metadata of the instance in the database
	// Raise geocube.EntityNotFound, geocube.EntityAlreadyExists
	UpdateInstance(ctx context.Context, instance *geocube.VariableInstance) error
	// DeleteInstance deletes the instance in the database
	DeleteInstance(ctx context.Context, instanceID string) error
	// DeletePendingInstances deletes instances that are not linked to any dataset
	DeletePendingInstances(ctx context.Context) (int64, error)

	// ReadVariable retrieves the variable and all its instances with the given id (but not the ConsolidationParams)
	// Raise an error geocube.EntityNotFound
	ReadVariable(ctx context.Context, variableID string) (*geocube.Variable, error)
	// ReadVariableFromInstanceID retrieves the variable with the instance given (other instances are not fetched) (but not the ConsolidationParams)
	// Raise an error geocube.EntityNotFound
	ReadVariableFromInstanceID(ctx context.Context, instanceID string) (*geocube.Variable, error)
	// ReadVariableFromName retrieves the variable and all its instances with the given name (but not the ConsolidationParams)
	// Raise an error geocube.EntityNotFound
	ReadVariableFromName(ctx context.Context, variableName string) (*geocube.Variable, error)
	// FindVariables retrieves all the variable having a similar name (support "*?"" and "(?i)" suffix for case insensitivity)
	// FindVariables does not retrieve ConsolidationParams
	FindVariables(ctx context.Context, namelike string, page, limit int) ([]*geocube.Variable, error)

	// CreateConsolidationParams creates or updates the consolidation parameters associated to the given id
	CreateConsolidationParams(ctx context.Context, id string, consolidationParams geocube.ConsolidationParams) error
	// ReadConsolidationParams reads and returns the consolidation paramaters associated to the given id
	ReadConsolidationParams(ctx context.Context, id string) (*geocube.ConsolidationParams, error)
	// DeleteConsolidationParams deletes the consolidation paramaters associated to the given id
	DeleteConsolidationParams(ctx context.Context, id string) error
	// DeletePendingConsolidationParams deletes consolidationParams that are not linked to any jobs nor variable
	DeletePendingConsolidationParams(ctx context.Context) (int64, error)

	/******************** Palettes *************************/
	// CreatePalette creates the palette in database
	// Raise EntityAlreadyExists
	CreatePalette(ctx context.Context, palette *geocube.Palette) error
	// ReadPalette creates the palette in database
	// Raise EntityNotFound
	ReadPalette(ctx context.Context, name string) (*geocube.Palette, error)
	// UpdatePalette udpates the palette in database
	// Raise EntityNotFound
	UpdatePalette(ctx context.Context, palette *geocube.Palette) error
	// DeletePalette creates the palette in database
	// Raise EntityNotFound
	DeletePalette(ctx context.Context, name string) error

	/******************** Containers *************************/
	// CreateContainer creates the container in database
	CreateContainer(ctx context.Context, container *geocube.Container) error
	// UpdateContainer updates nothing in database
	UpdateContainer(ctx context.Context, container *geocube.Container) error
	// ReadContainers retrieves the containers with the provided uris and there datasets
	// Preserve order, removing duplicates
	// Raise an error geocube.EntityNotFound
	ReadContainers(ctx context.Context, containersURI []string) ([]*geocube.Container, error)
	// DeleteContainer deletes empty container
	// Raise an error geocube.DependencyStillExists
	DeleteContainer(ctx context.Context, containerURI string) error
	// DeletePendingContainers deletes containers that are not linked to any dataset
	DeletePendingContainers(ctx context.Context) (int64, error)
	// CreateDatasets creates the batch of datasets in database
	CreateDatasets(ctx context.Context, datasets []*geocube.Dataset) error
	// DeleteDatasets deletes a batch of datasetsID in database
	DeleteDatasets(ctx context.Context, datasetsID []string) error
	// ListActiveDatasetsID retrieves all the active datasets id from the list of records representing the given variable
	// [Optional] recordsID: filter by list of vrecords
	// [Optional] recordTags: filter by record's tags
	// [Optional] fromTime, toTime: filter by record's datetime
	ListActiveDatasetsID(ctx context.Context, instanceID string, recordsID []string,
		recordTags geocube.Metadata, fromTime, toTime time.Time) ([]string, error)
	// FindDatasets fetches all the datasets that match the criterias
	// [Optional] containerURI, lockedByJobID: filter by container, or those locked by job
	// [Optional] instancesID, recordsID: filter by list of variable instances/records
	// [Optional] recordTags: filter by record's tags
	// [Optional] fromTime, toTime: filter by record's datetime
	// [Optional] geog, [refined]: filter the datasets that intersect the geographic ring, and optionally, refine with "refined" iif the dataset has the same SRID.
	// order : by record.datetime (ascending) and record.id
	FindDatasets(ctx context.Context, status geocube.DatasetStatus, containerURI, lockedByJobID string, instancesID, recordsID []string,
		recordTags geocube.Metadata, fromTime, toTime time.Time, geog *proj.GeographicRing, refined *proj.Ring, page, limit int, order bool) ([]*geocube.Dataset, error)
	// GetDatasetsGeometryUnion returns the union of AOI of all the locked datasets
	GetDatasetsGeometryUnion(ctx context.Context, lockedByJobID string) (*geom.MultiPolygon, error)

	// UpdateDatasets given an instance id and records ids
	UpdateDatasets(ctx context.Context, instanceID string, recordIds []string, dmapping geocube.DataMapping) (map[string]int64, error)

	// ComputeValidShapeFromCell compute valid shape in right crs from cell ring
	ComputeValidShapeFromCell(ctx context.Context, datasetIDS []string, cell *grid.Cell) (*proj.Shape, error)

	/******************** Layouts *************************/
	// CreateLayout creates the layout in the database
	CreateLayout(ctx context.Context, layout *geocube.Layout) error
	// DeleteLayout deletes the layout from the database
	DeleteLayout(ctx context.Context, name string) error
	// ReadLayout retrieve the layout
	ReadLayout(ctx context.Context, name string) (*geocube.Layout, error)
	// FindLayout retrieves the layouts (support "*?" and "(?i)" suffix for case insensitivity)
	// Raise geocube.EntityNotFound
	FindLayouts(ctx context.Context, nameLike string) ([]*geocube.Layout, error)

	// SaveContainerLayout saves the layout that defines the container
	SaveContainerLayout(ctx context.Context, containerURI string, layoutName string) error

	// DeleteContainerLayout removes the layout that defines the container
	// Raise geocube.EntityNotFound (it can be ignored)
	DeleteContainerLayout(ctx context.Context, containerURI string) error

	/******************** Grids *************************/
	// CreateGrid creates a grid in the database
	CreateGrid(ctx context.Context, grid *geocube.Grid) error
	// DeleteGrid deletes a grid from the database
	DeleteGrid(ctx context.Context, gridName string) error
	// ReadGrid retrieve the grid (without the cells)
	ReadGrid(ctx context.Context, name string) (*geocube.Grid, error)
	// FindGrids retrieves the grid name & description (not the cells) (support "*?" and "(?i)" suffix for case insensitivity)
	FindGrids(ctx context.Context, nameLike string) ([]*geocube.Grid, error)
	// FindCells find the cells of the grid intersecting the AOI
	// Returns the cells  and the intersection with the AOI
	FindCells(ctx context.Context, gridName string, aoi *geocube.AOI) ([]geocube.Cell, []geom.MultiPolygon, error)

	/******************** Jobs *************************/
	// CreateJob creates the job in the database
	CreateJob(ctx context.Context, job *geocube.Job) error
	// FindJobs retrieves the jobs but not their tasks (support "*?" and "(?i)" suffix for case insensitivity)
	// Raise geocube.EntityNotFound
	FindJobs(ctx context.Context, nameLike string) ([]*geocube.Job, error)
	// ReadJob retrieves the job but not its tasks
	// Raise geocube.EntityNotFound
	ReadJob(ctx context.Context, jobID string) (*geocube.Job, error)
	// UpdateJob updates the job status and updateTime
	// Raise geocube.EntityNotFound
	UpdateJob(ctx context.Context, job *geocube.Job) error
	// DeleteJob deletes the job from the database and release the datasets
	// But its Params and all its tasks must have been deleted before
	DeleteJob(ctx context.Context, jobID string) error

	// ReadJobWithTask retrieves the job with the given task
	// Raise geocube.EntityNotFound if the job or the task is not found
	ReadJobWithTask(ctx context.Context, jobID string, taskID string) (*geocube.Job, error)
	// ListJobsID retrieves all the JobID that fit the requirements
	// nameLike supports "*?" and "(?i)" suffix for case insensitivity
	ListJobsID(ctx context.Context, nameLike string, states []geocube.JobState) ([]string, error)

	/******************** LockDatasets *************************/
	// LockDatasets locks datasets in the database (each element of datasetsID must be unique)
	LockDatasets(ctx context.Context, jobID string, datasetsID []string, flag int) error
	// ReleaseDatasets releases all the datasets of the job from the database with the given flag
	ReleaseDatasets(ctx context.Context, jobID string, flag int) error

	/******************** Task *************************/
	// CreateTasks creates the batch of tasks in the database
	CreateTasks(ctx context.Context, jobID string, tasks []*geocube.Task) error
	// ReadTasks retrieves all the tasks of the job with the given states
	// If states is nil, all the states will be retrieved
	ReadTasks(ctx context.Context, jobID string, states []geocube.TaskState) ([]*geocube.Task, error)
	// UpdateTask updates the task status
	// Raise geocube.EntityNotFound
	UpdateTask(ctx context.Context, task *geocube.Task) error
	// DeleteTask deletes the task
	DeleteTask(ctx context.Context, taskID string) error

	/******************** Consolidation *************************/
	// ChangeDatasetsStatus changes the status of all the datasets locked by the job whom status is fromStatus to toStatus
	ChangeDatasetsStatus(ctx context.Context, lockedByJobID string, fromStatus geocube.DatasetStatus, toStatus geocube.DatasetStatus) error
}
