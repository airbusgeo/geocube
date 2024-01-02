package svc

import (
	"context"
	"os"
	"path"

	"github.com/airbusgeo/geocube/interface/storage/uri"
	internalImage "github.com/airbusgeo/geocube/internal/image"
	"github.com/airbusgeo/geocube/internal/log"
	"github.com/airbusgeo/geocube/internal/utils"
	"github.com/google/uuid"
)

var PredownloadDir = os.TempDir()

// DownloadAck to acknowledge that a job result can be released (ctx.Done() must be checked before acknowledgment)
type DownloadAck struct {
	Ctx     context.Context
	AckChan chan<- struct{}
}

// downloadJobResult is the result of a download job. The file at the Uri is retained as long as Ack is not acknowledged.
type downloadJobResult struct {
	Uri string
	Ack DownloadAck
}

// downloadJob is a job to download locally the uri, required <ref_count> times. The local uri will be send in the resultChan as soon as it is available.
type downloadJob struct {
	uri        uri.DefaultUri
	refCount   int
	resultChan chan downloadJobResult
}

type DatasetsAvailability map[string]<-chan downloadJobResult // give availability channel for a given URI

func downloadRemoteDatasetsWorker(ctx context.Context, jobs <-chan downloadJob) {
	downloadDir := path.Join(PredownloadDir, "geocube_downloader")
	if err := os.MkdirAll(downloadDir, 0777); err != nil {
		log.Logger(ctx).Sugar().Errorf("downloadRemoteDatasetsWorker[%s]: %v", downloadDir, err)
	}
	for j := range jobs {
		if !utils.IsCancelled(ctx) {
			destFile := downloadJobResult{
				Uri: path.Join(downloadDir, uuid.New().String()),
			}
			if err := j.uri.DownloadToFile(ctx, destFile.Uri); err != nil {
				log.Logger(ctx).Warn("download.DownloadToFile: " + err.Error())
				if err := os.Remove(destFile.Uri); err != nil {
					log.Logger(ctx).Warn("download.DownloadToFile.Remove: " + err.Error())
				}
				destFile.Uri = j.uri.String()
			} else {
				removeChan := make(chan struct{}, 1)
				destCtx, cancelRemoveChan := context.WithCancel(ctx)
				destFile.Ack = DownloadAck{Ctx: destCtx, AckChan: removeChan}
				go removeLocalFileWorker(ctx, destFile.Uri, removeChan, j.refCount, cancelRemoveChan)
			}
			for i := 0; i < j.refCount; i++ {
				j.resultChan <- destFile
			}
		}
		close(j.resultChan)
	}
}

func removeLocalFileWorker(ctx context.Context, uri string, acq chan struct{}, count int, cancelWorker context.CancelFunc) {
	defer func() {
		cancelWorker()
		close(acq)
		if err := os.Remove(uri); err != nil {
			log.Logger(ctx).Sugar().Errorf("removeFile[%s]: %v", uri, err.Error())
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-acq:
			if count -= 1; count == 0 {
				return
			}
			log.Logger(ctx).Sugar().Debugf("removeFile[%s]: %d remain(s)", uri, count)
		}
	}
}

func PredownloadRemoteDatasets(ctx context.Context, slices []SliceMeta, datasetsAvailability []DatasetsAvailability) {
	// Get all remote files in order of appearance
	downloadIndex := map[string]int{}
	downloadJobs := []downloadJob{}
	for _, slice := range slices {
		for _, dataset := range slice.Datasets {
			if path, err := uri.ParseUri(dataset.URI); err == nil && path.Protocol() != "file" {
				if idx, ok := downloadIndex[dataset.URI]; ok {
					downloadJobs[idx].refCount += 1
				} else {
					downloadIndex[dataset.URI] = len(downloadIndex)
					downloadJobs = append(downloadJobs, downloadJob{uri: path, refCount: 1})
				}
			}
		}
	}
	// Create a jobResult channel for each job
	for i, j := range downloadJobs {
		downloadJobs[i].resultChan = make(chan downloadJobResult, j.refCount)
	}
	// Link jobResultChannels to each slice
	for i, slice := range slices {
		availability := map[string]<-chan downloadJobResult{}
		for _, dataset := range slice.Datasets {
			if idx, ok := downloadIndex[dataset.URI]; ok {
				availability[dataset.URI] = downloadJobs[idx].resultChan
			}
		}
		datasetsAvailability[i] = availability
	}
	// Download each file
	downloadChan := make(chan downloadJob, len(downloadJobs))
	for _, job := range downloadJobs {
		downloadChan <- job
	}
	close(downloadChan)
	go downloadRemoteDatasetsWorker(ctx, downloadChan)
}

func WaitForAvailability(datasets []*internalImage.Dataset, availability DatasetsAvailability) []DownloadAck {
	var acqs []DownloadAck
	for uri, available := range availability {
		if res := <-available; res.Ack.Ctx != nil && uri != res.Uri {
			for _, dataset := range datasets {
				if dataset.URI == uri {
					dataset.URI = res.Uri
				}
			}
			acqs = append(acqs, res.Ack)
		}
	}
	return acqs
}
