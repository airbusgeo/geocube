package mocks

import (
	"context"
	"time"

	"github.com/airbusgeo/geocube/internal/utils/grid"

	"github.com/airbusgeo/geocube/interface/database"
	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/geocube/internal/utils/proj"
	"github.com/stretchr/testify/mock"
	"github.com/twpayne/go-geom"
)

type GeocubeBackend struct {
	mock.Mock
}

func (_m *GeocubeBackend) StartTransaction(ctx context.Context) (database.GeocubeTxBackend, error) {
	ret := _m.Called(ctx)

	var r0 database.GeocubeTxBackend
	if rf, ok := ret.Get(0).(func(context.Context) database.GeocubeTxBackend); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(database.GeocubeTxBackend)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func (_m *GeocubeBackend) CreateRecords(ctx context.Context, records []*geocube.Record) error {
	ret := _m.Called(ctx, records)
	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, []*geocube.Record) error); ok {
		r0 = rf(ctx, records)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

func (_m *GeocubeBackend) DeleteRecords(ctx context.Context, ids []string) (int64, error) {
	panic("implement me")
}

func (_m *GeocubeBackend) FindRecords(ctx context.Context, namelike string, tags geocube.Metadata, fromTime, toTime time.Time, jobID string, aoi *geocube.AOI, page, limit int, order, loadAOI bool) ([]*geocube.Record, error) {
	ret := _m.Called(ctx, namelike, tags, fromTime, toTime, jobID, aoi, page, limit, order, loadAOI)

	var r0 []*geocube.Record
	if rf, ok := ret.Get(0).(func(context.Context, string, geocube.Metadata, time.Time, time.Time, string, *geocube.AOI, int, int, bool, bool) []*geocube.Record); ok {
		r0 = rf(ctx, namelike, tags, fromTime, toTime, jobID, aoi, page, limit, order, loadAOI)
	} else {
		r0 = ret.Get(0).([]*geocube.Record)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, geocube.Metadata, time.Time, time.Time, string, *geocube.AOI, int, int, bool, bool) error); ok {
		r1 = rf(ctx, namelike, tags, fromTime, toTime, jobID, aoi, page, limit, order, loadAOI)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func (_m *GeocubeBackend) AddRecordsTags(ctx context.Context, ids []string, tags geocube.Metadata) (int64, error) {
	panic("implement me")
}

func (_m *GeocubeBackend) RemoveRecordsTags(ctx context.Context, ids []string, tagsKey []string) (int64, error) {
	panic("implement me")
}

func (_m *GeocubeBackend) DeletePendingRecords(ctx context.Context) (int64, error) {
	panic("implement me")
}

func (_m *GeocubeBackend) ReadRecords(ctx context.Context, ids []string) ([]*geocube.Record, error) {
	panic("implement me")
}

func (_m *GeocubeBackend) CreateAOI(ctx context.Context, aoi *geocube.AOI) error {
	panic("implement me")
}

func (_m *GeocubeBackend) ReadAOI(ctx context.Context, aoiID string) (*geocube.AOI, error) {
	panic("implement me")
}

func (_m *GeocubeBackend) DeletePendingAOIs(ctx context.Context) (int64, error) {
	panic("implement me")
}

func (_m *GeocubeBackend) CreateVariable(ctx context.Context, variable *geocube.Variable) error {
	panic("implement me")
}

func (_m *GeocubeBackend) UpdateVariable(ctx context.Context, variable *geocube.Variable) error {
	panic("implement me")
}

func (_m *GeocubeBackend) DeleteVariable(ctx context.Context, variableID string) error {
	panic("implement me")
}

func (_m *GeocubeBackend) DeletePendingVariables(ctx context.Context) (int64, error) {
	panic("implement me")
}

func (_m *GeocubeBackend) CreateInstance(ctx context.Context, variableID string, instance *geocube.VariableInstance) (err error) {
	panic("implement me")
}

func (_m *GeocubeBackend) UpdateInstance(ctx context.Context, instance *geocube.VariableInstance) error {
	panic("implement me")
}

func (_m *GeocubeBackend) DeleteInstance(ctx context.Context, instanceID string) error {
	panic("implement me")
}

func (_m *GeocubeBackend) DeletePendingInstances(ctx context.Context) (int64, error) {
	panic("implement me")
}

func (_m *GeocubeBackend) ReadVariable(ctx context.Context, variableID string) (*geocube.Variable, error) {
	panic("implement me")
}

func (_m *GeocubeBackend) ReadVariableFromInstanceID(ctx context.Context, instanceID string) (*geocube.Variable, error) {
	ret := _m.Called(ctx, instanceID)

	var r0 *geocube.Variable
	if rf, ok := ret.Get(0).(func(context.Context, string) *geocube.Variable); ok {
		r0 = rf(ctx, instanceID)
	} else {
		r0 = ret.Get(0).(*geocube.Variable)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, instanceID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func (_m *GeocubeBackend) ReadVariableFromName(ctx context.Context, variableName string) (*geocube.Variable, error) {
	panic("implement me")
}

func (_m *GeocubeBackend) FindVariables(ctx context.Context, namelike string, page, limit int) ([]*geocube.Variable, error) {
	panic("implement me")
}

func (_m *GeocubeBackend) CreateConsolidationParams(ctx context.Context, id string, consolidationParams geocube.ConsolidationParams) error {
	ret := _m.Called(ctx, id, consolidationParams)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, geocube.ConsolidationParams) error); ok {
		r0 = rf(ctx, id, consolidationParams)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *GeocubeBackend) ReadConsolidationParams(ctx context.Context, id string) (*geocube.ConsolidationParams, error) {
	panic("implement me")
}

func (_m *GeocubeBackend) DeleteConsolidationParams(ctx context.Context, id string) error {
	panic("implement me")
}

func (_m *GeocubeBackend) DeletePendingConsolidationParams(ctx context.Context) (int64, error) {
	panic("implement me")
}

func (_m *GeocubeBackend) CreatePalette(ctx context.Context, palette *geocube.Palette) error {
	panic("implement me")
}

func (_m *GeocubeBackend) ReadPalette(ctx context.Context, name string) (*geocube.Palette, error) {
	panic("implement me")
}

func (_m *GeocubeBackend) UpdatePalette(ctx context.Context, palette *geocube.Palette) error {
	panic("implement me")
}

func (_m *GeocubeBackend) DeletePalette(ctx context.Context, name string) error {
	panic("implement me")
}

func (_m *GeocubeBackend) CreateContainer(ctx context.Context, container *geocube.Container) error {
	panic("implement me")
}

func (_m *GeocubeBackend) UpdateContainer(ctx context.Context, container *geocube.Container) error {
	panic("implement me")
}

func (_m *GeocubeBackend) ReadContainers(ctx context.Context, containersURI []string) ([]*geocube.Container, error) {
	panic("implement me")
}

func (_m *GeocubeBackend) DeleteContainer(ctx context.Context, containerURI string) error {
	panic("implement me")
}

func (_m *GeocubeBackend) DeletePendingContainers(ctx context.Context) (int64, error) {
	panic("implement me")
}

func (_m *GeocubeBackend) CreateDatasets(ctx context.Context, datasets []*geocube.Dataset) error {
	panic("implement me")
}

func (_m *GeocubeBackend) DeleteDatasets(ctx context.Context, datasetsID []string) error {
	panic("implement me")
}

func (_m *GeocubeBackend) ListActiveDatasetsID(ctx context.Context, instanceID string, recordsID []string, recordTags geocube.Metadata, fromTime, toTime time.Time) ([]string, error) {
	ret := _m.Called(ctx, instanceID, recordsID, recordTags, fromTime, toTime)

	var r0 []string
	if rf, ok := ret.Get(0).(func(context.Context, string, []string, geocube.Metadata, time.Time, time.Time) []string); ok {
		r0 = rf(ctx, instanceID, recordsID, recordTags, fromTime, toTime)
	} else {
		r0 = ret.Get(0).([]string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, []string, geocube.Metadata, time.Time, time.Time) error); ok {
		r1 = rf(ctx, instanceID, recordsID, recordTags, fromTime, toTime)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func (_m *GeocubeBackend) FindDatasets(ctx context.Context, status geocube.DatasetStatus, containerURI, lockedByJobID string, instancesID, recordsID []string, recordTags geocube.Metadata, fromTime, toTime time.Time, geog *proj.GeographicRing, refined *proj.Ring, page, limit int, order bool) ([]*geocube.Dataset, error) {
	ret := _m.Called(ctx, status, containerURI, lockedByJobID, instancesID, recordsID, recordTags, fromTime, toTime, geog, refined, page, limit, order)

	var r0 []*geocube.Dataset
	if rf, ok := ret.Get(0).(func(context.Context, geocube.DatasetStatus, string, string, []string, []string, geocube.Metadata, time.Time, time.Time, *proj.GeographicRing, *proj.Ring, int, int, bool) []*geocube.Dataset); ok {
		r0 = rf(ctx, status, containerURI, lockedByJobID, instancesID, recordsID, recordTags, fromTime, toTime, geog, refined, page, limit, order)
	} else {
		r0 = ret.Get(0).([]*geocube.Dataset)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, geocube.DatasetStatus, string, string, []string, []string, geocube.Metadata, time.Time, time.Time, *proj.GeographicRing, *proj.Ring, int, int, bool) error); ok {
		r1 = rf(ctx, status, containerURI, lockedByJobID, instancesID, recordsID, recordTags, fromTime, toTime, geog, refined, page, limit, order)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func (_m *GeocubeBackend) GetDatasetsGeometryUnion(ctx context.Context, lockedByJobID string) (*geom.MultiPolygon, error) {
	ret := _m.Called(ctx, lockedByJobID)

	var r0 *geom.MultiPolygon
	if rf, ok := ret.Get(0).(func(context.Context, string) *geom.MultiPolygon); ok {
		r0 = rf(ctx, lockedByJobID)
	} else {
		r0 = ret.Get(0).(*geom.MultiPolygon)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, lockedByJobID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func (_m *GeocubeBackend) UpdateDatasets(ctx context.Context, instanceID string, recordIds []string, dmapping geocube.DataMapping) (map[string]int64, error) {
	panic("implement me")
}

func (_m *GeocubeBackend) CreateLayout(ctx context.Context, layout *geocube.Layout) error {
	panic("implement me")
}

func (_m *GeocubeBackend) DeleteLayout(ctx context.Context, name string) error {
	panic("implement me")
}

func (_m *GeocubeBackend) ReadLayout(ctx context.Context, layoutID string) (*geocube.Layout, error) {
	ret := _m.Called(ctx, layoutID)

	var r0 *geocube.Layout
	if rf, ok := ret.Get(0).(func(context.Context, string) *geocube.Layout); ok {
		r0 = rf(ctx, layoutID)
	} else {
		r0 = ret.Get(0).(*geocube.Layout)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, layoutID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func (_m *GeocubeBackend) FindLayouts(ctx context.Context, nameLike string) ([]*geocube.Layout, error) {
	panic("implement me")
}

func (_m *GeocubeBackend) SaveContainerLayout(ctx context.Context, containerURI string, layoutName string) error {
	ret := _m.Called(ctx, containerURI, layoutName)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) error); ok {
		r0 = rf(ctx, containerURI, layoutName)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
func (_m *GeocubeBackend) DeleteContainerLayout(ctx context.Context, containerURI string) error {
	ret := _m.Called(ctx, containerURI)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, containerURI)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *GeocubeBackend) CreateGrid(ctx context.Context, grid *geocube.Grid) error {
	panic("implement me")
}

func (_m *GeocubeBackend) DeleteGrid(ctx context.Context, gridName string) error {
	panic("implement me")
}

func (_m *GeocubeBackend) ReadGrid(ctx context.Context, name string) (*geocube.Grid, error) {
	panic("implement me")
}

func (_m *GeocubeBackend) FindGrids(ctx context.Context, nameLike string) ([]*geocube.Grid, error) {
	panic("implement me")
}

func (_m *GeocubeBackend) FindCells(ctx context.Context, gridName string, aoi *geocube.AOI) ([]geocube.Cell, []geom.MultiPolygon, error) {
	panic("implement me")
}

func (_m *GeocubeBackend) CreateJob(ctx context.Context, job *geocube.Job) error {
	ret := _m.Called(ctx, job)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *geocube.Job) error); ok {
		r0 = rf(ctx, job)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *GeocubeBackend) FindJobs(ctx context.Context, nameLike string) ([]*geocube.Job, error) {
	panic("implement me")
}

func (_m *GeocubeBackend) ReadJob(ctx context.Context, jobID string) (*geocube.Job, error) {
	ret := _m.Called(ctx, jobID)

	var r0 *geocube.Job
	if rf, ok := ret.Get(0).(func(context.Context, string) *geocube.Job); ok {
		r0 = rf(ctx, jobID)
	} else {
		r0 = ret.Get(0).(*geocube.Job)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, jobID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func (_m *GeocubeBackend) UpdateJob(ctx context.Context, job *geocube.Job) error {
	ret := _m.Called(ctx, job)

	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *geocube.Job) error); ok {
		r1 = rf(ctx, job)
	} else {
		r1 = ret.Error(0)
	}

	return r1
}

func (_m *GeocubeBackend) DeleteJob(ctx context.Context, jobID string) error {
	panic("implement me")
}

func (_m *GeocubeBackend) ReadJobWithTask(ctx context.Context, jobID string, taskID string) (*geocube.Job, error) {
	ret := _m.Called(ctx, jobID, taskID)

	var r0 *geocube.Job
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *geocube.Job); ok {
		r0 = rf(ctx, jobID, taskID)
	} else {
		r0 = ret.Get(0).(*geocube.Job)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, jobID, taskID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func (_m *GeocubeBackend) ListJobsID(ctx context.Context, nameLike string, states []geocube.JobState) ([]string, error) {
	panic("implement me")
}

func (_m *GeocubeBackend) LockDatasets(ctx context.Context, jobID string, datasetsID []string, flag int) error {
	ret := _m.Called(ctx, jobID, datasetsID, flag)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, []string, int) error); ok {
		r0 = rf(ctx, jobID, datasetsID, flag)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *GeocubeBackend) ReleaseDatasets(ctx context.Context, jobID string, flag int) error {
	ret := _m.Called(ctx, jobID, flag)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, int) error); ok {
		r0 = rf(ctx, jobID, flag)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *GeocubeBackend) CreateTasks(ctx context.Context, jobID string, tasks []*geocube.Task) error {
	panic("implement me")
}

func (_m *GeocubeBackend) ReadTasks(ctx context.Context, jobID string, states []geocube.TaskState) ([]*geocube.Task, error) {
	ret := _m.Called(ctx, jobID, states)

	var r0 []*geocube.Task
	if rf, ok := ret.Get(0).(func(context.Context, string, []geocube.TaskState) []*geocube.Task); ok {
		r0 = rf(ctx, jobID, states)
	} else {
		r0 = ret.Get(0).([]*geocube.Task)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, []geocube.TaskState) error); ok {
		r1 = rf(ctx, jobID, states)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func (_m *GeocubeBackend) ComputeValidShapeFromCell(ctx context.Context, datasetIDS []string, cell *grid.Cell) (*proj.Shape, error) {
	ret := _m.Called(ctx, datasetIDS, cell)

	var r0 *proj.Shape
	if rf, ok := ret.Get(0).(func(context.Context, []string, *grid.Cell) *proj.Shape); ok {
		r0 = rf(ctx, datasetIDS, cell)
	} else {
		r0 = ret.Get(0).(*proj.Shape)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, []string, *grid.Cell) error); ok {
		r1 = rf(ctx, datasetIDS, cell)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func (_m *GeocubeBackend) UpdateTask(ctx context.Context, task *geocube.Task) error {
	ret := _m.Called(ctx, task)

	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *geocube.Task) error); ok {
		r1 = rf(ctx, task)
	} else {
		r1 = ret.Error(0)
	}

	return r1
}

func (_m *GeocubeBackend) DeleteTask(ctx context.Context, taskID string) error {
	panic("implement me")
}

func (_m *GeocubeBackend) ChangeDatasetsStatus(ctx context.Context, lockedByJobID string, fromStatus geocube.DatasetStatus, toStatus geocube.DatasetStatus) error {
	panic("implement me")
}

type GeocubeTxBackend struct {
	mock.Mock
	GeocubeBackend
}

func (_m *GeocubeTxBackend) Commit() error {
	ret := _m.Called()

	var r1 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(0)
	}

	return r1
}

func (_m *GeocubeTxBackend) Rollback() error {
	ret := _m.Called()

	var r1 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(0)
	}

	return r1
}

func (_m *GeocubeTxBackend) ReadVariableFromInstanceID(ctx context.Context, instanceID string) (*geocube.Variable, error) {
	ret := _m.Called(ctx, instanceID)

	var r0 *geocube.Variable
	if rf, ok := ret.Get(0).(func(context.Context, string) *geocube.Variable); ok {
		r0 = rf(ctx, instanceID)
	} else {
		r0 = ret.Get(0).(*geocube.Variable)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, instanceID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func (_m *GeocubeTxBackend) ReadConsolidationParams(ctx context.Context, id string) (*geocube.ConsolidationParams, error) {
	ret := _m.Called(ctx, id)

	var r0 *geocube.ConsolidationParams
	if rf, ok := ret.Get(0).(func(context.Context, string) *geocube.ConsolidationParams); ok {
		r0 = rf(ctx, id)
	} else {
		r0 = ret.Get(0).(*geocube.ConsolidationParams)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func (_m *GeocubeTxBackend) CreateJob(ctx context.Context, job *geocube.Job) error {
	ret := _m.Called(ctx, job)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *geocube.Job) error); ok {
		r0 = rf(ctx, job)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *GeocubeTxBackend) CreateConsolidationParams(ctx context.Context, id string, consolidationParams geocube.ConsolidationParams) error {
	ret := _m.Called(ctx, id, consolidationParams)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, geocube.ConsolidationParams) error); ok {
		r0 = rf(ctx, id, consolidationParams)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *GeocubeTxBackend) LockDatasets(ctx context.Context, jobID string, datasetsID []string, flag int) error {
	ret := _m.Called(ctx, jobID, datasetsID, flag)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, []string, int) error); ok {
		r0 = rf(ctx, jobID, datasetsID, flag)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *GeocubeTxBackend) FindRecords(ctx context.Context, namelike string, tags geocube.Metadata, fromTime, toTime time.Time, jobID string, aoi *geocube.AOI, page, limit int, order, loadAOI bool) ([]*geocube.Record, error) {
	ret := _m.Called(ctx, namelike, tags, fromTime, toTime, jobID, aoi, page, limit, order, loadAOI)

	var r0 []*geocube.Record
	if rf, ok := ret.Get(0).(func(context.Context, string, geocube.Metadata, time.Time, time.Time, string, *geocube.AOI, int, int, bool, bool) []*geocube.Record); ok {
		r0 = rf(ctx, namelike, tags, fromTime, toTime, jobID, aoi, page, limit, order, loadAOI)
	} else {
		r0 = ret.Get(0).([]*geocube.Record)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, geocube.Metadata, time.Time, time.Time, string, *geocube.AOI, int, int, bool, bool) error); ok {
		r1 = rf(ctx, namelike, tags, fromTime, toTime, jobID, aoi, page, limit, order, loadAOI)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func (_m *GeocubeTxBackend) GetDatasetsGeometryUnion(ctx context.Context, lockedByJobID string) (*geom.MultiPolygon, error) {
	ret := _m.Called(ctx, lockedByJobID)

	var r0 *geom.MultiPolygon
	if rf, ok := ret.Get(0).(func(context.Context, string) *geom.MultiPolygon); ok {
		r0 = rf(ctx, lockedByJobID)
	} else {
		r0 = ret.Get(0).(*geom.MultiPolygon)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, lockedByJobID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func (_m *GeocubeTxBackend) ReadLayout(ctx context.Context, layoutID string) (*geocube.Layout, error) {
	ret := _m.Called(ctx, layoutID)

	var r0 *geocube.Layout
	if rf, ok := ret.Get(0).(func(context.Context, string) *geocube.Layout); ok {
		r0 = rf(ctx, layoutID)
	} else {
		r0 = ret.Get(0).(*geocube.Layout)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, layoutID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func (_m *GeocubeTxBackend) FindDatasets(ctx context.Context, status geocube.DatasetStatus, containerURI, lockedByJobID string, instancesID, recordsID []string, recordTags geocube.Metadata, fromTime, toTime time.Time, geog *proj.GeographicRing, refined *proj.Ring, page, limit int, order bool) ([]*geocube.Dataset, error) {
	ret := _m.Called(ctx, status, containerURI, lockedByJobID, instancesID, recordsID, recordTags, fromTime, toTime, geog, refined, page, limit, order)

	var r0 []*geocube.Dataset
	if rf, ok := ret.Get(0).(func(context.Context, geocube.DatasetStatus, string, string, []string, []string, geocube.Metadata, time.Time, time.Time, *proj.GeographicRing, *proj.Ring, int, int, bool) []*geocube.Dataset); ok {
		r0 = rf(ctx, status, containerURI, lockedByJobID, instancesID, recordsID, recordTags, fromTime, toTime, geog, refined, page, limit, order)
	} else {
		r0 = ret.Get(0).([]*geocube.Dataset)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, geocube.DatasetStatus, string, string, []string, []string, geocube.Metadata, time.Time, time.Time, *proj.GeographicRing, *proj.Ring, int, int, bool) error); ok {
		r1 = rf(ctx, status, containerURI, lockedByJobID, instancesID, recordsID, recordTags, fromTime, toTime, geog, refined, page, limit, order)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func (_m *GeocubeTxBackend) ReleaseDatasets(ctx context.Context, jobID string, flag int) error {
	ret := _m.Called(ctx, jobID, flag)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, int) error); ok {
		r0 = rf(ctx, jobID, flag)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *GeocubeTxBackend) UpdateJob(ctx context.Context, job *geocube.Job) error {
	ret := _m.Called(ctx, job)

	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *geocube.Job) error); ok {
		r1 = rf(ctx, job)
	} else {
		r1 = ret.Error(0)
	}

	return r1
}
