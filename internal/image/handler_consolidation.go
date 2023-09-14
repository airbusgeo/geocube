package image

import (
	"context"
	"fmt"
	"math"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/airbusgeo/geocube/interface/storage"
	"github.com/google/uuid"

	"github.com/airbusgeo/geocube/interface/storage/uri"
	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/geocube/internal/log"
	"github.com/airbusgeo/geocube/internal/utils"
	"github.com/airbusgeo/geocube/internal/utils/affine"
	"golang.org/x/sync/errgroup"
)

const (
	TaskCancelledConsolidationError = ErrorConst("consolidation event is cancelled")
	NotImplementedError             = ErrorConst("consolidation without interleave records is not supported")
)

type ErrorConst string

func (e ErrorConst) Error() string {
	return string(e)
}

type Handler interface {
	Consolidate(ctx context.Context, cEvent *geocube.ConsolidationEvent, workspace string) error
}

type handlerConsolidation struct {
	cog                  CogGenerator
	mucog                MucogGenerator
	cancelledJobsStorage string
	workers              int
	localDownload        bool // Locally download the datasets before starting the consolidation (generally faster than letting GDAL to download them tile by tile)
}

func NewHandleConsolidation(c CogGenerator, m MucogGenerator, cancelledJobsStorage string, workers int, localDownload bool) Handler {
	return &handlerConsolidation{
		cog:                  c,
		mucog:                m,
		cancelledJobsStorage: cancelledJobsStorage,
		workers:              workers,
		localDownload:        localDownload,
	}
}

// Consolidate generate MUCOG file from list of COG (Cloud Optimized Geotiff).
func (h *handlerConsolidation) Consolidate(ctx context.Context, cEvent *geocube.ConsolidationEvent, workspace string) error {
	if h.isCancelled(ctx, cEvent) {
		return TaskCancelledConsolidationError
	}

	workDir := path.Join(workspace, cEvent.TaskID)
	if err := os.MkdirAll(workDir, 0777); err != nil {
		return err
	}
	defer h.cleanWorkspace(ctx, workDir)

	var tmpFileMutex sync.Mutex
	datasetsByRecords, tmpFileCounter, err := h.getLocalDatasetsByRecord(ctx, cEvent, workDir)
	if err != nil {
		return fmt.Errorf("failed to get local records datasets: %w", err)
	}

	log.Logger(ctx).Sugar().Infof("starting to create COG files")
	cogListFile := make([]string, len(cEvent.Records))

	records := make(chan struct {
		string
		int
	}, len(cEvent.Records))

	if cEvent.Container.OverviewsMinSize == geocube.OVERVIEWS_DEFAULT_MIN_SIZE {
		cEvent.Container.OverviewsMinSize = 256
	}
	// Start COG workers
	g, gCtx := errgroup.WithContext(ctx)
	for w := 0; w < h.workers; w++ {
		g.Go(func() error {
			for record := range records {
				if h.isCancelled(gCtx, cEvent) {
					return fmt.Errorf("consolidation event is cancelled")
				}
				recordID, recordIdx := record.string, record.int
				localDatasets := datasetsByRecords[recordID]

				gCtx := log.With(gCtx, "Record", recordID)
				if cogFile, ok := h.isAlreadyUsableCOG(gCtx, localDatasets, cEvent.Container, recordID, workDir); ok {
					log.Logger(gCtx).Sugar().Debugf("skip record (already a cog): %s (%d/%d)", recordID, recordIdx+1, len(cEvent.Records))
					cogListFile[recordIdx] = cogFile
					continue
				}

				pixToCRS := affine.NewAffine(
					cEvent.Container.Transform[0],
					cEvent.Container.Transform[1],
					cEvent.Container.Transform[2],
					cEvent.Container.Transform[3],
					cEvent.Container.Transform[4],
					cEvent.Container.Transform[5],
				)

				// Get the optimized extent, regarding blocksize using warpVRT
				pixToCRS, width, height, err := optimizeTransform(gCtx, localDatasets, cEvent.Container.CRS, pixToCRS, cEvent.Container.Width, cEvent.Container.Height, cEvent.Container.BlockXSize, cEvent.Container.BlockYSize)
				if err != nil {
					return fmt.Errorf("Consolidate.%w", err)
				}

				mergeDataset, err := MergeDatasets(gCtx, localDatasets, &GdalDatasetDescriptor{
					Height:      int(height),
					Width:       int(width),
					Bands:       cEvent.Container.BandsCount,
					DataMapping: cEvent.Container.DatasetFormat,
					WktCRS:      cEvent.Container.CRS,
					ValidPixPc:  -1,
					Resampling:  cEvent.Container.ResamplingAlg,
					PixToCRS:    pixToCRS,
				})
				if err != nil {
					return fmt.Errorf("Consolidate.%w", err)
				}

				cogDatasetPath, err := h.cog.Create(mergeDataset, cEvent.Container, recordID, workDir)
				mergeDataset.Close()
				if err != nil {
					return fmt.Errorf("Consolidate.%w", err)
				}

				// Delete tmpFile if possible to free memory
				tmpFileMutex.Lock()
				for _, dataset := range localDatasets {
					if _, ok := tmpFileCounter[dataset.URI]; ok {
						tmpFileCounter[dataset.URI]--
						if tmpFileCounter[dataset.URI] == 0 {
							uri := dataset.URI
							go func() {
								os.Remove(uri)
							}()
						}
					}
				}
				tmpFileMutex.Unlock()

				log.Logger(gCtx).Sugar().Debugf("add cog %s for record: %s (%d/%d)", cogDatasetPath, recordID, recordIdx+1, len(cEvent.Records))
				cogListFile[recordIdx] = cogDatasetPath
			}
			return nil
		})
	}

	// Push record tasks
	for i, record := range cEvent.Records {
		records <- struct {
			string
			int
		}{record.ID, i}
	}
	close(records)

	if err := g.Wait(); err != nil {
		return err
	}

	log.Logger(ctx).Sugar().Infof("%d COGs have been generated", len(cogListFile))

	if len(cogListFile) == 1 {
		if err := uploadFile(ctx, cogListFile[0], cEvent.Container.URI); err != nil {
			return fmt.Errorf("failed to upload file on: %s : %w", cEvent.Container.URI, err)
		}

		log.Logger(ctx).Sugar().Infof("Upload cog on : %s", cEvent.Container.URI)
		return nil
	}

	mucogFilePath, err := h.mucog.Create(workDir, cogListFile, cEvent.Container.InterlacingPattern)
	if err != nil {
		return fmt.Errorf("failed to create mucog: %w", err)
	}

	log.Logger(ctx).Sugar().Debugf("mucog has been generated : %s", mucogFilePath)
	if err := uploadFile(ctx, mucogFilePath, cEvent.Container.URI); err != nil {
		return fmt.Errorf("failed to upload file on: %s : %w", cEvent.Container.URI, err)
	}

	log.Logger(ctx).Sugar().Infof("Upload mucog on : %s", cEvent.Container.URI)

	return nil
}

type FileToDownload struct {
	URI      uri.DefaultUri
	LocalURI string
}

// getLocalDatasetsByRecord references all datasets and download them in local filesystem.
func (h *handlerConsolidation) getLocalDatasetsByRecord(ctx context.Context, cEvent *geocube.ConsolidationEvent, workDir string) (map[string][]*Dataset, map[string]int, error) {
	// Prepare local dataset and list files to download
	datasetsByRecord := map[string][]*Dataset{}
	filesToDownload := map[uri.DefaultUri]string{}
	tmpFileCounter := map[string]int{}
	for _, record := range cEvent.Records {
		var datasets []*Dataset
		for _, dataset := range record.Datasets {
			localUri := dataset.URI
			if h.localDownload {
				sourceUri, err := uri.ParseUri(dataset.URI)
				if err != nil {
					return nil, nil, fmt.Errorf("getLocalDatasetsByRecord: %w", err)
				}
				if sourceUri.Protocol() != "" {
					var ok bool
					if localUri, ok = filesToDownload[sourceUri]; !ok {
						localUri = path.Join(workDir, uuid.New().String())
						filesToDownload[sourceUri] = localUri
						tmpFileCounter[localUri] = 0
					}
					tmpFileCounter[localUri] += 1
				}
			}
			gDataset := &Dataset{
				URI:         localUri,
				SubDir:      dataset.Subdir,
				Bands:       dataset.Bands,
				DataMapping: dataset.DatasetFormat,
			}
			datasets = append(datasets, gDataset)
		}
		datasetsByRecord[record.ID] = datasets
	}

	// Push download jobs
	if len(filesToDownload) > 0 {
		log.Logger(ctx).Sugar().Debugf("downloading %d files", len(filesToDownload))
		files := make(chan FileToDownload, len(filesToDownload))
		for uri, localUri := range filesToDownload {
			files <- FileToDownload{URI: uri, LocalURI: localUri}
		}
		close(files)

		// Start download workers
		g, gCtx := errgroup.WithContext(ctx)
		for i := 0; i < 20; i++ {
			g.Go(func() error {
				for file := range files {
					if utils.IsCancelled(ctx) {
						return ctx.Err()
					}
					if err := file.URI.DownloadToFile(gCtx, file.LocalURI); err != nil {
						return fmt.Errorf("%s: %w", file.URI.String(), err)
					}
				}
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return nil, nil, fmt.Errorf("failed to download one of the sources: %w", err)
		}
	}

	return datasetsByRecord, tmpFileCounter, nil
}

// uploadFile upload content from local file to storage file (URI) destination.
func uploadFile(ctx context.Context, source, destination string) error {
	gsURI, err := uri.ParseUri(destination)
	if err != nil {
		return fmt.Errorf("failed to parse uri: %w", err)
	}

	f, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	defer f.Close()

	if err := gsURI.UploadFile(ctx, f); err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	return nil
}

// cleanWorkspace remove local workspace content.
func (h *handlerConsolidation) cleanWorkspace(ctx context.Context, workspace string) {
	if err := os.RemoveAll(workspace); err != nil {
		log.Logger(ctx).Sugar().Errorf("failed to clean workspace: %s", err.Error())
		return
	}
	log.Logger(ctx).Sugar().Debugf("Workspace cleaned")
}

func (h *handlerConsolidation) isCancelled(ctx context.Context, event *geocube.ConsolidationEvent) bool {
	if utils.IsCancelled(ctx) {
		return true
	}

	path := utils.URLJoin(h.cancelledJobsStorage, fmt.Sprintf("%s_%s", event.JobID, event.TaskID))
	cancelledJobsURI, err := uri.ParseUri(path)
	if err != nil {
		log.Logger(ctx).Sugar().Errorf("failed to parse uri: %s: %s", path, err.Error())
		return false
	}

	exist, err := cancelledJobsURI.Exist(ctx)
	switch err {
	case nil:
	case storage.ErrFileNotFound:
	default:
		log.Logger(ctx).Sugar().Errorf("failed to check uri existence: %s: %s", path, err.Error())
		return false
	}

	return exist
}

/*
isAlreadyUsableCOG return if file is already a Cloud Optimized Geotiff and internal structure is similar that container, otherwise false, and the path of the file.
*/
func (h *handlerConsolidation) isAlreadyUsableCOG(ctx context.Context, records []*Dataset, container geocube.ConsolidationContainer, recordID, workDir string) (string, bool) {
	if len(records) > 1 {
		return "", false
	}

	isMucogDataset := records[0].SubDir != ""
	var localFilePath string
	if isMucogDataset {
		localFilePath = fmt.Sprintf("%s:%s", records[0].SubDir, records[0].URI)
	} else {
		localFilePath = records[0].URI
	}

	ds, err := h.cog.Open(ctx, localFilePath)
	if err != nil {
		log.Logger(ctx).Sugar().Debugf("file is not a cog : %v", err)
		return "", false
	}
	defer ds.Close()

	var errors []string
	if ds.Structure().BlockSizeX != container.BlockXSize || ds.Structure().BlockSizeY != container.BlockYSize {
		errors = append(errors, "cog blockSize is different than container target blockSize")
	}

	if ds.Structure().NBands != container.BandsCount {
		errors = append(errors, "cog number of bands is different than container target number of bands")
	}

	if ds.Structure().DataType != container.DatasetFormat.DType.ToGDAL() {
		errors = append(errors, "cog dataType is different than container target dataType")
	}

	band := ds.Bands()[0]
	ovrCount := len(band.Overviews())
	if ovrCount != h.computeNbOverviews(container.Width, container.Height, container.OverviewsMinSize) {
		errors = append(errors, "cog does not have the required number of overviews")
	}

	srWKT, err := ds.SpatialRef().WKT()
	if err != nil {
		errors = append(errors, err.Error())
	}
	if !strings.EqualFold(srWKT, container.CRS) {
		errors = append(errors, "cog crs is different than container target crs")
	}

	geoTransform, err := ds.GeoTransform()
	if err != nil {
		errors = append(errors, err.Error())
	}
	if !h.isSameGeoTransForm(geoTransform, container.Transform) {
		errors = append(errors, "cog geoTransform is different than container target geoTransform")
	}

	if len(errors) != 0 {
		log.Logger(ctx).Sugar().Debugf("cog is not reusable as is: " + strings.Join(errors, ", "))
		return "", false
	}

	if isMucogDataset {
		// Cannot extract a cog from a mucog (to be fixed)
		return "", false
	}

	return localFilePath, true
}

/*
isSameGeoTransForm compares two geoTransfrom with fix tolerance (10^-8)
*/
func (h *handlerConsolidation) isSameGeoTransForm(gt1 [6]float64, gt2 [6]float64) bool {
	if len(gt1) != len(gt2) {
		return false
	}

	tolerance := math.Pow(10, -8)
	for i := range gt1 {
		if diff := math.Abs(gt1[i] - gt2[i]); diff < tolerance {
			continue
		}
		return false
	}
	return true
}

/**
 *	computeNbOverviews returns the number of overviews requested
 */
func (h *handlerConsolidation) computeNbOverviews(width, height, minSize int) int {
	if minSize == geocube.NO_OVERVIEW {
		return 0
	}
	nb := 0
	for width > minSize || height > minSize {
		nb += 1
		width /= 2
		height /= 2
	}
	return nb
}

func optimizeTransform(ctx context.Context, datasets []*Dataset, crs string, pixToCRS *affine.Affine, width, height, blockSizeX, blockSizeY int) (*affine.Affine, int, int, error) {
	// Get the extent using warpVRT
	extent, err := WarpedExtent(ctx, datasets, crs, pixToCRS[1], pixToCRS[5])
	if err != nil {
		return nil, 0, 0, fmt.Errorf("optimizeTransform: %w", err)
	}
	crsToPix := pixToCRS.Inverse()
	// Get the coordinates of extent in the container
	x0, y0 := crsToPix.Transform(extent[0], extent[1])
	x1, y1 := crsToPix.Transform(extent[2], extent[3])
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if y0 > y1 {
		y0, y1 = y1, y0
	}

	w, h, bsX, bsY := float64(width), float64(height), float64(blockSizeX), float64(blockSizeY)
	ox := math.Max(bsX*math.Floor(x0/bsX), 0)
	oy := math.Max(bsY*math.Floor(y0/bsY), 0)
	w = math.Ceil(math.Min(x1, w) - ox)
	h = math.Ceil(math.Min(y1, h) - oy)

	return pixToCRS.Multiply(affine.Translation(ox, oy)), int(w), int(h), nil
}
