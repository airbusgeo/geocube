package client

import (
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/airbusgeo/geocube/client/go/pb"
)

type Job pb.Job
type ConsolidationParams pb.ConsolidationParams

// IndexDataset indexes a dataset
func (c Client) IndexDataset(uri string, managed bool, containerSubdir, recordID, instanceID string, bands []int64, dformat *pb.DataFormat, realMin, realMax, exponent float64) error {
	dataset := &pb.Dataset{
		RecordId:        recordID,
		InstanceId:      instanceID,
		ContainerSubdir: containerSubdir,
		Bands:           bands,
		Dformat:         dformat,
		RealMinValue:    realMin,
		RealMaxValue:    realMax,
		Exponent:        exponent,
	}
	return c.IndexDatasets(uri, managed, []*pb.Dataset{dataset})
}

// IndexDatasets indexes a batch of datasets
func (c Client) IndexDatasets(uri string, managed bool, datasets []*pb.Dataset) error {
	_, err := c.gcc.IndexDatasets(c.ctx,
		&pb.IndexDatasetsRequest{Container: &pb.Container{
			Uri:      uri,
			Managed:  managed,
			Datasets: datasets}})

	return grpcError(err)
}

// CleanJobs removes the terminated jobs
// nameLike, state : [optional] filter by name or state (DONE/FAILED)
func (c Client) CleanJobs(nameLike, state string) (int32, error) {
	cresp, err := c.gcc.CleanJobs(c.ctx, &pb.CleanJobsRequest{
		NameLike: nameLike,
		State:    state,
	})
	if err != nil {
		return 0, grpcError(err)
	}
	return cresp.GetCount(), nil
}

// ConfigConsolidation configures the parameters associated to this variable
func (c Client) ConfigConsolidation(variableID string, dformat *pb.DataFormat, exponent float64, bandsInterleave bool, compression int, createOverviews bool, downsamplingAlg string, storageClass int) error {
	_, err := c.gcc.ConfigConsolidation(c.ctx, &pb.ConfigConsolidationRequest{
		VariableId: variableID,
		ConsolidationParams: &pb.ConsolidationParams{
			Dformat:         dformat,
			Exponent:        exponent,
			BandsInterleave: bandsInterleave,
			Compression:     pb.ConsolidationParams_Compression(compression),
			CreateOverviews: createOverviews,
			DownsamplingAlg: toResampling(downsamplingAlg),
			StorageClass:    pb.StorageClass(storageClass),
		}})
	return grpcError(err)
}

// GetConsolidationParams read the consolidation parameters associated to this variable
func (c Client) GetConsolidationParams(variableID string) (*ConsolidationParams, error) {
	resp, err := c.gcc.GetConsolidationParams(c.ctx, &pb.GetConsolidationParamsRequest{VariableId: variableID})
	if Code(err) == codes.NotFound {
		return nil, nil
	}
	return (*ConsolidationParams)(resp.ConsolidationParams), grpcError(err)
}

// ConsolidateDatasetsFromRecords starts a consolidation job of the datasets defined by the given parameters
func (c Client) ConsolidateDatasetsFromRecords(name string, instanceID, layoutID string, recordsID []string) (string, error) {
	resp, err := c.gcc.Consolidate(c.ctx,
		&pb.ConsolidateRequest{
			JobName:       name,
			LayoutId:      layoutID,
			InstanceId:    instanceID,
			RecordsLister: &pb.ConsolidateRequest_Records{Records: &pb.RecordList{Ids: recordsID}},
		})

	if err != nil {
		return "", grpcError(err)
	}

	return resp.GetJobId(), nil
}

// ConsolidateDatasetsFromFilters starts a consolidation job of the datasets defined by the given parameters
func (c Client) ConsolidateDatasetsFromFilters(name string, instanceID, layoutID string, tags map[string]string, fromTime, toTime time.Time) (string, error) {
	fromTs := timestamppb.New(fromTime)
	if err := fromTs.CheckValid(); err != nil {
		return "", err
	}
	toTs := timestamppb.New(toTime)
	if err := toTs.CheckValid(); err != nil {
		return "", err
	}
	resp, err := c.gcc.Consolidate(c.ctx,
		&pb.ConsolidateRequest{
			JobName:       name,
			LayoutId:      layoutID,
			InstanceId:    instanceID,
			RecordsLister: &pb.ConsolidateRequest_Filters{Filters: &pb.RecordFilters{Tags: tags, FromTime: fromTs, ToTime: toTs}},
		})

	if err != nil {
		return "", grpcError(err)
	}

	return resp.GetJobId(), nil
}

// ToString returns a string with a representation of the job
func (j *Job) ToString() string {
	return fmt.Sprintf("Job %s:\n"+
		"  Id:              %s\n"+
		"  Type:            %s\n"+
		"  State:           %s\n"+
		"  Active tasks:    %d\n"+
		"  Failed tasks:    %d\n"+
		"  Creation:        %s\n"+
		"  LastUpdate:      %s\n"+
		"  Logs:            \n  %s",
		j.Name, j.Id, j.Type, j.State, j.ActiveTasks, j.FailedTasks, j.CreationTime.AsTime().Format("2 Jan 2006 15:04:05"),
		j.LastUpdateTime.AsTime().Format("2 Jan 2006 15:04:05"), strings.Join(j.Log, "\n  "))
}

// ListJobs returns the jobs with a name like name (or all if name="")
func (c Client) ListJobs(nameLike string) ([]*Job, error) {
	jresp, err := c.gcc.ListJobs(c.ctx, &pb.ListJobsRequest{NameLike: nameLike})
	if err != nil {
		return nil, grpcError(err)
	}
	var jobs []*Job
	for _, j := range jresp.Jobs {
		jobs = append(jobs, (*Job)(j))
	}
	return jobs, nil
}

// GetJob returns the job with the given ID
func (c Client) GetJob(jobID string) (*Job, error) {
	jresp, err := c.gcc.GetJob(c.ctx, &pb.GetJobRequest{Id: jobID})
	if err != nil {
		return nil, grpcError(err)
	}
	return (*Job)(jresp.GetJob()), nil
}

// RetryJob retries the job with the given ID
func (c Client) RetryJob(jobID string, forceAnyState bool) error {
	_, err := c.gcc.RetryJob(c.ctx, &pb.RetryJobRequest{Id: jobID, ForceAnyState: forceAnyState})
	return grpcError(err)
}

// CancelJob retries the job with the given ID
func (c Client) CancelJob(jobID string) error {
	_, err := c.gcc.CancelJob(c.ctx, &pb.CancelJobRequest{Id: jobID})
	return grpcError(err)
}
