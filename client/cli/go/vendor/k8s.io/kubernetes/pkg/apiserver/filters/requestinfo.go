/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package filters

import (
	"errors"
	"fmt"
	"net/http"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apiserver/request"
)

// WithRequestInfo attaches a RequestInfo to the context.
func WithRequestInfo(handler http.Handler, resolver *request.RequestInfoFactory, requestContextMapper api.RequestContextMapper) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx, ok := requestContextMapper.Get(req)
		if !ok {
			internalError(w, req, errors.New("no context found for request"))
			return
		}

		info, err := resolver.NewRequestInfo(req)
		if err != nil {
			internalError(w, req, fmt.Errorf("failed to create RequestInfo: %v", err))
			return
		}

		requestContextMapper.Update(req, request.WithRequestInfo(ctx, info))

		handler.ServeHTTP(w, req)
	})
}
