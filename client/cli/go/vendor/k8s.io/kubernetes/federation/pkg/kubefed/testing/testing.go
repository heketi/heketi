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

package testing

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"k8s.io/kubernetes/federation/pkg/kubefed/util"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/runtime"
)

type fakeAdminConfig struct {
	pathOptions *clientcmd.PathOptions
	hostFactory cmdutil.Factory
}

func NewFakeAdminConfig(f cmdutil.Factory, kubeconfigGlobal string) (util.AdminConfig, error) {
	pathOptions := clientcmd.NewDefaultPathOptions()
	pathOptions.GlobalFile = kubeconfigGlobal
	pathOptions.EnvVar = ""

	return &fakeAdminConfig{
		pathOptions: pathOptions,
		hostFactory: f,
	}, nil
}

func (f *fakeAdminConfig) PathOptions() *clientcmd.PathOptions {
	return f.pathOptions
}

func (f *fakeAdminConfig) HostFactory(host, kubeconfigPath string) cmdutil.Factory {
	return f.hostFactory
}

func FakeKubeconfigFiles() ([]string, error) {
	kubeconfigs := []clientcmdapi.Config{
		{
			Clusters: map[string]*clientcmdapi.Cluster{
				"syndicate": {
					Server: "https://10.20.30.40",
				},
			},
			AuthInfos: map[string]*clientcmdapi.AuthInfo{
				"syndicate": {
					Token: "badge",
				},
			},
			Contexts: map[string]*clientcmdapi.Context{
				"syndicate": {
					Cluster:  "syndicate",
					AuthInfo: "syndicate",
				},
			},
			CurrentContext: "syndicate",
		},
		{
			Clusters: map[string]*clientcmdapi.Cluster{
				"ally": {
					Server: "ally256.example.com:80",
				},
			},
			AuthInfos: map[string]*clientcmdapi.AuthInfo{
				"ally": {
					Token: "souvenir",
				},
			},
			Contexts: map[string]*clientcmdapi.Context{
				"ally": {
					Cluster:  "ally",
					AuthInfo: "ally",
				},
			},
			CurrentContext: "ally",
		},
		{
			Clusters: map[string]*clientcmdapi.Cluster{
				"ally": {
					Server: "https://ally64.example.com",
				},
				"confederate": {
					Server: "10.8.8.8",
				},
			},
			AuthInfos: map[string]*clientcmdapi.AuthInfo{
				"ally": {
					Token: "souvenir",
				},
				"confederate": {
					Token: "totem",
				},
			},
			Contexts: map[string]*clientcmdapi.Context{
				"ally": {
					Cluster:  "ally",
					AuthInfo: "ally",
				},
				"confederate": {
					Cluster:  "confederate",
					AuthInfo: "confederate",
				},
			},
			CurrentContext: "confederate",
		},
	}
	kubefiles := []string{}
	for _, cfg := range kubeconfigs {
		fakeKubeFile, _ := ioutil.TempFile("", "")
		err := clientcmd.WriteToFile(cfg, fakeKubeFile.Name())
		if err != nil {
			return nil, err
		}

		kubefiles = append(kubefiles, fakeKubeFile.Name())
	}
	return kubefiles, nil
}

func RemoveFakeKubeconfigFiles(kubefiles []string) {
	for _, file := range kubefiles {
		os.Remove(file)
	}
}

func DefaultHeader() http.Header {
	header := http.Header{}
	header.Set("Content-Type", runtime.ContentTypeJSON)
	return header
}

func ObjBody(codec runtime.Codec, obj runtime.Object) io.ReadCloser {
	return ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(codec, obj))))
}

func DefaultClientConfig() *restclient.Config {
	return &restclient.Config{
		APIPath: "/api",
		ContentConfig: restclient.ContentConfig{
			NegotiatedSerializer: api.Codecs,
			ContentType:          runtime.ContentTypeJSON,
			GroupVersion:         &registered.GroupOrDie(api.GroupName).GroupVersion,
		},
	}
}
