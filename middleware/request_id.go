//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/heketi/heketi/pkg/utils"
)

type contextKey string

var requestIDKey = contextKey("X-Request-ID")

type RequestID struct {
}

// GetRequestID returns the request id from HTTP context.
func GetRequestID(ctx context.Context) string {
	reqID, _ := ctx.Value(requestIDKey).(string)
	return reqID
}

func (reqID *RequestID) ServeHTTP(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {

	var skip bool
	// We don't need to add request id to GET ops. However, we have device resync
	// operation defined under GET which should have been a POST. As that is an API
	// change I am working around it for now.
	if r.Method == http.MethodGet {
		skip = true
		path := strings.TrimRight(r.URL.Path, "/")
		urlPart := strings.Split(path, "/")
		if len(urlPart) >= 4 {
			if urlPart[1] == "devices" && urlPart[3] == "resync" {
				skip = false
			}
		}

	}

	if skip {
		next(w, r)
	} else {
		newCtx := context.WithValue(r.Context(), requestIDKey, utils.GenUUID())
		next(w, r.WithContext(newCtx))
	}

}
