//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/lpabon/godbc"

	"github.com/heketi/heketi/executors"
	"github.com/heketi/heketi/pkg/idgen"
)

type OpClass int

const (
	TrackNormal OpClass = iota
	TrackToken
)

// OpTracker is used to track and manage how many operations are being
// processed by the server.
type OpTracker struct {
	// configuration
	Limit uint64

	// internals
	lock      sync.RWMutex
	normalOps map[string]bool
}

func newOpTracker(limit uint64) *OpTracker {
	return &OpTracker{
		Limit:     limit,
		normalOps: make(map[string]bool),
	}
}

func (ot *OpTracker) insert(id string, c OpClass) {
	godbc.Require(id != "", "id must not be empty")
	godbc.Require(!ot.normalOps[id], "id already tracked", id)
	ot.normalOps[id] = true
}

// Add records a new in-flight operation.
func (ot *OpTracker) Add(id string, c OpClass) {
	ot.lock.Lock()
	defer ot.lock.Unlock()
	ot.insert(id, c)
}

// Remove removes an operation from tracking.
func (ot *OpTracker) Remove(id string) {
	ot.lock.Lock()
	defer ot.lock.Unlock()
	godbc.Require(id != "", "id must not be empty")
	godbc.Require(ot.normalOps[id], "id not tracked", id)
	delete(ot.normalOps, id)
}

// Get returns the number of operations currently tracked.
func (ot *OpTracker) Get() uint64 {
	ot.lock.RLock()
	defer ot.lock.RUnlock()
	return uint64(len(ot.normalOps))
}

// ThrottleOrAdd returns true if adding the operation would put
// the number of operations over the limit, otherwise it adds
// the operation and returns false. ThrottleOrAdd exists to perform
// the check and set atomically.
func (ot *OpTracker) ThrottleOrAdd(id string, c OpClass) bool {
	ot.lock.Lock()
	defer ot.lock.Unlock()
	n := len(ot.normalOps)
	if uint64(n) >= ot.Limit {
		logger.Warning(
			"operations in-flight (%v) exceeds limit (%v)", n, ot.Limit)
		return true
	}
	ot.insert(id, c)
	return false
}

// ThrottleOrToken exists for use cases where throttling is required
// but a pre-existing unique identifier does not. It will return
// true and an empty-string if the number of operations is over the limit,
// otherwise it return false, and a unique token. This token must
// be passed to Remove to indicate the operation has completed.
func (ot *OpTracker) ThrottleOrToken() (bool, string) {
	token := idgen.GenUUID()
	if ot.ThrottleOrAdd(token, TrackToken) {
		return true, ""
	}
	return false, token
}

// AsyncHttpOperation runs all the steps of an operation with the long-running
// parts wrapped in an async http function. If AsyncHttpOperation returns nil
// then it has started the async function and the caller should respond to the
// client with success - otherwise an error object is returned. In the async
// function the Exec and Finalize or Rollback steps of the operation will be
// performed.
func AsyncHttpOperation(app *App,
	w http.ResponseWriter,
	r *http.Request,
	op Operation) error {

	// check if the request needs to be rate limited
	if app.optracker.ThrottleOrAdd(op.Id(), TrackNormal) {
		return ErrTooManyOperations
	}

	label := op.Label()
	if err := op.Build(); err != nil {
		logger.LogError("%v Build Failed: %v", label, err)
		// creating the operation db data failed. this is no longer
		// an in-flight operation
		app.optracker.Remove(op.Id())
		return err
	}

	app.asyncManager.AsyncHttpRedirectFunc(w, r, func() (string, error) {
		// decrement the op counter once the operation is done
		// either success or failure
		defer app.optracker.Remove(op.Id())
		logger.Info("Started async operation: %v", label)
		if err := op.Exec(app.executor); err != nil {
			if IsRetry(err) && op.MaxRetries() > 0 {
				logger.Warning("%v Exec requested retry", label)
				err := retryOperation(op, app.executor)
				if err != nil {
					return "", err
				}
				return op.ResourceUrl(), nil
			}
			if rerr := op.Rollback(app.executor); rerr != nil {
				logger.LogError("%v Rollback error: %v", label, rerr)
			}
			logger.LogError("%v Failed: %v", label, err)
			return "", err
		}
		if err := op.Finalize(); err != nil {
			logger.LogError("%v Finalize failed: %v", label, err)
			return "", err
		}
		logger.Info("%v succeeded", label)
		return op.ResourceUrl(), nil
	})
	return nil
}

// RunOperation performs all steps of an Operation and returns
// an error if any of those steps fail. This function is meant to
// make it easy to run an operation outside of the rest endpoints
// and should only be used in test code.
func RunOperation(o Operation,
	executor executors.Executor) (err error) {

	label := o.Label()
	defer func() {
		if err != nil {
			logger.LogError("Error in %v: %v", label, err)
		}
	}()

	logger.Info("Running %v", o.Label())
	if err := o.Build(); err != nil {
		logger.LogError("%v Build Failed: %v", label, err)
		return err
	}
	if err := o.Exec(executor); err != nil {
		if IsRetry(err) && o.MaxRetries() > 0 {
			logger.Warning("%v Exec requested retry", label)
			return retryOperation(o, executor)
		}
		if rerr := o.Rollback(executor); rerr != nil {
			logger.LogError("%v Rollback error: %v", label, rerr)
		}
		logger.LogError("%v Failed: %v", label, err)
		return err
	}
	if err := o.Finalize(); err != nil {
		return err
	}
	return nil
}

func retryOperation(o Operation,
	executor executors.Executor) (err error) {

	label := o.Label()
	max := o.MaxRetries()
	for i := 0; i < max; i++ {
		logger.Info("Retry %v (%v)", label, i+1)
		if e := o.Rollback(executor); e != nil {
			// when retrying rollback must succeed cleanly or it
			// is not safe to retry
			logger.LogError("%v Rollback error: %v", label, e)
			return e
		}
		if e := o.Build(); e != nil {
			logger.LogError("%v Build Failed: %v", label, e)
			return e
		}
		err = o.Exec(executor)
		if err == nil {
			// exec succeeded. Finalize it and we're outta here.
			return o.Finalize()
		}
		logger.LogError("%v Failed: %v", label, err)
		if !IsRetry(err) {
			break
		}
	}
	if e := o.Rollback(executor); e != nil {
		logger.LogError("%v Rollback error: %v", label, e)
	}
	// if we exceeded our retries, pull the "real" error out
	// of the retry error so we return that
	if oe := Original(err); oe != nil {
		err = oe
	}
	return
}

// OperationHttpErrorf writes the appropriate http error responses for
// errors returned from AsyncHttpOperation, as well as formatting the
// given error response string.
func OperationHttpErrorf(
	w http.ResponseWriter, e error, f string, v ...interface{}) {

	var msg string
	status := http.StatusInternalServerError
	switch e {
	case ErrTooManyOperations:
		status = http.StatusTooManyRequests
		msg = "Server busy. Retry operation later."
	default:
		msg = fmt.Sprintf(f, v...)
	}

	http.Error(w, msg, status)
}
