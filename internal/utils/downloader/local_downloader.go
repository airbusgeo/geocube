package downloader

import (
	"context"
	"fmt"
	"os"
	"path"
	"sync"

	"github.com/airbusgeo/geocube/interface/storage/uri"
	"github.com/airbusgeo/geocube/internal/log"

	"github.com/google/uuid"
)

type LocalFileInfo struct {
	ReferenceNb int
	Size        int64
	LocalURI    chan string
}

type downloadJob struct {
	URI        string
	ResultChan chan string
}

type LocalDownloader struct {
	filesInfo     map[string]*LocalFileInfo
	mutex         sync.Mutex
	jobs          chan downloadJob
	storageDir    string
	availableSize int64
}

func NewLocalDownloader(ctx context.Context, filesInfo map[string]*LocalFileInfo, storageDir string, availableSize int64, parallelDownloads int) *LocalDownloader {
	l := LocalDownloader{
		filesInfo:     filesInfo,
		jobs:          make(chan downloadJob, len(filesInfo)),
		availableSize: availableSize,
		storageDir:    storageDir,
	}

	l.startDownload(ctx, parallelDownloads)

	return &l
}

func (l *LocalDownloader) GetURI(remoteURI string) string {
	job := downloadJob{
		URI:        remoteURI,
		ResultChan: make(chan string),
	}
	l.jobs <- job
	return <-job.ResultChan
}

func (l *LocalDownloader) startDownload(ctx context.Context, parallelDownloads int) {

	for i := 0; i < parallelDownloads; i++ {
		go func() {
			for job := range l.jobs {
				l.download(ctx, job)
			}
		}()
	}
}

func (l *LocalDownloader) download(ctx context.Context, job downloadJob) {
	result := job.URI
	defer close(job.ResultChan)

	l.mutex.Lock()
	info, ok := l.filesInfo[job.URI]
	if ok && info.LocalURI == nil && l.availableSize >= info.Size {
		l.filesInfo[job.URI].LocalURI = make(chan string, info.ReferenceNb+1)
		l.availableSize -= info.Size
		go l.downloadFile(ctx, job.URI, info)
	}
	l.mutex.Unlock()

	if info != nil && info.LocalURI != nil {
		result = <-info.LocalURI
	}

	job.ResultChan <- result
}

func (l *LocalDownloader) downloadFile(ctx context.Context, fromURI string, info *LocalFileInfo) error {
	localURI := path.Join(l.storageDir, uuid.New().String())
	log.Logger(ctx).Sugar().Debugf("download %s to %s", fromURI, localURI)
	if err := func() error {
		uri, err := uri.ParseUri(fromURI)
		if err != nil {
			return fmt.Errorf("getLocalDatasetsByRecord: %w", err)
		}
		if err := uri.DownloadToFile(ctx, localURI); err != nil {
			return fmt.Errorf("getLocalDatasetsByRecord: %w", err)
		}
		return nil
	}(); err != nil {
		log.Logger(ctx).Sugar().Errorf("download %s to %s failed: %v", fromURI, localURI, err)
		// Release memory
		l.mutex.Lock()
		l.availableSize += info.Size
		l.mutex.Unlock()
	} else {
		// Publish localURI
		for i := 0; i <= info.ReferenceNb; i++ {
			info.LocalURI <- localURI
		}
	}
	close(info.LocalURI)

	return nil
}

func (l *LocalDownloader) ReleaseURI(ctx context.Context, uri string) {
	l.mutex.Lock()
	if info, ok := l.filesInfo[uri]; ok && info.LocalURI != nil {
		info.ReferenceNb -= 1
		if info.ReferenceNb == 0 {
			localUri := <-info.LocalURI
			log.Logger(ctx).Sugar().Debugf("remove %s", localUri)
			go os.Remove(localUri)
			l.availableSize += info.Size
		}
		//l.filesInfo[uri].LocalURI = nil
	}
	l.mutex.Unlock()
}
