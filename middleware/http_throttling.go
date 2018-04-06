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
	//in memeory storage for ReqLimiter
	requestCache map[string]time.Time
	lock         sync.RWMutex
	stop         chan<- interface{}
}

//in memeory storage for ReqLimiter
//var limiter ReqLimiter

//Function to check can heketi can take more request
func (r *ReqLimiter) reachedMaxRequest() bool {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return r.ServingCount >= r.Maxcount
}

//Function to add request id to the queue
func (r *ReqLimiter) incRequest(reqid string) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.RequestCache[reqid] = time.Now()
	r.ServingCount++
}

//Function to remove request id to the queue
func (r *ReqLimiter) decRequest(reqid string) {
	r.lock.Lock()
	defer r.lock.Unlock()
	delete(r.RequestCache, reqid)
	r.ServingCount--

}

//NewHTTPThrottler Function to return the ReqLimiter
func NewHTTPThrottler(count uint32) *ReqLimiter {
	return &ReqLimiter{
		Maxcount:     count,
		RequestCache: make(map[string]time.Time),
	}

}

func (r *ReqLimiter) ServeHTTP(hw http.ResponseWriter, hr *http.Request, next http.HandlerFunc) {

	switch hr.Method {

	case http.MethodPost, http.MethodDelete:
		if !r.reachedMaxRequest() {

			next(hw, hr)

			res, ok := hw.(negroni.ResponseWriter)
			if !ok {
				return
			}
			//if request is accepted for Async operation
			if res.Status() == http.StatusAccepted {
				reqID := res.Header().Get("X-Request-ID")
				//Add request Id to in-memory
				if reqID != "" {
					r.incRequest(reqID)
				}

			}
		} else {
			http.Error(hw, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)

		}
	case http.MethodGet:
		next(hw, hr)

		res, ok := hw.(negroni.ResponseWriter)
		if !ok {
			return
		}
		urlPart := strings.Split(hr.URL.Path, "/")
		if len(urlPart) >= 3 {
			if isSuccess(res.Status()) || res.Status() == http.StatusInternalServerError {
				//extract the reqID from URL
				reqID := urlPart[2]

				//check Request Id present in im-memeory
				if _, ok := r.RequestCache[reqID]; ok {

					//check operation is not pending
					if hr.Header.Get("X-Pending") != "true" {
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
func (r *ReqLimiter) Cleanup(ct uint32) {
	c := time.Duration(ct)
	t := time.NewTicker(c * time.Minute)
	r.lock.Lock()
	defer r.lock.Unlock()
	for {
		select {
		case <-t.C:
			for reqID, value := range r.RequestCache {
				if time.Now().Sub(value) > c {
					delete(r.RequestCache, reqID)
					r.ServingCount--
				}
			}
		}
	}
}

// To check success status code
func isSuccess(status int) bool {

	if status >= http.StatusOK && status < http.StatusResetContent {
		return true
	}
	return false
}
