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

package util

import (
	"sync"

	fed_clientset "k8s.io/kubernetes/federation/client/clientset_generated/federation_internalclientset"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/typed/discovery"
	oldclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
)

func NewClientCache(loader clientcmd.ClientConfig) *ClientCache {
	return &ClientCache{
		clientsets:    make(map[unversioned.GroupVersion]*internalclientset.Clientset),
		configs:       make(map[unversioned.GroupVersion]*restclient.Config),
		fedClientSets: make(map[unversioned.GroupVersion]fed_clientset.Interface),
		loader:        loader,
	}
}

// ClientCache caches previously loaded clients for reuse, and ensures MatchServerVersion
// is invoked only once
type ClientCache struct {
	loader        clientcmd.ClientConfig
	clientsets    map[unversioned.GroupVersion]*internalclientset.Clientset
	fedClientSets map[unversioned.GroupVersion]fed_clientset.Interface
	configs       map[unversioned.GroupVersion]*restclient.Config

	matchVersion bool

	defaultConfigLock sync.Mutex
	defaultConfig     *restclient.Config
	discoveryClient   discovery.DiscoveryInterface
}

// also looks up the discovery client.  We can't do this during init because the flags won't have been set
// because this is constructed pre-command execution before the command tree is even set up
func (c *ClientCache) getDefaultConfig() (restclient.Config, discovery.DiscoveryInterface, error) {
	c.defaultConfigLock.Lock()
	defer c.defaultConfigLock.Unlock()

	if c.defaultConfig != nil && c.discoveryClient != nil {
		return *c.defaultConfig, c.discoveryClient, nil
	}

	config, err := c.loader.ClientConfig()
	if err != nil {
		return restclient.Config{}, nil, err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return restclient.Config{}, nil, err
	}
	if c.matchVersion {
		if err := discovery.MatchesServerVersion(discoveryClient); err != nil {
			return restclient.Config{}, nil, err
		}
	}

	c.defaultConfig = config
	c.discoveryClient = discoveryClient
	return *c.defaultConfig, c.discoveryClient, nil
}

// ClientConfigForVersion returns the correct config for a server
func (c *ClientCache) ClientConfigForVersion(requiredVersion *unversioned.GroupVersion) (*restclient.Config, error) {
	// TODO: have a better config copy method
	config, discoveryClient, err := c.getDefaultConfig()
	if err != nil {
		return nil, err
	}
	if requiredVersion == nil && config.GroupVersion != nil {
		// if someone has set the values via flags, our config will have the groupVersion set
		// that means it is required.
		requiredVersion = config.GroupVersion
	}

	// required version may still be nil, since config.GroupVersion may have been nil.  Do the check
	// before looking up from the cache
	if requiredVersion != nil {
		if config, ok := c.configs[*requiredVersion]; ok {
			return config, nil
		}
	}

	negotiatedVersion, err := discovery.NegotiateVersion(discoveryClient, requiredVersion, registered.EnabledVersions())
	if err != nil {
		return nil, err
	}
	config.GroupVersion = negotiatedVersion

	// TODO this isn't what we want.  Each clientset should be setting defaults as it sees fit.
	oldclient.SetKubernetesDefaults(&config)

	if requiredVersion != nil {
		c.configs[*requiredVersion] = &config
	}

	// `version` does not necessarily equal `config.Version`.  However, we know that we call this method again with
	// `config.Version`, we should get the config we've just built.
	configCopy := config
	c.configs[*config.GroupVersion] = &configCopy

	return &config, nil
}

// ClientSetForVersion initializes or reuses a clientset for the specified version, or returns an
// error if that is not possible
func (c *ClientCache) ClientSetForVersion(requiredVersion *unversioned.GroupVersion) (*internalclientset.Clientset, error) {
	if requiredVersion != nil {
		if clientset, ok := c.clientsets[*requiredVersion]; ok {
			return clientset, nil
		}
	}
	config, err := c.ClientConfigForVersion(requiredVersion)
	if err != nil {
		return nil, err
	}

	clientset, err := internalclientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	c.clientsets[*config.GroupVersion] = clientset

	// `version` does not necessarily equal `config.Version`.  However, we know that if we call this method again with
	// `version`, we should get a client based on the same config we just found.  There's no guarantee that a client
	// is copiable, so create a new client and save it in the cache.
	if requiredVersion != nil {
		configCopy := *config
		clientset, err := internalclientset.NewForConfig(&configCopy)
		if err != nil {
			return nil, err
		}
		c.clientsets[*requiredVersion] = clientset
	}

	return clientset, nil
}

func (c *ClientCache) FederationClientSetForVersion(version *unversioned.GroupVersion) (fed_clientset.Interface, error) {
	if version != nil {
		if clientSet, found := c.fedClientSets[*version]; found {
			return clientSet, nil
		}
	}
	config, err := c.ClientConfigForVersion(version)
	if err != nil {
		return nil, err
	}

	// TODO: support multi versions of client with clientset
	clientSet, err := fed_clientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	c.fedClientSets[*config.GroupVersion] = clientSet

	if version != nil {
		configCopy := *config
		clientSet, err := fed_clientset.NewForConfig(&configCopy)
		if err != nil {
			return nil, err
		}
		c.fedClientSets[*version] = clientSet
	}

	return clientSet, nil
}

func (c *ClientCache) FederationClientForVersion(version *unversioned.GroupVersion) (*restclient.RESTClient, error) {
	fedClientSet, err := c.FederationClientSetForVersion(version)
	if err != nil {
		return nil, err
	}
	return fedClientSet.Federation().RESTClient().(*restclient.RESTClient), nil
}
