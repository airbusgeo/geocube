package gcs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/airbusgeo/geocube/internal/log"

	"google.golang.org/api/googleapi"

	"cloud.google.com/go/storage"
	geocubeStorage "github.com/airbusgeo/geocube/interface/storage"
	"github.com/airbusgeo/geocube/internal/utils"
)

type gsStrategy struct {
	gsClient *storage.Client
	ctx      context.Context
}

type Writer interface {
	io.Writer
}

var retriableOAuth2Errors = []string{
	"cannot assign requested address",
	"connection refused",
	"connection reset",
	"timeout",
	"broken pipe",
	"client connection force closed",
	"502 Bad Gateway",
}

var retriableSuffixErrors = []string{
	"http2: client connection lost",
	"http2: client connection force closed via ClientConn.Close",
	"EOF", // Unexpected EOF is a temporary error
}

func gsError(err error) error {
	if err == nil {
		return nil
	}
	if utils.Temporary(err) {
		return err
	}

	// grpc & oauth2 does not transfer the temporary status of error
	// see ./vendor/golang.org/x/oauth2/oauth2/jwt/jwt.go func (js jwtSource) Token()
	// see ./vendor/google.golang.org/grpc/internal/transport/http2_client.go func (t *http2Client) getTrAuthData
	if strings.Contains(err.Error(), "oauth2: cannot fetch token:") {
		for _, e := range retriableOAuth2Errors {
			if strings.Contains(err.Error(), e) {
				return utils.MakeTemporary(err)
			}
		}
	}

	for _, e := range retriableSuffixErrors {
		if strings.HasSuffix(err.Error(), e) {
			return utils.MakeTemporary(err)
		}
	}
	return err
}

func NewGsStrategy(ctx context.Context) (geocubeStorage.Strategy, error) {
	var err error

	gsClient, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create gs Client : %w", gsError(err))
	}

	return gsStrategy{
		gsClient: gsClient,
		ctx:      ctx,
	}, nil
}

func (s gsStrategy) Download(ctx context.Context, uri string, options ...geocubeStorage.Option) ([]byte, error) {
	bucket, path, err := s.decodeURI(ctx, uri)
	if err != nil {
		return nil, fmt.Errorf("failed to decode URI %s : %w", uri, err)
	}

	return s.downloadObject(ctx, bucket, path, options...)
}

func (s gsStrategy) DownloadToFile(ctx context.Context, source, destination string, options ...geocubeStorage.Option) error {
	bucket, path, err := s.decodeURI(ctx, source)
	if err != nil {
		return fmt.Errorf("failed to decode URI %s : %w", source, err)
	}

	if _, err := os.Stat(filepath.Dir(destination)); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(destination), os.ModePerm); err != nil {
			return err
		}
	}

	writer, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("failed to create destination file")
	}

	if err = s.downloadObjectTo(ctx, bucket, path, writer, options...); err != nil {
		writer.Close()
		return fmt.Errorf("failed to download object to destination: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return fmt.Errorf("DownloadToFile: failed to close writer: %w", err)
	}

	return nil
}

func (s gsStrategy) Upload(ctx context.Context, uri string, data []byte, options ...geocubeStorage.Option) error {
	bucket, object, err := s.decodeURI(ctx, uri)
	if err != nil {
		return fmt.Errorf("failed to decode URI %s : %w", uri, err)
	}

	return s.uploadObject(ctx, bucket, object, data, options...)
}

func (s gsStrategy) UploadFile(ctx context.Context, uri string, data io.ReadCloser, options ...geocubeStorage.Option) error {
	bucket, object, err := s.decodeURI(ctx, uri)
	if err != nil {
		return fmt.Errorf("failed to decode URI %s : %w", uri, err)
	}

	opts := geocubeStorage.Apply(options...)
	writer := s.gsClient.Bucket(bucket).Object(object).NewWriter(ctx)
	if opts.StorageClass != "" {
		writer.StorageClass = opts.StorageClass
	}
	_, err = io.Copy(writer, data)
	if err != nil {
		writer.Close()
		return fmt.Errorf("UploadFile: failed to copy: %w", gsError(err))

	}
	err = writer.Close()
	if err != nil {
		return fmt.Errorf("UploadFile: failed to close writer: %w", gsError(err))
	}

	return nil
}

func (s gsStrategy) Delete(ctx context.Context, uri string, options ...geocubeStorage.Option) error {
	bucket, object, err := s.decodeURI(ctx, uri)
	if err != nil {
		return fmt.Errorf("failed to decode URI %s : %w", uri, err)
	}

	return s.deleteObject(ctx, bucket, object, options...)
}

func (s gsStrategy) Exist(ctx context.Context, uri string) (bool, error) {
	bucket, object, err := s.decodeURI(ctx, uri)
	if err != nil {
		return false, fmt.Errorf("failed to decode URI %s : %w", uri, err)
	}

	if _, err = s.gsClient.Bucket(bucket).Object(object).Attrs(ctx); err != nil {
		switch err {
		case storage.ErrBucketNotExist:
			return false, fmt.Errorf("bucket not exist: %w", err)
		case storage.ErrObjectNotExist:
			return false, geocubeStorage.ErrFileNotFound
		default:
			return false, fmt.Errorf("failed to check if file exist on storage: %w", gsError(err))
		}
	}

	return true, nil
}

func (s gsStrategy) GetAttrs(ctx context.Context, uri string) (geocubeStorage.Attrs, error) {
	bucket, path, err := s.decodeURI(ctx, uri)
	if err != nil {
		return geocubeStorage.Attrs{}, fmt.Errorf("failed to decode URI %s : %w", uri, err)
	}

	attrs, err := s.gsClient.Bucket(bucket).Object(path).Attrs(ctx)
	if err == storage.ErrObjectNotExist {
		return geocubeStorage.Attrs{}, geocubeStorage.ErrFileNotFound
	} else if err != nil {
		return geocubeStorage.Attrs{}, fmt.Errorf("failed to get file attributes from GCS : %w", gsError(err))
	}

	return geocubeStorage.Attrs{
		StorageClass: attrs.StorageClass,
		ContentType:  attrs.ContentType,
	}, nil
}

var (
	metrics = make(map[string][]streamAtMetrics)
	lock    = sync.Mutex{}
)

type streamAtMetrics struct {
	Calls  int
	Volume int
}

func GetMetrics(ctx context.Context) {
	lock.Lock()
	defer lock.Unlock()
	for key, streamAtMetricsList := range metrics {
		log.Logger(ctx).Sugar().Debugf("GCS Metrics: %s - %d calls - %d octets", key, len(streamAtMetricsList), streamAtMetricsList[len(streamAtMetricsList)-1].Volume)
	}
	metrics = map[string][]streamAtMetrics{}
}

func (s gsStrategy) StreamAt(key string, off int64, n int64) (io.ReadCloser, int64, error) {
	bucket, object, err := bucketObject(key)
	if err != nil {
		return nil, 0, err
	}
	gbucket := s.gsClient.Bucket(bucket)

	r, err := gbucket.Object(object).NewRangeReader(s.ctx, off, n)
	if err != nil {
		var gerr *googleapi.Error
		if off > 0 && errors.As(err, &gerr) && gerr.Code == 416 {
			return nil, 0, io.EOF
		}
		if errors.Is(err, storage.ErrObjectNotExist) || errors.Is(err, storage.ErrBucketNotExist) {
			return nil, -1, syscall.ENOENT
		}
		err = addTemporaryCheck(err)
		return nil, 0, fmt.Errorf("new reader for gs://%s/%s: %w", bucket, object, err)
	}

	lock.Lock()
	defer lock.Unlock()
	if metrics[key] != nil {
		metrics[key] = append(metrics[key], streamAtMetrics{
			Calls:  len(metrics[key]) + 1,
			Volume: metrics[key][len(metrics[key])-1].Volume + int(n),
		})
	} else {
		metrics[key] = []streamAtMetrics{{
			Calls:  1,
			Volume: int(n),
		}}
	}

	return readWrapper{r}, r.Attrs.Size, nil
}

func (s *gsStrategy) decodeURI(_ context.Context, uri string) (string, string, error) {
	bucket, path, err := Parse(uri)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse URI : %s : %w", uri, err)
	}

	return bucket, path, nil
}

func (s gsStrategy) downloadObject(ctx context.Context, bucket, path string, opts ...geocubeStorage.Option) ([]byte, error) {
	buf := &bytes.Buffer{}
	err := s.downloadObjectTo(ctx, bucket, path, buf, opts...)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s gsStrategy) downloadObjectTo(ctx context.Context, bucket, path string, w Writer, opts ...geocubeStorage.Option) error {
	op := geocubeStorage.Apply(opts...)
	d := op.Delay
	var err error
	var r *storage.Reader
	curOffset := op.Offset
	bytesRemaining := op.Length
	for try := 0; try < op.MaxTries; try++ {
		if try > 0 {
			time.Sleep(d)
			d *= 2
		}
		bckt := s.gsClient.Bucket(bucket)
		r, err = bckt.Object(path).NewRangeReader(ctx, curOffset, bytesRemaining)
		if err != nil {
			err = gsError(err)
			if utils.Temporary(err) {
				continue
			} else {
				return fmt.Errorf("newreader: %w", err)
			}
		}

		var n int64
		n, err = io.Copy(w, r)
		r.Close()
		if err == nil {
			return nil
		}
		err = gsError(err)
		if !utils.Temporary(err) {
			return fmt.Errorf("copy: %w", err)
		}

		curOffset += n
		if bytesRemaining > 0 {
			bytesRemaining -= n
		}
	}
	return fmt.Errorf("failed after %d retries: %w", op.MaxTries, err)
}

func (s gsStrategy) uploadObject(ctx context.Context, bucket, object string, data []byte, opts ...geocubeStorage.Option) error {
	r := bytes.NewReader(data)
	return gsError(s.uploadObjectFrom(ctx, bucket, object, r, opts...))
}

func (s gsStrategy) uploadObjectFrom(ctx context.Context, bucket, object string, r io.ReadSeeker, opts ...geocubeStorage.Option) error {
	op := geocubeStorage.Apply(opts...)
	d := op.Delay
	var err error
	var w *storage.Writer
	off, _ := r.Seek(0, io.SeekCurrent)
	for try := 0; try < op.MaxTries; try++ {
		if try > 0 {
			time.Sleep(d)
			d *= 2
			_, err = r.Seek(off, io.SeekStart)
			if err != nil {
				err = gsError(err)
				if utils.Temporary(err) {
					continue
				} else {
					return fmt.Errorf("r.reset: %w", err)
				}
			}
		}
		w = s.gsClient.Bucket(bucket).Object(object).NewWriter(ctx)
		if op.StorageClass != "" {
			w.StorageClass = op.StorageClass
		}
		_, err = io.Copy(w, r)
		if err != nil {
			w.Close()
			err = gsError(err)
			if utils.Temporary(err) {
				continue
			} else {
				return fmt.Errorf("copy: %w", err)
			}
		}
		err = gsError(w.Close())
		if err == nil {
			return nil
		}
		if !utils.Temporary(err) {
			return fmt.Errorf("w.close: %w", err)
		}
	}
	return fmt.Errorf("failed after %d retries: %w", op.MaxTries, err)
}

func (s gsStrategy) deleteObject(ctx context.Context, bucket, object string, opts ...geocubeStorage.Option) error {
	op := geocubeStorage.Apply(opts...)
	d := op.Delay
	var err error
	for try := 0; try < op.MaxTries; try++ {
		if try > 0 {
			time.Sleep(d)
			d *= 2
		}
		err = gsError(s.gsClient.Bucket(bucket).Object(object).Delete(ctx))
		if err == nil {
			return nil
		}
		if !utils.Temporary(err) {
			return fmt.Errorf("w.close: %w", err)
		}
	}
	return fmt.Errorf("failed after %d retries: %w", op.MaxTries, err)
}
