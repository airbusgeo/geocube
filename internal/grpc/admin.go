package grpc

import (
	"context"

	"github.com/airbusgeo/geocube/internal/geocube"
	pb "github.com/airbusgeo/geocube/internal/pb"
)

// GeocubeServiceAdmin contains all the admin services
type GeocubeServiceAdmin interface {
	// TidyPending remove from the database the entities that are not linked to any dataset
	TidyPending(ctx context.Context, aois, records, variables, instances, containers, params bool, simulate bool) ([]int64, error)
	// UpdateDatasets given the instance id
	UpdateDatasets(ctx context.Context, simulate bool, instanceID string, RecordIds []string, dmapping geocube.DataMapping) (map[string]int64, error)
	// DeleteDatasets given the instance id
	DeleteDatasets(ctx context.Context, jobName string, instancesID, recordsID []string, executionLevel geocube.ExecutionLevel) (*geocube.Job, error)
}

// ServiceAdmin is the GRPC service
type ServiceAdmin struct {
	gsvca GeocubeServiceAdmin
}

var _ pb.AdminServer = &ServiceAdmin{}

// NewAdmin returns a new GRPC ServiceAdmin connected to an admin Service
func NewAdmin(gsvca GeocubeServiceAdmin) *ServiceAdmin {
	return &ServiceAdmin{gsvca: gsvca}
}

// TidyDB implements AdminServer
func (svc *ServiceAdmin) TidyDB(ctx context.Context, req *pb.TidyDBRequest) (*pb.TidyDBResponse, error) {
	nbs, err := svc.gsvca.TidyPending(ctx, req.GetPendingAOIs(), req.GetPendingRecords(), req.GetPendingVariables(),
		req.GetPendingInstances(), req.GetPendingContainers(), req.GetPendingParams(), req.GetSimulate())
	if err != nil {
		return nil, formatError("backend.%w", err)
	}
	return &pb.TidyDBResponse{
		NbAOIs:       nbs[0],
		NbRecords:    nbs[1],
		NbInstances:  nbs[2],
		NbVariables:  nbs[3],
		NbContainers: nbs[4],
		NbParams:     nbs[5],
	}, nil
}

// UpdateDatasets implements AdminServer
func (svc *ServiceAdmin) UpdateDatasets(ctx context.Context, req *pb.UpdateDatasetsRequest) (*pb.UpdateDatasetsResponse, error) {
	results, err := svc.gsvca.UpdateDatasets(ctx, req.Simulate, req.InstanceId, req.RecordIds,
		geocube.DataMapping{
			DataFormat: *geocube.NewDataFormatFromProtobuf(req.GetDformat()),
			RangeExt:   geocube.Range{Min: req.RealMinValue, Max: req.RealMaxValue},
			Exponent:   req.Exponent})
	if err != nil {
		return nil, formatError("backend.%w", err)
	}
	return &pb.UpdateDatasetsResponse{
		Results: results,
	}, nil
}

// DeleteDatasets implements AdminServer
func (svc *ServiceAdmin) DeleteDatasets(ctx context.Context, req *pb.DeleteDatasetsRequest) (*pb.DeleteDatasetsResponse, error) {
	job, err := svc.gsvca.DeleteDatasets(ctx, req.JobName, req.GetInstanceIds(), req.GetRecordIds(), geocube.ExecutionLevel(req.ExecutionLevel))
	if err != nil {
		return nil, formatError("backend.%w", err)
	}
	jobpb, err := job.ToProtobuf(0, 100)
	if err != nil {
		return nil, formatError("backend.%w", err)
	}
	return &pb.DeleteDatasetsResponse{
		Job: jobpb,
	}, nil
}
