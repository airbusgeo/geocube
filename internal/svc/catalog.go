package svc

import (
	"context"
	"fmt"
	"image"
	"math"
	"strconv"
	"time"

	"github.com/airbusgeo/geocube/internal/geocube"
	internalImage "github.com/airbusgeo/geocube/internal/image"
	pb "github.com/airbusgeo/geocube/internal/pb"
	"github.com/airbusgeo/geocube/internal/utils"
	"github.com/airbusgeo/geocube/internal/utils/affine"
	"github.com/airbusgeo/geocube/internal/utils/bitmap"
	"github.com/airbusgeo/geocube/internal/utils/proj"
	"github.com/airbusgeo/godal"
)

// GetCubeOptions defines user-options for a GetCube
type GetCubeOptions struct {
	Format               string
	HeadersOnly          bool
	Resampling           geocube.Resampling
	Predownload          bool
	FilterPartialImagePc int // Filter images that have less than % of valid pixels (-1 to deactivate)
}

// CubeSlice is a slice of a cube, an image corresponding to a group of record
type CubeSlice struct {
	Image        *bitmap.Bitmap
	Err          error
	Records      []*geocube.Record
	Metadata     map[string]string
	DatasetsMeta SliceMeta
}

// SliceMeta info to provide direct access to raw images
type SliceMeta struct {
	Datasets []*internalImage.Dataset
}

// CubeInfo stores various information about the Cube
type CubeInfo struct {
	NbImages      int
	NbDatasets    int
	Resampling    geocube.Resampling
	RefDataFormat geocube.DataFormat
}

// ToProtobuf
func (s *SliceMeta) ToProtobuf() *pb.DatasetMeta {
	datasetMeta := &pb.DatasetMeta{
		InternalsMeta: make([]*pb.InternalMeta, len(s.Datasets)),
	}

	// Populate the datasetMeta part of the header
	for i, d := range s.Datasets {
		datasetMeta.InternalsMeta[i] = &pb.InternalMeta{
			ContainerUri:    d.URI,
			ContainerSubdir: d.SubDir,
			Bands:           d.Bands,
			Dformat:         d.DataMapping.DataFormat.ToProtobuf(),
			RangeMin:        d.DataMapping.RangeExt.Min,
			RangeMax:        d.DataMapping.RangeExt.Max,
			Exponent:        d.DataMapping.Exponent,
		}
	}
	return datasetMeta
}

// NewSliceMetaFromProtobuf creates SliceMeta from protobuf
func NewSliceMetaFromProtobuf(pbmeta *pb.DatasetMeta) *SliceMeta {
	s := &SliceMeta{
		Datasets: make([]*internalImage.Dataset, len(pbmeta.InternalsMeta)),
	}
	// Populate the datasetMeta part of the header
	for i, meta := range pbmeta.InternalsMeta {
		s.Datasets[i] = &internalImage.Dataset{
			URI:    meta.ContainerUri,
			SubDir: meta.ContainerSubdir,
			Bands:  meta.Bands,
			DataMapping: geocube.DataMapping{
				DataFormat: *geocube.NewDataFormatFromProtobuf(meta.Dformat),
				RangeExt:   geocube.Range{Min: meta.RangeMin, Max: meta.RangeMax},
				Exponent:   meta.Exponent,
			},
		}
	}
	return s
}

// ListDatasets implements GeocubeService
func (svc *Service) ListDatasets(ctx context.Context, instanceID string, recordsID []string, recordTags geocube.Metadata, fromTime, toTime time.Time) ([]SliceMeta, []*geocube.Record, error) {
	// Find the datasets that fit
	datasets, err := svc.db.FindDatasets(ctx, geocube.DatasetStatusACTIVE, nil, "", []string{instanceID}, recordsID, recordTags, fromTime, toTime, nil, nil, 0, 0, true)
	if err != nil {
		return nil, nil, fmt.Errorf("GetCubeFromRecords.%w", err)
	}

	// Group datasets by record
	datasetsByRecord, records, err := svc.groupDatasetsByRecord(ctx, datasets)
	if err != nil {
		return nil, nil, fmt.Errorf("GetCubeFromRecords.%w", err)
	}
	return datasetsByRecord, records, nil
}

// GetCubeFromDatasets implements GeocubeDownloaderService
// panics if instancesID is empty
func (svc *Service) GetCubeFromMetadatas(ctx context.Context, metadatas []SliceMeta, grecords [][]*geocube.Record,
	refDf geocube.DataFormat, crs *godal.SpatialRef, pixToCRS *affine.Affine, width, height int, options GetCubeOptions) (CubeInfo, <-chan CubeSlice, error) {
	var err error
	var nbDs int
	for _, element := range metadatas {
		nbDs += len(element.Datasets)
	}
	outDesc := internalImage.GdalDatasetDescriptor{
		PixToCRS:   pixToCRS,
		Width:      width,
		Height:     height,
		Bands:      len(metadatas[0].Datasets[0].Bands),
		Resampling: options.Resampling,
		DataMapping: geocube.DataMapping{
			DataFormat: refDf,
			RangeExt:   refDf.Range,
			Exponent:   1,
		},
		ValidPixPc: options.FilterPartialImagePc,
		Format:     options.Format,
	}
	outDesc.WktCRS, err = crs.WKT()
	if err != nil {
		return CubeInfo{}, nil, fmt.Errorf("getCubeFromMetadatas.ToWKT: %w", err)
	}
	stream, err := svc.getCubeStream(ctx, metadatas, grecords, outDesc, options)
	if err != nil {
		return CubeInfo{}, nil, err
	}
	return CubeInfo{NbImages: len(metadatas), NbDatasets: nbDs}, stream, nil
}

// GetCubeFromRecords implements GeocubeService
// panics if instancesID is empty
func (svc *Service) GetCubeFromRecords(ctx context.Context, grecordsID [][]string, instancesID []string, crs *godal.SpatialRef, pixToCRS *affine.Affine,
	width, height int, options GetCubeOptions) (CubeInfo, <-chan CubeSlice, error) {
	// Prepare the request
	outDesc, geogExtent, err := svc.getCubePrepare(ctx, instancesID, crs, pixToCRS, width, height, options)
	if err != nil {
		return CubeInfo{}, nil, err
	}

	// Flatten grecords
	var recordsID []string
	for _, rs := range grecordsID {
		recordsID = append(recordsID, rs...)
	}

	// Find the datasets that fit
	datasets, err := svc.db.FindDatasets(ctx, geocube.DatasetStatusACTIVE, nil, "", instancesID, recordsID, geocube.Metadata{}, time.Time{}, time.Time{}, geogExtent, nil, 0, 0, true)
	if err != nil {
		return CubeInfo{}, nil, fmt.Errorf("GetCubeFromRecords.%w", err)
	}

	// Group datasets by record
	datasetsByRecord, records, err := svc.groupDatasetsByRecord(ctx, datasets)
	if err != nil {
		return CubeInfo{}, nil, fmt.Errorf("GetCubeFromRecords.%w", err)
	}

	// Group datasets by group of records and set the original order
	recordIdx := map[string]int{}
	for i, record := range records {
		recordIdx[record.ID] = i
	}
	var grecords [][]*geocube.Record
	datasetsByRecord, grecords = groupDatasetsByRecordsGroup(datasetsByRecord, records, recordIdx, grecordsID)

	// GetCube
	stream, err := svc.getCubeStream(ctx, datasetsByRecord, grecords, outDesc, options)
	return CubeInfo{NbImages: len(datasetsByRecord),
		NbDatasets:    len(datasets),
		Resampling:    outDesc.Resampling,
		RefDataFormat: outDesc.DataMapping.DataFormat,
	}, stream, err
}

// GetCubeFromFilters implements GeocubeService
// panics if instancesID is empty
func (svc *Service) GetCubeFromFilters(ctx context.Context, recordTags geocube.Metadata, fromTime, toTime time.Time, instancesID []string, crs *godal.SpatialRef, pixToCRS *affine.Affine,
	width, height int, options GetCubeOptions) (CubeInfo, <-chan CubeSlice, error) {
	// Prepare the request
	outDesc, geogExtent, err := svc.getCubePrepare(ctx, instancesID, crs, pixToCRS, width, height, options)
	if err != nil {
		return CubeInfo{}, nil, err
	}

	// Find the datasets that fit
	datasets, err := svc.db.FindDatasets(ctx, geocube.DatasetStatusACTIVE, nil, "", instancesID, nil, recordTags, fromTime, toTime, geogExtent, nil, 0, 0, true)
	if err != nil {
		return CubeInfo{}, nil, fmt.Errorf("GetCubeFromFilters.%w", err)
	}

	// Group datasets by record
	datasetsByRecord, records, err := svc.groupDatasetsByRecord(ctx, datasets)
	if err != nil {
		return CubeInfo{}, nil, fmt.Errorf("GetCubeFromFilters.%w", err)
	}

	// Create groups of one record
	grecords := make([][]*geocube.Record, len(records))
	for i, r := range records {
		grecords[i] = []*geocube.Record{r}
	}

	// GetCube
	stream, err := svc.getCubeStream(ctx, datasetsByRecord, grecords, outDesc, options)
	return CubeInfo{NbImages: len(datasetsByRecord),
		NbDatasets:    len(datasets),
		Resampling:    outDesc.Resampling,
		RefDataFormat: outDesc.DataMapping.DataFormat,
	}, stream, err
}

func (svc *Service) getCubePrepare(ctx context.Context, instancesID []string, crs *godal.SpatialRef, pixToCRS *affine.Affine, width, height int, options GetCubeOptions) (internalImage.GdalDatasetDescriptor, *proj.GeographicRing, error) {
	// Validate the input
	variable, err := svc.db.ReadVariableFromInstanceID(ctx, instancesID[0])
	if err != nil {
		return internalImage.GdalDatasetDescriptor{}, nil, fmt.Errorf("getCubePrepare.%w", err)
	}
	for _, instanceID := range instancesID {
		if err := variable.CheckInstanceExists(instanceID); err != nil {
			return internalImage.GdalDatasetDescriptor{}, nil, fmt.Errorf("getCubePrepare.%w", err)
		}
	}
	if options.Resampling == geocube.Resampling(pb.Resampling_UNDEFINED) {
		options.Resampling = variable.Resampling
	}

	// Describe the output
	outDesc := internalImage.GdalDatasetDescriptor{
		PixToCRS:   pixToCRS,
		Width:      width,
		Height:     height,
		Bands:      len(variable.Bands),
		Resampling: options.Resampling,
		DataMapping: geocube.DataMapping{
			DataFormat: variable.DFormat,
			RangeExt:   variable.DFormat.Range,
			Exponent:   1,
		},
		ValidPixPc: options.FilterPartialImagePc,
		Format:     options.Format,
	}
	outDesc.WktCRS, err = crs.WKT()
	if err != nil {
		return internalImage.GdalDatasetDescriptor{}, nil, fmt.Errorf("getCubePrepare.ToWKT: %w", err)
	}

	if variable.Palette != "" {
		if outDesc.Palette, err = svc.db.ReadPalette(ctx, variable.Palette); err != nil {
			return internalImage.GdalDatasetDescriptor{}, nil, fmt.Errorf("getCubePrepare.%w", err)
		}
	}

	// Get the extent
	geogExtent, err := proj.NewGeographicRingFromExtent(pixToCRS, width, height, crs)
	if err != nil {
		return internalImage.GdalDatasetDescriptor{}, nil, fmt.Errorf("getCubePrepare.%w", err)
	}

	return outDesc, &geogExtent, nil
}

// getCubeGroupByRecordsGroup groups datasets and records according to the original recordGroups
func groupDatasetsByRecordsGroup(datasetsByRecord []SliceMeta, records []*geocube.Record, recordIdx map[string]int, recordGroups [][]string) ([]SliceMeta, [][]*geocube.Record) {
	grecords := make([][]*geocube.Record, len(recordGroups))
	newDatasetsByRecord := make([]SliceMeta, len(recordGroups))
	i := 0
	for _, grecord := range recordGroups {
		for _, recordID := range grecord {
			if idx, ok := recordIdx[recordID]; ok {
				grecords[i] = append(grecords[i], records[idx])
				newDatasetsByRecord[i].Datasets = append(newDatasetsByRecord[i].Datasets, datasetsByRecord[idx].Datasets...)
			}
		}
		if len(grecords[i]) > 0 {
			i += 1
		}
	}
	return newDatasetsByRecord[0:i], grecords[0:i]
}

// getCubeGroupByRecords groups datasets by record.ID
func (svc *Service) groupDatasetsByRecord(ctx context.Context, datasets []*geocube.Dataset) ([]SliceMeta, []*geocube.Record, error) {
	// Group datasets by records
	var recordsID []string
	var datasetsByRecord []SliceMeta
	for i := 0; i < len(datasets); {
		// Get the range of datasets with same RecordID
		var ds SliceMeta
		recordID := datasets[i].RecordID
		for ; i < len(datasets) && datasets[i].RecordID == recordID; i++ {
			ds.Datasets = append(ds.Datasets, &internalImage.Dataset{
				URI:         datasets[i].ContainerURI,
				SubDir:      datasets[i].ContainerSubDir,
				Bands:       datasets[i].Bands,
				DataMapping: datasets[i].DataMapping,
			})
		}
		datasetsByRecord = append(datasetsByRecord, ds)
		recordsID = append(recordsID, recordID)
	}
	// Fetch records
	records, err := svc.db.ReadRecords(ctx, recordsID)
	return datasetsByRecord, records, err
}

// getNumberOfWorkers estimates the number of workers depending on the ramSize
func getNumberOfWorkers(memoryUsageBytes int) int {
	return utils.MinI(10, utils.MaxI(1, ramSize/memoryUsageBytes))
}

func (svc *Service) getCubeStream(ctx context.Context, datasetsByRecord []SliceMeta, grecords [][]*geocube.Record, outDesc internalImage.GdalDatasetDescriptor, options GetCubeOptions) (<-chan CubeSlice, error) {
	if options.HeadersOnly {
		// Push the headers into a channel
		headersOut := make(chan CubeSlice, len(grecords))
		for i, records := range grecords {
			headersOut <- CubeSlice{
				Image:        bitmap.NewBitmapHeader(image.Rect(0, 0, outDesc.Width, outDesc.Height), outDesc.DataMapping.DType, outDesc.Bands),
				Err:          nil,
				Records:      records,
				Metadata:     map[string]string{},
				DatasetsMeta: datasetsByRecord[i]}
		}
		close(headersOut)

		return headersOut, nil
	}

	// Predownload datasets if required
	datasetsAvailability := make([]DatasetsAvailability, len(datasetsByRecord))
	if options.Predownload {
		PredownloadRemoteDatasets(ctx, datasetsByRecord, datasetsAvailability)
	}

	// Create a job for each batch of datasets with the same record id and a result channel
	var jobs []mergeDatasetJob
	var unorderedSlices []<-chan CubeSlice
	for i, datasets := range datasetsByRecord {
		ackChan := make(chan CubeSlice /** set ", 1" to release the worker as soon as it finishes */)
		jobs = append(jobs, mergeDatasetJob{
			ID:    len(jobs),
			Slice: datasets, Records: grecords[i],
			OutDesc:           &outDesc,
			AvailabilityChans: datasetsAvailability[i],
			ResultChan:        ackChan,
		})
		unorderedSlices = append(unorderedSlices, ackChan)
	}

	// Create a channel for returning the results in order
	orderedSlices := make(chan CubeSlice)

	// Order results
	go orderResults(ctx, unorderedSlices, orderedSlices)

	// Start workers
	{
		jobChan := make(chan mergeDatasetJob, len(jobs))
		nbWorkers := utils.MinI(len(jobs), utils.MinI(svc.cubeWorkers, getNumberOfWorkers(outDesc.Height*outDesc.Width*outDesc.DataMapping.DType.Size()*10)))
		for i := 0; i < nbWorkers; i++ {
			go mergeDatasetsWorker(ctx, jobChan)
		}
		// Push jobs
		for _, j := range jobs {
			jobChan <- j
		}
		close(jobChan)
	}

	return orderedSlices, nil
}

func (svc *Service) infoFromTile(a, b, z int) (*proj.GeographicRing, internalImage.GdalDatasetDescriptor, error) {
	// Create the geographic extent from tile coordinates (a, b) and zoom level z
	outDesc := internalImage.GdalDatasetDescriptor{Width: 256, Height: 256}

	// Get WebMercator CRS
	crs, err := proj.CRSFromEPSG(3857)
	if err != nil {
		return nil, outDesc, fmt.Errorf("infoFromTile: %w", err)
	}
	outDesc.WktCRS, _ = crs.WKT()

	// Get the tile to CRS transform
	outDesc.PixToCRS, err = pixToWebMercatorTransform(z, crs)
	if err != nil {
		return nil, outDesc, fmt.Errorf("infoFromTile.%w", err)
	}

	// Get transform from tile coordinates to crs coordinates
	outDesc.PixToCRS = outDesc.PixToCRS.Multiply(affine.Translation(float64(outDesc.Width*a), float64(outDesc.Height*b)))

	// Create the geographic bbox
	geogExtent, err := proj.NewGeographicRingFromExtent(outDesc.PixToCRS, outDesc.Width, outDesc.Height, crs)
	if err != nil {
		return nil, outDesc, fmt.Errorf("infoFromTile: %w", err)
	}
	return &geogExtent, outDesc, nil
}

// GetXYZTile implements GeocubeService
func (svc *Service) GetXYZTile(ctx context.Context, instanceID string, recordsID []string, a, b, z int, min, max float64) ([]byte, error) {
	geogExtent, outDesc, err := svc.infoFromTile(a, b, z)
	if err != nil {
		return nil, fmt.Errorf("GetXYZTileFromFilters.%w", err)
	}

	// Get an image from these filters
	ds, err := svc.getMosaic(ctx, []string{instanceID}, recordsID, geocube.Metadata{}, time.Time{}, time.Time{}, *geogExtent, &outDesc)
	if err != nil {
		return nil, fmt.Errorf("GetXYZTile.%w", err)
	}
	if ds == nil {
		return nil, geocube.NewEntityNotFound("", "", "", "No data found")
	}
	defer ds.Close()

	return svc.getXYZTile(ctx, instanceID, ds, outDesc, min, max)
}

// GetXYZTileFromFilters implements GeocubeService
func (svc *Service) GetXYZTileFromFilters(ctx context.Context, instanceID string, recordTags geocube.Metadata, fromTime, toTime time.Time, a, b, z int, min, max float64) ([]byte, error) {
	geogExtent, outDesc, err := svc.infoFromTile(a, b, z)
	if err != nil {
		return nil, fmt.Errorf("GetXYZTileFromFilters.%w", err)
	}

	// Get an image from these filters
	ds, err := svc.getMosaic(ctx, []string{instanceID}, nil, recordTags, fromTime, toTime, *geogExtent, &outDesc)
	if err != nil {
		return nil, fmt.Errorf("GetXYZTile.%w", err)
	}
	if ds == nil {
		return nil, geocube.NewEntityNotFound("", "", "", "No data found")
	}
	defer ds.Close()

	return svc.getXYZTile(ctx, instanceID, ds, outDesc, min, max)
}

func (svc *Service) getXYZTile(ctx context.Context, instanceID string, ds *godal.Dataset, outDesc internalImage.GdalDatasetDescriptor, min, max float64) ([]byte, error) {
	// Get Palette
	var palette *geocube.Palette
	{
		variable, err := svc.db.ReadVariableFromInstanceID(ctx, instanceID)
		if err != nil {
			return nil, fmt.Errorf("GetXYZTile.%w", err)
		}
		if variable.Palette != "" {
			if palette, err = svc.db.ReadPalette(ctx, variable.Palette); err != nil {
				return nil, fmt.Errorf("GetXYZTile.%w", err)
			}
		}
	}

	// Set Min/Max
	if min < max {
		outDesc.DataMapping.Range = geocube.Range{Min: min, Max: max}
	}

	// Translate to PNG
	bytes, err := internalImage.DatasetToPngAsBytes(ctx, ds, outDesc.DataMapping, palette, true)
	if err != nil {
		return nil, fmt.Errorf("GetMosaic.%w", err)
	}

	return bytes, nil
}

func pixToWebMercatorTransform(z int, crs3857 *godal.SpatialRef) (*affine.Affine, error) {
	// Origin of tiles
	lon0 := -180.0
	lat0 := (2*math.Atan(math.Exp(math.Pi)) - math.Pi/2) * 180 / math.Pi // ~ 85.051129°

	transform, err := proj.CreateLonLatProj(crs3857, false)
	if err != nil {
		return nil, fmt.Errorf("pixToWebMercatorTransform.%w", err)
	}
	defer transform.Close()

	x, y := []float64{lon0}, []float64{lat0}
	transform.TransformEx(x, y, []float64{0}, nil)

	// Resolution
	axis, err := crs3857.SemiMajor()
	if err != nil {
		return nil, fmt.Errorf("pixToWebMercatorTransform.SemiMajorAxis: %w", err)
	}
	resolution := 2.0 * math.Pi * axis / float64(256*int(1<<z))

	// Affine transform from pixel to webmercator coordinates
	return affine.Translation(x[0], y[0]).Multiply(affine.Scale(resolution, -resolution)), nil
}

// orderResults waits for the result of workers and streams the results sorted by job.id
func orderResults(ctx context.Context, unordered []<-chan CubeSlice, ordered chan<- CubeSlice) {
	defer close(ordered)
	var slice CubeSlice
	for _, chanOut := range unordered {
		// Wait for the next job to finish
		select {
		case slice = <-chanOut:
		case <-ctx.Done():
			return
		}

		// Stream the results
		if slice.Image != nil || slice.Err != nil {
			select {
			case ordered <- slice:
			case <-ctx.Done():
				return
			}
		}
	}
}

type mergeDatasetJob struct {
	ID                int
	Slice             SliceMeta
	Records           []*geocube.Record
	OutDesc           *internalImage.GdalDatasetDescriptor
	AvailabilityChans DatasetsAvailability
	ResultChan        chan<- CubeSlice
}

func mergeTags(records []*geocube.Record) map[string]string {
	// Common tags
	tags := records[0].Tags
	for key, tag := range records[0].Tags {
		for i := 1; i < len(records); i++ {
			if v, ok := records[i].Tags[key]; !ok || v != tag {
				delete(tags, key)
				break
			}
		}
	}

	// Other tags
	for i, r := range records {
		for key, tag := range r.Tags {
			if _, ok := tags[key]; !ok {
				tags[key+"."+strconv.Itoa(i)] = tag
			}
		}
	}
	return tags
}

// mergeDatasetsWorker panics if datasets is empty
func mergeDatasetsWorker(ctx context.Context, jobs <-chan mergeDatasetJob) {
	for job := range jobs {
		func() {
			defer close(job.ResultChan)
			// In case of early cancellation
			if utils.IsCancelled(ctx) {
				return
			}

			metadata := map[string]string{}

			// Wait datasets
			var acks []DownloadAck
			if len(job.AvailabilityChans) > 0 {
				start := time.Now()
				acks = WaitForAvailability(job.Slice.Datasets, job.AvailabilityChans)
				metadata[fmt.Sprintf("WaitDownload %d", len(job.AvailabilityChans))] = fmt.Sprintf("%v", time.Since(start))
			}

			// Run mergeDatasets
			start := time.Now()
			var bmp *bitmap.Bitmap
			ds, err := internalImage.MergeDatasets(ctx, job.Slice.Datasets, job.OutDesc)
			// Acq downloaded images
			for _, ack := range acks {
				if !utils.IsCancelled(ack.Ctx) {
					ack.AckChan <- struct{}{}
				}
			}
			if err == nil {
				// Convert to image
				switch job.OutDesc.Format {
				case "GTiff":
					tags := mergeTags(job.Records)
					bmp = bitmap.NewBitmapHeader(image.Rect(0, 0, job.OutDesc.Width, job.OutDesc.Height), job.OutDesc.DataMapping.DType, job.OutDesc.Bands)
					var bytes []byte
					bytes, err = internalImage.DatasetToTiffAsBytes(ds, job.OutDesc.DataMapping, tags, nil)
					bmp.Chunks = &bitmap.ByteArray{Bytes: bytes}
					ds.Close()

				default:
					bmp, err = bitmap.NewStreamableBitmapFromDataset(ds)
					// ds is closed by "bmp"
				}
			}

			metadata[fmt.Sprintf("Merge %d", len(job.Slice.Datasets))] = fmt.Sprintf("%v", time.Since(start))

			// Send bitmap
			select {
			case <-ctx.Done():
				return
			case job.ResultChan <- CubeSlice{
				Image:        bmp,
				Err:          err,
				Records:      job.Records,
				Metadata:     metadata,
				DatasetsMeta: job.Slice}:
			}
		}()
	}
}

// getMosaic returns a mosaic given recordsID and instancesID (both not empty)
// The caller is responsible to close the output dataset
func (svc *Service) getMosaic(ctx context.Context, instancesID, recordsID []string, recordTags geocube.Metadata, fromTime, toTime time.Time, geogExtent proj.GeographicRing, outDesc *internalImage.GdalDatasetDescriptor) (*godal.Dataset, error) {
	// Read Variable
	variable, err := svc.db.ReadVariableFromInstanceID(ctx, instancesID[0])
	if err != nil {
		return nil, fmt.Errorf("GetMosaic.%w", err)
	}
	for _, instanceID := range instancesID {
		if err := variable.CheckInstanceExists(instanceID); err != nil {
			return nil, fmt.Errorf("GetMosaic.%w", err)
		}
	}

	// Retrieve datasets
	datasets, err := svc.db.FindDatasets(ctx, geocube.DatasetStatusACTIVE, nil, "", instancesID, recordsID, recordTags, fromTime, toTime, &geogExtent, nil, 0, 0, true)
	if err != nil {
		return nil, fmt.Errorf("GetMosaic.%w", err)
	}
	if len(datasets) == 0 {
		return nil, nil
	}

	// Merge datasets
	outDesc.Resampling = variable.Resampling
	outDesc.DataMapping = geocube.DataMapping{
		DataFormat: variable.DFormat,
		RangeExt:   variable.DFormat.Range,
		Exponent:   1,
	}
	ds := make([]*internalImage.Dataset, len(datasets))
	for i, d := range datasets {
		ds[i] = &internalImage.Dataset{
			URI:         d.ContainerURI,
			SubDir:      d.ContainerSubDir,
			Bands:       d.Bands,
			DataMapping: d.DataMapping,
		}
	}

	return internalImage.MergeDatasets(ctx, ds, outDesc)
}
