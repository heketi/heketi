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

package admission

import (
	"fmt"
	"io"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	admission "k8s.io/apiserver/pkg/admission"
	"k8s.io/client-go/tools/cache"
	api "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/storage"
	storageutil "k8s.io/kubernetes/pkg/apis/storage/util"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kubeapiserveradmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"
)

const (
	PluginName = "DefaultStorageClass"
)

func init() {
	admission.RegisterPlugin(PluginName, func(config io.Reader) (admission.Interface, error) {
		plugin := newPlugin()
		return plugin, nil
	})
}

// claimDefaulterPlugin holds state for and implements the admission plugin.
type claimDefaulterPlugin struct {
	*admission.Handler
	client internalclientset.Interface

	reflector *cache.Reflector
	stopChan  chan struct{}
	store     cache.Store
}

var _ admission.Interface = &claimDefaulterPlugin{}
var _ = kubeapiserveradmission.WantsInternalClientSet(&claimDefaulterPlugin{})

// newPlugin creates a new admission plugin.
func newPlugin() *claimDefaulterPlugin {
	return &claimDefaulterPlugin{
		Handler: admission.NewHandler(admission.Create),
	}
}

func (a *claimDefaulterPlugin) SetInternalClientSet(client internalclientset.Interface) {
	a.client = client
	a.store = cache.NewStore(cache.MetaNamespaceKeyFunc)
	a.reflector = cache.NewReflector(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return client.Storage().StorageClasses().List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return client.Storage().StorageClasses().Watch(options)
			},
		},
		&storage.StorageClass{},
		a.store,
		0,
	)

	if client != nil {
		a.Run()
	}
}

// Validate ensures an authorizer is set.
func (a *claimDefaulterPlugin) Validate() error {
	if a.client == nil {
		return fmt.Errorf("missing client")
	}
	if a.reflector == nil {
		return fmt.Errorf("missing reflector")
	}
	if a.store == nil {
		return fmt.Errorf("missing store")
	}
	return nil
}

func (a *claimDefaulterPlugin) Run() {
	if a.stopChan == nil {
		a.stopChan = make(chan struct{})
	}
	a.reflector.RunUntil(a.stopChan)
}
func (a *claimDefaulterPlugin) Stop() {
	if a.stopChan != nil {
		close(a.stopChan)
		a.stopChan = nil
	}
}

// Admit sets the default value of a PersistentVolumeClaim's storage class, in case the user did
// not provide a value.
//
// 1.  Find available StorageClasses.
// 2.  Figure which is the default
// 3.  Write to the PVClaim
func (c *claimDefaulterPlugin) Admit(a admission.Attributes) error {
	if a.GetResource().GroupResource() != api.Resource("persistentvolumeclaims") {
		return nil
	}

	if len(a.GetSubresource()) != 0 {
		return nil
	}

	pvc, ok := a.GetObject().(*api.PersistentVolumeClaim)
	// if we can't convert then we don't handle this object so just return
	if !ok {
		return nil
	}

	if storageutil.HasStorageClassAnnotation(pvc.ObjectMeta) {
		// The user asked for a class.
		return nil
	}

	glog.V(4).Infof("no storage class for claim %s (generate: %s)", pvc.Name, pvc.GenerateName)

	def, err := getDefaultClass(c.store)
	if err != nil {
		return admission.NewForbidden(a, err)
	}
	if def == nil {
		// No default class selected, do nothing about the PVC.
		return nil
	}

	glog.V(4).Infof("defaulting storage class for claim %s (generate: %s) to %s", pvc.Name, pvc.GenerateName, def.Name)
	if pvc.ObjectMeta.Annotations == nil {
		pvc.ObjectMeta.Annotations = map[string]string{}
	}
	pvc.Annotations[storageutil.StorageClassAnnotation] = def.Name
	return nil
}

// getDefaultClass returns the default StorageClass from the store, or nil.
func getDefaultClass(store cache.Store) (*storage.StorageClass, error) {
	defaultClasses := []*storage.StorageClass{}
	for _, c := range store.List() {
		class, ok := c.(*storage.StorageClass)
		if !ok {
			return nil, errors.NewInternalError(fmt.Errorf("error converting stored object to StorageClass: %v", c))
		}
		if storageutil.IsDefaultAnnotation(class.ObjectMeta) {
			defaultClasses = append(defaultClasses, class)
			glog.V(4).Infof("getDefaultClass added: %s", class.Name)
		}
	}

	if len(defaultClasses) == 0 {
		return nil, nil
	}
	if len(defaultClasses) > 1 {
		glog.V(4).Infof("getDefaultClass %s defaults found", len(defaultClasses))
		return nil, errors.NewInternalError(fmt.Errorf("%d default StorageClasses were found", len(defaultClasses)))
	}
	return defaultClasses[0], nil
}
