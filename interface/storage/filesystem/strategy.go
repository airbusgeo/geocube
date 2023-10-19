package filesystem

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	geocubeStorage "github.com/airbusgeo/geocube/interface/storage"
	"github.com/airbusgeo/geocube/internal/utils"
)

type fileSystemStrategy struct {
}

func NewFileSystemStrategy(ctx context.Context) (geocubeStorage.Strategy, error) {
	return fileSystemStrategy{}, nil
}

func formatError(err error) error {
	var epath *os.PathError
	if errors.As(err, &epath) && os.IsNotExist(epath) {
		return geocubeStorage.ErrFileNotFound
	}
	return err
}

func (s fileSystemStrategy) Download(ctx context.Context, uri string, options ...geocubeStorage.Option) ([]byte, error) {
	uri = strings.Replace(uri, "file://", "", -1)
	f, err := os.Open(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", formatError(err))
	}

	defer f.Close()
	return io.ReadAll(f)
}

func (s fileSystemStrategy) DownloadToFile(ctx context.Context, source, destination string, options ...geocubeStorage.Option) error {
	sourceURI := strings.Replace(source, "file://", "", -1)
	destURI := strings.Replace(destination, "file://", "", -1)
	sourceFile, err := os.Open(sourceURI)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", formatError(err))
	}

	defer sourceFile.Close()

	if _, err := os.Stat(filepath.Dir(destURI)); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(destURI), os.ModePerm); err != nil {
			return err
		}
	}

	destFile, err := os.Create(destURI)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}

	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}
	return nil
}

func (s fileSystemStrategy) Upload(ctx context.Context, uri string, data []byte, options ...geocubeStorage.Option) error {
	uri = strings.Replace(uri, "file://", "", -1)

	if _, err := os.Stat(filepath.Dir(uri)); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(uri), os.ModePerm); err != nil {
			return err
		}
	}

	f, err := os.Create(uri)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	defer f.Close()

	_, err = io.Copy(f, bytes.NewReader(data))
	return err
}

func (s fileSystemStrategy) UploadFile(ctx context.Context, uri string, data io.ReadCloser, options ...geocubeStorage.Option) error {
	uri = strings.Replace(uri, "file://", "", -1)

	if _, err := os.Stat(filepath.Dir(uri)); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(uri), os.ModePerm); err != nil {
			return err
		}
	}

	f, err := os.Create(uri)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	_, err = io.Copy(f, data)
	return err
}

func (s fileSystemStrategy) Delete(ctx context.Context, uri string, options ...geocubeStorage.Option) error {
	opts := geocubeStorage.Apply(options...)

	if err := os.Remove(uri); err != nil {
		if !opts.IgnoreNotFound || !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove file: %w", err)
		}
	}

	return nil
}

func (s fileSystemStrategy) BulkDelete(ctx context.Context, uris []string, options ...geocubeStorage.Option) error {
	workers := 20
	maxErrors := int64(100)
	if len(uris) < workers {
		workers = len(uris)
	}
	tasks := make(chan string)
	wg := utils.ErrWaitGroup{}

	nbErrors := atomic.Int64{}
	for range workers {
		wg.Go(func() error {
			for uri := range tasks {
				if err := s.Delete(ctx, uri, options...); err != nil {
					if nbErrors.Add(1) < maxErrors {
						wg.AppendError(err)
					}
				}
			}
			return nil
		})
	}
	for _, uri := range uris {
		tasks <- uri
	}
	close(tasks)

	errs := utils.MergeErrors(true, nil, wg.Wait()...)
	if nbErrors.Load() >= maxErrors {
		errs = utils.MergeErrors(true, errs, utils.MakeTemporary(fmt.Errorf("[...] total: %d errors", nbErrors.Load())))
	}

	return errs
}

func (s fileSystemStrategy) Exist(ctx context.Context, uri string) (bool, error) {
	if _, err := os.Stat(uri); err != nil {
		if os.IsNotExist(err) {
			return false, geocubeStorage.ErrFileNotFound
		}
		return false, err
	}
	return true, nil
}

func (s fileSystemStrategy) GetAttrs(ctx context.Context, uri string) (geocubeStorage.Attrs, error) {
	f, err := os.Open(uri)
	if err != nil {
		return geocubeStorage.Attrs{}, fmt.Errorf("failed to open file: %w", formatError(err))
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return geocubeStorage.Attrs{}, err
	}

	// Only the first 512 bytes are used to sniff the content type.
	buffer := make([]byte, 512)
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return geocubeStorage.Attrs{}, err
	}

	b, err := f.Read(buffer)
	if err != nil && err != io.EOF {
		return geocubeStorage.Attrs{}, err
	}

	buffer = buffer[:b]

	// Always returns a valid content-type and "application/octet-stream"
	// if no others seemed to match.
	contentType := http.DetectContentType(buffer)
	return geocubeStorage.Attrs{
		ContentType:  contentType,
		StorageClass: "filesystem",
		Size:         fi.Size(),
	}, nil
}

func (s fileSystemStrategy) StreamAt(key string, off int64, n int64) (io.ReadCloser, int64, error) {
	f, err := os.Open(key)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open file: %w", formatError(err))
	}
	defer f.Close()

	return io.NopCloser(f), 0, nil
}
