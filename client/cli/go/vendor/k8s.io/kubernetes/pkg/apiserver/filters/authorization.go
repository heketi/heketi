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
	"net/http"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apiserver/request"
	"k8s.io/kubernetes/pkg/auth/authorizer"
)

// WithAuthorizationCheck passes all authorized requests on to handler, and returns a forbidden error otherwise.
func WithAuthorization(handler http.Handler, getAttribs RequestAttributeGetter, a authorizer.Authorizer) http.Handler {
	if a == nil {
		glog.Warningf("Authorization is disabled")
		return handler
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		attrs, err := getAttribs.GetAttribs(req)
		if err != nil {
			internalError(w, req, err)
			return
		}
		authorized, reason, err := a.Authorize(attrs)
		if authorized {
			handler.ServeHTTP(w, req)
			return
		}
		if err != nil {
			internalError(w, req, err)
			return
		}

		glog.V(4).Infof("Forbidden: %#v, Reason: %s", req.RequestURI, reason)
		forbidden(w, req)
	})
}

// RequestAttributeGetter is a function that extracts authorizer.Attributes from an http.Request
type RequestAttributeGetter interface {
	GetAttribs(req *http.Request) (authorizer.Attributes, error)
}

type requestAttributeGetter struct {
	requestContextMapper api.RequestContextMapper
}

// NewAttributeGetter returns an object which implements the RequestAttributeGetter interface.
func NewRequestAttributeGetter(requestContextMapper api.RequestContextMapper) RequestAttributeGetter {
	return &requestAttributeGetter{requestContextMapper}
}

func (r *requestAttributeGetter) GetAttribs(req *http.Request) (authorizer.Attributes, error) {
	attribs := authorizer.AttributesRecord{}

	ctx, ok := r.requestContextMapper.Get(req)
	if !ok {
		return nil, errors.New("no context found for request")
	}

	user, ok := api.UserFrom(ctx)
	if ok {
		attribs.User = user
	}

	requestInfo, found := request.RequestInfoFrom(ctx)
	if !found {
		return nil, errors.New("no RequestInfo found in the context")
	}

	// Start with common attributes that apply to resource and non-resource requests
	attribs.ResourceRequest = requestInfo.IsResourceRequest
	attribs.Path = requestInfo.Path
	attribs.Verb = requestInfo.Verb

	attribs.APIGroup = requestInfo.APIGroup
	attribs.APIVersion = requestInfo.APIVersion
	attribs.Resource = requestInfo.Resource
	attribs.Subresource = requestInfo.Subresource
	attribs.Namespace = requestInfo.Namespace
	attribs.Name = requestInfo.Name

	return &attribs, nil
}
