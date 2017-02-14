/*
Copyright 2015 The Kubernetes Authors.

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

package benchmark

import (
	"net/http"
	"net/http/httptest"

	"github.com/golang/glog"
	restclient "k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	v1core "k8s.io/kubernetes/pkg/client/clientset_generated/clientset/typed/core/v1"
	"k8s.io/kubernetes/pkg/client/record"
	"k8s.io/kubernetes/plugin/pkg/scheduler"
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"
	"k8s.io/kubernetes/plugin/pkg/scheduler/factory"
	"k8s.io/kubernetes/test/integration/framework"
)

// mustSetupScheduler starts the following components:
// - k8s api server (a.k.a. master)
// - scheduler
// It returns scheduler config factory and destroyFunc which should be used to
// remove resources after finished.
// Notes on rate limiter:
//   - client rate limit is set to 5000.
func mustSetupScheduler() (schedulerConfigurator scheduler.Configurator, destroyFunc func()) {

	h := &framework.MasterHolder{Initialized: make(chan struct{})}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		<-h.Initialized
		h.M.GenericAPIServer.Handler.ServeHTTP(w, req)
	}))

	framework.RunAMasterUsingServer(framework.NewIntegrationTestMasterConfig(), s, h)

	clientSet := clientset.NewForConfigOrDie(&restclient.Config{
		Host:          s.URL,
		ContentConfig: restclient.ContentConfig{GroupVersion: &api.Registry.GroupOrDie(v1.GroupName).GroupVersion},
		QPS:           5000.0,
		Burst:         5000,
	})

	schedulerConfigurator = factory.NewConfigFactory(clientSet, v1.DefaultSchedulerName, v1.DefaultHardPodAffinitySymmetricWeight, v1.DefaultFailureDomains)

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: clientSet.Core().Events("")})

	sched, err := scheduler.NewFromConfigurator(schedulerConfigurator, func(conf *scheduler.Config) {
		conf.Recorder = eventBroadcaster.NewRecorder(v1.EventSource{Component: "scheduler"})
	})
	if err != nil {
		glog.Fatalf("Error creating scheduler: %v", err)
	}

	sched.Run()

	destroyFunc = func() {
		glog.Infof("destroying")
		sched.StopEverything()
		s.Close()
		glog.Infof("destroyed")
	}
	return
}
