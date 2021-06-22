package client

import (
	"bytes"
	"compress/flate"
	"errors"
	"fmt"
	"io"
	"time"

	pb "github.com/airbusgeo/geocube/client/go/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Format pb.FileFormat

const (
	Format_Raw   = Format(pb.FileFormat_Raw)
	Format_GTiff = Format(pb.FileFormat_GTiff)
)

type CubeHeader struct {
	Count      int64
	NbDatasets int64
}

type CubeElem struct {
	Data    []byte
	Shape   [3]int32
	DType   pb.DataFormat_Dtype
	Records []*Record
	Err     string
}

type CubeIterator struct {
	stream  pb.Geocube_GetCubeClient
	currval CubeElem
	header  CubeHeader
	err     error
}

func NewCubeIterator(stream pb.Geocube_GetCubeClient) (*CubeIterator, error) {
	cit := CubeIterator{stream: stream}

	// Get global header
	resp := cit.next()
	if resp == nil {
		if cit.err != nil {
			return nil, cit.err
		}
		return nil, fmt.Errorf("empty response : expecting a global header")
	}
	header := resp.GetGlobalHeader()
	if header == nil {
		return nil, fmt.Errorf("excepting a global header")
	}
	cit.header = CubeHeader{Count: header.Count, NbDatasets: header.NbDatasets}

	return &cit, nil
}

func (cit *CubeIterator) next() *pb.GetCubeResponse {
	resp, err := cit.stream.Recv()
	if err != nil {
		if err != io.EOF {
			cit.err = err
		}
		return nil
	}
	return resp
}

// Next implements Iterator
func (cit *CubeIterator) Next() bool {
	var resp *pb.GetCubeResponse

	// Get header
	var nbParts int32
	var data bytes.Buffer
	{
		if resp = cit.next(); resp == nil {
			return false
		}
		// Parse header
		header := resp.GetHeader()
		if header == nil {
			cit.err = errors.New("fatal: excepting a header")
			return false
		}

		// Reset currval
		cit.currval = CubeElem{Records: make([]*Record, len(header.Records))}
		for i, r := range header.Records {
			cit.currval.Records[i] = recordFromPb(r)
		}

		if header.GetError() != "" {
			cit.currval.Err = header.GetError()
			return true
		}

		shape := header.GetShape()
		nbParts = header.GetNbParts()
		cit.currval.Shape = [3]int32{shape.GetDim1(), shape.GetDim2(), shape.GetDim3()}
		cit.currval.DType = header.GetDtype()
		data.Grow(int(header.GetSize()))
		data.Write(header.GetData())
	}

	// Get chunks
	for i := int32(1); i < nbParts; i++ {
		if resp = cit.next(); resp == nil {
			return false
		}

		// Parse chunk
		chunk := resp.GetChunk()
		if chunk == nil || chunk.GetPart() != i {
			cit.err = errors.New("fatal: excepting a chunk")
			return false
		}
		data.Write(chunk.GetData())
	}

	if nbParts > 0 {
		inflater := flate.NewReader(&data)
		var b bytes.Buffer
		b.Grow(int(cit.currval.Shape[0]) * int(cit.currval.Shape[1]) * int(cit.currval.Shape[2]) * sizeOf(cit.currval.DType))

		if _, err := io.Copy(&b, inflater); err != nil {
			cit.currval.Err = err.Error()
		} else if err := inflater.Close(); err != nil {
			cit.currval.Err = err.Error()
		} else {
			cit.currval.Data = b.Bytes()
		}
	}
	return true
}

func sizeOf(dt pb.DataFormat_Dtype) int {
	switch dt {
	case pb.DataFormat_UInt8:
		return 1
	case pb.DataFormat_Int16, pb.DataFormat_UInt16:
		return 2
	case pb.DataFormat_Int32, pb.DataFormat_UInt32, pb.DataFormat_Float32:
		return 4
	case pb.DataFormat_Float64, pb.DataFormat_Complex64:
		return 8
	}
	return 0
}

// Header (global Header)
func (cit *CubeIterator) Header() CubeHeader {
	return cit.header
}

// Value implements Iterator
func (cit *CubeIterator) Value() *CubeElem {
	return &cit.currval
}

// Err implements Iterator
func (cit *CubeIterator) Err() error {
	return cit.err
}

// GetCubeFromRecords gets a cube from a list of records
func (c Client) GetCubeFromRecords(instancesID, recordsID []string, crs string, pix2crs [6]float64, sizeX, sizeY int64, format Format, compression int, headersOnly bool) (*CubeIterator, error) {
	stream, err := c.gcc.GetCube(c.ctx,
		&pb.GetCubeRequest{
			RecordsLister:    &pb.GetCubeRequest_Records{Records: &pb.RecordList{Ids: recordsID}},
			InstancesId:      instancesID,
			Crs:              crs,
			PixToCrs:         &pb.GeoTransform{A: pix2crs[0], B: pix2crs[1], C: pix2crs[2], D: pix2crs[3], E: pix2crs[4], F: pix2crs[5]},
			Size:             &pb.Size{Width: int32(sizeX), Height: int32(sizeY)},
			CompressionLevel: int32(compression),
			HeadersOnly:      headersOnly,
			Format:           pb.FileFormat(format),
		})
	if err != nil {
		return nil, grpcError(err)
	}
	return NewCubeIterator(stream)
}

// GetCube gets a cube from a list of filters
func (c Client) GetCube(instancesID []string, tags map[string]string, fromTime, toTime time.Time, crs string, pix2crs [6]float64, sizeX, sizeY int64, format Format, compression int, headersOnly bool) (*CubeIterator, error) {
	fromTs := timestamppb.New(fromTime)
	if err := fromTs.CheckValid(); err != nil {
		return nil, err
	}
	toTs := timestamppb.New(toTime)
	if err := toTs.CheckValid(); err != nil {
		return nil, err
	}

	stream, err := c.gcc.GetCube(c.ctx,
		&pb.GetCubeRequest{
			RecordsLister:    &pb.GetCubeRequest_Filters{Filters: &pb.RecordFilters{Tags: tags, FromTime: fromTs, ToTime: toTs}},
			InstancesId:      instancesID,
			Crs:              crs,
			PixToCrs:         &pb.GeoTransform{A: pix2crs[0], B: pix2crs[1], C: pix2crs[2], D: pix2crs[3], E: pix2crs[4], F: pix2crs[5]},
			Size:             &pb.Size{Width: int32(sizeX), Height: int32(sizeY)},
			CompressionLevel: int32(compression),
			HeadersOnly:      headersOnly,
			Format:           pb.FileFormat(format),
		})
	if err != nil {
		return nil, grpcError(err)
	}
	return NewCubeIterator(stream)
}
