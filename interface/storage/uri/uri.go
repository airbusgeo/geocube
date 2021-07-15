package uri

import (
	"context"
	"fmt"
	"io"
	pathPkg "path"
	"regexp"
	"strings"

	"github.com/airbusgeo/geocube/interface/storage"
	"github.com/airbusgeo/geocube/interface/storage/filesystem"
	"github.com/airbusgeo/geocube/interface/storage/gcs"
	"github.com/airbusgeo/geocube/internal/utils"
)

var (
	BadUriErr = fmt.Errorf("badly formatted storage uri")
	uriRegex  = regexp.MustCompile("^(?P<Protocol>.+)://(?P<BucketName>.+?)(/(?P<Path>(?:.*/)*(?P<FileName>.*)))?$")
)

type Factory interface {
	ParseUri(uri string) (Uri, error)
	NewUri(provider, bucketName, path string) Uri
}

type Uri interface {
	Protocol() string
	Bucket() string
	Path() string
	FileName() string
	String() string
}

type DefaultFactory struct{}

func (f *DefaultFactory) ParseUri(uri string) (Uri, error) {
	return ParseUri(uri)
}

func (f *DefaultFactory) NewUri(provider, bucketName, path string) Uri {
	return NewUri(provider, bucketName, path)
}

// ParseUri parse a storage uri (e.g. gcs://bucket-name/path/to/file)
func ParseUri(rawURI string) (DefaultUri, error) {
	if strings.HasPrefix(rawURI, "/") {
		//local path
		return DefaultUri{
			path: rawURI,
		}, nil
	}
	matches, err := utils.FindRegexGroups(uriRegex, rawURI)
	if err != nil {
		return DefaultUri{}, BadUriErr
	}

	protocol, ok := matches["Protocol"]
	if !ok {
		return DefaultUri{}, fmt.Errorf("invalid protocol: %w", BadUriErr)
	}
	bucket, ok := matches["BucketName"]
	if !ok {
		return DefaultUri{}, fmt.Errorf("invalid bucket name: %w", BadUriErr)
	}
	path, ok := matches["Path"]
	if !ok {
		return DefaultUri{}, fmt.Errorf("invalid path: %w", BadUriErr)
	}
	fileName, ok := matches["FileName"]
	if !ok {
		return DefaultUri{}, fmt.Errorf("invalid filename: %w", BadUriErr)
	}

	if protocol == "file" {
		// use full path to directory as bucket name
		bucket = pathPkg.Join(bucket, pathPkg.Dir(path))
		path = fileName
	}
	return DefaultUri{
		protocol: protocol,
		bucket:   bucket,
		path:     path,
		fileName: fileName,
	}, nil
}

func NewUri(protocol, bucketName, path string) Uri {
	return DefaultUri{
		protocol: protocol,
		bucket:   bucketName,
		path:     path,
		fileName: pathPkg.Base(path),
	}
}

type DefaultUri struct {
	protocol string
	bucket   string
	path     string
	fileName string
}

func (u DefaultUri) Protocol() string {
	return u.protocol
}

func (u DefaultUri) Bucket() string {
	return u.bucket
}

func (u DefaultUri) Path() string {
	return u.path
}

func (u DefaultUri) FileName() string {
	return u.fileName
}

func (u DefaultUri) String() string {
	if u.protocol == "" && u.bucket == "" {
		return fmt.Sprintf("%s", u.path)
	}
	return fmt.Sprintf("%s://%s/%s", u.protocol, u.bucket, u.path)
}

func (u DefaultUri) NewStorageStrategy(ctx context.Context) (storage.Strategy, error) {
	return u.getStrategy(ctx)
}

func (u DefaultUri) getStrategy(ctx context.Context) (storage.Strategy, error) {
	switch strings.ToLower(u.protocol) {
	case "gs":
		return gcs.NewGsStrategy(ctx)
	case "file", "":
		return filesystem.NewFileSystemStrategy(ctx)
	case "s3":
		return nil, fmt.Errorf("not supported yet")
	default:
		return nil, fmt.Errorf("failed to determine storage strategy")
	}
}

func (u DefaultUri) Download(ctx context.Context) ([]byte, error) {
	strategy, err := u.getStrategy(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage strategy: %w", err)
	}

	return strategy.Download(ctx, u.String())
}

func (u DefaultUri) DownloadToFile(ctx context.Context, destination string) error {
	strategy, err := u.getStrategy(ctx)
	if err != nil {
		return fmt.Errorf("failed to get storage strategy: %w", err)
	}

	return strategy.DownloadToFile(ctx, u.String(), destination)
}

func (u DefaultUri) Upload(ctx context.Context, data []byte) error {
	strategy, err := u.getStrategy(ctx)
	if err != nil {
		return fmt.Errorf("failed to get storage strategy: %w", err)
	}

	return strategy.Upload(ctx, u.String(), data)
}

func (u DefaultUri) UploadFile(ctx context.Context, data io.ReadCloser) error {
	strategy, err := u.getStrategy(ctx)
	if err != nil {
		return fmt.Errorf("failed to get storage strategy: %w", err)
	}

	return strategy.UploadFile(ctx, u.String(), data)
}

func (u DefaultUri) Delete(ctx context.Context) error {
	strategy, err := u.getStrategy(ctx)
	if err != nil {
		return fmt.Errorf("failed to get storage strategy: %w", err)
	}

	return strategy.Delete(ctx, u.String())
}

func (u DefaultUri) GetAttrs(ctx context.Context) (storage.Attrs, error) {
	strategy, err := u.getStrategy(ctx)
	if err != nil {
		return storage.Attrs{}, fmt.Errorf("failed to get storage strategy: %w", err)
	}

	return strategy.GetAttrs(ctx, u.String())
}

func (u DefaultUri) Exist(ctx context.Context) (bool, error) {
	strategy, err := u.getStrategy(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get storage strategy: %w", err)
	}
	return strategy.Exist(ctx, u.String())
}
