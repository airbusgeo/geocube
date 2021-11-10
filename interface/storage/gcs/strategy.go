package gcs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	geocubeStorage "github.com/airbusgeo/geocube/interface/storage"
	"github.com/airbusgeo/geocube/internal/utils"

	"errors"
)

var (
	ErrFileNotFound = errors.New("file not found")
)

type gsStrategy struct {
	gsClient *storage.Client
}

type Writer interface {
	io.Writer
}

func gsError(err error) error {
	if err == nil {
		return nil
	}
	// grpc & oauth2 does not transfer the temporary status of error
	// see /home/varoquaux/geocube/sar/vendor/golang.org/x/oauth2/oauth2.go func (js jwtSource) Token()
	// see /home/varoquaux/geocube/sar/vendor/google.golang.org/grpc/internal/transport/http2_client.go func (t *http2Client) getTrAuthData
	if strings.Contains(err.Error(), "oauth2: cannot fetch token:") {
		if strings.Contains(err.Error(), "read: connection refused") || strings.Contains(err.Error(), "dial tcp: i/o timeout") {
			return utils.MakeTemporary(err)
		}
	}

	// Unexpected EOF is a temporary error
	if strings.HasSuffix(err.Error(), "EOF") {
		return utils.MakeTemporary(err)
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
			return false, fmt.Errorf("object not exist: %w", err)
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
		return geocubeStorage.Attrs{}, ErrFileNotFound
	} else if err != nil {
		return geocubeStorage.Attrs{}, fmt.Errorf("failed to get file attributes from GCS : %w", gsError(err))
	}

	return geocubeStorage.Attrs{
		StorageClass: attrs.StorageClass,
		ContentType:  attrs.ContentType,
	}, nil
}

func (s *gsStrategy) decodeURI(ctx context.Context, uri string) (string, string, error) {
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
		if err != nil {
			err = gsError(err)
			if utils.Temporary(err) {
				curOffset += n
				if bytesRemaining > 0 {
					bytesRemaining -= n
				}
				continue
			} else {
				return fmt.Errorf("copy: %w", err)
			}
		}

		break //err==nil
	}
	return err
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
		err = w.Close()
		if err != nil {
			err = gsError(err)
			if utils.Temporary(err) {
				continue
			} else {
				return fmt.Errorf("w.close: %w", err)
			}
		}
		break //err==nil
	}
	return err
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
		err = s.gsClient.Bucket(bucket).Object(object).Delete(ctx)
		if err != nil {
			err = gsError(err)
			if utils.Temporary(err) {
				continue
			} else {
				return fmt.Errorf("w.close: %w", err)
			}
		}

		break //err==nil
	}
	return err
}
