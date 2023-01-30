package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	ErrFileNotFound = errors.New("file not found")
)

type Strategy interface {
	Download(ctx context.Context, uri string, options ...Option) ([]byte, error)
	DownloadToFile(ctx context.Context, source string, destination string, options ...Option) error
	Upload(ctx context.Context, uri string, data []byte, options ...Option) error
	UploadFile(ctx context.Context, uri string, data io.ReadCloser, options ...Option) error
	Delete(ctx context.Context, uri string, options ...Option) error
	Exist(ctx context.Context, uri string) (bool, error)
	GetAttrs(ctx context.Context, uri string) (Attrs, error)
	StreamAt(key string, off int64, n int64) (io.ReadCloser, int64, error)
}

func NewStorageClient(ctx context.Context, storageStrategy Strategy) (*Client, error) {
	return &Client{StorageStrategy: storageStrategy}, nil
}

type Client struct {
	StorageStrategy Strategy
}

/*
Download enables to download file content in slice of byte.
*/
func (c *Client) Download(ctx context.Context, uri string, options ...Option) ([]byte, error) {
	return c.StorageStrategy.Download(ctx, uri, options...)
}

/*
DownloadTo enables to download file content in local file.
*/
func (c *Client) DownloadTo(ctx context.Context, source string, destination string, options ...Option) error {
	return c.StorageStrategy.DownloadToFile(ctx, source, destination, options...)
}

/*
Upload enables to upload file content in to remote file.
*/
func (c *Client) Upload(ctx context.Context, uri string, data []byte, options ...Option) error {
	return c.StorageStrategy.Upload(ctx, uri, data, options...)
}

/*
UploadFile enables to upload file in to remote file.
*/
func (c *Client) UploadFile(ctx context.Context, uri string, data io.ReadCloser, options ...Option) error {
	return c.StorageStrategy.UploadFile(ctx, uri, data, options...)
}

/*
Delete enables to delete file.
*/
func (c *Client) Delete(ctx context.Context, uri string, options ...Option) error {
	return c.StorageStrategy.Delete(ctx, uri, options...)
}

/*
Exist checks if file exist.
*/
func (c *Client) Exist(ctx context.Context, uri string) (bool, error) {
	return c.StorageStrategy.Exist(ctx, uri)
}

/*
GetAttrs returns file attributes.
*/
func (c *Client) GetAttrs(ctx context.Context, uri string) (Attrs, error) {
	return c.StorageStrategy.GetAttrs(ctx, uri)
}

/*
StreamAt streams storage files
*/
func (c *Client) StreamAt(key string, off int64, n int64) (io.ReadCloser, int64, error) {
	return c.StorageStrategy.StreamAt(key, off, n)
}

type Option func(o *option)

type option struct {
	MaxTries     int
	Delay        time.Duration
	StorageClass string
	Offset       int64
	Length       int64
	Exclude      ExcludeFunc
	Concurrency  int
}

type ExcludeFunc func(objectName string) bool

type Attrs struct {
	ContentType  string
	StorageClass string
}

func MaxTries(n int) Option {
	if n <= 0 {
		n = 1
	}
	return func(o *option) {
		o.MaxTries = n
	}
}

func OnErrorRetryDelay(d time.Duration) Option {
	if d < 0 {
		d = 0
	}
	return func(o *option) {
		o.Delay = d
	}
}

func StorageClass(cl string) Option {
	return func(o *option) {
		o.StorageClass = cl
	}
}

func Offset(off int64) Option {
	if off < 0 {
		panic("offset cannot be negative")
	}
	return func(o *option) {
		o.Offset = off
	}
}

func Length(l int64) Option {
	if l <= 0 {
		panic("length must be >0")
	}
	return func(o *option) {
		o.Length = l
	}
}

func Exclude(ex ExcludeFunc) Option {
	return func(o *option) {
		o.Exclude = ex
	}
}
func Concurrency(c int) Option {
	if c <= 0 {
		panic("concurrency must be >= 1")
	}
	return func(o *option) {
		o.Concurrency = c
	}
}

func Apply(opts ...Option) option {
	opt := option{
		MaxTries:    10,
		Delay:       time.Second,
		Offset:      0,
		Length:      -1,
		Concurrency: 5,
		Exclude:     func(_ string) bool { return false },
	}
	for _, o := range opts {
		o(&opt)
	}
	return opt
}
