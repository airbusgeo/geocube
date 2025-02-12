package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/airbusgeo/geocube/interface/storage/gcs"
	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/geocube/internal/log"
	pb "github.com/airbusgeo/geocube/internal/pb"
	internal "github.com/airbusgeo/geocube/internal/svc"
	"github.com/airbusgeo/geocube/internal/utils/affine"
	"github.com/airbusgeo/geocube/internal/utils/bitmap"
	"github.com/airbusgeo/geocube/internal/utils/proj"
	"github.com/airbusgeo/godal"
)

// GeocubeDownloaderService contains the downloader service
type GeocubeDownloaderService interface {
	// GetCubeFromMetadatas requests a cube of data from metadatas generated with a previous call to GetCube()
	GetCubeFromMetadatas(ctx context.Context, metadatas []internal.SliceMeta, grecords [][]*geocube.Record,
		refDf geocube.DataFormat, crs *godal.SpatialRef, pixToCRS *affine.Affine, width, height int, options internal.GetCubeOptions) (internal.CubeInfo, <-chan internal.CubeSlice, error)
}

// DownloaderService is the GRPC service
type DownloaderService struct {
	pb.UnimplementedGeocubeDownloaderServer
	gdsvc            GeocubeDownloaderService
	maxConnectionAge time.Duration
	chunkSizeBytes   int
}

var _ pb.GeocubeDownloaderServer = &DownloaderService{}

// NewDownloader returns a new GRPC DownloaderService connected to an DownloaderService
func NewDownloader(gdsvc GeocubeDownloaderService, maxConnectionAgeSec int, chunkSizeBytes int) *DownloaderService {
	return &DownloaderService{gdsvc: gdsvc, maxConnectionAge: time.Duration(maxConnectionAgeSec), chunkSizeBytes: chunkSizeBytes}
}

// Version returns version of the geocube
func (svc *DownloaderService) Version(ctx context.Context, req *pb.GetVersionRequest) (*pb.GetVersionResponse, error) {
	return &pb.GetVersionResponse{Version: GeocubeServerVersion}, nil
}

// GetCube implements DownloaderService
func (svc *DownloaderService) DownloadCube(req *pb.GetCubeMetadataRequest, stream pb.GeocubeDownloader_DownloadCubeServer) error {
	globalHeader := &pb.GetCubeResponseHeader{
		ResamplingAlg: req.ResamplingAlg,
		RefDformat:    req.RefDformat,
		Geotransform:  req.PixToCrs,
		Crs:           req.Crs,
	}

	if len(req.GetGroupedRecords()) == 0 {
		return stream.Send(&pb.GetCubeMetadataResponse{Response: &pb.GetCubeMetadataResponse_GlobalHeader{GlobalHeader: globalHeader}})
	}

	start := time.Now()
	t := req.GetPixToCrs()
	pixToCRS := affine.NewAffine(t.GetA(), t.GetB(), t.GetC(), t.GetD(), t.GetE(), t.GetF())
	if !pixToCRS.IsInvertible() {
		return newValidationError("Invalid pixToCRS transform: not invertible")
	}
	crs, _, err := proj.CRSFromUserInput(req.GetCrs())
	if err != nil {
		return newValidationError(fmt.Sprintf("Invalid crs: %s (%v)", req.GetCrs(), err))
	}
	width, height := int(req.GetSize().GetWidth()), int(req.GetSize().GetHeight())
	if width <= 0 || height <= 0 {
		return newValidationError(fmt.Sprintf("Invalid shape: %dx%d", width, height))
	}
	ctx, cancel := context.WithTimeout(stream.Context(), svc.maxConnectionAge*time.Second)
	defer func() {
		cancel()
	}()
	if len(req.GetDatasetsMeta()) != len(req.GetGroupedRecords()) {
		return newValidationError("number of datasetsMeta must be equal to the number of record lists : each datasetMeta is attached to a record list")
	}
	sliceMetas := make([]internal.SliceMeta, 0, len(req.GetDatasetsMeta()))
	for i, metadata := range req.GetDatasetsMeta() {
		sliceMetas = append(sliceMetas, *internal.NewSliceMetaFromProtobuf(metadata))
		for _, element := range sliceMetas[i].Datasets {
			if len(element.Bands) != len(sliceMetas[0].Datasets[0].Bands) {
				return newValidationError("Bands number is not constant")
			}
		}
	}
	grecords := make([][]*geocube.Record, 0, len(req.GetGroupedRecords()))
	for _, pbgrecords := range req.GetGroupedRecords() {
		records := make([]*geocube.Record, len(pbgrecords.GetRecords()))
		for i, pbrecord := range pbgrecords.GetRecords() {
			if records[i], err = geocube.RecordFromProtobuf(pbrecord); err != nil {
				return formatError("backend.%w", err)
			}
			record, _ := geocube.RecordFromProtobuf(pbrecord)
			records[i] = record
		}
		grecords = append(grecords, records)
	}
	info, slicesQueue, err := svc.gdsvc.GetCubeFromMetadatas(ctx,
		sliceMetas,
		grecords,
		geocube.DataFormat{DType: bitmap.DType(req.GetRefDformat().Dtype),
			NoData: req.GetRefDformat().NoData,
			Range: geocube.Range{Min: req.GetRefDformat().GetMinValue(),
				Max: req.GetRefDformat().GetMaxValue()},
		},
		crs,
		pixToCRS,
		width,
		height,
		internal.GetCubeOptions{
			Format:               req.Format.String(),
			Resampling:           geocube.Resampling(req.GetResamplingAlg()),
			Predownload:          req.Predownload,
			FilterPartialImagePc: 0, // Filter only empty images
		})
	if err != nil {
		return formatError("GetCube.%w", err)
	}

	globalHeader.Count = int64(info.NbImages)
	globalHeader.NbDatasets = int64(info.NbDatasets)
	if err := stream.Send(&pb.GetCubeMetadataResponse{Response: &pb.GetCubeMetadataResponse_GlobalHeader{GlobalHeader: globalHeader}}); err != nil {
		return formatError("GetCube.Send: %w", err)
	}

	log.Logger(ctx).Sugar().Infof("GetCube : %d images from %d datasets (%v)\n", info.NbImages, info.NbDatasets, time.Since(start))

	n := 1
	for slice := range slicesQueue {
		header := getCubeCreateHeader(&slice, svc.chunkSizeBytes, false)

		getCubeLog(ctx, slice, header, false, n)
		n++

		// Send header
		if err := stream.Send(&pb.GetCubeMetadataResponse{Response: &pb.GetCubeMetadataResponse_Header{Header: header}}); err != nil {
			return formatError("backend.GetCube.SendHeader%w", err)
		}

		// Send chunks
		for i := int32(1); i < header.NbParts; i++ {
			if chunk, err := slice.Image.Chunks.Next(svc.chunkSizeBytes); err != nil {
				return formatError("backend.GetCube.SendChunks.%w", err)
			} else if err := stream.Send(&pb.GetCubeMetadataResponse{Response: &pb.GetCubeMetadataResponse_Chunk{Chunk: &pb.ImageChunk{Part: i, Data: chunk}}}); err != nil {
				return formatError("backend.GetCube.SendChunks.%w", err)
			}
		}
		slice.Image = nil
	}

	log.Logger(ctx).Sugar().Infof("GetCube: %d images streamed from %d datasets (%v)\n", info.NbImages, info.NbDatasets, time.Since(start))

	defer gcs.GetMetrics(ctx)
	return ctx.Err()
}
