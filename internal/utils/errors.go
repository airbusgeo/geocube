package utils

import (
	"context"
	"errors"
	"fmt"
	neturl "net/url"
	"sync"
	"syscall"

	"google.golang.org/api/googleapi"
)

type errTmpIf interface{ Temporary() bool }
type errTmp struct{ error }

func (t errTmp) Temporary() bool { return true }
func (t *errTmp) Unwrap() error  { return t.error }

func MakeTemporary(err error) error {
	return errTmp{err}
}

//Temporary inspects the error trace and returns whether the error is transient
func Temporary(err error) bool {
	var uerr *neturl.Error
	if errors.As(err, &uerr) {
		err = uerr.Err
	}

	//First override some default syscall temporary statuses
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.EIO, syscall.EBUSY, syscall.ECANCELED, syscall.ECONNABORTED, syscall.ECONNRESET, syscall.ENOMEM, syscall.EPIPE:
			return true
		}
	}
	//first check explicitely marked error
	var tmp errTmpIf
	if errors.As(err, &tmp) {
		return tmp.Temporary()
	}
	var gapiError *googleapi.Error
	if errors.As(err, &gapiError) {
		return gapiError.Code == 429 || (gapiError.Code >= 500 && gapiError.Code < 600)
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return false
}

// ErrWaitGroup is a collection of goroutines working on subtasks that are part of the same overall task.
type ErrWaitGroup struct {
	wg sync.WaitGroup

	errMutex sync.Mutex
	errs     []error
}

// Wait blocks until all function calls from the Go method have returned, then
// returns all the non-nil error (if any) from them.
func (g *ErrWaitGroup) Wait() []error {
	g.wg.Wait()
	return g.errs
}

// Go calls the given function in a new goroutine.
func (g *ErrWaitGroup) Go(f func() error) {
	g.wg.Add(1)

	go func() {
		defer g.wg.Done()

		if err := f(); err != nil {
			g.errMutex.Lock()
			g.errs = append(g.errs, err)
			g.errMutex.Unlock()
		}
	}()
}

// MergeErrors, appending texts
// if priorityToErr is true, priority to the fatal error then to the temporary
// else, priority to no error, then to the temporary and finally to the fatal error.
func MergeErrors(priorityToError bool, err error, newErrs ...error) error {
	if len(newErrs) == 0 {
		return err
	}
	newErr := newErrs[0]

	if err == nil {
		err = newErr
	} else if newErr == nil {
		if !priorityToError {
			err = nil
		}
	} else if priorityToError != Temporary(newErr) {
		err = fmt.Errorf("%w\n %v", newErr, err)
	} else {
		err = fmt.Errorf("%w\n %v", err, newErr)
	}
	return MergeErrors(priorityToError, err, newErrs[1:]...)
}
