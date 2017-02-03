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

package daemonset

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	federation_api "k8s.io/kubernetes/federation/apis/federation/v1beta1"
	fake_fedclientset "k8s.io/kubernetes/federation/client/clientset_generated/federation_release_1_5/fake"
	"k8s.io/kubernetes/federation/pkg/federation-controller/util"
	//"k8s.io/kubernetes/federation/pkg/federation-controller/util/deletionhelper"
	. "k8s.io/kubernetes/federation/pkg/federation-controller/util/test"
	"k8s.io/kubernetes/pkg/api/unversioned"
	api_v1 "k8s.io/kubernetes/pkg/api/v1"
	extensionsv1 "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	kubeclientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_5"
	fake_kubeclientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_5/fake"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/wait"

	"github.com/stretchr/testify/assert"
)

func TestDaemonSetController(t *testing.T) {
	cluster1 := NewCluster("cluster1", api_v1.ConditionTrue)
	cluster2 := NewCluster("cluster2", api_v1.ConditionTrue)

	fakeClient := &fake_fedclientset.Clientset{}
	RegisterFakeList("clusters", &fakeClient.Fake, &federation_api.ClusterList{Items: []federation_api.Cluster{*cluster1}})
	RegisterFakeList("daemonsets", &fakeClient.Fake, &extensionsv1.DaemonSetList{Items: []extensionsv1.DaemonSet{}})
	daemonsetWatch := RegisterFakeWatch("daemonsets", &fakeClient.Fake)
	// daemonsetUpdateChan := RegisterFakeCopyOnUpdate("daemonsets", &fakeClient.Fake, daemonsetWatch)
	clusterWatch := RegisterFakeWatch("clusters", &fakeClient.Fake)

	cluster1Client := &fake_kubeclientset.Clientset{}
	cluster1Watch := RegisterFakeWatch("daemonsets", &cluster1Client.Fake)
	RegisterFakeList("daemonsets", &cluster1Client.Fake, &extensionsv1.DaemonSetList{Items: []extensionsv1.DaemonSet{}})
	cluster1CreateChan := RegisterFakeCopyOnCreate("daemonsets", &cluster1Client.Fake, cluster1Watch)
	// cluster1UpdateChan := RegisterFakeCopyOnUpdate("daemonsets", &cluster1Client.Fake, cluster1Watch)

	cluster2Client := &fake_kubeclientset.Clientset{}
	cluster2Watch := RegisterFakeWatch("daemonsets", &cluster2Client.Fake)
	RegisterFakeList("daemonsets", &cluster2Client.Fake, &extensionsv1.DaemonSetList{Items: []extensionsv1.DaemonSet{}})
	cluster2CreateChan := RegisterFakeCopyOnCreate("daemonsets", &cluster2Client.Fake, cluster2Watch)

	daemonsetController := NewDaemonSetController(fakeClient)
	informer := ToFederatedInformerForTestOnly(daemonsetController.daemonsetFederatedInformer)
	informer.SetClientFactory(func(cluster *federation_api.Cluster) (kubeclientset.Interface, error) {
		switch cluster.Name {
		case cluster1.Name:
			return cluster1Client, nil
		case cluster2.Name:
			return cluster2Client, nil
		default:
			return nil, fmt.Errorf("Unknown cluster")
		}
	})

	daemonsetController.clusterAvailableDelay = time.Second
	daemonsetController.daemonsetReviewDelay = 50 * time.Millisecond
	daemonsetController.smallDelay = 20 * time.Millisecond
	daemonsetController.updateTimeout = 5 * time.Second

	stop := make(chan struct{})
	daemonsetController.Run(stop)

	daemonset1 := extensionsv1.DaemonSet{
		ObjectMeta: api_v1.ObjectMeta{
			Name:      "test-daemonset",
			Namespace: "ns",
			SelfLink:  "/api/v1/namespaces/ns/daemonsets/test-daemonset",
		},
		Spec: extensionsv1.DaemonSetSpec{
			Selector: &unversioned.LabelSelector{
				MatchLabels: make(map[string]string),
			},
		},
	}

	// Test add federated daemonset.
	daemonsetWatch.Add(&daemonset1)
	/*
		// TODO: Re-enable this when we have fixed these flaky tests: https://github.com/kubernetes/kubernetes/issues/36540.
		// There should be 2 updates to add both the finalizers.
		updatedDaemonSet := GetDaemonSetFromChan(daemonsetUpdateChan)
		assert.True(t, daemonsetController.hasFinalizerFunc(updatedDaemonSet, deletionhelper.FinalizerDeleteFromUnderlyingClusters))
		updatedDaemonSet = GetDaemonSetFromChan(daemonsetUpdateChan)
		assert.True(t, daemonsetController.hasFinalizerFunc(updatedDaemonSet, api_v1.FinalizerOrphan))
		daemonset1 = *updatedDaemonSet
	*/
	createdDaemonSet := GetDaemonSetFromChan(cluster1CreateChan)
	assert.NotNil(t, createdDaemonSet)
	assert.Equal(t, daemonset1.Namespace, createdDaemonSet.Namespace)
	assert.Equal(t, daemonset1.Name, createdDaemonSet.Name)
	assert.True(t, daemonsetsEqual(daemonset1, *createdDaemonSet),
		fmt.Sprintf("expected: %v, actual: %v", daemonset1, *createdDaemonSet))

	// Wait for the daemonset to appear in the informer store
	err := WaitForStoreUpdate(
		daemonsetController.daemonsetFederatedInformer.GetTargetStore(),
		cluster1.Name, getDaemonSetKey(daemonset1.Namespace, daemonset1.Name), wait.ForeverTestTimeout)
	assert.Nil(t, err, "daemonset should have appeared in the informer store")

	/*
		        // TODO: Re-enable this when we have fixed these flaky tests: https://github.com/kubernetes/kubernetes/issues/36540.
			// Test update federated daemonset.
			daemonset1.Annotations = map[string]string{
				"A": "B",
			}
			daemonsetWatch.Modify(&daemonset1)
			updatedDaemonSet = GetDaemonSetFromChan(cluster1UpdateChan)
			assert.NotNil(t, updatedDaemonSet)
			assert.Equal(t, daemonset1.Name, updatedDaemonSet.Name)
			assert.Equal(t, daemonset1.Namespace, updatedDaemonSet.Namespace)
			assert.True(t, daemonsetsEqual(daemonset1, *updatedDaemonSet),
				fmt.Sprintf("expected: %v, actual: %v", daemonset1, *updatedDaemonSet))

			// Test update federated daemonset.
			daemonset1.Spec.Template.Name = "TEST"
			daemonsetWatch.Modify(&daemonset1)
			updatedDaemonSet = GetDaemonSetFromChan(cluster1UpdateChan)
			assert.NotNil(t, updatedDaemonSet)
			assert.Equal(t, daemonset1.Name, updatedDaemonSet.Name)
			assert.Equal(t, daemonset1.Namespace, updatedDaemonSet.Namespace)
			assert.True(t, daemonsetsEqual(daemonset1, *updatedDaemonSet),
				fmt.Sprintf("expected: %v, actual: %v", daemonset1, *updatedDaemonSet))
	*/

	// Test add cluster
	clusterWatch.Add(cluster2)
	createdDaemonSet2 := GetDaemonSetFromChan(cluster2CreateChan)
	assert.NotNil(t, createdDaemonSet2)
	assert.Equal(t, daemonset1.Name, createdDaemonSet2.Name)
	assert.Equal(t, daemonset1.Namespace, createdDaemonSet2.Namespace)
	assert.True(t, daemonsetsEqual(daemonset1, *createdDaemonSet2),
		fmt.Sprintf("expected: %v, actual: %v", daemonset1, *createdDaemonSet2))

	close(stop)
}

func daemonsetsEqual(a, b extensionsv1.DaemonSet) bool {
	return util.ObjectMetaEquivalent(a.ObjectMeta, b.ObjectMeta) && reflect.DeepEqual(a.Spec, b.Spec)
}

func GetDaemonSetFromChan(c chan runtime.Object) *extensionsv1.DaemonSet {
	daemonset := GetObjectFromChan(c).(*extensionsv1.DaemonSet)
	return daemonset
}
