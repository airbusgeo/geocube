// Copyright 2021 Airbus Defence and Space
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gcs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"syscall"

	"cloud.google.com/go/storage"
	"github.com/airbusgeo/errs"
	"github.com/airbusgeo/osio/internal"
	"google.golang.org/api/googleapi"
)

type Handler struct {
	ctx              context.Context
	client           *storage.Client
	billingProjectID string
}

//Option is an option that can be passed to RegisterHandler
type GCSOption func(o *Handler)

// Client sets the cloud.google.com/go/storage.Client that will be used
// by the handler
func GCSClient(cl *storage.Client) GCSOption {
	return func(o *Handler) {
		o.client = cl
	}
}

// BillingProject sets the project name which should be billed for the requests.
// This is mandatory if the bucket is in requester-pays mode.
func GCSBillingProject(projectID string) GCSOption {
	return func(o *Handler) {
		o.billingProjectID = projectID
	}
}

// Handle creates a KeyStreamerAt suitable for constructing an Adapter
// that accesses objects on Google Cloud Storage
func Handle(ctx context.Context, opts ...GCSOption) (*Handler, error) {
	handler := &Handler{
		ctx: ctx,
	}
	for _, o := range opts {
		o(handler)
	}
	if handler.client == nil {
		cl, err := storage.NewClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("storage.newclient: %w", err)
		}
		handler.client = cl
	}
	return handler, nil
}

type readWrapper struct {
	io.ReadCloser
}

func (r readWrapper) Read(buf []byte) (int, error) {
	n, err := r.ReadCloser.Read(buf)
	if err != nil {
		return n, errs.AddTemporaryCheck(err)
	}
	return n, nil
}
func (r readWrapper) Close() error {
	err := r.ReadCloser.Close()
	if err != nil {
		return errs.AddTemporaryCheck(err)
	}
	return nil
}

func (gcs *Handler) StreamAt(key string, off int64, n int64) (io.ReadCloser, int64, error) {
	bucket, object, err := internal.BucketObject(key)
	if err != nil {
		return nil, 0, err
	}
	gbucket := gcs.client.Bucket(bucket)
	if gcs.billingProjectID != "" {
		gbucket = gbucket.UserProject(gcs.billingProjectID)
	}
	r, err := gbucket.Object(object).NewRangeReader(gcs.ctx, off, n)
	if err != nil {
		var gerr *googleapi.Error
		if off > 0 && errors.As(err, &gerr) && gerr.Code == 416 {
			return nil, 0, io.EOF
		}
		if errors.Is(err, storage.ErrObjectNotExist) || errors.Is(err, storage.ErrBucketNotExist) {
			return nil, -1, syscall.ENOENT
		}
		err = errs.AddTemporaryCheck(err)
		return nil, 0, fmt.Errorf("new reader for gs://%s/%s: %w", bucket, object, err)
	}
	return readWrapper{r}, r.Attrs.Size, nil
}

func (gcs *Handler) ReadAt(key string, p []byte, off int64) (int, int64, error) {
	panic("deprecated (kept for retrocompatibility)")
}
