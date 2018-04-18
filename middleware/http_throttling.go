//Package middleware for heketi
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//
package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/urfave/negroni"
)

var (
	throttleNow func() time.Time = time.Now
)

//ReqLimiter struct holds data related to Throttling
type ReqLimiter struct {
	maxcount     uint32
	servingCount uint32
	//counter for incoming request
	reqRecvCount uint32
	//in memeory storage for ReqLimiter
	requestCache map[string]time.Time
	lock         sync.RWMutex
	stop         chan<- interface{}
}

//Function to check can heketi can take more request
func (r *ReqLimiter) reachedMaxRequest() bool {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return r.servingCount >= r.maxcount
}

//Function to check total received request
func (r *ReqLimiter) reqReceivedcount() bool {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return r.reqRecvCount >= r.maxcount
}

//Function to add request id to the queue
func (r *ReqLimiter) incRequest(reqid string) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.requestCache[reqid] = throttleNow()
	r.servingCount++
}

//Function to remove request id to the queue
func (r *ReqLimiter) decRequest(reqID string) {
	r.lock.Lock()
	defer r.lock.Unlock()
	if _, ok := r.requestCache[reqID]; ok {
		delete(r.requestCache, reqID)
		r.servingCount--
	}
}

//Function to increment recvRequestCount
func (r *ReqLimiter) incRecvCount() {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.reqRecvCount++
}

//Function to decrement recvRequestCount
func (r *ReqLimiter) decRecvCount() {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.reqRecvCount--

}

//Function to check ID present in map
func (r *ReqLimiter) checkReqIDPresent(reqID string) bool {
	r.lock.RLock()
	defer r.lock.RUnlock()
	_, ok := r.requestCache[reqID]
	return ok

}

//NewHTTPThrottler Function to return the ReqLimiter
func NewHTTPThrottler(count uint32) *ReqLimiter {
	return &ReqLimiter{
		maxcount:     count,
		requestCache: make(map[string]time.Time),
	}

}

func (r *ReqLimiter) ServeHTTP(hw http.ResponseWriter, hr *http.Request, next http.HandlerFunc) {

	switch hr.Method {

	case http.MethodPost, http.MethodDelete:

		//recevied a request increment the counter
		//this is required ,sometimes it will take time to get response for current request
		r.incRecvCount()

		//avoid overload by checking maximum and currently received requests counts
		if !r.reachedMaxRequest() && !r.reqReceivedcount() {

			next(hw, hr)

			res, ok := hw.(negroni.ResponseWriter)
			if !ok {
				r.decRecvCount()
				return
			}
			//if request is accepted for Async operation
			if res.Status() == http.StatusAccepted {

				reqID := GetRequestID(hr.Context())
				//Add request Id to in-memory
				if reqID != "" {
					r.incRequest(reqID)
				}

			}
		} else {
			http.Error(hw, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)

		}
		//completed  request decrement the counter
		r.decRecvCount()
	case http.MethodGet:
		next(hw, hr)

		res, ok := hw.(negroni.ResponseWriter)
		if !ok {
			return
		}
		path := strings.TrimRight(hr.URL.Path, "/")
		urlPart := strings.Split(path, "/")
		if len(urlPart) >= 3 {
			if checkRespStatus(res.Status()) {
				//extract the reqID from URL
				reqID := urlPart[2]

				//check Request Id present in in-memeory
				if r.checkReqIDPresent(reqID) {
					//check operation is not pending
					if hw.Header().Get("X-Pending") != "true" {
						r.decRequest(reqID)
					}

				}
			}
		}

	default:
		next(hw, hr)
	}
	return
}

//Cleanup up function to remove stale reqID
func (r *ReqLimiter) Cleanup(ct time.Duration) {
	t := time.NewTicker(ct)
	stop := make(chan interface{})
	r.stop = stop

	defer t.Stop()
	for {
		select {
		case <-stop:
			return

		case <-t.C:
			r.lock.Lock()
			for reqID, value := range r.requestCache {

				//using time.Now() which is helps for testing
				if time.Now().Sub(value) > ct {
					delete(r.requestCache, reqID)
					r.servingCount--
				}
			}
			r.lock.Unlock()
		}
	}
}

//Stop Cleanup
func (r *ReqLimiter) Stop() {
	r.stop <- true
}

// To check success status,internalserver,See Other  code
func checkRespStatus(status int) bool {

	if status >= http.StatusOK && status < http.StatusResetContent {
		return true
	} else if status == http.StatusSeeOther {
		return true
	} else if status == http.StatusInternalServerError {
		return true
	}

	return false
}
