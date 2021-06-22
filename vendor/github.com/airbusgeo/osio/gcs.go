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

package osio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"syscall"

	"cloud.google.com/go/storage"
	"google.golang.org/api/googleapi"
)

type GCSHandler struct {
	ctx              context.Context
	client           *storage.Client
	billingProjectID string
}

//Option is an option that can be passed to RegisterHandler
type GCSOption func(o *GCSHandler)

// Client sets the cloud.google.com/go/storage.Client that will be used
// by the handler
func GCSClient(cl *storage.Client) GCSOption {
	return func(o *GCSHandler) {
		o.client = cl
	}
}

// BillingProject sets the project name which should be billed for the requests.
// This is mandatory if the bucket is in requester-pays mode.
func GCSBillingProject(projectID string) GCSOption {
	return func(o *GCSHandler) {
		o.billingProjectID = projectID
	}
}

// GCSHandle creates a KeyReaderAt suitable for constructing an Adapter
// that accesses objects on Google Cloud Storage
func GCSHandle(ctx context.Context, opts ...GCSOption) (*GCSHandler, error) {
	handler := &GCSHandler{
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

func gcsparse(gsUri string) (bucket, object string) {
	gsUri = strings.TrimPrefix(gsUri, "gs://")
	gsUri = strings.TrimLeft(gsUri, "/")
	firstSlash := strings.Index(gsUri, "/")
	if firstSlash == -1 {
		bucket = gsUri
		object = ""
	} else {
		bucket = gsUri[0:firstSlash]
		object = gsUri[firstSlash+1:]
	}
	return
}

func (gcs *GCSHandler) ReadAt(key string, p []byte, off int64) (int, int64, error) {
	bucket, object := gcsparse(key)
	if len(bucket) == 0 || len(object) == 0 {
		return 0, 0, fmt.Errorf("invalid key")
	}
	gbucket := gcs.client.Bucket(bucket)
	if gcs.billingProjectID != "" {
		gbucket = gbucket.UserProject(gcs.billingProjectID)
	}
	r, err := gbucket.Object(object).NewRangeReader(gcs.ctx, off, int64(len(p)))
	//fmt.Printf("read %s [%d-%d]\n", key, off, off+int64(len(p)))
	if err != nil {
		var gerr *googleapi.Error
		if off > 0 && errors.As(err, &gerr) && gerr.Code == 416 {
			return 0, 0, io.EOF
		}
		if errors.Is(err, storage.ErrObjectNotExist) || errors.Is(err, storage.ErrBucketNotExist) {
			return 0, -1, syscall.ENOENT
		}
		return 0, 0, fmt.Errorf("new reader for gs://%s/%s: %w", bucket, object, err)
	}
	defer r.Close()
	n, err := io.ReadFull(r, p)
	if err == io.ErrUnexpectedEOF {
		err = io.EOF
	}
	return n, r.Attrs.Size, err
}
