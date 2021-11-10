package grpc

import (
	"bytes"
	"compress/flate"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/airbusgeo/godal"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/google/uuid"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/geocube/internal/log"
	pb "github.com/airbusgeo/geocube/internal/pb"
	internal "github.com/airbusgeo/geocube/internal/svc"
	"github.com/airbusgeo/geocube/internal/utils"
	"github.com/airbusgeo/geocube/internal/utils/affine"
	"github.com/airbusgeo/geocube/internal/utils/grid"
	"github.com/airbusgeo/geocube/internal/utils/proj"
)

const (
	GeocubeServerVersion = "0.2.0"
	StreamTilesBatchSize = 1000
)

// GeocubeService contains all the business services
type GeocubeService interface {
	CreateAOI(ctx context.Context, aoi *geocube.AOI) error
	GetAOI(ctx context.Context, aoiID string) (*geocube.AOI, error)
	CreateRecords(ctx context.Context, records []*geocube.Record) error
	DeleteRecords(ctx context.Context, ids []string) (int64, error)
	ListRecords(ctx context.Context, namelike string, tags geocube.Metadata, fromTime, toTime time.Time, aoi *geocube.AOI, page, limit int, withAOI bool) ([]*geocube.Record, error)
	AddRecordsTags(ctx context.Context, ids []string, tags geocube.Metadata) (int64, error)
	RemoveRecordsTags(ctx context.Context, ids []string, tagsKey []string) (int64, error)

	CreateVariable(ctx context.Context, variable *geocube.Variable) error
	UpdateVariable(ctx context.Context, variableID string, name, unit, description, palette *string, resampling *geocube.Resampling) error
	// Retrieves variable with the first not-empty parameter
	GetVariable(ctx context.Context, variableID, instanceID, variableName string) (*geocube.Variable, error)
	InstantiateVariable(ctx context.Context, variableID string, instance *geocube.VariableInstance) error
	ListVariables(ctx context.Context, namelike string, page, limit int) ([]*geocube.Variable, error)
	UpdateInstance(ctx context.Context, id string, name *string, newMetadata map[string]string, delMetadataKeys []string) error
	// DeleteVariable delete the variable and all its instances iif not used anymore
	DeleteVariable(ctx context.Context, id string) error
	// DeleteInstance delete the instance iif not used anymore
	DeleteInstance(ctx context.Context, id string) error
	CreatePalette(ctx context.Context, palette *geocube.Palette, replaceIfExists bool) error

	// Index datasets that are not fully known. Checks that the container is reachable and get some missing informations.
	IndexExternalDatasets(ctx context.Context, container *geocube.Container, datasets []*geocube.Dataset) error
	ConfigConsolidation(ctx context.Context, variableID string, params geocube.ConsolidationParams) error
	GetConsolidationParams(ctx context.Context, ID string) (*geocube.ConsolidationParams, error)
	ConsolidateFromRecords(ctx context.Context, jobName, instanceID, layoutID string, recordsID []string) (string, error)
	ConsolidateFromFilters(ctx context.Context, jobName, instanceID, layoutID string, tags map[string]string, fromTime, toTime time.Time) (string, error)
	ListJobs(ctx context.Context, nameLike string) ([]*geocube.Job, error)
	GetJob(ctx context.Context, jobID string) (*geocube.Job, error)
	RetryJob(ctx context.Context, jobID string, forceAnyState bool) error
	CancelJob(ctx context.Context, jobID string) error
	CleanJobs(ctx context.Context, nameLike string, state *geocube.JobState) (int, error)

	CreateLayout(ctx context.Context, layout *geocube.Layout) error
	ListLayouts(ctx context.Context, nameLike string) ([]*geocube.Layout, error)
	TileAOI(ctx context.Context, aoi *geocube.AOI, crs string, resolution float32, width, height int32) (<-chan *grid.Cell, error)

	GetXYZTile(ctx context.Context, recordsID []string, instanceID string, a, b, z int) ([]byte, error)
	GetCubeFromRecords(ctx context.Context, recordsID [][]string, instancesID []string, crs *godal.SpatialRef, pixToCRS *affine.Affine, width, height int, format string, headersOnly bool) (internal.CubeInfo, <-chan internal.CubeSlice, error)
	GetCubeFromFilters(ctx context.Context, recordTags geocube.Metadata, fromTime, toTime time.Time, instancesID []string, crs *godal.SpatialRef, pixToCRS *affine.Affine, width, height int, format string, headersOnly bool) (internal.CubeInfo, <-chan internal.CubeSlice, error)
}

// Service is the GRPC service
type Service struct {
	gsvc             GeocubeService
	maxConnectionAge time.Duration
}

type initGetCube struct {
	gids     [][]string
	pixToCRS *affine.Affine
	crs      *godal.SpatialRef
	width    int
	height   int
	deflater *flate.Writer
}

var _ pb.GeocubeServer = &Service{}

// New returns a new GRPC Service connected to a business Service
func New(gsvc GeocubeService, maxConnectionAgeSec int) *Service {
	return &Service{gsvc: gsvc, maxConnectionAge: time.Duration(maxConnectionAgeSec)}
}

func newValidationError(desc string) error {
	return formatError("", geocube.NewValidationError(desc))
}

func formatError(format string, err error) error {
	var gcerr geocube.GeocubeError
	if errors.As(err, &gcerr) {
		var st *status.Status
		message := gcerr.Desc()
		if utils.Temporary(err) {
			message += " (this error may be temporary)"
		}
		switch gcerr.Code() {
		case geocube.EntityValidationError:
			st = status.New(codes.InvalidArgument, gcerr.Desc())

		case geocube.EntityNotFound:
			st = status.New(codes.NotFound, message)
			if tmp, err := st.WithDetails(&errdetails.ResourceInfo{
				ResourceType: gcerr.Detail(geocube.DetailNotFoundEntity),
				ResourceName: gcerr.Detail(geocube.DetailNotFoundID)}); err == nil {
				st = tmp
			}

		case geocube.EntityAlreadyExists:
			st = status.New(codes.AlreadyExists, message)
			if tmp, err := st.WithDetails(&errdetails.ResourceInfo{
				ResourceType: gcerr.Detail(geocube.DetailAlreadyExistsEntity),
				ResourceName: gcerr.Detail(geocube.DetailAlreadyExistsID)}); err == nil {
				st = tmp
			}

		case geocube.DependencyStillExists:
			st = status.New(codes.FailedPrecondition, message)
			if tmp, err := st.WithDetails(&errdetails.ResourceInfo{
				Owner:        gcerr.Detail(geocube.DetailDependencyStillExistsEntity1),
				ResourceType: gcerr.Detail(geocube.DetailDependencyStillExistsEntity2),
				ResourceName: gcerr.Detail(geocube.DetailDependencyStillExistsID)}); err == nil {
				st = tmp
			}

		case geocube.ShouldNeverHappen:
			st = status.New(codes.Unknown, message)

		case geocube.UnhandledEvent:
			st = status.New(codes.FailedPrecondition, message)
		}
		return st.Err()
	}
	if utils.Temporary(err) {
		return status.New(codes.Unavailable, err.Error()).Err()
	}
	return fmt.Errorf(format, err)
}

// CreateAOI creates an aoi
func (svc *Service) CreateAOI(ctx context.Context, req *pb.CreateAOIRequest) (*pb.CreateAOIResponse, error) {
	// Convert pb.AOI to geocube.AOI
	aoi, err := geocube.NewAOIFromProtobuf(req.GetAoi().GetPolygons(), false)
	if err != nil {
		return nil, formatError("", err) // ValidationError
	}

	// Create AOI
	if err := svc.gsvc.CreateAOI(ctx, aoi); err != nil {
		return nil, formatError("backend.%w", err)
	}

	// Format response
	return &pb.CreateAOIResponse{Id: aoi.ID}, nil
}

// GetAOI retrieves an aoi
func (svc *Service) GetAOI(ctx context.Context, req *pb.GetAOIRequest) (*pb.GetAOIResponse, error) {
	// Check uuid
	if _, err := uuid.Parse(req.Id); err != nil {
		return nil, newValidationError("Invalid AOI.uuid " + req.Id + ": " + err.Error())
	}

	// Get AOI
	aoi, err := svc.gsvc.GetAOI(ctx, req.Id)
	if err != nil {
		return nil, formatError("backend.%w", err)
	}

	// Format response
	return &pb.GetAOIResponse{Aoi: aoi.ToProtobuf()}, nil
}

// CreateRecords creates a batch of records with a common aoi
func (svc *Service) CreateRecords(ctx context.Context, req *pb.CreateRecordsRequest) (*pb.CreateRecordsResponse, error) {
	// Convert []pb.NewRecord to records
	records := make([]*geocube.Record, len(req.GetRecords()))
	sids := make([]string, len(req.GetRecords()))
	for i, record := range req.GetRecords() {
		r, err := geocube.NewRecordFromProtobuf(record)
		if err != nil {
			return nil, formatError("", err) // ValidationError
		}
		records[i] = r
		sids[i] = r.ID
	}

	// Create records
	if err := svc.gsvc.CreateRecords(ctx, records); err != nil {
		return nil, formatError("backend.%w", err)
	}

	// Format response
	return &pb.CreateRecordsResponse{Ids: sids}, nil
}

// DeleteRecords deletes a batch of records iif no dataset has a reference on
func (svc *Service) DeleteRecords(ctx context.Context, req *pb.DeleteRecordsRequest) (*pb.DeleteRecordsResponse, error) {
	// Test whether pb.ids are uuid
	for _, id := range req.Ids {
		if _, err := uuid.Parse(id); err != nil {
			return nil, newValidationError("Invalid uuid: " + err.Error())
		}
	}

	nb, err := svc.gsvc.DeleteRecords(ctx, req.Ids)
	if err != nil {
		return nil, formatError("backend.%w", err)
	}
	return &pb.DeleteRecordsResponse{Nb: nb}, nil
}

func timeFromTimestamp(ts *timestamp.Timestamp) time.Time {
	if ts.CheckValid() != nil {
		return time.Time{}
	}
	return ts.AsTime()
}

// ListRecords retrieves all the records with respect to the request
func (svc *Service) ListRecords(req *pb.ListRecordsRequest, stream pb.Geocube_ListRecordsServer) error {
	// Convert request
	ctx := stream.Context()
	limit := int(req.GetLimit())

	// Convert times
	fromTime := timeFromTimestamp(req.GetFromTime())
	toTime := timeFromTimestamp(req.GetToTime())

	// Convert pb.AOI to geocube.AOI
	aoi, err := geocube.NewAOIFromProtobuf(req.GetAoi().GetPolygons(), true)
	if err != nil {
		return formatError("", err) // ValidationError
	}

	// List records
	records, err := svc.gsvc.ListRecords(ctx, req.GetName(), req.GetTags(), fromTime, toTime, aoi, int(req.GetPage()), limit, req.WithAoi)
	if err != nil {
		return formatError("backend.%w", err)
	}

	// Format response
	for _, record := range records {
		if err := stream.Send(&pb.ListRecordsResponseItem{Record: record.ToProtobuf(req.WithAoi)}); err != nil {
			return formatError("backend.ListRecords.send: %w", err)
		}
	}
	return nil
}

// AddRecordsTags adds tags on list of records
func (svc *Service) AddRecordsTags(ctx context.Context, req *pb.AddRecordsTagsRequest) (*pb.AddRecordsTagsResponse, error) {
	for _, id := range req.Ids {
		if _, err := uuid.Parse(id); err != nil {
			return nil, newValidationError("Invalid uuid: " + err.Error())
		}
	}

	nb, err := svc.gsvc.AddRecordsTags(ctx, req.Ids, req.Tags)
	if err != nil {
		return nil, formatError("backend.%w", err)
	}
	return &pb.AddRecordsTagsResponse{Nb: nb}, nil
}

// AddRecordsTags removes tags on list of records
func (svc *Service) RemoveRecordsTags(ctx context.Context, req *pb.RemoveRecordsTagsRequest) (*pb.RemoveRecordsTagsResponse, error) {
	for _, id := range req.Ids {
		if _, err := uuid.Parse(id); err != nil {
			return nil, newValidationError("Invalid uuid: " + err.Error())
		}
	}

	nb, err := svc.gsvc.RemoveRecordsTags(ctx, req.Ids, req.TagsKey)
	if err != nil {
		return nil, formatError("backend.%w", err)
	}
	return &pb.RemoveRecordsTagsResponse{Nb: nb}, nil
}

// CreateVariable creates the definition of a variable
func (svc *Service) CreateVariable(ctx context.Context, v *pb.CreateVariableRequest) (*pb.CreateVariableResponse, error) {
	// Convert pb.Variable to geocube.Variable
	variable, err := geocube.NewVariableFromProtobuf(v.GetVariable())
	if err != nil {
		return nil, formatError("", err) // ValidationError
	}

	// Create variable
	if err = svc.gsvc.CreateVariable(ctx, variable); err != nil {
		return nil, formatError("backend.%w", err)
	}

	// Format response
	return &pb.CreateVariableResponse{Id: variable.ID}, err
}

// InstantiateVariable creates an instance of the variable
func (svc *Service) InstantiateVariable(ctx context.Context, req *pb.InstantiateVariableRequest) (*pb.InstantiateVariableResponse, error) {
	// Convert pb.VariableInstance to geocube.VariableInstance
	instance, err := geocube.NewInstance(req.GetInstanceName(), req.GetInstanceMetadata())
	if err != nil {
		return nil, formatError("backend.%w", err)
	}

	// Test whether pb.id is uuid
	if _, err := uuid.Parse(req.GetVariableId()); err != nil {
		return nil, newValidationError("Invalid uuid: " + err.Error())
	}

	// Create instance
	if err = svc.gsvc.InstantiateVariable(ctx, req.GetVariableId(), instance); err != nil {
		return nil, formatError("backend.%w", err)
	}

	// Format response
	return &pb.InstantiateVariableResponse{Instance: &pb.Instance{Id: instance.ID, Name: instance.Name, Metadata: instance.Metadata}}, nil
}

// GetVariable retrieves the definition of the variable with the given id
func (svc *Service) GetVariable(ctx context.Context, req *pb.GetVariableRequest) (*pb.GetVariableResponse, error) {
	var variable *geocube.Variable
	var err error

	// Validate
	if req.GetId() != "" {
		if _, err := uuid.Parse(req.GetId()); err != nil {
			return nil, newValidationError("Invalid uuid: " + err.Error())
		}
	} else if req.GetInstanceId() != "" {
		if _, err := uuid.Parse(req.GetInstanceId()); err != nil {
			return nil, newValidationError("Invalid uuid: " + err.Error())
		}
	}

	// Get Variable from id
	variable, err = svc.gsvc.GetVariable(ctx, req.GetId(), req.GetInstanceId(), req.GetName())
	if err != nil {
		return nil, formatError("backend.%w", err)
	}

	// Format response
	return &pb.GetVariableResponse{
		Variable: variable.ToProtobuf(),
	}, nil
}

// ListVariables lists variables with search parameters
func (svc *Service) ListVariables(req *pb.ListVariablesRequest, stream pb.Geocube_ListVariablesServer) error {
	// Convert request
	ctx := stream.Context()
	limit := int(req.GetLimit())

	// List variables
	variables, err := svc.gsvc.ListVariables(ctx, req.GetName(), int(req.GetPage()), limit)
	if err != nil {
		return formatError("backend.ListVariables.%w", err)
	}

	// Format response
	for _, variable := range variables {
		if err := stream.Send(&pb.ListVariablesResponseItem{Variable: variable.ToProtobuf()}); err != nil {
			return formatError("backend.ListVariables: %w", err)
		}
	}
	return nil
}

func optionalResampling(alg pb.Resampling) *geocube.Resampling {
	if alg != pb.Resampling_UNDEFINED {
		r := geocube.Resampling(alg)
		return &r
	}
	return nil
}

func optionalString(value *wrappers.StringValue) *string {
	if value == nil {
		return nil
	}
	return &value.Value
}

// UpdateVariable updates a variable
func (svc *Service) UpdateVariable(ctx context.Context, req *pb.UpdateVariableRequest) (*pb.UpdateVariableResponse, error) {
	// Test whether pb.id is uuid
	if _, err := uuid.Parse(req.GetId()); err != nil {
		return nil, newValidationError("Invalid uuid: " + err.Error())
	}

	// Update variable
	if err := svc.gsvc.UpdateVariable(ctx, req.GetId(),
		optionalString(req.GetName()), optionalString(req.GetUnit()), optionalString(req.GetDescription()),
		optionalString(req.GetPalette()), optionalResampling(req.GetResamplingAlg())); err != nil {
		return nil, formatError("backend.%w", err)
	}

	// Format response
	return &pb.UpdateVariableResponse{}, nil
}

// UpdateInstance updates the name and metadata of the instance with the given id
func (svc *Service) UpdateInstance(ctx context.Context, req *pb.UpdateInstanceRequest) (*pb.UpdateInstanceResponse, error) {
	// Test whether pb.id is uuid
	if _, err := uuid.Parse(req.GetId()); err != nil {
		return nil, newValidationError("Invalid uuid: " + err.Error())
	}

	// Update instance
	if err := svc.gsvc.UpdateInstance(ctx, req.GetId(), optionalString(req.GetName()), req.GetAddMetadata(), req.GetDelMetadataKeys()); err != nil {
		return nil, formatError("backend.%w", err)
	}

	// Format response
	return &pb.UpdateInstanceResponse{}, nil
}

// DeleteVariable deletes the variable and all its instances iif not used anymore
func (svc *Service) DeleteVariable(ctx context.Context, req *pb.DeleteVariableRequest) (*pb.DeleteVariableResponse, error) {
	// Test whether pb.id is uuid
	if _, err := uuid.Parse(req.GetId()); err != nil {
		return nil, newValidationError("Invalid uuid: " + err.Error())
	}

	// Delete variable
	if err := svc.gsvc.DeleteVariable(ctx, req.GetId()); err != nil {
		return nil, formatError("backend.%w", err)
	}

	// Format response
	return &pb.DeleteVariableResponse{}, nil
}

// DeleteInstance deletes the instance if not used anymore
func (svc *Service) DeleteInstance(ctx context.Context, req *pb.DeleteInstanceRequest) (*pb.DeleteInstanceResponse, error) {
	// Test whether pb.id is uuid
	if _, err := uuid.Parse(req.GetId()); err != nil {
		return nil, newValidationError("Invalid uuid: " + err.Error())
	}

	// Delete instance
	if err := svc.gsvc.DeleteInstance(ctx, req.GetId()); err != nil {
		return nil, formatError("backend.%w", err)
	}

	// Format response
	return &pb.DeleteInstanceResponse{}, nil
}

// CreatePalette creates a palette
func (svc *Service) CreatePalette(ctx context.Context, req *pb.CreatePaletteRequest) (*pb.CreatePaletteResponse, error) {
	p, err := geocube.NewPaletteFromPb(req.Palette)
	if err != nil {
		return nil, formatError("", err) // ValidationError
	}

	if err := svc.gsvc.CreatePalette(ctx, &p, req.Replace); err != nil {
		return nil, formatError("backend.%w", err)
	}

	// Format response
	return &pb.CreatePaletteResponse{}, nil
}

// IndexDatasets adds datasets in database
func (svc *Service) IndexDatasets(ctx context.Context, req *pb.IndexDatasetsRequest) (*pb.IndexDatasetsResponse, error) {
	// Convert pb.Container to container
	container, err := geocube.NewContainerFromProtobuf(req.GetContainer())
	if err != nil {
		return nil, formatError("", err) // ValidationError
	}

	// Convert []pb.NewDataset to datasets
	datasets := make([]*geocube.Dataset, len(req.GetContainer().GetDatasets()))
	for i, dataset := range req.GetContainer().GetDatasets() {
		d, err := geocube.NewDatasetFromProtobuf(dataset, container.URI)
		if err != nil {
			return nil, formatError("", err) // ValidationError
		}
		datasets[i] = d
	}

	// Create datasets
	if err := svc.gsvc.IndexExternalDatasets(ctx, container, datasets); err != nil {
		return nil, formatError("backend.%w", err)
	}

	return &pb.IndexDatasetsResponse{}, nil
}

// IngestDatasets TODO
func (svc *Service) IngestDatasets(req *pb.IngestDatasetsRequest, stream pb.Geocube_IngestDatasetsServer) error {
	return fmt.Errorf("not implemented")
}

// GetConsolidationParams reads the configuration parameters associated to a variable
func (svc *Service) GetConsolidationParams(ctx context.Context, req *pb.GetConsolidationParamsRequest) (*pb.GetConsolidationParamsResponse, error) {
	// Test whether pb.Id is uuid
	if _, err := uuid.Parse(req.VariableId); err != nil {
		return nil, newValidationError("Invalid uuid " + req.VariableId + ": " + err.Error())
	}
	// Read the consolidation parameters
	params, err := svc.gsvc.GetConsolidationParams(ctx, req.GetVariableId())
	if err != nil {
		return nil, formatError("backend.%w", err)
	}
	// Convert to pb.ConsolidationParameters
	return &pb.GetConsolidationParamsResponse{ConsolidationParams: params.ToProtobuf()}, nil
}

// ConfigConsolidation configures the consolidation parameters associated to a variable
func (svc *Service) ConfigConsolidation(ctx context.Context, req *pb.ConfigConsolidationRequest) (*pb.ConfigConsolidationResponse, error) {
	// Test whether pb.variableId is uuid
	if _, err := uuid.Parse(req.GetVariableId()); err != nil {
		return nil, newValidationError("Invalid uuid " + req.GetVariableId() + ": " + err.Error())
	}
	// Convert pb.ConfigConsolidation to ConsolidationParameters
	params, err := geocube.NewConsolidationParamsFromProtobuf(req.GetConsolidationParams())
	if err != nil {
		return nil, formatError("", err) // ValidationError
	}

	// Set the consolidation parameters
	if err = svc.gsvc.ConfigConsolidation(ctx, req.GetVariableId(), *params); err != nil {
		return nil, formatError("backend.%w", err)
	}
	return &pb.ConfigConsolidationResponse{}, nil
}

// Consolidate starts a consolidation job
func (svc *Service) Consolidate(ctx context.Context, req *pb.ConsolidateRequest) (*pb.ConsolidateResponse, error) {
	// Test whether ids are uuid
	for _, id := range req.GetRecords().GetIds() {
		if _, err := uuid.Parse(id); err != nil {
			return nil, newValidationError("Invalid Record.uuid " + id + ": " + err.Error())
		}
	}
	if _, err := uuid.Parse(req.GetInstanceId()); err != nil {
		return nil, newValidationError("Invalid Instance.uuid " + req.GetInstanceId() + ": " + err.Error())
	}
	if _, err := uuid.Parse(req.GetLayoutId()); err != nil {
		return nil, newValidationError("Invalid Layout.uuid " + req.GetLayoutId() + ": " + err.Error())
	}

	// Consolidate
	var (
		jobID string
		err   error
	)
	if req.GetRecords() == nil {
		filters := req.GetFilters()
		// Convert times
		fromTime := timeFromTimestamp(filters.GetFromTime())
		toTime := timeFromTimestamp(filters.GetToTime())
		jobID, err = svc.gsvc.ConsolidateFromFilters(ctx, req.GetJobName(), req.GetInstanceId(), req.GetLayoutId(), filters.GetTags(), fromTime, toTime)
	} else {
		if len(req.GetRecords().GetIds()) == 0 {
			return &pb.ConsolidateResponse{}, newValidationError("At least one record must be provided")
		}
		jobID, err = svc.gsvc.ConsolidateFromRecords(ctx, req.GetJobName(), req.GetInstanceId(), req.GetLayoutId(), req.GetRecords().GetIds())
	}
	if err != nil {
		return nil, formatError("backend.%w", err)
	}

	return &pb.ConsolidateResponse{JobId: jobID}, nil
}

// CleanJobs remove all the finished job from the database
func (svc *Service) CleanJobs(ctx context.Context, req *pb.CleanJobsRequest) (*pb.CleanJobsResponse, error) {
	// Parse jobstate
	var jobState *geocube.JobState
	if req.GetState() != "" {
		state, err := geocube.JobStateString(req.State)
		if err != nil {
			return nil, newValidationError("Invalid state: " + err.Error())
		}
		if state != geocube.JobStateDONE && state != geocube.JobStateFAILED {
			return nil, newValidationError("Invalid state: must be one of " + strings.Join([]string{geocube.JobStateDONE.String(), geocube.JobStateFAILED.String()}, ", "))
		}
		jobState = &state
	}

	count, err := svc.gsvc.CleanJobs(ctx, req.NameLike, jobState)
	if err != nil {
		return nil, formatError("backend.%w", err)
	}
	return &pb.CleanJobsResponse{Count: int32(count)}, nil
}

// ListJobs list job with name like nameLike
func (svc *Service) ListJobs(ctx context.Context, req *pb.ListJobsRequest) (*pb.ListJobsResponse, error) {
	// List jobs
	jobs, err := svc.gsvc.ListJobs(ctx, req.GetNameLike())
	if err != nil {
		return nil, formatError("backend.%w", err)
	}

	// Format response
	resp := pb.ListJobsResponse{}
	for _, job := range jobs {
		pbjob, err := job.ToProtobuf()
		if err != nil {
			return nil, formatError("toprotobuf: %w", err)
		}
		resp.Jobs = append(resp.Jobs, pbjob)
	}

	return &resp, nil
}

// GetJob retrieves a job
func (svc *Service) GetJob(ctx context.Context, req *pb.GetJobRequest) (*pb.GetJobResponse, error) {
	// Convert request
	if _, err := uuid.Parse(req.GetId()); err != nil {
		return nil, newValidationError("Invalid uuid: " + err.Error())
	}

	// Get Job
	job, err := svc.gsvc.GetJob(ctx, req.GetId())
	if err != nil {
		return nil, formatError("backend.%w", err)
	}

	// Format response
	pbjob, err := job.ToProtobuf()
	if err != nil {
		return nil, formatError("toprotobuf: %w", err)
	}

	return &pb.GetJobResponse{Job: pbjob}, nil
}

// RetryJob retries a failed job
func (svc *Service) RetryJob(ctx context.Context, req *pb.RetryJobRequest) (*pb.RetryJobResponse, error) {
	// Convert request
	if _, err := uuid.Parse(req.GetId()); err != nil {
		return nil, newValidationError("Invalid uuid: " + err.Error())
	}

	// Retry Job
	if err := svc.gsvc.RetryJob(ctx, req.GetId(), req.GetForceAnyState()); err != nil {
		return nil, formatError("backend.%w", err)
	}

	return &pb.RetryJobResponse{}, nil
}

// CancelJob cancels a job
func (svc *Service) CancelJob(ctx context.Context, req *pb.CancelJobRequest) (*pb.CancelJobResponse, error) {
	// Convert request
	if _, err := uuid.Parse(req.GetId()); err != nil {
		return nil, newValidationError("Invalid uuid: " + err.Error())
	}

	// Retry Job
	if err := svc.gsvc.CancelJob(ctx, req.GetId()); err != nil {
		return nil, formatError("backend.CancelJob.%w", err)
	}

	return &pb.CancelJobResponse{}, nil
}

func (svc *Service) prepareGetCube(req *pb.GetCubeRequest) (*initGetCube, error) {

	// Validate
	if len(req.GetInstancesId()) == 0 {
		return nil, newValidationError("At least one instance must be provided")
	}
	// Test whether ids are uuid and convert to [][]string
	var gids [][]string
	for _, id := range req.GetRecords().GetIds() {
		gids = append(gids, []string{id})
		if _, err := uuid.Parse(id); err != nil {
			return nil, newValidationError("Invalid Record.uuid " + id + ": " + err.Error())
		}
	}
	for _, records := range req.GetGrecords().GetRecords() {
		ids := make([]string, len(records.GetIds()))
		for i, id := range records.GetIds() {
			ids[i] = id
			if _, err := uuid.Parse(id); err != nil {
				return nil, newValidationError("Invalid Record.uuid " + id + ": " + err.Error())
			}
		}
		gids = append(gids, ids)
	}
	for _, id := range req.GetInstancesId() {
		if _, err := uuid.Parse(id); err != nil {
			return nil, newValidationError("Invalid Instance.uuid " + id + ": " + err.Error())
		}
	}

	// Get the transform
	t := req.GetPixToCrs()
	pixToCRS := affine.NewAffine(t.GetA(), t.GetB(), t.GetC(), t.GetD(), t.GetE(), t.GetF())
	if !pixToCRS.IsInvertible() {
		return nil, newValidationError("Invalid pixToCRS transform: not invertible")
	}

	// Get the CRS
	crs, _, err := proj.CRSFromUserInput(req.GetCrs())
	if err != nil {
		return nil, newValidationError(fmt.Sprintf("Invalid crs: %s (%v)", req.GetCrs(), err))
	}

	// Get the shape
	width, height := int(req.GetSize().GetWidth()), int(req.GetSize().GetHeight())
	if width <= 0 || height <= 0 {
		return nil, newValidationError(fmt.Sprintf("Invalid shape: %dx%d", width, height))
	}

	// Get the level of compression
	deflater, err := flate.NewWriter(nil, int(req.GetCompressionLevel()))
	if err != nil {
		return nil, newValidationError(err.Error())
	}
	init := initGetCube{
		gids:     gids,
		pixToCRS: pixToCRS,
		crs:      crs,
		width:    width,
		height:   height,
		deflater: deflater,
	}
	return &init, nil
}

// GetCube retrieves, rescale and reproject datasets and serves them as a cube
func (svc *Service) GetCube(req *pb.GetCubeRequest, stream pb.Geocube_GetCubeServer) error {
	start := time.Now()

	ctx, cancel := context.WithTimeout(stream.Context(), svc.maxConnectionAge*time.Second)
	defer func() {
		cancel()
	}()

	initCube, err := svc.prepareGetCube(req)
	if err != nil {
		return err
	}
	defer initCube.crs.Close()

	var compressed bytes.Buffer

	// Get the cube
	var slicesQueue <-chan internal.CubeSlice
	var info internal.CubeInfo
	if req.GetRecords() == nil && req.GetGrecords() == nil {
		filters := req.GetFilters()
		// Convert times
		fromTime := timeFromTimestamp(filters.GetFromTime())
		toTime := timeFromTimestamp(filters.GetToTime())
		info, slicesQueue, err = svc.gsvc.GetCubeFromFilters(ctx,
			filters.GetTags(),
			fromTime,
			toTime,
			req.GetInstancesId(),
			initCube.crs,
			initCube.pixToCRS,
			initCube.width,
			initCube.height,
			req.Format.String(),
			req.HeadersOnly)
		if err != nil {
			return formatError("backend.%w", err)
		}
	} else {
		if len(initCube.gids) == 0 || len(initCube.gids[0]) == 0 {
			return newValidationError("At least one record must be provided")
		}

		info, slicesQueue, err = svc.gsvc.GetCubeFromRecords(ctx,
			initCube.gids,
			req.InstancesId,
			initCube.crs,
			initCube.pixToCRS,
			initCube.width,
			initCube.height,
			req.Format.String(),
			req.HeadersOnly)
		if err != nil {
			return formatError("backend.%w", err)
		}
	}

	// Return header
	if err := stream.Send(&pb.GetCubeResponse{Response: &pb.GetCubeResponse_GlobalHeader{GlobalHeader: &pb.GetCubeResponseHeader{
		Count:      int64(info.NbImages),
		NbDatasets: int64(info.NbDatasets),
	}}}); err != nil {
		return formatError("backend.GetCube.%w", err)
	}

	if req.GetHeadersOnly() {
		log.Logger(ctx).Sugar().Infof("GetCubeHeader : %d images from %d datasets (%v)\n", info.NbImages, info.NbDatasets, time.Since(start))
	} else {
		log.Logger(ctx).Sugar().Infof("GetCube : %d images from %d datasets (%v)\n", info.NbImages, info.NbDatasets, time.Since(start))
	}

	// Start the compression routine
	compressedSlicesQueue := make(chan internal.CubeSlice)
	go compressSlicesQueue(slicesQueue, compressedSlicesQueue, initCube.deflater, compressed)

	// If context close, compressedSlicesQueue is automatically closed
	n := 1
	for res := range compressedSlicesQueue {
		// Create the header
		internalsMeta := make([]*pb.InternalMeta, len(res.DatasetsMeta.Internals))
		refDf := &pb.DataFormat{Dtype: pb.DataFormat_Dtype(res.DatasetsMeta.RefDataMapping.DType),
			NoData:   res.DatasetsMeta.RefDataMapping.NoData,
			MinValue: res.DatasetsMeta.RefDataMapping.Range.Min,
			MaxValue: res.DatasetsMeta.RefDataMapping.Range.Max}
		datasetMeta := &pb.DatasetMeta{RefDformat: refDf,
			ResamplingAlg: pb.Resampling(res.DatasetsMeta.Resampling),
			InternalsMeta: internalsMeta}
		header := &pb.ImageHeader{Records: make([]*pb.Record, len(res.Records)),
			DatasetsMeta: datasetMeta}

		//populate the datasetMeta part of the header
		for i, r := range res.Records {
			header.Records[i] = r.ToProtobuf(false)
		}

		for i, d := range res.DatasetsMeta.Internals {
			header.DatasetsMeta.InternalsMeta[i] = d.ToProtobuf()
		}
		var chunks []*pb.GetCubeResponse
		if res.Err != nil {
			// Only send a header with the error
			header.Error = formatError("backend.GetCube.%w", res.Err).Error()
			chunks = []*pb.GetCubeResponse{{Response: &pb.GetCubeResponse_Header{Header: header}}}
		} else {
			// Send the image in chunks
			chunks = divideInChunks(header, res.Image)
		}
		getCubeLog(ctx, res, header, req.GetHeadersOnly(), n)
		n++

		// Send chunks
		for _, c := range chunks {
			if err := stream.Send(c); err != nil {
				return formatError("backend.GetCube.%w", err)
			}
		}
	}
	return ctx.Err()
}

func getCubeLog(ctx context.Context, slice internal.CubeSlice, header *pb.ImageHeader, headerOnly bool, n int) {
	if header.Error != "" {
		log.Logger(ctx).Sugar().Debugf("stream image %d : %s\n", n, header.Error)
	} else if !headerOnly {
		metadata := ""
		for k, v := range slice.Metadata {
			metadata += fmt.Sprintf(" [%s: %s]", k, v)
		}

		shape := header.Shape
		log.Logger(ctx).Sugar().Debugf("stream image %d %dx%dx%d %dbytes in %d parts %s\n", n, shape.Dim1, shape.Dim2, shape.Dim3, header.Size, header.NbParts, metadata)
	}
}

func compressSlicesQueue(sliceQueue <-chan internal.CubeSlice, compressedSliceQueue chan<- internal.CubeSlice, deflater *flate.Writer, compressed bytes.Buffer) {
	defer func() {
		close(compressedSliceQueue)
	}()
	for res := range sliceQueue {
		// Get image
		if res.Image != nil && res.Image.Bytes != nil {
			start := time.Now()
			// Compress image
			compressed.Reset()
			deflater.Reset(&compressed)
			if _, err := deflater.Write(res.Image.Bytes); err != nil {
				res.Err = err
			} else if err := deflater.Close(); err != nil {
				res.Err = err
			} else {
				res.Image.Bytes = make([]byte, compressed.Len())
				copy(res.Image.Bytes, compressed.Bytes())
			}
			res.Metadata["Compression"] = fmt.Sprintf("%v", time.Since(start))
		}
		compressedSliceQueue <- res
	}
}

func divideInChunks(header *pb.ImageHeader, image *geocube.Bitmap) []*pb.GetCubeResponse {
	chunkSize := 64 * 1024 // 1Mo/4Mo maximum

	chunks := []*pb.GetCubeResponse{{Response: &pb.GetCubeResponse_Header{Header: header}}}

	// Split the image into chunks
	nbParts := (len(image.Bytes) + chunkSize - 1) / chunkSize
	order := pb.ByteOrder_LittleEndian
	if image.ByteOrder == binary.BigEndian {
		order = pb.ByteOrder_BigEndian
	}

	reader := bytes.NewBuffer(image.Bytes)

	// Image header
	header.Shape = &pb.Shape{Dim1: int32(image.Bands),
		Dim2: int32(image.SizeX()),
		Dim3: int32(image.SizeY())}
	header.Dtype = pb.DataFormat_Dtype(image.DType)
	header.NbParts = int32(nbParts)
	header.Size = int64(len(image.Bytes))
	header.Order = order
	header.Data = reader.Next(chunkSize)
	// header.DatasetMeta = &pb.DatasetMeta{}

	// Image chunks
	for part := 1; part < nbParts; part++ {
		chunks = append(chunks, &pb.GetCubeResponse{
			Response: &pb.GetCubeResponse_Chunk{
				Chunk: &pb.ImageChunk{
					Part: int32(part),
					Data: reader.Next(chunkSize),
				},
			},
		})
	}

	return chunks
}

// GetXYZTile TODO
func (svc *Service) GetXYZTile(ctx context.Context, req *pb.GetTileRequest) (*pb.GetTileResponse, error) {
	var err error

	// Test whether ids are uuid
	if _, err = uuid.Parse(req.GetInstanceId()); err != nil {
		return nil, newValidationError("Invalid Instance.uuid " + req.GetInstanceId() + ": " + err.Error())
	}

	var image []byte
	if records := req.GetRecords(); records != nil {
		if len(req.GetRecords().GetIds()) == 0 {
			return nil, newValidationError("At least one record must be provided")
		}
		for _, id := range records.GetIds() {
			if _, err := uuid.Parse(id); err != nil {
				return nil, newValidationError("Invalid Record.uuid " + id + ": " + err.Error())
			}
		}

		// Get Tile
		if image, err = svc.gsvc.GetXYZTile(ctx, records.GetIds(), req.GetInstanceId(), int(req.GetX()), int(req.GetY()), int(req.GetZ())); err != nil {
			return nil, formatError("backend.%w", err)
		}
	} else {
		return nil, fmt.Errorf("TODO/Not implemented")
	}

	// Format response
	return &pb.GetTileResponse{Image: &pb.ImageFile{Data: image}}, nil
}

// CreateLayout creates a layout
func (svc *Service) CreateLayout(ctx context.Context, req *pb.CreateLayoutRequest) (*pb.CreateLayoutResponse, error) {
	// Convert pb.Layout to geocube.Layout
	layout, err := geocube.NewLayoutFromProtobuf(req.GetLayout())
	if err != nil {
		return nil, formatError("", err) // ValidationError
	}

	// Create layout
	if err = svc.gsvc.CreateLayout(ctx, layout); err != nil {
		return nil, formatError("backend.%w", err)
	}

	// Format response
	return &pb.CreateLayoutResponse{Id: layout.ID}, nil
}

// ListLayouts lists layouts with name like nameLike
func (svc *Service) ListLayouts(ctx context.Context, req *pb.ListLayoutsRequest) (*pb.ListLayoutsResponse, error) {
	// List layouts
	layouts, err := svc.gsvc.ListLayouts(ctx, req.GetNameLike())
	if err != nil {
		return nil, formatError("backend.%w", err)
	}

	// Format response
	resp := pb.ListLayoutsResponse{}
	for _, layout := range layouts {
		resp.Layouts = append(resp.Layouts, layout.ToProtobuf())
	}

	return &resp, nil
}

// TileAOI creates tiles from an aoi
func (svc *Service) TileAOI(req *pb.TileAOIRequest, stream pb.Geocube_TileAOIServer) error {
	ctx := stream.Context()

	// Convert pb.AOI to geocube.AOI
	aoi, err := geocube.NewAOIFromProtobuf(req.GetAoi().GetPolygons(), false)
	if err != nil {
		return formatError("", err) // ValidationError
	}

	// Tile AOI
	cells, err := svc.gsvc.TileAOI(ctx, aoi, req.Crs, req.Resolution, req.GetSizePx().GetWidth(), req.GetSizePx().GetHeight())
	if err != nil {
		return formatError("backend.%w", err)
	}

	// Format response
	tiles := make([]*pb.Tile, 0, StreamTilesBatchSize)
	for c := range cells {
		crs, err := c.CRS.WKT()
		if err != nil {
			return formatError("backend.%w", err)
		}
		tiles = append(tiles, &pb.Tile{
			Transform: &pb.GeoTransform{
				A: c.PixelToCRS[0],
				B: c.PixelToCRS[1],
				C: c.PixelToCRS[2],
				D: c.PixelToCRS[3],
				E: c.PixelToCRS[4],
				F: c.PixelToCRS[5],
			},
			SizePx: &pb.Size{
				Width:  int32(c.SizeX),
				Height: int32(c.SizeY),
			},
			Crs: crs,
		})

		// Send tiles by batches
		if len(tiles) == StreamTilesBatchSize {
			if err := stream.Send(&pb.TileAOIResponse{Tiles: tiles}); err != nil {
				return formatError("backend.TileAOI.send: %w", err)
			}
			tiles = tiles[:0]
		}
	}

	// Send remaining
	if err := stream.Send(&pb.TileAOIResponse{Tiles: tiles}); err != nil {
		return formatError("backend.TileAOI.send: %w", err)
	}

	return nil
}

// Version returns version of the geocube
func (svc *Service) Version(ctx context.Context, req *pb.GetVersionRequest) (*pb.GetVersionResponse, error) {
	return &pb.GetVersionResponse{Version: GeocubeServerVersion}, nil
}
