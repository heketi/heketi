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

package rest

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/apis/extensions"
	extensionsapiv1beta1 "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	extensionsclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/extensions/internalversion"
	"k8s.io/kubernetes/pkg/genericapiserver"
	horizontalpodautoscaleretcd "k8s.io/kubernetes/pkg/registry/autoscaling/horizontalpodautoscaler/etcd"
	jobetcd "k8s.io/kubernetes/pkg/registry/batch/job/etcd"
	expcontrolleretcd "k8s.io/kubernetes/pkg/registry/extensions/controller/etcd"
	daemonetcd "k8s.io/kubernetes/pkg/registry/extensions/daemonset/etcd"
	deploymentetcd "k8s.io/kubernetes/pkg/registry/extensions/deployment/etcd"
	ingressetcd "k8s.io/kubernetes/pkg/registry/extensions/ingress/etcd"
	networkpolicyetcd "k8s.io/kubernetes/pkg/registry/extensions/networkpolicy/etcd"
	pspetcd "k8s.io/kubernetes/pkg/registry/extensions/podsecuritypolicy/etcd"
	replicasetetcd "k8s.io/kubernetes/pkg/registry/extensions/replicaset/etcd"
	thirdpartyresourceetcd "k8s.io/kubernetes/pkg/registry/extensions/thirdpartyresource/etcd"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/wait"
)

type RESTStorageProvider struct {
	ResourceInterface ResourceInterface
}

var _ genericapiserver.RESTStorageProvider = &RESTStorageProvider{}

func (p RESTStorageProvider) NewRESTStorage(apiResourceConfigSource genericapiserver.APIResourceConfigSource, restOptionsGetter genericapiserver.RESTOptionsGetter) (genericapiserver.APIGroupInfo, bool) {
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(extensions.GroupName)

	if apiResourceConfigSource.AnyResourcesForVersionEnabled(extensionsapiv1beta1.SchemeGroupVersion) {
		apiGroupInfo.VersionedResourcesStorageMap[extensionsapiv1beta1.SchemeGroupVersion.Version] = p.v1beta1Storage(apiResourceConfigSource, restOptionsGetter)
		apiGroupInfo.GroupMeta.GroupVersion = extensionsapiv1beta1.SchemeGroupVersion
	}

	return apiGroupInfo, true
}

func (p RESTStorageProvider) v1beta1Storage(apiResourceConfigSource genericapiserver.APIResourceConfigSource, restOptionsGetter genericapiserver.RESTOptionsGetter) map[string]rest.Storage {
	version := extensionsapiv1beta1.SchemeGroupVersion

	storage := map[string]rest.Storage{}

	if apiResourceConfigSource.ResourceEnabled(version.WithResource("horizontalpodautoscalers")) {
		hpaStorage, hpaStatusStorage := horizontalpodautoscaleretcd.NewREST(restOptionsGetter(extensions.Resource("horizontalpodautoscalers")))
		storage["horizontalpodautoscalers"] = hpaStorage
		storage["horizontalpodautoscalers/status"] = hpaStatusStorage

		controllerStorage := expcontrolleretcd.NewStorage(restOptionsGetter(api.Resource("replicationControllers")))
		storage["replicationcontrollers"] = controllerStorage.ReplicationController
		storage["replicationcontrollers/scale"] = controllerStorage.Scale
	}
	if apiResourceConfigSource.ResourceEnabled(version.WithResource("thirdpartyresources")) {
		thirdPartyResourceStorage := thirdpartyresourceetcd.NewREST(restOptionsGetter(extensions.Resource("thirdpartyresources")))
		storage["thirdpartyresources"] = thirdPartyResourceStorage
	}

	if apiResourceConfigSource.ResourceEnabled(version.WithResource("daemonsets")) {
		daemonSetStorage, daemonSetStatusStorage := daemonetcd.NewREST(restOptionsGetter(extensions.Resource("daemonsets")))
		storage["daemonsets"] = daemonSetStorage
		storage["daemonsets/status"] = daemonSetStatusStorage
	}
	if apiResourceConfigSource.ResourceEnabled(version.WithResource("deployments")) {
		deploymentStorage := deploymentetcd.NewStorage(restOptionsGetter(extensions.Resource("deployments")))
		storage["deployments"] = deploymentStorage.Deployment
		storage["deployments/status"] = deploymentStorage.Status
		storage["deployments/rollback"] = deploymentStorage.Rollback
		storage["deployments/scale"] = deploymentStorage.Scale
	}
	if apiResourceConfigSource.ResourceEnabled(version.WithResource("jobs")) {
		jobsStorage, jobsStatusStorage := jobetcd.NewREST(restOptionsGetter(extensions.Resource("jobs")))
		storage["jobs"] = jobsStorage
		storage["jobs/status"] = jobsStatusStorage
	}
	if apiResourceConfigSource.ResourceEnabled(version.WithResource("ingresses")) {
		ingressStorage, ingressStatusStorage := ingressetcd.NewREST(restOptionsGetter(extensions.Resource("ingresses")))
		storage["ingresses"] = ingressStorage
		storage["ingresses/status"] = ingressStatusStorage
	}
	if apiResourceConfigSource.ResourceEnabled(version.WithResource("podsecuritypolicy")) {
		podSecurityExtensionsStorage := pspetcd.NewREST(restOptionsGetter(extensions.Resource("podsecuritypolicy")))
		storage["podSecurityPolicies"] = podSecurityExtensionsStorage
	}
	if apiResourceConfigSource.ResourceEnabled(version.WithResource("replicasets")) {
		replicaSetStorage := replicasetetcd.NewStorage(restOptionsGetter(extensions.Resource("replicasets")))
		storage["replicasets"] = replicaSetStorage.ReplicaSet
		storage["replicasets/status"] = replicaSetStorage.Status
		storage["replicasets/scale"] = replicaSetStorage.Scale
	}
	if apiResourceConfigSource.ResourceEnabled(version.WithResource("networkpolicies")) {
		networkExtensionsStorage := networkpolicyetcd.NewREST(restOptionsGetter(extensions.Resource("networkpolicies")))
		storage["networkpolicies"] = networkExtensionsStorage
	}

	return storage
}

func (p RESTStorageProvider) PostStartHook() (string, genericapiserver.PostStartHookFunc, error) {
	return "extensions/third-party-resources", p.postStartHookFunc, nil
}
func (p RESTStorageProvider) postStartHookFunc(hookContext genericapiserver.PostStartHookContext) error {
	clientset, err := extensionsclient.NewForConfig(hookContext.LoopbackClientConfig)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to initialize clusterroles: %v", err))
		return nil
	}

	thirdPartyControl := ThirdPartyController{
		master: p.ResourceInterface,
		client: clientset,
	}
	go wait.Forever(func() {
		if err := thirdPartyControl.SyncResources(); err != nil {
			glog.Warningf("third party resource sync failed: %v", err)
		}
	}, 10*time.Second)

	return nil
}

func (p RESTStorageProvider) GroupName() string {
	return extensions.GroupName
}
