package image

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"strings"

	"github.com/airbusgeo/geocube/interface/storage"

	"github.com/airbusgeo/geocube/interface/storage/uri"
	"github.com/airbusgeo/geocube/internal/geocube"
	"github.com/airbusgeo/geocube/internal/log"
	"github.com/airbusgeo/geocube/internal/utils"
	"github.com/airbusgeo/geocube/internal/utils/affine"
	"github.com/google/uuid"
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
}

func NewHandleConsolidation(c CogGenerator, m MucogGenerator, cancelledJobsStorage string, workers int) Handler {
	return &handlerConsolidation{
		cog:                  c,
		mucog:                m,
		cancelledJobsStorage: cancelledJobsStorage,
		workers:              workers,
	}
}

// Consolidate generate MUCOG file from list of COG (Cloud Optimized Geotiff).
func (h *handlerConsolidation) Consolidate(ctx context.Context, cEvent *geocube.ConsolidationEvent, workspace string) error {
	if h.isCancelled(ctx, cEvent) {
		return TaskCancelledConsolidationError
	}

	if !cEvent.Container.InterleaveRecords {
		return NotImplementedError
	}

	id := uuid.New()
	workDir := path.Join(workspace, id.String())
	if err := os.Mkdir(workDir, 0777); err != nil {
		return err
	}
	defer h.cleanWorkspace(ctx, workDir)

	datasetsByRecords, err := h.getLocalDatasetsByRecord(ctx, cEvent, workDir)
	if err != nil {
		return fmt.Errorf("failed to get local records datasets: %w", err)
	}

	log.Logger(ctx).Sugar().Infof("starting to create COG files")
	cogListFile := make([]string, len(cEvent.Records))

	records := make(chan struct {
		string
		int
	})
	// Start download workers
	g, gCtx := errgroup.WithContext(ctx)
	for w := 0; w < h.workers; w++ {
		g.Go(func() error {
			for record := range records {
				if h.isCancelled(gCtx, cEvent) {
					return errors.New("consolidation event is cancelled")
				}
				recordID, recordIdx := record.string, record.int
				localDatasetsByRecords := datasetsByRecords[recordID]

				if cogFile, ok := h.isAlreadyUsableCOG(gCtx, localDatasetsByRecords, cEvent.Container, recordID, workDir); ok {
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

				mergeDataset, err := MergeDatasets(gCtx, localDatasetsByRecords, &GdalDatasetDescriptor{
					Height:      cEvent.Container.Height,
					Width:       cEvent.Container.Width,
					Bands:       cEvent.Container.BandsCount,
					DataMapping: cEvent.Container.DatasetFormat,
					WktCRS:      cEvent.Container.CRS,
					ValidPixPc:  -1,
					Resampling:  cEvent.Container.ResamplingAlg,
					PixToCRS:    pixToCRS,
				})
				if err != nil {
					return fmt.Errorf("failed to merge dataset: %w", err)
				}

				cogDatasetPath, err := h.cog.Create(mergeDataset, cEvent.Container, recordID, workDir)
				mergeDataset.Close()
				if err != nil {
					return fmt.Errorf("failed to merge source images: %w", err)
				}

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
		if err := h.uploadFile(ctx, cogListFile[0], cEvent.Container.URI); err != nil {
			return fmt.Errorf("failed to upload file on: %s : %w", cEvent.Container.URI, err)
		}

		log.Logger(ctx).Sugar().Infof("Upload cog on : %s", cEvent.Container.URI)
		return nil
	}

	mucogFilePath, err := h.mucog.Create(workDir, cogListFile)
	if err != nil {
		return fmt.Errorf("failed to create mucog: %w", err)
	}

	log.Logger(ctx).Sugar().Debugf("mucog has been generated : %s", mucogFilePath)
	if err := h.uploadFile(ctx, mucogFilePath, cEvent.Container.URI); err != nil {
		return fmt.Errorf("failed to upload file on: %s : %w", cEvent.Container.URI, err)
	}

	log.Logger(ctx).Sugar().Infof("Upload mucog on : %s", cEvent.Container.URI)

	return nil
}

type FileToDownload struct {
	URI, LocalURI string
}

// getLocalDatasetsByRecord download all datasets in local filesystem.
func (h *handlerConsolidation) getLocalDatasetsByRecord(ctx context.Context, cEvent *geocube.ConsolidationEvent, workDir string) (map[string][]*Dataset, error) {
	// Prepare local dataset and list files to download
	datasetsByRecord := map[string][]*Dataset{}
	filesToDownload := map[string]string{}
	for _, record := range cEvent.Records {
		var datasets []*Dataset
		for _, dataset := range record.Datasets {
			localUri, ok := filesToDownload[dataset.URI]
			if !ok {
				localUri = path.Join(workDir, uuid.New().String())
				filesToDownload[dataset.URI] = localUri
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
	log.Logger(ctx).Sugar().Debugf("downloading datasets")
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
				if err := h.downloadFile(gCtx, file.URI, file.LocalURI); err != nil {
					return err
				}
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("failed to download one of the sources: %w", err)
	}

	return datasetsByRecord, nil
}

// downloadFile download content from storage file (URI) to local destination.
func (h *handlerConsolidation) downloadFile(ctx context.Context, source, destination string) error {
	sourceUri, err := uri.ParseUri(source)
	if err != nil {
		return fmt.Errorf("failed to parse source uri %s: %w", source, err)
	}

	if err = sourceUri.DownloadToFile(ctx, destination); err != nil {
		return fmt.Errorf("failed to download dataset %s: %w", source, err)
	}

	return nil
}

// uploadFile upload content from local file to storage file (URI) destination.
func (h *handlerConsolidation) uploadFile(ctx context.Context, source, destination string) error {
	gsURI, err := uri.ParseUri(destination)
	if err != nil {
		return fmt.Errorf("failed to parse uri: %w", err)
	}

	f, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open mucog file: %w", err)
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
		log.Logger(ctx).Sugar().Debugf("cog is not reusable in state: " + strings.Join(errors, ", "))
		return "", false
	}

	if isMucogDataset {
		newCogFilePath, err := h.cog.Create(ds, container, recordID, workDir)
		if err != nil {
			return "", false
		}

		return newCogFilePath, true

	}

	return localFilePath, true
}

/*
	isSameGeoTransForm compare two geoTransfrom with fix tolerance (10^-8)
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
	if minSize == geocube.OVERVIEWS_DEFAULT_MIN_SIZE {
		minSize = 256
	}
	nb := 0
	for width > minSize && height > minSize {
		nb += 1
		width /= 2
		height /= 2
	}
	return nb
}
