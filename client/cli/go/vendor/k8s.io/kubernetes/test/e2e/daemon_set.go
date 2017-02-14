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

package e2e

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	extensionsinternal "k8s.io/kubernetes/pkg/apis/extensions"
	extensions "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	// this should not be a multiple of 5, because node status updates
	// every 5 seconds. See https://github.com/kubernetes/kubernetes/pull/14915.
	dsRetryPeriod  = 2 * time.Second
	dsRetryTimeout = 5 * time.Minute

	daemonsetLabelPrefix = "daemonset-"
	daemonsetNameLabel   = daemonsetLabelPrefix + "name"
	daemonsetColorLabel  = daemonsetLabelPrefix + "color"
)

// This test must be run in serial because it assumes the Daemon Set pods will
// always get scheduled.  If we run other tests in parallel, this may not
// happen.  In the future, running in parallel may work if we have an eviction
// model which lets the DS controller kick out other pods to make room.
// See http://issues.k8s.io/21767 for more details
var _ = framework.KubeDescribe("Daemon set [Serial]", func() {
	var f *framework.Framework

	AfterEach(func() {
		// Clean up
		daemonsets, err := f.ClientSet.Extensions().DaemonSets(f.Namespace.Name).List(metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred(), "unable to dump DaemonSets")
		if daemonsets != nil && len(daemonsets.Items) > 0 {
			for _, ds := range daemonsets.Items {
				By(fmt.Sprintf("Deleting DaemonSet %q with reaper", ds.Name))
				dsReaper, err := kubectl.ReaperFor(extensionsinternal.Kind("DaemonSet"), f.InternalClientset)
				Expect(err).NotTo(HaveOccurred())
				err = dsReaper.Stop(f.Namespace.Name, ds.Name, 0, nil)
				Expect(err).NotTo(HaveOccurred())
				err = wait.Poll(dsRetryPeriod, dsRetryTimeout, checkRunningOnNoNodes(f, ds.Spec.Template.Labels))
				Expect(err).NotTo(HaveOccurred(), "error waiting for daemon pod to be reaped")
			}
		}
		if daemonsets, err := f.ClientSet.Extensions().DaemonSets(f.Namespace.Name).List(metav1.ListOptions{}); err == nil {
			framework.Logf("daemonset: %s", runtime.EncodeOrDie(api.Codecs.LegacyCodec(api.Registry.EnabledVersions()...), daemonsets))
		} else {
			framework.Logf("unable to dump daemonsets: %v", err)
		}
		if pods, err := f.ClientSet.Core().Pods(f.Namespace.Name).List(metav1.ListOptions{}); err == nil {
			framework.Logf("pods: %s", runtime.EncodeOrDie(api.Codecs.LegacyCodec(api.Registry.EnabledVersions()...), pods))
		} else {
			framework.Logf("unable to dump pods: %v", err)
		}
		err = clearDaemonSetNodeLabels(f.ClientSet)
		Expect(err).NotTo(HaveOccurred())
	})

	f = framework.NewDefaultFramework("daemonsets")

	image := "gcr.io/google_containers/serve_hostname:v1.4"
	dsName := "daemon-set"

	var ns string
	var c clientset.Interface

	BeforeEach(func() {
		ns = f.Namespace.Name

		c = f.ClientSet
		err := clearDaemonSetNodeLabels(c)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should run and stop simple daemon", func() {
		label := map[string]string{daemonsetNameLabel: dsName}

		By(fmt.Sprintf("Creating simple DaemonSet %q", dsName))
		_, err := c.Extensions().DaemonSets(ns).Create(newDaemonSet(dsName, image, label))
		Expect(err).NotTo(HaveOccurred())

		By("Check that daemon pods launch on every node of the cluster.")
		Expect(err).NotTo(HaveOccurred())
		err = wait.Poll(dsRetryPeriod, dsRetryTimeout, checkRunningOnAllNodes(f, label))
		Expect(err).NotTo(HaveOccurred(), "error waiting for daemon pod to start")
		err = checkDaemonStatus(f, dsName)
		Expect(err).NotTo(HaveOccurred())

		By("Stop a daemon pod, check that the daemon pod is revived.")
		podList := listDaemonPods(c, ns, label)
		pod := podList.Items[0]
		err = c.Core().Pods(ns).Delete(pod.Name, nil)
		Expect(err).NotTo(HaveOccurred())
		err = wait.Poll(dsRetryPeriod, dsRetryTimeout, checkRunningOnAllNodes(f, label))
		Expect(err).NotTo(HaveOccurred(), "error waiting for daemon pod to revive")
	})

	It("should run and stop complex daemon", func() {
		complexLabel := map[string]string{daemonsetNameLabel: dsName}
		nodeSelector := map[string]string{daemonsetColorLabel: "blue"}
		framework.Logf("Creating daemon %q with a node selector", dsName)
		ds := newDaemonSet(dsName, image, complexLabel)
		ds.Spec.Template.Spec.NodeSelector = nodeSelector
		_, err := c.Extensions().DaemonSets(ns).Create(ds)
		Expect(err).NotTo(HaveOccurred())

		By("Initially, daemon pods should not be running on any nodes.")
		err = wait.Poll(dsRetryPeriod, dsRetryTimeout, checkRunningOnNoNodes(f, complexLabel))
		Expect(err).NotTo(HaveOccurred(), "error waiting for daemon pods to be running on no nodes")

		By("Change label of node, check that daemon pod is launched.")
		nodeList := framework.GetReadySchedulableNodesOrDie(f.ClientSet)
		Expect(len(nodeList.Items)).To(BeNumerically(">", 0))
		newNode, err := setDaemonSetNodeLabels(c, nodeList.Items[0].Name, nodeSelector)
		Expect(err).NotTo(HaveOccurred(), "error setting labels on node")
		daemonSetLabels, _ := separateDaemonSetNodeLabels(newNode.Labels)
		Expect(len(daemonSetLabels)).To(Equal(1))
		err = wait.Poll(dsRetryPeriod, dsRetryTimeout, checkDaemonPodOnNodes(f, complexLabel, []string{newNode.Name}))
		Expect(err).NotTo(HaveOccurred(), "error waiting for daemon pods to be running on new nodes")
		err = checkDaemonStatus(f, dsName)
		Expect(err).NotTo(HaveOccurred())

		By("remove the node selector and wait for daemons to be unscheduled")
		_, err = setDaemonSetNodeLabels(c, nodeList.Items[0].Name, map[string]string{})
		Expect(err).NotTo(HaveOccurred(), "error removing labels on node")
		Expect(wait.Poll(dsRetryPeriod, dsRetryTimeout, checkRunningOnNoNodes(f, complexLabel))).
			NotTo(HaveOccurred(), "error waiting for daemon pod to not be running on nodes")
	})

	It("should run and stop complex daemon with node affinity", func() {
		complexLabel := map[string]string{daemonsetNameLabel: dsName}
		nodeSelector := map[string]string{daemonsetColorLabel: "blue"}
		framework.Logf("Creating daemon %q with a node affinity", dsName)
		ds := newDaemonSet(dsName, image, complexLabel)
		ds.Spec.Template.Spec.Affinity = &v1.Affinity{
			NodeAffinity: &v1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								{
									Key:      daemonsetColorLabel,
									Operator: v1.NodeSelectorOpIn,
									Values:   []string{nodeSelector[daemonsetColorLabel]},
								},
							},
						},
					},
				},
			},
		}
		_, err := c.Extensions().DaemonSets(ns).Create(ds)
		Expect(err).NotTo(HaveOccurred())

		By("Initially, daemon pods should not be running on any nodes.")
		err = wait.Poll(dsRetryPeriod, dsRetryTimeout, checkRunningOnNoNodes(f, complexLabel))
		Expect(err).NotTo(HaveOccurred(), "error waiting for daemon pods to be running on no nodes")

		By("Change label of node, check that daemon pod is launched.")
		nodeList := framework.GetReadySchedulableNodesOrDie(f.ClientSet)
		Expect(len(nodeList.Items)).To(BeNumerically(">", 0))
		newNode, err := setDaemonSetNodeLabels(c, nodeList.Items[0].Name, nodeSelector)
		Expect(err).NotTo(HaveOccurred(), "error setting labels on node")
		daemonSetLabels, _ := separateDaemonSetNodeLabels(newNode.Labels)
		Expect(len(daemonSetLabels)).To(Equal(1))
		err = wait.Poll(dsRetryPeriod, dsRetryTimeout, checkDaemonPodOnNodes(f, complexLabel, []string{newNode.Name}))
		Expect(err).NotTo(HaveOccurred(), "error waiting for daemon pods to be running on new nodes")
		err = checkDaemonStatus(f, dsName)
		Expect(err).NotTo(HaveOccurred())

		By("remove the node selector and wait for daemons to be unscheduled")
		_, err = setDaemonSetNodeLabels(c, nodeList.Items[0].Name, map[string]string{})
		Expect(err).NotTo(HaveOccurred(), "error removing labels on node")
		Expect(wait.Poll(dsRetryPeriod, dsRetryTimeout, checkRunningOnNoNodes(f, complexLabel))).
			NotTo(HaveOccurred(), "error waiting for daemon pod to not be running on nodes")
	})

	It("should retry creating failed daemon pods", func() {
		label := map[string]string{daemonsetNameLabel: dsName}

		By(fmt.Sprintf("Creating a simple DaemonSet %q", dsName))
		_, err := c.Extensions().DaemonSets(ns).Create(newDaemonSet(dsName, image, label))
		Expect(err).NotTo(HaveOccurred())

		By("Check that daemon pods launch on every node of the cluster.")
		Expect(err).NotTo(HaveOccurred())
		err = wait.Poll(dsRetryPeriod, dsRetryTimeout, checkRunningOnAllNodes(f, label))
		Expect(err).NotTo(HaveOccurred(), "error waiting for daemon pod to start")
		err = checkDaemonStatus(f, dsName)
		Expect(err).NotTo(HaveOccurred())

		By("Set a daemon pod's phase to 'Failed', check that the daemon pod is revived.")
		podList := listDaemonPods(c, ns, label)
		pod := podList.Items[0]
		pod.ResourceVersion = ""
		pod.Status.Phase = v1.PodFailed
		_, err = c.Core().Pods(ns).UpdateStatus(&pod)
		Expect(err).NotTo(HaveOccurred(), "error failing a daemon pod")
		err = wait.Poll(dsRetryPeriod, dsRetryTimeout, checkRunningOnAllNodes(f, label))
		Expect(err).NotTo(HaveOccurred(), "error waiting for daemon pod to revive")
	})
})

func newDaemonSet(dsName, image string, label map[string]string) *extensions.DaemonSet {
	return &extensions.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: dsName,
		},
		Spec: extensions.DaemonSetSpec{
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: label,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  dsName,
							Image: image,
							Ports: []v1.ContainerPort{{ContainerPort: 9376}},
						},
					},
				},
			},
		},
	}
}

func listDaemonPods(c clientset.Interface, ns string, label map[string]string) *v1.PodList {
	selector := labels.Set(label).AsSelector()
	options := metav1.ListOptions{LabelSelector: selector.String()}
	podList, err := c.Core().Pods(ns).List(options)
	Expect(err).NotTo(HaveOccurred())
	Expect(len(podList.Items)).To(BeNumerically(">", 0))
	return podList
}

func separateDaemonSetNodeLabels(labels map[string]string) (map[string]string, map[string]string) {
	daemonSetLabels := map[string]string{}
	otherLabels := map[string]string{}
	for k, v := range labels {
		if strings.HasPrefix(k, daemonsetLabelPrefix) {
			daemonSetLabels[k] = v
		} else {
			otherLabels[k] = v
		}
	}
	return daemonSetLabels, otherLabels
}

func clearDaemonSetNodeLabels(c clientset.Interface) error {
	nodeList := framework.GetReadySchedulableNodesOrDie(c)
	for _, node := range nodeList.Items {
		_, err := setDaemonSetNodeLabels(c, node.Name, map[string]string{})
		if err != nil {
			return err
		}
	}
	return nil
}

func setDaemonSetNodeLabels(c clientset.Interface, nodeName string, labels map[string]string) (*v1.Node, error) {
	nodeClient := c.Core().Nodes()
	var newNode *v1.Node
	var newLabels map[string]string
	err := wait.Poll(dsRetryPeriod, dsRetryTimeout, func() (bool, error) {
		node, err := nodeClient.Get(nodeName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		// remove all labels this test is creating
		daemonSetLabels, otherLabels := separateDaemonSetNodeLabels(node.Labels)
		if reflect.DeepEqual(daemonSetLabels, labels) {
			newNode = node
			return true, nil
		}
		node.Labels = otherLabels
		for k, v := range labels {
			node.Labels[k] = v
		}
		newNode, err = nodeClient.Update(node)
		if err == nil {
			newLabels, _ = separateDaemonSetNodeLabels(newNode.Labels)
			return true, err
		}
		if se, ok := err.(*apierrs.StatusError); ok && se.ErrStatus.Reason == metav1.StatusReasonConflict {
			framework.Logf("failed to update node due to resource version conflict")
			return false, nil
		}
		return false, err
	})
	if err != nil {
		return nil, err
	} else if len(newLabels) != len(labels) {
		return nil, fmt.Errorf("Could not set daemon set test labels as expected.")
	}

	return newNode, nil
}

func checkDaemonPodOnNodes(f *framework.Framework, selector map[string]string, nodeNames []string) func() (bool, error) {
	return func() (bool, error) {
		selector := labels.Set(selector).AsSelector()
		options := metav1.ListOptions{LabelSelector: selector.String()}
		podList, err := f.ClientSet.Core().Pods(f.Namespace.Name).List(options)
		if err != nil {
			return false, nil
		}
		pods := podList.Items

		nodesToPodCount := make(map[string]int)
		for _, pod := range pods {
			if controller.IsPodActive(&pod) {
				nodesToPodCount[pod.Spec.NodeName] += 1
			}
		}
		framework.Logf("nodesToPodCount: %#v", nodesToPodCount)

		// Ensure that exactly 1 pod is running on all nodes in nodeNames.
		for _, nodeName := range nodeNames {
			if nodesToPodCount[nodeName] != 1 {
				return false, nil
			}
		}

		// Ensure that sizes of the lists are the same. We've verified that every element of nodeNames is in
		// nodesToPodCount, so verifying the lengths are equal ensures that there aren't pods running on any
		// other nodes.
		return len(nodesToPodCount) == len(nodeNames), nil
	}
}

func checkRunningOnAllNodes(f *framework.Framework, selector map[string]string) func() (bool, error) {
	return func() (bool, error) {
		nodeList, err := f.ClientSet.Core().Nodes().List(metav1.ListOptions{})
		framework.ExpectNoError(err)
		nodeNames := make([]string, 0)
		for _, node := range nodeList.Items {
			nodeNames = append(nodeNames, node.Name)
		}
		return checkDaemonPodOnNodes(f, selector, nodeNames)()
	}
}

func checkRunningOnNoNodes(f *framework.Framework, selector map[string]string) func() (bool, error) {
	return checkDaemonPodOnNodes(f, selector, make([]string, 0))
}

func checkDaemonStatus(f *framework.Framework, dsName string) error {
	ds, err := f.ClientSet.Extensions().DaemonSets(f.Namespace.Name).Get(dsName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Could not get daemon set from v1.")
	}
	desired, scheduled, ready := ds.Status.DesiredNumberScheduled, ds.Status.CurrentNumberScheduled, ds.Status.NumberReady
	if desired != scheduled && desired != ready {
		return fmt.Errorf("Error in daemon status. DesiredScheduled: %d, CurrentScheduled: %d, Ready: %d", desired, scheduled, ready)
	}
	return nil
}
