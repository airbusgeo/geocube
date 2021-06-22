package client

import (
	"fmt"
	"io"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/airbusgeo/geocube/client/go/pb"
)

type AOI [][][][2]float64

type Record struct {
	ID    string
	Name  string
	Time  time.Time
	Tags  map[string]string
	AOIID string
	AOI   *AOI
}

func recordFromPb(pbrec *pb.Record) *Record {
	if pbrec == nil {
		return nil
	}
	return &Record{
		ID:    pbrec.Id,
		Name:  pbrec.Name,
		Time:  pbrec.Time.AsTime(),
		Tags:  pbrec.Tags,
		AOIID: pbrec.AoiId,
		AOI:   aoiFromPb(pbrec.Aoi),
	}
}

// ToString returns a string representation of a Record
func (r *Record) ToString() string {
	s := fmt.Sprintf("Record %s:\n"+
		"  Id:              %s\n"+
		"  Time:            %s\n"+
		"  AOI ID:          %s\n"+
		"  Tags:            ", r.Name, r.ID, r.Time, r.AOIID)
	appendDict(r.Tags, &s)
	if r.AOI != nil {
		g, _ := GeometryFromAOI(*r.AOI).MarshalJSON()
		s += fmt.Sprintf("  AOI:             %s\n", g)
	}

	return s
}

// CreateAOI creates an aoi from a geometry
// If error EntityAlreadyExists, returns the id of the existing one and the error.
func (c Client) CreateAOI(g AOI) (string, error) {
	resp, err := c.gcc.CreateAOI(c.ctx, &pb.CreateAOIRequest{Aoi: pbFromAOI(g)})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.AlreadyExists {
			for _, detail := range s.Details() {
				switch t := detail.(type) {
				case *errdetails.ResourceInfo:
					return t.GetResourceName(), grpcError(err)
				}
			}
		}
		return "", grpcError(err)
	}
	return resp.GetId(), nil
}

// GetAOI retrieves an AOI from a aoi_id
func (c Client) GetAOI(aoiID string) (AOI, error) {
	resp, err := c.gcc.GetAOI(c.ctx, &pb.GetAOIRequest{Id: aoiID})
	if err != nil {
		return AOI{}, grpcError(err)
	}
	return *aoiFromPb(resp.Aoi), nil
}

// CreateRecords creates a batch of records with an aoi, one record for each time in times
func (c Client) CreateRecords(name, aoiID string, times []time.Time, tags map[string]string) ([]string, error) {
	records := make([]*pb.NewRecord, len(times))
	for i, t := range times {
		ts := timestamppb.New(t)
		if err := ts.CheckValid(); err != nil {
			return nil, err
		}
		records[i] = &pb.NewRecord{
			Name:  name,
			Time:  ts,
			Tags:  tags,
			AoiId: aoiID}
	}

	resp, err := c.gcc.CreateRecords(c.ctx, &pb.CreateRecordsRequest{Records: records})
	if err != nil {
		return nil, grpcError(err)
	}
	return resp.GetIds(), nil
}

// DeleteRecords deletes a batch of records iif no dataset has reference on
// Returns the number of deleted records
func (c Client) DeleteRecords(ids []string) (int64, error) {
	resp, err := c.gcc.DeleteRecords(c.ctx, &pb.DeleteRecordsRequest{Ids: ids})
	if err != nil {
		return 0, grpcError(err)
	}
	return resp.Nb, nil
}

// AddRecordsTags add or update existing tag form list of records
// returns the number of updated records
func (c Client) AddRecordsTags(ids []string, tags map[string]string) (int64, error) {
	resp, err := c.gcc.AddRecordsTags(c.ctx, &pb.AddRecordsTagsRequest{
		Ids:  ids,
		Tags: tags,
	})
	if err != nil {
		return 0, grpcError(err)
	}
	return resp.Nb, nil
}

// AddRecordsTags delete tags from list of records
// returns the number of updated records
func (c Client) RemoveRecordsTags(ids []string, tags []string) (int64, error) {
	resp, err := c.gcc.RemoveRecordsTags(c.ctx, &pb.RemoveRecordsTagsRequest{
		Ids:     ids,
		TagsKey: tags,
	})
	if err != nil {
		return 0, grpcError(err)
	}
	return resp.Nb, nil
}

func (c Client) streamListRecords(name string, tags map[string]string, aoi AOI, fromTime, toTime time.Time, limit, page int, returnAOI bool) (pb.Geocube_ListRecordsClient, error) {

	fromTs := timestamppb.New(fromTime)
	if err := fromTs.CheckValid(); err != nil {
		return nil, err
	}
	toTs := timestamppb.New(toTime)
	if err := toTs.CheckValid(); err != nil {
		return nil, err
	}

	res, err := c.gcc.ListRecords(c.ctx, &pb.ListRecordsRequest{
		Name:     name,
		Tags:     tags,
		Aoi:      pbFromAOI(aoi),
		FromTime: fromTs,
		ToTime:   toTs,
		Limit:    int32(limit),
		Page:     int32(page),
		WithAoi:  returnAOI,
	})
	return res, grpcError(err)
}

// ListRecords lists records that fit the search parameters (all are optionnal)
func (c Client) ListRecords(name string, tags map[string]string, aoi AOI, fromTime, toTime time.Time, limit, page int, returnAOI bool) ([]*Record, error) {
	streamrecords, err := c.streamListRecords(name, tags, aoi, fromTime, toTime, limit, page, returnAOI)
	if err != nil {
		return nil, err
	}
	records := []*Record{}

	for {
		resp, err := streamrecords.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		records = append(records, recordFromPb(resp.Record))
	}

	return records, nil
}
