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

package errs

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"syscall"

	"google.golang.org/api/googleapi"
)

//Temporary inspects the error trace and returns wether the error is transient
func Temporary(err error) bool {
	if err == nil {
		return false
	}
	//override some default syscall temporary statuses
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.EIO, syscall.EBUSY, syscall.ECANCELED, syscall.ECONNABORTED, syscall.ECONNRESET, syscall.ENOMEM, syscall.EPIPE:
			return true
		}
	}
	//check explicitely marked error
	type tempIf interface{ Temporary() bool }
	var tmp tempIf
	if errors.As(err, &tmp) {
		return tmp.Temporary()
	}

	//google api errors
	var gapiError *googleapi.Error
	if errors.As(err, &gapiError) {
		switch gapiError.Code {
		case 429, 500, 502, 503, 504:
			return true
		}
	}

	//not really needed, as context.DeadlineExceeded implements Temporary()
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}

	//cancelled contexts
	if errors.Is(err, context.Canceled) {
		return true
	}

	//hack for https://github.com/golang/oauth2/pull/380
	errmsg := err.Error()
	if strings.Contains(errmsg, "i/o timeout") ||
		strings.Contains(errmsg, "connection reset by peer") ||
		strings.Contains(errmsg, "TLS handshake timeout") {
		return true
	}

	return false
}

type tempErr struct {
	error
}
type permErr struct {
	error
}

func (t *tempErr) Temporary() bool {
	return true
}
func (t *permErr) Temporary() bool {
	return false
}

func (t *tempErr) Unwrap() error {
	return t.error
}
func (t *permErr) Unwrap() error {
	return t.error
}
func (t *tempErr) Cause() error {
	return t.error
}
func (t *permErr) Cause() error {
	return t.error
}

func MakeTemporary(err error) error {
	return &tempErr{err}
}
func MakePermanent(err error) error {
	return &permErr{err}
}

type werr struct {
	error
}

func (t *werr) Temporary() bool {
	return Temporary(t.error)
}

// AddTemporary ensures that err exposes a `Temporary() bool` method
func AddTemporaryCheck(err error) error {
	if err == nil || err == io.EOF || err == io.ErrUnexpectedEOF {
		return err
	}
	type terr interface {
		Temporary() bool
	}
	if _, ok := err.(terr); ok {
		return err
	}
	return &werr{err}
}
