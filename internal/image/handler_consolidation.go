package image

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"


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
}

func NewHandleConsolidation(c CogGenerator, m MucogGenerator, cancelledJobsStorage string) Handler {
	return &handlerConsolidation{
		cog:                  c,
		mucog:                m,
		cancelledJobsStorage: cancelledJobsStorage,
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

	var cogListFile []string

	log.Logger(ctx).Sugar().Infof("starting to create COG files")
	for index, record := range cEvent.Records {
		recordID := record.ID
		localDatasetsByRecords := datasetsByRecords[recordID]

		pixToCRS := affine.NewAffine(
			cEvent.Container.Transform[0],
			cEvent.Container.Transform[1],
			cEvent.Container.Transform[2],
			cEvent.Container.Transform[3],
			cEvent.Container.Transform[4],
			cEvent.Container.Transform[5],
		)

		mergeDataset, err := MergeDatasets(ctx, localDatasetsByRecords, &GdalDatasetDescriptor{
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
		if err != nil {
			return fmt.Errorf("failed to merge source images: %w", err)
		}

		log.Logger(ctx).Sugar().Debugf("add cog %s for record: %s (%d/%d)", cogDatasetPath, recordID, index+1, len(cEvent.Records))
		cogListFile = append(cogListFile, cogDatasetPath)
	}

	if len(cogListFile) != len(cEvent.Records) {
		log.Logger(ctx).Sugar().Errorf("some cogs have not been generated (%d/%d)", len(cogListFile), len(cEvent.Records))
	}

	log.Logger(ctx).Sugar().Infof("%d COGs have been generated", len(cogListFile))

	if h.isCancelled(ctx, cEvent) {
		return errors.New("consolidation event is cancelled")
	}
	if len(cogListFile) == 1 {
		if err := h.uploadFile(ctx, cogListFile[0], cEvent.Container.URI); err != nil {
			return fmt.Errorf("failed to upload file on: %s : %w", cEvent.Container.URI, err)
		}
	} else {
		mucogFilePath, err := h.mucog.Create(workDir, cogListFile)
		if err != nil {
			return fmt.Errorf("failed to create mucog: %w", err)
		}

		log.Logger(ctx).Sugar().Debugf("mucog has been generated : %s", mucogFilePath)
		if err := h.uploadFile(ctx, mucogFilePath, cEvent.Container.URI); err != nil {
			return fmt.Errorf("failed to upload file on: %s : %w", cEvent.Container.URI, err)
		}
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
	if err != nil {
		log.Logger(ctx).Sugar().Errorf("failed to check uri existence: %s: %s", path, err.Error())
		return false
	}

	return exist
}

