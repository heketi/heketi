/*
Copyright 2014 The Kubernetes Authors.

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

package exists

import (
	"io"

	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"fmt"

	"k8s.io/kubernetes/pkg/admission"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/controller/informers"
)

func init() {
	admission.RegisterPlugin("NamespaceExists", func(client clientset.Interface, config io.Reader) (admission.Interface, error) {
		return NewExists(client), nil
	})
}

// exists is an implementation of admission.Interface.
// It rejects all incoming requests in a namespace context if the namespace does not exist.
// It is useful in deployments that want to enforce pre-declaration of a Namespace resource.
type exists struct {
	*admission.Handler
	client            clientset.Interface
	namespaceInformer cache.SharedIndexInformer
}

var _ = admission.WantsInformerFactory(&exists{})

func (e *exists) Admit(a admission.Attributes) (err error) {
	// if we're here, then we've already passed authentication, so we're allowed to do what we're trying to do
	// if we're here, then the API server has found a route, which means that if we have a non-empty namespace
	// its a namespaced resource.
	if len(a.GetNamespace()) == 0 || a.GetKind().GroupKind() == api.Kind("Namespace") {
		return nil
	}

	// we need to wait for our caches to warm
	if !e.WaitForReady() {
		return admission.NewForbidden(a, fmt.Errorf("not yet ready to handle request"))
	}
	namespace := &api.Namespace{
		ObjectMeta: api.ObjectMeta{
			Name:      a.GetNamespace(),
			Namespace: "",
		},
		Status: api.NamespaceStatus{},
	}
	_, exists, err := e.namespaceInformer.GetStore().Get(namespace)
	if err != nil {
		return errors.NewInternalError(err)
	}
	if exists {
		return nil
	}

	// in case of latency in our caches, make a call direct to storage to verify that it truly exists or not
	_, err = e.client.Core().Namespaces().Get(a.GetNamespace())
	if err != nil {
		if errors.IsNotFound(err) {
			return err
		}
		return errors.NewInternalError(err)
	}

	return nil
}

// NewExists creates a new namespace exists admission control handler
func NewExists(c clientset.Interface) admission.Interface {
	return &exists{
		client:  c,
		Handler: admission.NewHandler(admission.Create, admission.Update, admission.Delete),
	}
}

func (e *exists) SetInformerFactory(f informers.SharedInformerFactory) {
	e.namespaceInformer = f.Namespaces().Informer()
	e.SetReadyFunc(e.namespaceInformer.HasSynced)
}

func (e *exists) Validate() error {
	if e.namespaceInformer == nil {
		return fmt.Errorf("missing namespaceInformer")
	}
	return nil
}
