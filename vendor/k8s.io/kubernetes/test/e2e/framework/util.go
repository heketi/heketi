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

package framework

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	goRuntime "runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/golang/glog"
	"k8s.io/kubernetes/federation/client/clientset_generated/federation_release_1_5"
	"k8s.io/kubernetes/pkg/api"
	apierrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	"k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/apis/extensions"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/clientset_generated/release_1_5"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/typed/discovery"
	"k8s.io/kubernetes/pkg/client/typed/dynamic"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
	gcecloud "k8s.io/kubernetes/pkg/cloudprovider/providers/gce"
	"k8s.io/kubernetes/pkg/controller"
	deploymentutil "k8s.io/kubernetes/pkg/controller/deployment/util"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/kubelet/util/format"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/master/ports"
	"k8s.io/kubernetes/pkg/runtime"
	sshutil "k8s.io/kubernetes/pkg/ssh"
	"k8s.io/kubernetes/pkg/types"
	uexec "k8s.io/kubernetes/pkg/util/exec"
	labelsutil "k8s.io/kubernetes/pkg/util/labels"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/system"
	"k8s.io/kubernetes/pkg/util/uuid"
	"k8s.io/kubernetes/pkg/util/wait"
	utilyaml "k8s.io/kubernetes/pkg/util/yaml"
	"k8s.io/kubernetes/pkg/version"
	"k8s.io/kubernetes/pkg/watch"
	"k8s.io/kubernetes/plugin/pkg/scheduler/algorithm/predicates"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
	testutils "k8s.io/kubernetes/test/utils"

	"github.com/blang/semver"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/websocket"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	gomegatypes "github.com/onsi/gomega/types"
)

const (
	// How long to wait for the pod to be listable
	PodListTimeout = time.Minute
	// Initial pod start can be delayed O(minutes) by slow docker pulls
	// TODO: Make this 30 seconds once #4566 is resolved.
	PodStartTimeout = 5 * time.Minute

	// How long to wait for the pod to no longer be running
	podNoLongerRunningTimeout = 30 * time.Second

	// If there are any orphaned namespaces to clean up, this test is running
	// on a long lived cluster. A long wait here is preferably to spurious test
	// failures caused by leaked resources from a previous test run.
	NamespaceCleanupTimeout = 15 * time.Minute

	// Some pods can take much longer to get ready due to volume attach/detach latency.
	slowPodStartTimeout = 15 * time.Minute

	// How long to wait for a service endpoint to be resolvable.
	ServiceStartTimeout = 1 * time.Minute

	// How often to Poll pods, nodes and claims.
	Poll = 2 * time.Second

	// service accounts are provisioned after namespace creation
	// a service account is required to support pod creation in a namespace as part of admission control
	ServiceAccountProvisionTimeout = 2 * time.Minute

	// How long to try single API calls (like 'get' or 'list'). Used to prevent
	// transient failures from failing tests.
	// TODO: client should not apply this timeout to Watch calls. Increased from 30s until that is fixed.
	SingleCallTimeout = 5 * time.Minute

	// How long nodes have to be "ready" when a test begins. They should already
	// be "ready" before the test starts, so this is small.
	NodeReadyInitialTimeout = 20 * time.Second

	// How long pods have to be "ready" when a test begins.
	PodReadyBeforeTimeout = 5 * time.Minute

	// How long pods have to become scheduled onto nodes
	podScheduledBeforeTimeout = PodListTimeout + (20 * time.Second)

	podRespondingTimeout     = 2 * time.Minute
	ServiceRespondingTimeout = 2 * time.Minute
	EndpointRegisterTimeout  = time.Minute

	// How long claims have to become dynamically provisioned
	ClaimProvisionTimeout = 5 * time.Minute

	// When these values are updated, also update cmd/kubelet/app/options/options.go
	currentPodInfraContainerImageName    = "gcr.io/google_containers/pause"
	currentPodInfraContainerImageVersion = "3.0"

	// How long each node is given during a process that restarts all nodes
	// before the test is considered failed. (Note that the total time to
	// restart all nodes will be this number times the number of nodes.)
	RestartPerNodeTimeout = 5 * time.Minute

	// How often to Poll the statues of a restart.
	RestartPoll = 20 * time.Second

	// How long a node is allowed to become "Ready" after it is restarted before
	// the test is considered failed.
	RestartNodeReadyAgainTimeout = 5 * time.Minute

	// How long a pod is allowed to become "running" and "ready" after a node
	// restart before test is considered failed.
	RestartPodReadyAgainTimeout = 5 * time.Minute

	// Number of times we want to retry Updates in case of conflict
	UpdateRetries = 5

	// Number of objects that gc can delete in a second.
	// GC issues 2 requestes for single delete.
	gcThroughput = 10

	// TODO(justinsb): Avoid hardcoding this.
	awsMasterIP = "172.20.0.9"

	// Default time to wait for nodes to become schedulable.
	// Set so high for scale tests.
	NodeSchedulableTimeout = 4 * time.Hour
)

var (
	// Label allocated to the image puller static pod that runs on each node
	// before e2es.
	ImagePullerLabels = map[string]string{"name": "e2e-image-puller"}

	// For parsing Kubectl version for version-skewed testing.
	gitVersionRegexp = regexp.MustCompile("GitVersion:\"(v.+?)\"")

	// Slice of regexps for names of pods that have to be running to consider a Node "healthy"
	requiredPerNodePods = []*regexp.Regexp{
		regexp.MustCompile(".*kube-proxy.*"),
		regexp.MustCompile(".*fluentd-elasticsearch.*"),
		regexp.MustCompile(".*node-problem-detector.*"),
	}
)

type Address struct {
	internalIP string
	externalIP string
	hostname   string
}

// GetServerArchitecture fetches the architecture of the cluster's apiserver.
func GetServerArchitecture(c clientset.Interface) string {
	arch := ""
	sVer, err := c.Discovery().ServerVersion()
	if err != nil || sVer.Platform == "" {
		// If we failed to get the server version for some reason, default to amd64.
		arch = "amd64"
	} else {
		// Split the platform string into OS and Arch separately.
		// The platform string may for example be "linux/amd64", "linux/arm" or "windows/amd64".
		osArchArray := strings.Split(sVer.Platform, "/")
		arch = osArchArray[1]
	}
	return arch
}

// GetPauseImageName fetches the pause image name for the same architecture as the apiserver.
func GetPauseImageName(c clientset.Interface) string {
	return currentPodInfraContainerImageName + "-" + GetServerArchitecture(c) + ":" + currentPodInfraContainerImageVersion
}

// GetPauseImageNameForHostArch fetches the pause image name for the same architecture the test is running on.
// TODO: move this function to the test/utils
func GetPauseImageNameForHostArch() string {
	return currentPodInfraContainerImageName + "-" + goRuntime.GOARCH + ":" + currentPodInfraContainerImageVersion
}

// SubResource proxy should have been functional in v1.0.0, but SubResource
// proxy via tunneling is known to be broken in v1.0.  See
// https://github.com/kubernetes/kubernetes/pull/15224#issuecomment-146769463
//
// TODO(ihmccreery): remove once we don't care about v1.0 anymore, (tentatively
// in v1.3).
var SubResourcePodProxyVersion = version.MustParse("v1.1.0")
var subResourceServiceAndNodeProxyVersion = version.MustParse("v1.2.0")

func GetServicesProxyRequest(c clientset.Interface, request *restclient.Request) (*restclient.Request, error) {
	subResourceProxyAvailable, err := ServerVersionGTE(subResourceServiceAndNodeProxyVersion, c.Discovery())
	if err != nil {
		return nil, err
	}
	if subResourceProxyAvailable {
		return request.Resource("services").SubResource("proxy"), nil
	}
	return request.Prefix("proxy").Resource("services"), nil
}

// unique identifier of the e2e run
var RunId = uuid.NewUUID()

type CreateTestingNSFn func(baseName string, c clientset.Interface, labels map[string]string) (*api.Namespace, error)

type ContainerFailures struct {
	status   *api.ContainerStateTerminated
	Restarts int
}

func GetMasterHost() string {
	masterUrl, err := url.Parse(TestContext.Host)
	ExpectNoError(err)
	return masterUrl.Host
}

func nowStamp() string {
	return time.Now().Format(time.StampMilli)
}

func log(level string, format string, args ...interface{}) {
	fmt.Fprintf(GinkgoWriter, nowStamp()+": "+level+": "+format+"\n", args...)
}

func Logf(format string, args ...interface{}) {
	log("INFO", format, args...)
}

func Failf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log("INFO", msg)
	Fail(nowStamp()+": "+msg, 1)
}

func Skipf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log("INFO", msg)
	Skip(nowStamp() + ": " + msg)
}

func SkipUnlessNodeCountIsAtLeast(minNodeCount int) {
	if TestContext.CloudConfig.NumNodes < minNodeCount {
		Skipf("Requires at least %d nodes (not %d)", minNodeCount, TestContext.CloudConfig.NumNodes)
	}
}

func SkipUnlessAtLeast(value int, minValue int, message string) {
	if value < minValue {
		Skipf(message)
	}
}

func SkipIfProviderIs(unsupportedProviders ...string) {
	if ProviderIs(unsupportedProviders...) {
		Skipf("Not supported for providers %v (found %s)", unsupportedProviders, TestContext.Provider)
	}
}

func SkipUnlessProviderIs(supportedProviders ...string) {
	if !ProviderIs(supportedProviders...) {
		Skipf("Only supported for providers %v (not %s)", supportedProviders, TestContext.Provider)
	}
}

func SkipIfContainerRuntimeIs(runtimes ...string) {
	for _, runtime := range runtimes {
		if runtime == TestContext.ContainerRuntime {
			Skipf("Not supported under container runtime %s", runtime)
		}
	}
}

func ProviderIs(providers ...string) bool {
	for _, provider := range providers {
		if strings.ToLower(provider) == strings.ToLower(TestContext.Provider) {
			return true
		}
	}
	return false
}

func SkipUnlessServerVersionGTE(v semver.Version, c discovery.ServerVersionInterface) {
	gte, err := ServerVersionGTE(v, c)
	if err != nil {
		Failf("Failed to get server version: %v", err)
	}
	if !gte {
		Skipf("Not supported for server versions before %q", v)
	}
}

// Detects whether the federation namespace exists in the underlying cluster
func SkipUnlessFederated(c clientset.Interface) {
	federationNS := os.Getenv("FEDERATION_NAMESPACE")
	if federationNS == "" {
		federationNS = "federation"
	}

	_, err := c.Core().Namespaces().Get(federationNS)
	if err != nil {
		if apierrs.IsNotFound(err) {
			Skipf("Could not find federation namespace %s: skipping federated test", federationNS)
		} else {
			Failf("Unexpected error getting namespace: %v", err)
		}
	}
}

func SkipIfMissingResource(clientPool dynamic.ClientPool, gvr unversioned.GroupVersionResource, namespace string) {
	dynamicClient, err := clientPool.ClientForGroupVersionResource(gvr)
	if err != nil {
		Failf("Unexpected error getting dynamic client for %v: %v", gvr.GroupVersion(), err)
	}
	apiResource := unversioned.APIResource{Name: gvr.Resource, Namespaced: true}
	_, err = dynamicClient.Resource(&apiResource, namespace).List(&v1.ListOptions{})
	if err != nil {
		// not all resources support list, so we ignore those
		if apierrs.IsMethodNotSupported(err) || apierrs.IsNotFound(err) || apierrs.IsForbidden(err) {
			Skipf("Could not find %s resource, skipping test: %#v", gvr, err)
		}
		Failf("Unexpected error getting %v: %v", gvr, err)
	}
}

// ProvidersWithSSH are those providers where each node is accessible with SSH
var ProvidersWithSSH = []string{"gce", "gke", "aws"}

// providersWithMasterSSH are those providers where master node is accessible with SSH
var providersWithMasterSSH = []string{"gce", "gke", "kubemark", "aws"}

type podCondition func(pod *api.Pod) (bool, error)

// logPodStates logs basic info of provided pods for debugging.
func logPodStates(pods []api.Pod) {
	// Find maximum widths for pod, node, and phase strings for column printing.
	maxPodW, maxNodeW, maxPhaseW, maxGraceW := len("POD"), len("NODE"), len("PHASE"), len("GRACE")
	for i := range pods {
		pod := &pods[i]
		if len(pod.ObjectMeta.Name) > maxPodW {
			maxPodW = len(pod.ObjectMeta.Name)
		}
		if len(pod.Spec.NodeName) > maxNodeW {
			maxNodeW = len(pod.Spec.NodeName)
		}
		if len(pod.Status.Phase) > maxPhaseW {
			maxPhaseW = len(pod.Status.Phase)
		}
	}
	// Increase widths by one to separate by a single space.
	maxPodW++
	maxNodeW++
	maxPhaseW++
	maxGraceW++

	// Log pod info. * does space padding, - makes them left-aligned.
	Logf("%-[1]*[2]s %-[3]*[4]s %-[5]*[6]s %-[7]*[8]s %[9]s",
		maxPodW, "POD", maxNodeW, "NODE", maxPhaseW, "PHASE", maxGraceW, "GRACE", "CONDITIONS")
	for _, pod := range pods {
		grace := ""
		if pod.DeletionGracePeriodSeconds != nil {
			grace = fmt.Sprintf("%ds", *pod.DeletionGracePeriodSeconds)
		}
		Logf("%-[1]*[2]s %-[3]*[4]s %-[5]*[6]s %-[7]*[8]s %[9]s",
			maxPodW, pod.ObjectMeta.Name, maxNodeW, pod.Spec.NodeName, maxPhaseW, pod.Status.Phase, maxGraceW, grace, pod.Status.Conditions)
	}
	Logf("") // Final empty line helps for readability.
}

// errorBadPodsStates create error message of basic info of bad pods for debugging.
func errorBadPodsStates(badPods []api.Pod, desiredPods int, ns, desiredState string, timeout time.Duration) string {
	errStr := fmt.Sprintf("%d / %d pods in namespace %q are NOT in %s state in %v\n", len(badPods), desiredPods, ns, desiredState, timeout)
	// Pirnt bad pods info only if there are fewer than 10 bad pods
	if len(badPods) > 10 {
		return errStr + "There are too many bad pods. Please check log for details."
	}

	buf := bytes.NewBuffer(nil)
	w := tabwriter.NewWriter(buf, 0, 0, 1, ' ', 0)
	fmt.Fprintln(w, "POD\tNODE\tPHASE\tGRACE\tCONDITIONS")
	for _, badPod := range badPods {
		grace := ""
		if badPod.DeletionGracePeriodSeconds != nil {
			grace = fmt.Sprintf("%ds", *badPod.DeletionGracePeriodSeconds)
		}
		podInfo := fmt.Sprintf("%s\t%s\t%s\t%s\t%s",
			badPod.ObjectMeta.Name, badPod.Spec.NodeName, badPod.Status.Phase, grace, badPod.Status.Conditions)
		fmt.Fprintln(w, podInfo)
	}
	w.Flush()
	return errStr + buf.String()
}

// WaitForPodsSuccess waits till all labels matching the given selector enter
// the Success state. The caller is expected to only invoke this method once the
// pods have been created.
func WaitForPodsSuccess(c clientset.Interface, ns string, successPodLabels map[string]string, timeout time.Duration) error {
	successPodSelector := labels.SelectorFromSet(successPodLabels)
	start, badPods, desiredPods := time.Now(), []api.Pod{}, 0

	if wait.PollImmediate(30*time.Second, timeout, func() (bool, error) {
		podList, err := c.Core().Pods(ns).List(api.ListOptions{LabelSelector: successPodSelector})
		if err != nil {
			Logf("Error getting pods in namespace %q: %v", ns, err)
			return false, nil
		}
		if len(podList.Items) == 0 {
			Logf("Waiting for pods to enter Success, but no pods in %q match label %v", ns, successPodLabels)
			return true, nil
		}
		badPods = []api.Pod{}
		desiredPods = len(podList.Items)
		for _, pod := range podList.Items {
			if pod.Status.Phase != api.PodSucceeded {
				badPods = append(badPods, pod)
			}
		}
		successPods := len(podList.Items) - len(badPods)
		Logf("%d / %d pods in namespace %q are in Success state (%d seconds elapsed)",
			successPods, len(podList.Items), ns, int(time.Since(start).Seconds()))
		if len(badPods) == 0 {
			return true, nil
		}
		return false, nil
	}) != nil {
		logPodStates(badPods)
		LogPodsWithLabels(c, ns, successPodLabels, Logf)
		return errors.New(errorBadPodsStates(badPods, desiredPods, ns, "SUCCESS", timeout))

	}
	return nil
}

var ReadyReplicaVersion = version.MustParse("v1.4.0")

// WaitForPodsRunningReady waits up to timeout to ensure that all pods in
// namespace ns are either running and ready, or failed but controlled by a
// controller. Also, it ensures that at least minPods are running and
// ready. It has separate behavior from other 'wait for' pods functions in
// that it requests the list of pods on every iteration. This is useful, for
// example, in cluster startup, because the number of pods increases while
// waiting.
// If ignoreLabels is not empty, pods matching this selector are ignored and
// this function waits for minPods to enter Running/Ready and for all pods
// matching ignoreLabels to enter Success phase. Otherwise an error is returned
// even if there are minPods pods, some of which are in Running/Ready
// and some in Success. This is to allow the client to decide if "Success"
// means "Ready" or not.
func WaitForPodsRunningReady(c clientset.Interface, ns string, minPods int32, timeout time.Duration, ignoreLabels map[string]string) error {

	// This can be removed when we no longer have 1.3 servers running with upgrade tests.
	hasReadyReplicas, err := ServerVersionGTE(ReadyReplicaVersion, c.Discovery())
	if err != nil {
		Logf("Error getting the server version: %v", err)
		return err
	}

	ignoreSelector := labels.SelectorFromSet(ignoreLabels)
	start := time.Now()
	Logf("Waiting up to %v for all pods (need at least %d) in namespace '%s' to be running and ready",
		timeout, minPods, ns)
	wg := sync.WaitGroup{}
	wg.Add(1)
	var waitForSuccessError error
	badPods := []api.Pod{}
	desiredPods := 0
	go func() {
		waitForSuccessError = WaitForPodsSuccess(c, ns, ignoreLabels, timeout)
		wg.Done()
	}()

	if wait.PollImmediate(Poll, timeout, func() (bool, error) {
		// We get the new list of pods, replication controllers, and
		// replica sets in every iteration because more pods come
		// online during startup and we want to ensure they are also
		// checked.
		replicas, replicaOk := int32(0), int32(0)

		if hasReadyReplicas {
			rcList, err := c.Core().ReplicationControllers(ns).List(api.ListOptions{})
			if err != nil {
				Logf("Error getting replication controllers in namespace '%s': %v", ns, err)
				return false, nil
			}
			for _, rc := range rcList.Items {
				replicas += rc.Spec.Replicas
				replicaOk += rc.Status.ReadyReplicas
			}

			rsList, err := c.Extensions().ReplicaSets(ns).List(api.ListOptions{})
			if err != nil {
				Logf("Error getting replication sets in namespace %q: %v", ns, err)
				return false, nil
			}
			for _, rs := range rsList.Items {
				replicas += rs.Spec.Replicas
				replicaOk += rs.Status.ReadyReplicas
			}
		}

		podList, err := c.Core().Pods(ns).List(api.ListOptions{})
		if err != nil {
			Logf("Error getting pods in namespace '%s': %v", ns, err)
			return false, nil
		}
		nOk := int32(0)
		badPods = []api.Pod{}
		desiredPods = len(podList.Items)
		for _, pod := range podList.Items {
			if len(ignoreLabels) != 0 && ignoreSelector.Matches(labels.Set(pod.Labels)) {
				Logf("%v in state %v, ignoring", pod.Name, pod.Status.Phase)
				continue
			}
			if res, err := testutils.PodRunningReady(&pod); res && err == nil {
				nOk++
			} else {
				if pod.Status.Phase != api.PodFailed {
					Logf("The status of Pod %s is %s (Ready = false), waiting for it to be either Running (with Ready = true) or Failed", pod.ObjectMeta.Name, pod.Status.Phase)
					badPods = append(badPods, pod)
				} else if _, ok := pod.Annotations[api.CreatedByAnnotation]; !ok {
					Logf("Pod %s is Failed, but it's not controlled by a controller", pod.ObjectMeta.Name)
					badPods = append(badPods, pod)
				}
				//ignore failed pods that are controlled by some controller
			}
		}

		Logf("%d / %d pods in namespace '%s' are running and ready (%d seconds elapsed)",
			nOk, len(podList.Items), ns, int(time.Since(start).Seconds()))
		if hasReadyReplicas {
			Logf("expected %d pod replicas in namespace '%s', %d are Running and Ready.", replicas, ns, replicaOk)
		}

		if replicaOk == replicas && nOk >= minPods && len(badPods) == 0 {
			return true, nil
		}
		logPodStates(badPods)
		return false, nil
	}) != nil {
		return errors.New(errorBadPodsStates(badPods, desiredPods, ns, "RUNNING and READY", timeout))
	}
	wg.Wait()
	if waitForSuccessError != nil {
		return waitForSuccessError
	}
	return nil
}

func podFromManifest(filename string) (*api.Pod, error) {
	var pod api.Pod
	Logf("Parsing pod from %v", filename)
	data := ReadOrDie(filename)
	json, err := utilyaml.ToJSON(data)
	if err != nil {
		return nil, err
	}
	if err := runtime.DecodeInto(api.Codecs.UniversalDecoder(), json, &pod); err != nil {
		return nil, err
	}
	return &pod, nil
}

// Run a test container to try and contact the Kubernetes api-server from a pod, wait for it
// to flip to Ready, log its output and delete it.
func RunKubernetesServiceTestContainer(c clientset.Interface, ns string) {
	path := "test/images/clusterapi-tester/pod.yaml"
	p, err := podFromManifest(path)
	if err != nil {
		Logf("Failed to parse clusterapi-tester from manifest %v: %v", path, err)
		return
	}
	p.Namespace = ns
	if _, err := c.Core().Pods(ns).Create(p); err != nil {
		Logf("Failed to create %v: %v", p.Name, err)
		return
	}
	defer func() {
		if err := c.Core().Pods(ns).Delete(p.Name, nil); err != nil {
			Logf("Failed to delete pod %v: %v", p.Name, err)
		}
	}()
	timeout := 5 * time.Minute
	if err := waitForPodCondition(c, ns, p.Name, "clusterapi-tester", timeout, testutils.PodRunningReady); err != nil {
		Logf("Pod %v took longer than %v to enter running/ready: %v", p.Name, timeout, err)
		return
	}
	logs, err := GetPodLogs(c, ns, p.Name, p.Spec.Containers[0].Name)
	if err != nil {
		Logf("Failed to retrieve logs from %v: %v", p.Name, err)
	} else {
		Logf("Output of clusterapi-tester:\n%v", logs)
	}
}

func kubectlLogPod(c clientset.Interface, pod api.Pod, containerNameSubstr string, logFunc func(ftm string, args ...interface{})) {
	for _, container := range pod.Spec.Containers {
		if strings.Contains(container.Name, containerNameSubstr) {
			// Contains() matches all strings if substr is empty
			logs, err := GetPodLogs(c, pod.Namespace, pod.Name, container.Name)
			if err != nil {
				logs, err = getPreviousPodLogs(c, pod.Namespace, pod.Name, container.Name)
				if err != nil {
					logFunc("Failed to get logs of pod %v, container %v, err: %v", pod.Name, container.Name, err)
				}
			}
			By(fmt.Sprintf("Logs of %v/%v:%v on node %v", pod.Namespace, pod.Name, container.Name, pod.Spec.NodeName))
			logFunc("%s : STARTLOG\n%s\nENDLOG for container %v:%v:%v", containerNameSubstr, logs, pod.Namespace, pod.Name, container.Name)
		}
	}
}

func LogFailedContainers(c clientset.Interface, ns string, logFunc func(ftm string, args ...interface{})) {
	podList, err := c.Core().Pods(ns).List(api.ListOptions{})
	if err != nil {
		logFunc("Error getting pods in namespace '%s': %v", ns, err)
		return
	}
	logFunc("Running kubectl logs on non-ready containers in %v", ns)
	for _, pod := range podList.Items {
		if res, err := testutils.PodRunningReady(&pod); !res || err != nil {
			kubectlLogPod(c, pod, "", Logf)
		}
	}
}

func LogPodsWithLabels(c clientset.Interface, ns string, match map[string]string, logFunc func(ftm string, args ...interface{})) {
	podList, err := c.Core().Pods(ns).List(api.ListOptions{LabelSelector: labels.SelectorFromSet(match)})
	if err != nil {
		logFunc("Error getting pods in namespace %q: %v", ns, err)
		return
	}
	logFunc("Running kubectl logs on pods with labels %v in %v", match, ns)
	for _, pod := range podList.Items {
		kubectlLogPod(c, pod, "", logFunc)
	}
}

func LogContainersInPodsWithLabels(c clientset.Interface, ns string, match map[string]string, containerSubstr string, logFunc func(ftm string, args ...interface{})) {
	podList, err := c.Core().Pods(ns).List(api.ListOptions{LabelSelector: labels.SelectorFromSet(match)})
	if err != nil {
		Logf("Error getting pods in namespace %q: %v", ns, err)
		return
	}
	for _, pod := range podList.Items {
		kubectlLogPod(c, pod, containerSubstr, logFunc)
	}
}

// DeleteNamespaces deletes all namespaces that match the given delete and skip filters.
// Filter is by simple strings.Contains; first skip filter, then delete filter.
// Returns the list of deleted namespaces or an error.
func DeleteNamespaces(c clientset.Interface, deleteFilter, skipFilter []string) ([]string, error) {
	By("Deleting namespaces")
	nsList, err := c.Core().Namespaces().List(api.ListOptions{})
	Expect(err).NotTo(HaveOccurred())
	var deleted []string
	var wg sync.WaitGroup
OUTER:
	for _, item := range nsList.Items {
		if skipFilter != nil {
			for _, pattern := range skipFilter {
				if strings.Contains(item.Name, pattern) {
					continue OUTER
				}
			}
		}
		if deleteFilter != nil {
			var shouldDelete bool
			for _, pattern := range deleteFilter {
				if strings.Contains(item.Name, pattern) {
					shouldDelete = true
					break
				}
			}
			if !shouldDelete {
				continue OUTER
			}
		}
		wg.Add(1)
		deleted = append(deleted, item.Name)
		go func(nsName string) {
			defer wg.Done()
			defer GinkgoRecover()
			Expect(c.Core().Namespaces().Delete(nsName, nil)).To(Succeed())
			Logf("namespace : %v api call to delete is complete ", nsName)
		}(item.Name)
	}
	wg.Wait()
	return deleted, nil
}

func WaitForNamespacesDeleted(c clientset.Interface, namespaces []string, timeout time.Duration) error {
	By("Waiting for namespaces to vanish")
	nsMap := map[string]bool{}
	for _, ns := range namespaces {
		nsMap[ns] = true
	}
	//Now POLL until all namespaces have been eradicated.
	return wait.Poll(2*time.Second, timeout,
		func() (bool, error) {
			nsList, err := c.Core().Namespaces().List(api.ListOptions{})
			if err != nil {
				return false, err
			}
			for _, item := range nsList.Items {
				if _, ok := nsMap[item.Name]; ok {
					return false, nil
				}
			}
			return true, nil
		})
}

func waitForServiceAccountInNamespace(c clientset.Interface, ns, serviceAccountName string, timeout time.Duration) error {
	w, err := c.Core().ServiceAccounts(ns).Watch(api.SingleObject(api.ObjectMeta{Name: serviceAccountName}))
	if err != nil {
		return err
	}
	_, err = watch.Until(timeout, w, client.ServiceAccountHasSecrets)
	return err
}

func waitForPodCondition(c clientset.Interface, ns, podName, desc string, timeout time.Duration, condition podCondition) error {
	Logf("Waiting up to %[1]v for pod %[2]s status to be %[3]s", timeout, podName, desc)
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(Poll) {
		pod, err := c.Core().Pods(ns).Get(podName)
		if err != nil {
			if apierrs.IsNotFound(err) {
				Logf("Pod %q in namespace %q disappeared. Error: %v", podName, ns, err)
				return err
			}
			// Aligning this text makes it much more readable
			Logf("Get pod %[1]s in namespace '%[2]s' failed, ignoring for %[3]v. Error: %[4]v",
				podName, ns, Poll, err)
			continue
		}
		done, err := condition(pod)
		if done {
			return err
		}
		Logf("Waiting for pod %[1]s in namespace '%[2]s' status to be '%[3]s'"+
			"(found phase: %[4]q, readiness: %[5]t) (%[6]v elapsed)",
			podName, ns, desc, pod.Status.Phase, testutils.PodReady(pod), time.Since(start))
	}
	return fmt.Errorf("gave up waiting for pod '%s' to be '%s' after %v", podName, desc, timeout)
}

// WaitForMatchPodsCondition finds match pods based on the input ListOptions.
// waits and checks if all match pods are in the given podCondition
func WaitForMatchPodsCondition(c clientset.Interface, opts api.ListOptions, desc string, timeout time.Duration, condition podCondition) error {
	Logf("Waiting up to %v for matching pods' status to be %s", timeout, desc)
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(Poll) {
		pods, err := c.Core().Pods(api.NamespaceAll).List(opts)
		if err != nil {
			return err
		}
		conditionNotMatch := []string{}
		for _, pod := range pods.Items {
			done, err := condition(&pod)
			if done && err != nil {
				return fmt.Errorf("Unexpected error: %v", err)
			}
			if !done {
				conditionNotMatch = append(conditionNotMatch, format.Pod(&pod))
			}
		}
		if len(conditionNotMatch) <= 0 {
			return err
		}
		Logf("%d pods are not %s", len(conditionNotMatch), desc)
	}
	return fmt.Errorf("gave up waiting for matching pods to be '%s' after %v", desc, timeout)
}

// WaitForDefaultServiceAccountInNamespace waits for the default service account to be provisioned
// the default service account is what is associated with pods when they do not specify a service account
// as a result, pods are not able to be provisioned in a namespace until the service account is provisioned
func WaitForDefaultServiceAccountInNamespace(c clientset.Interface, namespace string) error {
	return waitForServiceAccountInNamespace(c, namespace, "default", ServiceAccountProvisionTimeout)
}

// WaitForFederationApiserverReady waits for the federation apiserver to be ready.
// It tests the readiness by sending a GET request and expecting a non error response.
func WaitForFederationApiserverReady(c *federation_release_1_5.Clientset) error {
	return wait.PollImmediate(time.Second, 1*time.Minute, func() (bool, error) {
		_, err := c.Federation().Clusters().List(v1.ListOptions{})
		if err != nil {
			return false, nil
		}
		return true, nil
	})
}

// WaitForPersistentVolumePhase waits for a PersistentVolume to be in a specific phase or until timeout occurs, whichever comes first.
func WaitForPersistentVolumePhase(phase api.PersistentVolumePhase, c clientset.Interface, pvName string, Poll, timeout time.Duration) error {
	Logf("Waiting up to %v for PersistentVolume %s to have phase %s", timeout, pvName, phase)
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(Poll) {
		pv, err := c.Core().PersistentVolumes().Get(pvName)
		if err != nil {
			Logf("Get persistent volume %s in failed, ignoring for %v: %v", pvName, Poll, err)
			continue
		} else {
			if pv.Status.Phase == phase {
				Logf("PersistentVolume %s found and phase=%s (%v)", pvName, phase, time.Since(start))
				return nil
			} else {
				Logf("PersistentVolume %s found but phase is %s instead of %s.", pvName, pv.Status.Phase, phase)
			}
		}
	}
	return fmt.Errorf("PersistentVolume %s not in phase %s within %v", pvName, phase, timeout)
}

// WaitForPersistentVolumeDeleted waits for a PersistentVolume to get deleted or until timeout occurs, whichever comes first.
func WaitForPersistentVolumeDeleted(c clientset.Interface, pvName string, Poll, timeout time.Duration) error {
	Logf("Waiting up to %v for PersistentVolume %s to get deleted", timeout, pvName)
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(Poll) {
		pv, err := c.Core().PersistentVolumes().Get(pvName)
		if err == nil {
			Logf("PersistentVolume %s found and phase=%s (%v)", pvName, pv.Status.Phase, time.Since(start))
			continue
		} else {
			if apierrs.IsNotFound(err) {
				Logf("PersistentVolume %s was removed", pvName)
				return nil
			} else {
				Logf("Get persistent volume %s in failed, ignoring for %v: %v", pvName, Poll, err)
			}
		}
	}
	return fmt.Errorf("PersistentVolume %s still exists within %v", pvName, timeout)
}

// WaitForPersistentVolumeClaimPhase waits for a PersistentVolumeClaim to be in a specific phase or until timeout occurs, whichever comes first.
func WaitForPersistentVolumeClaimPhase(phase api.PersistentVolumeClaimPhase, c clientset.Interface, ns string, pvcName string, Poll, timeout time.Duration) error {
	Logf("Waiting up to %v for PersistentVolumeClaim %s to have phase %s", timeout, pvcName, phase)
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(Poll) {
		pvc, err := c.Core().PersistentVolumeClaims(ns).Get(pvcName)
		if err != nil {
			Logf("Get persistent volume claim %s in failed, ignoring for %v: %v", pvcName, Poll, err)
			continue
		} else {
			if pvc.Status.Phase == phase {
				Logf("PersistentVolumeClaim %s found and phase=%s (%v)", pvcName, phase, time.Since(start))
				return nil
			} else {
				Logf("PersistentVolumeClaim %s found but phase is %s instead of %s.", pvcName, pvc.Status.Phase, phase)
			}
		}
	}
	return fmt.Errorf("PersistentVolumeClaim %s not in phase %s within %v", pvcName, phase, timeout)
}

// CreateTestingNS should be used by every test, note that we append a common prefix to the provided test name.
// Please see NewFramework instead of using this directly.
func CreateTestingNS(baseName string, c clientset.Interface, labels map[string]string) (*api.Namespace, error) {
	if labels == nil {
		labels = map[string]string{}
	}
	labels["e2e-run"] = string(RunId)

	namespaceObj := &api.Namespace{
		ObjectMeta: api.ObjectMeta{
			GenerateName: fmt.Sprintf("e2e-tests-%v-", baseName),
			Namespace:    "",
			Labels:       labels,
		},
		Status: api.NamespaceStatus{},
	}
	// Be robust about making the namespace creation call.
	var got *api.Namespace
	if err := wait.PollImmediate(Poll, 30*time.Second, func() (bool, error) {
		var err error
		got, err = c.Core().Namespaces().Create(namespaceObj)
		if err != nil {
			Logf("Unexpected error while creating namespace: %v", err)
			return false, nil
		}
		return true, nil
	}); err != nil {
		return nil, err
	}

	if TestContext.VerifyServiceAccount {
		if err := WaitForDefaultServiceAccountInNamespace(c, got.Name); err != nil {
			// Even if we fail to create serviceAccount in the namespace,
			// we have successfully create a namespace.
			// So, return the created namespace.
			return got, err
		}
	}
	return got, nil
}

// CheckTestingNSDeletedExcept checks whether all e2e based existing namespaces are in the Terminating state
// and waits until they are finally deleted. It ignores namespace skip.
func CheckTestingNSDeletedExcept(c clientset.Interface, skip string) error {
	// TODO: Since we don't have support for bulk resource deletion in the API,
	// while deleting a namespace we are deleting all objects from that namespace
	// one by one (one deletion == one API call). This basically exposes us to
	// throttling - currently controller-manager has a limit of max 20 QPS.
	// Once #10217 is implemented and used in namespace-controller, deleting all
	// object from a given namespace should be much faster and we will be able
	// to lower this timeout.
	// However, now Density test is producing ~26000 events and Load capacity test
	// is producing ~35000 events, thus assuming there are no other requests it will
	// take ~30 minutes to fully delete the namespace. Thus I'm setting it to 60
	// minutes to avoid any timeouts here.
	timeout := 60 * time.Minute

	Logf("Waiting for terminating namespaces to be deleted...")
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(15 * time.Second) {
		namespaces, err := c.Core().Namespaces().List(api.ListOptions{})
		if err != nil {
			Logf("Listing namespaces failed: %v", err)
			continue
		}
		terminating := 0
		for _, ns := range namespaces.Items {
			if strings.HasPrefix(ns.ObjectMeta.Name, "e2e-tests-") && ns.ObjectMeta.Name != skip {
				if ns.Status.Phase == api.NamespaceActive {
					return fmt.Errorf("Namespace %s is active", ns.ObjectMeta.Name)
				}
				terminating++
			}
		}
		if terminating == 0 {
			return nil
		}
	}
	return fmt.Errorf("Waiting for terminating namespaces to be deleted timed out")
}

// deleteNS deletes the provided namespace, waits for it to be completely deleted, and then checks
// whether there are any pods remaining in a non-terminating state.
func deleteNS(c clientset.Interface, clientPool dynamic.ClientPool, namespace string, timeout time.Duration) error {
	if err := c.Core().Namespaces().Delete(namespace, nil); err != nil {
		return err
	}

	// wait for namespace to delete or timeout.
	err := wait.PollImmediate(5*time.Second, timeout, func() (bool, error) {
		if _, err := c.Core().Namespaces().Get(namespace); err != nil {
			if apierrs.IsNotFound(err) {
				return true, nil
			}
			Logf("Error while waiting for namespace to be terminated: %v", err)
			return false, nil
		}
		return false, nil
	})

	// verify there is no more remaining content in the namespace
	remainingContent, cerr := hasRemainingContent(c, clientPool, namespace)
	if cerr != nil {
		return cerr
	}

	// if content remains, let's dump information about the namespace, and system for flake debugging.
	remainingPods := 0
	missingTimestamp := 0
	if remainingContent {
		// log information about namespace, and set of namespaces in api server to help flake detection
		logNamespace(c, namespace)
		logNamespaces(c, namespace)

		// if we can, check if there were pods remaining with no timestamp.
		remainingPods, missingTimestamp, _ = countRemainingPods(c, namespace)
	}

	// a timeout waiting for namespace deletion happened!
	if err != nil {
		// some content remains in the namespace
		if remainingContent {
			// pods remain
			if remainingPods > 0 {
				// but they were all undergoing deletion (kubelet is probably culprit)
				if missingTimestamp == 0 {
					return fmt.Errorf("namespace %v was not deleted with limit: %v, pods remaining: %v, pods missing deletion timestamp: %v", namespace, err, remainingPods, missingTimestamp)
				}
				// pods remained, but were not undergoing deletion (namespace controller is probably culprit)
				return fmt.Errorf("namespace %v was not deleted with limit: %v, pods remaining: %v", namespace, err, remainingPods)
			}
			// other content remains (namespace controller is probably screwed up)
			return fmt.Errorf("namespace %v was not deleted with limit: %v, namespaced content other than pods remain", namespace, err)
		}
		// no remaining content, but namespace was not deleted (namespace controller is probably wedged)
		return fmt.Errorf("namespace %v was not deleted with limit: %v, namespace is empty but is not yet removed", namespace, err)
	}
	return nil
}

// logNamespaces logs the number of namespaces by phase
// namespace is the namespace the test was operating against that failed to delete so it can be grepped in logs
func logNamespaces(c clientset.Interface, namespace string) {
	namespaceList, err := c.Core().Namespaces().List(api.ListOptions{})
	if err != nil {
		Logf("namespace: %v, unable to list namespaces: %v", namespace, err)
		return
	}

	numActive := 0
	numTerminating := 0
	for _, namespace := range namespaceList.Items {
		if namespace.Status.Phase == api.NamespaceActive {
			numActive++
		} else {
			numTerminating++
		}
	}
	Logf("namespace: %v, total namespaces: %v, active: %v, terminating: %v", namespace, len(namespaceList.Items), numActive, numTerminating)
}

// logNamespace logs detail about a namespace
func logNamespace(c clientset.Interface, namespace string) {
	ns, err := c.Core().Namespaces().Get(namespace)
	if err != nil {
		if apierrs.IsNotFound(err) {
			Logf("namespace: %v no longer exists", namespace)
			return
		}
		Logf("namespace: %v, unable to get namespace due to error: %v", namespace, err)
		return
	}
	Logf("namespace: %v, DeletionTimetamp: %v, Finalizers: %v, Phase: %v", ns.Name, ns.DeletionTimestamp, ns.Spec.Finalizers, ns.Status.Phase)
}

// countRemainingPods queries the server to count number of remaining pods, and number of pods that had a missing deletion timestamp.
func countRemainingPods(c clientset.Interface, namespace string) (int, int, error) {
	// check for remaining pods
	pods, err := c.Core().Pods(namespace).List(api.ListOptions{})
	if err != nil {
		return 0, 0, err
	}

	// nothing remains!
	if len(pods.Items) == 0 {
		return 0, 0, nil
	}

	// stuff remains, log about it
	logPodStates(pods.Items)

	// check if there were any pods with missing deletion timestamp
	numPods := len(pods.Items)
	missingTimestamp := 0
	for _, pod := range pods.Items {
		if pod.DeletionTimestamp == nil {
			missingTimestamp++
		}
	}
	return numPods, missingTimestamp, nil
}

// hasRemainingContent checks if there is remaining content in the namespace via API discovery
func hasRemainingContent(c clientset.Interface, clientPool dynamic.ClientPool, namespace string) (bool, error) {
	// some tests generate their own framework.Client rather than the default
	// TODO: ensure every test call has a configured clientPool
	if clientPool == nil {
		return false, nil
	}

	// find out what content is supported on the server
	groupVersionResources, err := c.Discovery().ServerPreferredNamespacedResources()
	if err != nil {
		return false, err
	}

	// TODO: temporary hack for https://github.com/kubernetes/kubernetes/issues/31798
	ignoredResources := sets.NewString("bindings")

	contentRemaining := false

	// dump how many of resource type is on the server in a log.
	for _, gvr := range groupVersionResources {
		// get a client for this group version...
		dynamicClient, err := clientPool.ClientForGroupVersionResource(gvr)
		if err != nil {
			// not all resource types support list, so some errors here are normal depending on the resource type.
			Logf("namespace: %s, unable to get client - gvr: %v, error: %v", namespace, gvr, err)
			continue
		}
		// get the api resource
		apiResource := unversioned.APIResource{Name: gvr.Resource, Namespaced: true}
		// TODO: temporary hack for https://github.com/kubernetes/kubernetes/issues/31798
		if ignoredResources.Has(apiResource.Name) {
			Logf("namespace: %s, resource: %s, ignored listing per whitelist", namespace, apiResource.Name)
			continue
		}
		obj, err := dynamicClient.Resource(&apiResource, namespace).List(&v1.ListOptions{})
		if err != nil {
			// not all resources support list, so we ignore those
			if apierrs.IsMethodNotSupported(err) || apierrs.IsNotFound(err) || apierrs.IsForbidden(err) {
				continue
			}
			return false, err
		}
		unstructuredList, ok := obj.(*runtime.UnstructuredList)
		if !ok {
			return false, fmt.Errorf("namespace: %s, resource: %s, expected *runtime.UnstructuredList, got %#v", namespace, apiResource.Name, obj)
		}
		if len(unstructuredList.Items) > 0 {
			Logf("namespace: %s, resource: %s, items remaining: %v", namespace, apiResource.Name, len(unstructuredList.Items))
			contentRemaining = true
		}
	}
	return contentRemaining, nil
}

func ContainerInitInvariant(older, newer runtime.Object) error {
	oldPod := older.(*api.Pod)
	newPod := newer.(*api.Pod)
	if len(oldPod.Spec.InitContainers) == 0 {
		return nil
	}
	if len(oldPod.Spec.InitContainers) != len(newPod.Spec.InitContainers) {
		return fmt.Errorf("init container list changed")
	}
	if oldPod.UID != newPod.UID {
		return fmt.Errorf("two different pods exist in the condition: %s vs %s", oldPod.UID, newPod.UID)
	}
	if err := initContainersInvariants(oldPod); err != nil {
		return err
	}
	if err := initContainersInvariants(newPod); err != nil {
		return err
	}
	oldInit, _, _ := podInitialized(oldPod)
	newInit, _, _ := podInitialized(newPod)
	if oldInit && !newInit {
		// TODO: we may in the future enable resetting PodInitialized = false if the kubelet needs to restart it
		// from scratch
		return fmt.Errorf("pod cannot be initialized and then regress to not being initialized")
	}
	return nil
}

func podInitialized(pod *api.Pod) (ok bool, failed bool, err error) {
	allInit := true
	initFailed := false
	for _, s := range pod.Status.InitContainerStatuses {
		switch {
		case initFailed && s.State.Waiting == nil:
			return allInit, initFailed, fmt.Errorf("container %s is after a failed container but isn't waiting", s.Name)
		case allInit && s.State.Waiting == nil:
			return allInit, initFailed, fmt.Errorf("container %s is after an initializing container but isn't waiting", s.Name)
		case s.State.Terminated == nil:
			allInit = false
		case s.State.Terminated.ExitCode != 0:
			allInit = false
			initFailed = true
		case !s.Ready:
			return allInit, initFailed, fmt.Errorf("container %s initialized but isn't marked as ready", s.Name)
		}
	}
	return allInit, initFailed, nil
}

func initContainersInvariants(pod *api.Pod) error {
	allInit, initFailed, err := podInitialized(pod)
	if err != nil {
		return err
	}
	if !allInit || initFailed {
		for _, s := range pod.Status.ContainerStatuses {
			if s.State.Waiting == nil || s.RestartCount != 0 {
				return fmt.Errorf("container %s is not waiting but initialization not complete", s.Name)
			}
			if s.State.Waiting.Reason != "PodInitializing" {
				return fmt.Errorf("container %s should have reason PodInitializing: %s", s.Name, s.State.Waiting.Reason)
			}
		}
	}
	_, c := api.GetPodCondition(&pod.Status, api.PodInitialized)
	if c == nil {
		return fmt.Errorf("pod does not have initialized condition")
	}
	if c.LastTransitionTime.IsZero() {
		return fmt.Errorf("PodInitialized condition should always have a transition time")
	}
	switch {
	case c.Status == api.ConditionUnknown:
		return fmt.Errorf("PodInitialized condition should never be Unknown")
	case c.Status == api.ConditionTrue && (initFailed || !allInit):
		return fmt.Errorf("PodInitialized condition was True but all not all containers initialized")
	case c.Status == api.ConditionFalse && (!initFailed && allInit):
		return fmt.Errorf("PodInitialized condition was False but all containers initialized")
	}
	return nil
}

type InvariantFunc func(older, newer runtime.Object) error

func CheckInvariants(events []watch.Event, fns ...InvariantFunc) error {
	errs := sets.NewString()
	for i := range events {
		j := i + 1
		if j >= len(events) {
			continue
		}
		for _, fn := range fns {
			if err := fn(events[i].Object, events[j].Object); err != nil {
				errs.Insert(err.Error())
			}
		}
	}
	if errs.Len() > 0 {
		return fmt.Errorf("invariants violated:\n* %s", strings.Join(errs.List(), "\n* "))
	}
	return nil
}

// Waits default amount of time (PodStartTimeout) for the specified pod to become running.
// Returns an error if timeout occurs first, or pod goes in to failed state.
func WaitForPodRunningInNamespace(c clientset.Interface, pod *api.Pod) error {
	// this short-cicuit is needed for cases when we pass a list of pods instead
	// of newly created pod (e.g. VerifyPods) which means we are getting already
	// running pod for which waiting does not make sense and will always fail
	if pod.Status.Phase == api.PodRunning {
		return nil
	}
	return waitTimeoutForPodRunningInNamespace(c, pod.Name, pod.Namespace, pod.ResourceVersion, PodStartTimeout)
}

// Waits default amount of time (PodStartTimeout) for the specified pod to become running.
// Returns an error if timeout occurs first, or pod goes in to failed state.
func WaitForPodNameRunningInNamespace(c clientset.Interface, podName, namespace string) error {
	return waitTimeoutForPodRunningInNamespace(c, podName, namespace, "", PodStartTimeout)
}

// Waits an extended amount of time (slowPodStartTimeout) for the specified pod to become running.
// The resourceVersion is used when Watching object changes, it tells since when we care
// about changes to the pod. Returns an error if timeout occurs first, or pod goes in to failed state.
func waitForPodRunningInNamespaceSlow(c clientset.Interface, podName, namespace, resourceVersion string) error {
	return waitTimeoutForPodRunningInNamespace(c, podName, namespace, resourceVersion, slowPodStartTimeout)
}

func waitTimeoutForPodRunningInNamespace(c clientset.Interface, podName, namespace, resourceVersion string, timeout time.Duration) error {
	w, err := c.Core().Pods(namespace).Watch(api.SingleObject(api.ObjectMeta{Name: podName, ResourceVersion: resourceVersion}))
	if err != nil {
		return err
	}
	_, err = watch.Until(timeout, w, client.PodRunning)
	return err
}

// Waits default amount of time (podNoLongerRunningTimeout) for the specified pod to stop running.
// Returns an error if timeout occurs first.
func WaitForPodNoLongerRunningInNamespace(c clientset.Interface, podName, namespace, resourceVersion string) error {
	return WaitTimeoutForPodNoLongerRunningInNamespace(c, podName, namespace, resourceVersion, podNoLongerRunningTimeout)
}

func WaitTimeoutForPodNoLongerRunningInNamespace(c clientset.Interface, podName, namespace, resourceVersion string, timeout time.Duration) error {
	w, err := c.Core().Pods(namespace).Watch(api.SingleObject(api.ObjectMeta{Name: podName, ResourceVersion: resourceVersion}))
	if err != nil {
		return err
	}
	_, err = watch.Until(timeout, w, client.PodCompleted)
	return err
}

func waitTimeoutForPodReadyInNamespace(c clientset.Interface, podName, namespace, resourceVersion string, timeout time.Duration) error {
	w, err := c.Core().Pods(namespace).Watch(api.SingleObject(api.ObjectMeta{Name: podName, ResourceVersion: resourceVersion}))
	if err != nil {
		return err
	}
	_, err = watch.Until(timeout, w, client.PodRunningAndReady)
	return err
}

// WaitForPodNotPending returns an error if it took too long for the pod to go out of pending state.
// The resourceVersion is used when Watching object changes, it tells since when we care
// about changes to the pod.
func WaitForPodNotPending(c clientset.Interface, ns, podName, resourceVersion string) error {
	w, err := c.Core().Pods(ns).Watch(api.SingleObject(api.ObjectMeta{Name: podName, ResourceVersion: resourceVersion}))
	if err != nil {
		return err
	}
	_, err = watch.Until(PodStartTimeout, w, client.PodNotPending)
	return err
}

// waitForPodTerminatedInNamespace returns an error if it took too long for the pod
// to terminate or if the pod terminated with an unexpected reason.
func waitForPodTerminatedInNamespace(c clientset.Interface, podName, reason, namespace string) error {
	return waitForPodCondition(c, namespace, podName, "terminated due to deadline exceeded", PodStartTimeout, func(pod *api.Pod) (bool, error) {
		if pod.Status.Phase == api.PodFailed {
			if pod.Status.Reason == reason {
				return true, nil
			} else {
				return true, fmt.Errorf("Expected pod %v in namespace %v to be terminated with reason %v, got reason: %v", podName, namespace, reason, pod.Status.Reason)
			}
		}

		return false, nil
	})
}

// waitForPodSuccessInNamespaceTimeout returns nil if the pod reached state success, or an error if it reached failure or ran too long.
func waitForPodSuccessInNamespaceTimeout(c clientset.Interface, podName string, namespace string, timeout time.Duration) error {
	return waitForPodCondition(c, namespace, podName, "success or failure", timeout, func(pod *api.Pod) (bool, error) {
		if pod.Spec.RestartPolicy == api.RestartPolicyAlways {
			return true, fmt.Errorf("pod %q will never terminate with a succeeded state since its restart policy is Always", podName)
		}
		switch pod.Status.Phase {
		case api.PodSucceeded:
			By("Saw pod success")
			return true, nil
		case api.PodFailed:
			return true, fmt.Errorf("pod %q failed with status: %+v", podName, pod.Status)
		default:
			return false, nil
		}
	})
}

// WaitForPodSuccessInNamespace returns nil if the pod reached state success, or an error if it reached failure or until podStartupTimeout.
func WaitForPodSuccessInNamespace(c clientset.Interface, podName string, namespace string) error {
	return waitForPodSuccessInNamespaceTimeout(c, podName, namespace, PodStartTimeout)
}

// WaitForPodSuccessInNamespaceSlow returns nil if the pod reached state success, or an error if it reached failure or until slowPodStartupTimeout.
func WaitForPodSuccessInNamespaceSlow(c clientset.Interface, podName string, namespace string) error {
	return waitForPodSuccessInNamespaceTimeout(c, podName, namespace, slowPodStartTimeout)
}

// waitForRCPodOnNode returns the pod from the given replication controller (described by rcName) which is scheduled on the given node.
// In case of failure or too long waiting time, an error is returned.
func waitForRCPodOnNode(c clientset.Interface, ns, rcName, node string) (*api.Pod, error) {
	label := labels.SelectorFromSet(labels.Set(map[string]string{"name": rcName}))
	var p *api.Pod = nil
	err := wait.PollImmediate(10*time.Second, 5*time.Minute, func() (bool, error) {
		Logf("Waiting for pod %s to appear on node %s", rcName, node)
		options := api.ListOptions{LabelSelector: label}
		pods, err := c.Core().Pods(ns).List(options)
		if err != nil {
			return false, err
		}
		for _, pod := range pods.Items {
			if pod.Spec.NodeName == node {
				Logf("Pod %s found on node %s", pod.Name, node)
				p = &pod
				return true, nil
			}
		}
		return false, nil
	})
	return p, err
}

// WaitForRCToStabilize waits till the RC has a matching generation/replica count between spec and status.
func WaitForRCToStabilize(c clientset.Interface, ns, name string, timeout time.Duration) error {
	options := api.ListOptions{FieldSelector: fields.Set{
		"metadata.name":      name,
		"metadata.namespace": ns,
	}.AsSelector()}
	w, err := c.Core().ReplicationControllers(ns).Watch(options)
	if err != nil {
		return err
	}
	_, err = watch.Until(timeout, w, func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Deleted:
			return false, apierrs.NewNotFound(unversioned.GroupResource{Resource: "replicationcontrollers"}, "")
		}
		switch rc := event.Object.(type) {
		case *api.ReplicationController:
			if rc.Name == name && rc.Namespace == ns &&
				rc.Generation <= rc.Status.ObservedGeneration &&
				rc.Spec.Replicas == rc.Status.Replicas {
				return true, nil
			}
			Logf("Waiting for rc %s to stabilize, generation %v observed generation %v spec.replicas %d status.replicas %d",
				name, rc.Generation, rc.Status.ObservedGeneration, rc.Spec.Replicas, rc.Status.Replicas)
		}
		return false, nil
	})
	return err
}

func WaitForPodToDisappear(c clientset.Interface, ns, podName string, label labels.Selector, interval, timeout time.Duration) error {
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		Logf("Waiting for pod %s to disappear", podName)
		options := api.ListOptions{LabelSelector: label}
		pods, err := c.Core().Pods(ns).List(options)
		if err != nil {
			return false, err
		}
		found := false
		for _, pod := range pods.Items {
			if pod.Name == podName {
				Logf("Pod %s still exists", podName)
				found = true
				break
			}
		}
		if !found {
			Logf("Pod %s no longer exists", podName)
			return true, nil
		}
		return false, nil
	})
}

// WaitForRCPodToDisappear returns nil if the pod from the given replication controller (described by rcName) no longer exists.
// In case of failure or too long waiting time, an error is returned.
func WaitForRCPodToDisappear(c clientset.Interface, ns, rcName, podName string) error {
	label := labels.SelectorFromSet(labels.Set(map[string]string{"name": rcName}))
	// NodeController evicts pod after 5 minutes, so we need timeout greater than that to observe effects.
	// The grace period must be set to 0 on the pod for it to be deleted during the partition.
	// Otherwise, it goes to the 'Terminating' state till the kubelet confirms deletion.
	return WaitForPodToDisappear(c, ns, podName, label, 20*time.Second, 10*time.Minute)
}

// WaitForService waits until the service appears (exist == true), or disappears (exist == false)
func WaitForService(c clientset.Interface, namespace, name string, exist bool, interval, timeout time.Duration) error {
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		_, err := c.Core().Services(namespace).Get(name)
		switch {
		case err == nil:
			if !exist {
				return false, nil
			}
			Logf("Service %s in namespace %s found.", name, namespace)
			return true, nil
		case apierrs.IsNotFound(err):
			if exist {
				return false, nil
			}
			Logf("Service %s in namespace %s disappeared.", name, namespace)
			return true, nil
		default:
			Logf("Get service %s in namespace %s failed: %v", name, namespace, err)
			return false, nil
		}
	})
	if err != nil {
		stateMsg := map[bool]string{true: "to appear", false: "to disappear"}
		return fmt.Errorf("error waiting for service %s/%s %s: %v", namespace, name, stateMsg[exist], err)
	}
	return nil
}

//WaitForServiceEndpointsNum waits until the amount of endpoints that implement service to expectNum.
func WaitForServiceEndpointsNum(c clientset.Interface, namespace, serviceName string, expectNum int, interval, timeout time.Duration) error {
	return wait.Poll(interval, timeout, func() (bool, error) {
		Logf("Waiting for amount of service:%s endpoints to be %d", serviceName, expectNum)
		list, err := c.Core().Endpoints(namespace).List(api.ListOptions{})
		if err != nil {
			return false, err
		}

		for _, e := range list.Items {
			if e.Name == serviceName && countEndpointsNum(&e) == expectNum {
				return true, nil
			}
		}
		return false, nil
	})
}

func countEndpointsNum(e *api.Endpoints) int {
	num := 0
	for _, sub := range e.Subsets {
		num += len(sub.Addresses)
	}
	return num
}

// WaitForReplicationController waits until the RC appears (exist == true), or disappears (exist == false)
func WaitForReplicationController(c clientset.Interface, namespace, name string, exist bool, interval, timeout time.Duration) error {
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		_, err := c.Core().ReplicationControllers(namespace).Get(name)
		if err != nil {
			Logf("Get ReplicationController %s in namespace %s failed (%v).", name, namespace, err)
			return !exist, nil
		} else {
			Logf("ReplicationController %s in namespace %s found.", name, namespace)
			return exist, nil
		}
	})
	if err != nil {
		stateMsg := map[bool]string{true: "to appear", false: "to disappear"}
		return fmt.Errorf("error waiting for ReplicationController %s/%s %s: %v", namespace, name, stateMsg[exist], err)
	}
	return nil
}

func WaitForEndpoint(c clientset.Interface, ns, name string) error {
	for t := time.Now(); time.Since(t) < EndpointRegisterTimeout; time.Sleep(Poll) {
		endpoint, err := c.Core().Endpoints(ns).Get(name)
		Expect(err).NotTo(HaveOccurred())
		if len(endpoint.Subsets) == 0 || len(endpoint.Subsets[0].Addresses) == 0 {
			Logf("Endpoint %s/%s is not ready yet", ns, name)
			continue
		} else {
			return nil
		}
	}
	return fmt.Errorf("Failed to get endpoints for %s/%s", ns, name)
}

// Context for checking pods responses by issuing GETs to them (via the API
// proxy) and verifying that they answer with ther own pod name.
type podProxyResponseChecker struct {
	c              clientset.Interface
	ns             string
	label          labels.Selector
	controllerName string
	respondName    bool // Whether the pod should respond with its own name.
	pods           *api.PodList
}

func PodProxyResponseChecker(c clientset.Interface, ns string, label labels.Selector, controllerName string, respondName bool, pods *api.PodList) podProxyResponseChecker {
	return podProxyResponseChecker{c, ns, label, controllerName, respondName, pods}
}

// CheckAllResponses issues GETs to all pods in the context and verify they
// reply with their own pod name.
func (r podProxyResponseChecker) CheckAllResponses() (done bool, err error) {
	successes := 0
	options := api.ListOptions{LabelSelector: r.label}
	currentPods, err := r.c.Core().Pods(r.ns).List(options)
	Expect(err).NotTo(HaveOccurred())
	for i, pod := range r.pods.Items {
		// Check that the replica list remains unchanged, otherwise we have problems.
		if !isElementOf(pod.UID, currentPods) {
			return false, fmt.Errorf("pod with UID %s is no longer a member of the replica set.  Must have been restarted for some reason.  Current replica set: %v", pod.UID, currentPods)
		}
		subResourceProxyAvailable, err := ServerVersionGTE(SubResourcePodProxyVersion, r.c.Discovery())
		if err != nil {
			return false, err
		}
		var body []byte
		if subResourceProxyAvailable {
			body, err = r.c.Core().RESTClient().Get().
				Namespace(r.ns).
				Resource("pods").
				SubResource("proxy").
				Name(string(pod.Name)).
				Do().
				Raw()
		} else {
			body, err = r.c.Core().RESTClient().Get().
				Prefix("proxy").
				Namespace(r.ns).
				Resource("pods").
				Name(string(pod.Name)).
				Do().
				Raw()
		}
		if err != nil {
			Logf("Controller %s: Failed to GET from replica %d [%s]: %v\npod status: %#v", r.controllerName, i+1, pod.Name, err, pod.Status)
			continue
		}
		// The response checker expects the pod's name unless !respondName, in
		// which case it just checks for a non-empty response.
		got := string(body)
		what := ""
		if r.respondName {
			what = "expected"
			want := pod.Name
			if got != want {
				Logf("Controller %s: Replica %d [%s] expected response %q but got %q",
					r.controllerName, i+1, pod.Name, want, got)
				continue
			}
		} else {
			what = "non-empty"
			if len(got) == 0 {
				Logf("Controller %s: Replica %d [%s] expected non-empty response",
					r.controllerName, i+1, pod.Name)
				continue
			}
		}
		successes++
		Logf("Controller %s: Got %s result from replica %d [%s]: %q, %d of %d required successes so far",
			r.controllerName, what, i+1, pod.Name, got, successes, len(r.pods.Items))
	}
	if successes < len(r.pods.Items) {
		return false, nil
	}
	return true, nil
}

// ServerVersionGTE returns true if v is greater than or equal to the server
// version.
//
// TODO(18726): This should be incorporated into client.VersionInterface.
func ServerVersionGTE(v semver.Version, c discovery.ServerVersionInterface) (bool, error) {
	serverVersion, err := c.ServerVersion()
	if err != nil {
		return false, fmt.Errorf("Unable to get server version: %v", err)
	}
	sv, err := version.Parse(serverVersion.GitVersion)
	if err != nil {
		return false, fmt.Errorf("Unable to parse server version %q: %v", serverVersion.GitVersion, err)
	}
	return sv.GTE(v), nil
}

func SkipUnlessKubectlVersionGTE(v semver.Version) {
	gte, err := KubectlVersionGTE(v)
	if err != nil {
		Failf("Failed to get kubectl version: %v", err)
	}
	if !gte {
		Skipf("Not supported for kubectl versions before %q", v)
	}
}

// KubectlVersionGTE returns true if the kubectl version is greater than or
// equal to v.
func KubectlVersionGTE(v semver.Version) (bool, error) {
	kv, err := KubectlVersion()
	if err != nil {
		return false, err
	}
	return kv.GTE(v), nil
}

// KubectlVersion gets the version of kubectl that's currently being used (see
// --kubectl-path in e2e.go to use an alternate kubectl).
func KubectlVersion() (semver.Version, error) {
	output := RunKubectlOrDie("version", "--client")
	matches := gitVersionRegexp.FindStringSubmatch(output)
	if len(matches) != 2 {
		return semver.Version{}, fmt.Errorf("Could not find kubectl version in output %v", output)
	}
	// Don't use the full match, as it contains "GitVersion:\"" and a
	// trailing "\"".  Just use the submatch.
	return version.Parse(matches[1])
}

func PodsResponding(c clientset.Interface, ns, name string, wantName bool, pods *api.PodList) error {
	By("trying to dial each unique pod")
	label := labels.SelectorFromSet(labels.Set(map[string]string{"name": name}))
	return wait.PollImmediate(Poll, podRespondingTimeout, PodProxyResponseChecker(c, ns, label, name, wantName, pods).CheckAllResponses)
}

func PodsCreated(c clientset.Interface, ns, name string, replicas int32) (*api.PodList, error) {
	label := labels.SelectorFromSet(labels.Set(map[string]string{"name": name}))
	return PodsCreatedByLabel(c, ns, name, replicas, label)
}

func PodsCreatedByLabel(c clientset.Interface, ns, name string, replicas int32, label labels.Selector) (*api.PodList, error) {
	timeout := 2 * time.Minute
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(5 * time.Second) {
		options := api.ListOptions{LabelSelector: label}

		// List the pods, making sure we observe all the replicas.
		pods, err := c.Core().Pods(ns).List(options)
		if err != nil {
			return nil, err
		}

		created := []api.Pod{}
		for _, pod := range pods.Items {
			if pod.DeletionTimestamp != nil {
				continue
			}
			created = append(created, pod)
		}
		Logf("Pod name %s: Found %d pods out of %d", name, len(created), replicas)

		if int32(len(created)) == replicas {
			pods.Items = created
			return pods, nil
		}
	}
	return nil, fmt.Errorf("Pod name %s: Gave up waiting %v for %d pods to come up", name, timeout, replicas)
}

func podsRunning(c clientset.Interface, pods *api.PodList) []error {
	// Wait for the pods to enter the running state. Waiting loops until the pods
	// are running so non-running pods cause a timeout for this test.
	By("ensuring each pod is running")
	e := []error{}
	error_chan := make(chan error)

	for _, pod := range pods.Items {
		go func(p api.Pod) {
			error_chan <- WaitForPodRunningInNamespace(c, &p)
		}(pod)
	}

	for range pods.Items {
		err := <-error_chan
		if err != nil {
			e = append(e, err)
		}
	}

	return e
}

func VerifyPods(c clientset.Interface, ns, name string, wantName bool, replicas int32) error {
	pods, err := PodsCreated(c, ns, name, replicas)
	if err != nil {
		return err
	}
	e := podsRunning(c, pods)
	if len(e) > 0 {
		return fmt.Errorf("failed to wait for pods running: %v", e)
	}
	err = PodsResponding(c, ns, name, wantName, pods)
	if err != nil {
		return fmt.Errorf("failed to wait for pods responding: %v", err)
	}
	return nil
}

func ServiceResponding(c clientset.Interface, ns, name string) error {
	By(fmt.Sprintf("trying to dial the service %s.%s via the proxy", ns, name))

	return wait.PollImmediate(Poll, ServiceRespondingTimeout, func() (done bool, err error) {
		proxyRequest, errProxy := GetServicesProxyRequest(c, c.Core().RESTClient().Get())
		if errProxy != nil {
			Logf("Failed to get services proxy request: %v:", errProxy)
			return false, nil
		}
		body, err := proxyRequest.Namespace(ns).
			Name(name).
			Do().
			Raw()
		if err != nil {
			Logf("Failed to GET from service %s: %v:", name, err)
			return false, nil
		}
		got := string(body)
		if len(got) == 0 {
			Logf("Service %s: expected non-empty response", name)
			return false, err // stop polling
		}
		Logf("Service %s: found nonempty answer: %s", name, got)
		return true, nil
	})
}

func restclientConfig(kubeContext string) (*clientcmdapi.Config, error) {
	Logf(">>> kubeConfig: %s\n", TestContext.KubeConfig)
	if TestContext.KubeConfig == "" {
		return nil, fmt.Errorf("KubeConfig must be specified to load client config")
	}
	c, err := clientcmd.LoadFromFile(TestContext.KubeConfig)
	if err != nil {
		return nil, fmt.Errorf("error loading KubeConfig: %v", err.Error())
	}
	if kubeContext != "" {
		Logf(">>> kubeContext: %s\n", kubeContext)
		c.CurrentContext = kubeContext
	}
	return c, nil
}

type ClientConfigGetter func() (*restclient.Config, error)

func LoadConfig() (*restclient.Config, error) {
	if TestContext.NodeE2E {
		// This is a node e2e test, apply the node e2e configuration
		return &restclient.Config{Host: TestContext.Host}, nil
	}
	c, err := restclientConfig(TestContext.KubeContext)
	if err != nil {
		return nil, err
	}

	return clientcmd.NewDefaultClientConfig(*c, &clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: TestContext.Host}}).ClientConfig()
}

func LoadFederatedConfig(overrides *clientcmd.ConfigOverrides) (*restclient.Config, error) {
	c, err := restclientConfig(federatedKubeContext)
	if err != nil {
		return nil, fmt.Errorf("error creating federation client config: %v", err.Error())
	}
	cfg, err := clientcmd.NewDefaultClientConfig(*c, overrides).ClientConfig()
	if cfg != nil {
		//TODO(colhom): this is only here because https://github.com/kubernetes/kubernetes/issues/25422
		cfg.NegotiatedSerializer = api.Codecs
	}
	if err != nil {
		return cfg, fmt.Errorf("error creating federation client config: %v", err.Error())
	}
	return cfg, nil
}

func LoadFederationClientset_1_5() (*federation_release_1_5.Clientset, error) {
	config, err := LoadFederatedConfig(&clientcmd.ConfigOverrides{})
	if err != nil {
		return nil, err
	}

	c, err := federation_release_1_5.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating federation clientset: %v", err.Error())
	}
	return c, nil
}

func LoadInternalClientset() (*clientset.Clientset, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("error creating client: %v", err.Error())
	}
	return clientset.NewForConfig(config)
}

func LoadClientset() (*release_1_5.Clientset, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("error creating client: %v", err.Error())
	}
	return release_1_5.NewForConfig(config)
}

// randomSuffix provides a random string to append to pods,services,rcs.
// TODO: Allow service names to have the same form as names
//       for pods and replication controllers so we don't
//       need to use such a function and can instead
//       use the UUID utility function.
func randomSuffix() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return strconv.Itoa(r.Int() % 10000)
}

func ExpectNoError(err error, explain ...interface{}) {
	if err != nil {
		Logf("Unexpected error occurred: %v", err)
	}
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), explain...)
}

func ExpectNoErrorWithRetries(fn func() error, maxRetries int, explain ...interface{}) {
	var err error
	for i := 0; i < maxRetries; i++ {
		err = fn()
		if err == nil {
			return
		}
		Logf("(Attempt %d of %d) Unexpected error occurred: %v", i+1, maxRetries, err)
	}
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), explain...)
}

// Stops everything from filePath from namespace ns and checks if everything matching selectors from the given namespace is correctly stopped.
func Cleanup(filePath, ns string, selectors ...string) {
	By("using delete to clean up resources")
	var nsArg string
	if ns != "" {
		nsArg = fmt.Sprintf("--namespace=%s", ns)
	}
	RunKubectlOrDie("delete", "--grace-period=0", "-f", filePath, nsArg)
	AssertCleanup(ns, selectors...)
}

// Asserts that cleanup of a namespace wrt selectors occurred.
func AssertCleanup(ns string, selectors ...string) {
	var nsArg string
	if ns != "" {
		nsArg = fmt.Sprintf("--namespace=%s", ns)
	}
	for _, selector := range selectors {
		resources := RunKubectlOrDie("get", "rc,svc", "-l", selector, "--no-headers", nsArg)
		if resources != "" {
			Failf("Resources left running after stop:\n%s", resources)
		}
		pods := RunKubectlOrDie("get", "pods", "-l", selector, nsArg, "-o", "go-template={{ range .items }}{{ if not .metadata.deletionTimestamp }}{{ .metadata.name }}{{ \"\\n\" }}{{ end }}{{ end }}")
		if pods != "" {
			Failf("Pods left unterminated after stop:\n%s", pods)
		}
	}
}

// validatorFn is the function which is individual tests will implement.
// we may want it to return more than just an error, at some point.
type validatorFn func(c clientset.Interface, podID string) error

// ValidateController is a generic mechanism for testing RC's that are running.
// It takes a container name, a test name, and a validator function which is plugged in by a specific test.
// "containername": this is grepped for.
// "containerImage" : this is the name of the image we expect to be launched.  Not to confuse w/ images (kitten.jpg)  which are validated.
// "testname":  which gets bubbled up to the logging/failure messages if errors happen.
// "validator" function: This function is given a podID and a client, and it can do some specific validations that way.
func ValidateController(c clientset.Interface, containerImage string, replicas int, containername string, testname string, validator validatorFn, ns string) {
	getPodsTemplate := "--template={{range.items}}{{.metadata.name}} {{end}}"
	// NB: kubectl adds the "exists" function to the standard template functions.
	// This lets us check to see if the "running" entry exists for each of the containers
	// we care about. Exists will never return an error and it's safe to check a chain of
	// things, any one of which may not exist. In the below template, all of info,
	// containername, and running might be nil, so the normal index function isn't very
	// helpful.
	// This template is unit-tested in kubectl, so if you change it, update the unit test.
	// You can read about the syntax here: http://golang.org/pkg/text/template/.
	getContainerStateTemplate := fmt.Sprintf(`--template={{if (exists . "status" "containerStatuses")}}{{range .status.containerStatuses}}{{if (and (eq .name "%s") (exists . "state" "running"))}}true{{end}}{{end}}{{end}}`, containername)

	getImageTemplate := fmt.Sprintf(`--template={{if (exists . "status" "containerStatuses")}}{{range .status.containerStatuses}}{{if eq .name "%s"}}{{.image}}{{end}}{{end}}{{end}}`, containername)

	By(fmt.Sprintf("waiting for all containers in %s pods to come up.", testname)) //testname should be selector
waitLoop:
	for start := time.Now(); time.Since(start) < PodStartTimeout; time.Sleep(5 * time.Second) {
		getPodsOutput := RunKubectlOrDie("get", "pods", "-o", "template", getPodsTemplate, "-l", testname, fmt.Sprintf("--namespace=%v", ns))
		pods := strings.Fields(getPodsOutput)
		if numPods := len(pods); numPods != replicas {
			By(fmt.Sprintf("Replicas for %s: expected=%d actual=%d", testname, replicas, numPods))
			continue
		}
		var runningPods []string
		for _, podID := range pods {
			running := RunKubectlOrDie("get", "pods", podID, "-o", "template", getContainerStateTemplate, fmt.Sprintf("--namespace=%v", ns))
			if running != "true" {
				Logf("%s is created but not running", podID)
				continue waitLoop
			}

			currentImage := RunKubectlOrDie("get", "pods", podID, "-o", "template", getImageTemplate, fmt.Sprintf("--namespace=%v", ns))
			if currentImage != containerImage {
				Logf("%s is created but running wrong image; expected: %s, actual: %s", podID, containerImage, currentImage)
				continue waitLoop
			}

			// Call the generic validator function here.
			// This might validate for example, that (1) getting a url works and (2) url is serving correct content.
			if err := validator(c, podID); err != nil {
				Logf("%s is running right image but validator function failed: %v", podID, err)
				continue waitLoop
			}

			Logf("%s is verified up and running", podID)
			runningPods = append(runningPods, podID)
		}
		// If we reach here, then all our checks passed.
		if len(runningPods) == replicas {
			return
		}
	}
	// Reaching here means that one of more checks failed multiple times.  Assuming its not a race condition, something is broken.
	Failf("Timed out after %v seconds waiting for %s pods to reach valid state", PodStartTimeout.Seconds(), testname)
}

// KubectlCmd runs the kubectl executable through the wrapper script.
func KubectlCmd(args ...string) *exec.Cmd {
	defaultArgs := []string{}

	// Reference a --server option so tests can run anywhere.
	if TestContext.Host != "" {
		defaultArgs = append(defaultArgs, "--"+clientcmd.FlagAPIServer+"="+TestContext.Host)
	}
	if TestContext.KubeConfig != "" {
		defaultArgs = append(defaultArgs, "--"+clientcmd.RecommendedConfigPathFlag+"="+TestContext.KubeConfig)

		// Reference the KubeContext
		if TestContext.KubeContext != "" {
			defaultArgs = append(defaultArgs, "--"+clientcmd.FlagContext+"="+TestContext.KubeContext)
		}

	} else {
		if TestContext.CertDir != "" {
			defaultArgs = append(defaultArgs,
				fmt.Sprintf("--certificate-authority=%s", filepath.Join(TestContext.CertDir, "ca.crt")),
				fmt.Sprintf("--client-certificate=%s", filepath.Join(TestContext.CertDir, "kubecfg.crt")),
				fmt.Sprintf("--client-key=%s", filepath.Join(TestContext.CertDir, "kubecfg.key")))
		}
	}
	kubectlArgs := append(defaultArgs, args...)

	//We allow users to specify path to kubectl, so you can test either "kubectl" or "cluster/kubectl.sh"
	//and so on.
	cmd := exec.Command(TestContext.KubectlPath, kubectlArgs...)

	//caller will invoke this and wait on it.
	return cmd
}

// kubectlBuilder is used to build, customize and execute a kubectl Command.
// Add more functions to customize the builder as needed.
type kubectlBuilder struct {
	cmd     *exec.Cmd
	timeout <-chan time.Time
}

func NewKubectlCommand(args ...string) *kubectlBuilder {
	b := new(kubectlBuilder)
	b.cmd = KubectlCmd(args...)
	return b
}

func (b *kubectlBuilder) WithEnv(env []string) *kubectlBuilder {
	b.cmd.Env = env
	return b
}

func (b *kubectlBuilder) WithTimeout(t <-chan time.Time) *kubectlBuilder {
	b.timeout = t
	return b
}

func (b kubectlBuilder) WithStdinData(data string) *kubectlBuilder {
	b.cmd.Stdin = strings.NewReader(data)
	return &b
}

func (b kubectlBuilder) WithStdinReader(reader io.Reader) *kubectlBuilder {
	b.cmd.Stdin = reader
	return &b
}

func (b kubectlBuilder) ExecOrDie() string {
	str, err := b.Exec()
	Logf("stdout: %q", str)
	// In case of i/o timeout error, try talking to the apiserver again after 2s before dying.
	// Note that we're still dying after retrying so that we can get visibility to triage it further.
	if isTimeout(err) {
		Logf("Hit i/o timeout error, talking to the server 2s later to see if it's temporary.")
		time.Sleep(2 * time.Second)
		retryStr, retryErr := RunKubectl("version")
		Logf("stdout: %q", retryStr)
		Logf("err: %v", retryErr)
	}
	Expect(err).NotTo(HaveOccurred())
	return str
}

func isTimeout(err error) bool {
	switch err := err.(type) {
	case net.Error:
		if err.Timeout() {
			return true
		}
	case *url.Error:
		if err, ok := err.Err.(net.Error); ok && err.Timeout() {
			return true
		}
	}
	return false
}

func (b kubectlBuilder) Exec() (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := b.cmd
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	Logf("Running '%s %s'", cmd.Path, strings.Join(cmd.Args[1:], " ")) // skip arg[0] as it is printed separately
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("error starting %v:\nCommand stdout:\n%v\nstderr:\n%v\nerror:\n%v\n", cmd, cmd.Stdout, cmd.Stderr, err)
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- cmd.Wait()
	}()
	select {
	case err := <-errCh:
		if err != nil {
			var rc int = 127
			if ee, ok := err.(*exec.ExitError); ok {
				Logf("rc: %d", rc)
				rc = int(ee.Sys().(syscall.WaitStatus).ExitStatus())
			}
			return "", uexec.CodeExitError{
				Err:  fmt.Errorf("error running %v:\nCommand stdout:\n%v\nstderr:\n%v\nerror:\n%v\n", cmd, cmd.Stdout, cmd.Stderr, err),
				Code: rc,
			}
		}
	case <-b.timeout:
		b.cmd.Process.Kill()
		return "", fmt.Errorf("timed out waiting for command %v:\nCommand stdout:\n%v\nstderr:\n%v\n", cmd, cmd.Stdout, cmd.Stderr)
	}
	Logf("stderr: %q", stderr.String())
	return stdout.String(), nil
}

// RunKubectlOrDie is a convenience wrapper over kubectlBuilder
func RunKubectlOrDie(args ...string) string {
	return NewKubectlCommand(args...).ExecOrDie()
}

// RunKubectl is a convenience wrapper over kubectlBuilder
func RunKubectl(args ...string) (string, error) {
	return NewKubectlCommand(args...).Exec()
}

// RunKubectlOrDieInput is a convenience wrapper over kubectlBuilder that takes input to stdin
func RunKubectlOrDieInput(data string, args ...string) string {
	return NewKubectlCommand(args...).WithStdinData(data).ExecOrDie()
}

func StartCmdAndStreamOutput(cmd *exec.Cmd) (stdout, stderr io.ReadCloser, err error) {
	stdout, err = cmd.StdoutPipe()
	if err != nil {
		return
	}
	stderr, err = cmd.StderrPipe()
	if err != nil {
		return
	}
	Logf("Asynchronously running '%s %s'", cmd.Path, strings.Join(cmd.Args, " "))
	err = cmd.Start()
	return
}

// Rough equivalent of ctrl+c for cleaning up processes. Intended to be run in defer.
func TryKill(cmd *exec.Cmd) {
	if err := cmd.Process.Kill(); err != nil {
		Logf("ERROR failed to kill command %v! The process may leak", cmd)
	}
}

// testContainerOutputMatcher runs the given pod in the given namespace and waits
// for all of the containers in the podSpec to move into the 'Success' status, and tests
// the specified container log against the given expected output using the given matcher.
func (f *Framework) testContainerOutputMatcher(scenarioName string,
	pod *api.Pod,
	containerIndex int,
	expectedOutput []string,
	matcher func(string, ...interface{}) gomegatypes.GomegaMatcher) {
	By(fmt.Sprintf("Creating a pod to test %v", scenarioName))
	if containerIndex < 0 || containerIndex >= len(pod.Spec.Containers) {
		Failf("Invalid container index: %d", containerIndex)
	}
	ExpectNoError(f.MatchContainerOutput(pod, pod.Spec.Containers[containerIndex].Name, expectedOutput, matcher))
}

// MatchContainerOutput creates a pod and waits for all it's containers to exit with success.
// It then tests that the matcher with each expectedOutput matches the output of the specified container.
func (f *Framework) MatchContainerOutput(
	pod *api.Pod,
	containerName string,
	expectedOutput []string,
	matcher func(string, ...interface{}) gomegatypes.GomegaMatcher) error {
	podClient := f.PodClient()
	ns := f.Namespace.Name

	createdPod := podClient.Create(pod)
	defer func() {
		By("delete the pod")
		podClient.DeleteSync(createdPod.Name, &api.DeleteOptions{}, podNoLongerRunningTimeout)
	}()

	// Wait for client pod to complete.
	if err := WaitForPodSuccessInNamespace(f.ClientSet, createdPod.Name, ns); err != nil {
		return fmt.Errorf("expected pod %q success: %v", pod.Name, err)
	}

	// Grab its logs.  Get host first.
	podStatus, err := podClient.Get(createdPod.Name)
	if err != nil {
		return fmt.Errorf("failed to get pod status: %v", err)
	}

	Logf("Trying to get logs from node %s pod %s container %s: %v",
		podStatus.Spec.NodeName, podStatus.Name, containerName, err)

	// Sometimes the actual containers take a second to get started, try to get logs for 60s
	logs, err := GetPodLogs(f.ClientSet, ns, podStatus.Name, containerName)
	if err != nil {
		Logf("Failed to get logs from node %q pod %q container %q. %v",
			podStatus.Spec.NodeName, podStatus.Name, containerName, err)
		return fmt.Errorf("failed to get logs from %s for %s: %v", podStatus.Name, containerName, err)
	}

	for _, expected := range expectedOutput {
		m := matcher(expected)
		matches, err := m.Match(logs)
		if err != nil {
			return fmt.Errorf("expected %q in container output: %v", expected, err)
		} else if !matches {
			return fmt.Errorf("expected %q in container output: %s", expected, m.FailureMessage(logs))
		}
	}

	return nil
}

func RunDeployment(config testutils.DeploymentConfig) error {
	By(fmt.Sprintf("creating deployment %s in namespace %s", config.Name, config.Namespace))
	config.NodeDumpFunc = DumpNodeDebugInfo
	config.ContainerDumpFunc = LogFailedContainers
	return testutils.RunDeployment(config)
}

func RunReplicaSet(config testutils.ReplicaSetConfig) error {
	By(fmt.Sprintf("creating replicaset %s in namespace %s", config.Name, config.Namespace))
	config.NodeDumpFunc = DumpNodeDebugInfo
	config.ContainerDumpFunc = LogFailedContainers
	return testutils.RunReplicaSet(config)
}

func RunRC(config testutils.RCConfig) error {
	By(fmt.Sprintf("creating replication controller %s in namespace %s", config.Name, config.Namespace))
	config.NodeDumpFunc = DumpNodeDebugInfo
	config.ContainerDumpFunc = LogFailedContainers
	return testutils.RunRC(config)
}

type EventsLister func(opts v1.ListOptions, ns string) (*v1.EventList, error)

func DumpEventsInNamespace(eventsLister EventsLister, namespace string) {
	By(fmt.Sprintf("Collecting events from namespace %q.", namespace))
	events, err := eventsLister(v1.ListOptions{}, namespace)
	Expect(err).NotTo(HaveOccurred())

	By(fmt.Sprintf("Found %d events.", len(events.Items)))
	// Sort events by their first timestamp
	sortedEvents := events.Items
	if len(sortedEvents) > 1 {
		sort.Sort(byFirstTimestamp(sortedEvents))
	}
	for _, e := range sortedEvents {
		Logf("At %v - event for %v: %v %v: %v", e.FirstTimestamp, e.InvolvedObject.Name, e.Source, e.Reason, e.Message)
	}
	// Note that we don't wait for any Cleanup to propagate, which means
	// that if you delete a bunch of pods right before ending your test,
	// you may or may not see the killing/deletion/Cleanup events.
}

func DumpAllNamespaceInfo(c clientset.Interface, cs *release_1_5.Clientset, namespace string) {
	DumpEventsInNamespace(func(opts v1.ListOptions, ns string) (*v1.EventList, error) {
		return cs.Core().Events(ns).List(opts)
	}, namespace)

	// If cluster is large, then the following logs are basically useless, because:
	// 1. it takes tens of minutes or hours to grab all of them
	// 2. there are so many of them that working with them are mostly impossible
	// So we dump them only if the cluster is relatively small.
	maxNodesForDump := 20
	if nodes, err := c.Core().Nodes().List(api.ListOptions{}); err == nil {
		if len(nodes.Items) <= maxNodesForDump {
			dumpAllPodInfo(c)
			dumpAllNodeInfo(c)
		} else {
			Logf("skipping dumping cluster info - cluster too large")
		}
	} else {
		Logf("unable to fetch node list: %v", err)
	}
}

// byFirstTimestamp sorts a slice of events by first timestamp, using their involvedObject's name as a tie breaker.
type byFirstTimestamp []v1.Event

func (o byFirstTimestamp) Len() int      { return len(o) }
func (o byFirstTimestamp) Swap(i, j int) { o[i], o[j] = o[j], o[i] }

func (o byFirstTimestamp) Less(i, j int) bool {
	if o[i].FirstTimestamp.Equal(o[j].FirstTimestamp) {
		return o[i].InvolvedObject.Name < o[j].InvolvedObject.Name
	}
	return o[i].FirstTimestamp.Before(o[j].FirstTimestamp)
}

func dumpAllPodInfo(c clientset.Interface) {
	pods, err := c.Core().Pods("").List(api.ListOptions{})
	if err != nil {
		Logf("unable to fetch pod debug info: %v", err)
	}
	logPodStates(pods.Items)
}

func dumpAllNodeInfo(c clientset.Interface) {
	// It should be OK to list unschedulable Nodes here.
	nodes, err := c.Core().Nodes().List(api.ListOptions{})
	if err != nil {
		Logf("unable to fetch node list: %v", err)
		return
	}
	names := make([]string, len(nodes.Items))
	for ix := range nodes.Items {
		names[ix] = nodes.Items[ix].Name
	}
	DumpNodeDebugInfo(c, names, Logf)
}

func DumpNodeDebugInfo(c clientset.Interface, nodeNames []string, logFunc func(fmt string, args ...interface{})) {
	for _, n := range nodeNames {
		logFunc("\nLogging node info for node %v", n)
		node, err := c.Core().Nodes().Get(n)
		if err != nil {
			logFunc("Error getting node info %v", err)
		}
		logFunc("Node Info: %v", node)

		logFunc("\nLogging kubelet events for node %v", n)
		for _, e := range getNodeEvents(c, n) {
			logFunc("source %v type %v message %v reason %v first ts %v last ts %v, involved obj %+v",
				e.Source, e.Type, e.Message, e.Reason, e.FirstTimestamp, e.LastTimestamp, e.InvolvedObject)
		}
		logFunc("\nLogging pods the kubelet thinks is on node %v", n)
		podList, err := GetKubeletPods(c, n)
		if err != nil {
			logFunc("Unable to retrieve kubelet pods for node %v", n)
			continue
		}
		for _, p := range podList.Items {
			logFunc("%v started at %v (%d+%d container statuses recorded)", p.Name, p.Status.StartTime, len(p.Status.InitContainerStatuses), len(p.Status.ContainerStatuses))
			for _, c := range p.Status.InitContainerStatuses {
				logFunc("\tInit container %v ready: %v, restart count %v",
					c.Name, c.Ready, c.RestartCount)
			}
			for _, c := range p.Status.ContainerStatuses {
				logFunc("\tContainer %v ready: %v, restart count %v",
					c.Name, c.Ready, c.RestartCount)
			}
		}
		HighLatencyKubeletOperations(c, 10*time.Second, n, logFunc)
		// TODO: Log node resource info
	}
}

// logNodeEvents logs kubelet events from the given node. This includes kubelet
// restart and node unhealthy events. Note that listing events like this will mess
// with latency metrics, beware of calling it during a test.
func getNodeEvents(c clientset.Interface, nodeName string) []api.Event {
	selector := fields.Set{
		"involvedObject.kind":      "Node",
		"involvedObject.name":      nodeName,
		"involvedObject.namespace": api.NamespaceAll,
		"source":                   "kubelet",
	}.AsSelector()
	options := api.ListOptions{FieldSelector: selector}
	events, err := c.Core().Events(api.NamespaceSystem).List(options)
	if err != nil {
		Logf("Unexpected error retrieving node events %v", err)
		return []api.Event{}
	}
	return events.Items
}

// waitListSchedulableNodesOrDie is a wrapper around listing nodes supporting retries.
func waitListSchedulableNodesOrDie(c clientset.Interface) *api.NodeList {
	var nodes *api.NodeList
	var err error
	if wait.PollImmediate(Poll, SingleCallTimeout, func() (bool, error) {
		nodes, err = c.Core().Nodes().List(api.ListOptions{FieldSelector: fields.Set{
			"spec.unschedulable": "false",
		}.AsSelector()})
		return err == nil, nil
	}) != nil {
		ExpectNoError(err, "Timed out while listing nodes for e2e cluster.")
	}
	return nodes
}

// Node is schedulable if:
// 1) doesn't have "unschedulable" field set
// 2) it's Ready condition is set to true
// 3) doesn't have NetworkUnavailable condition set to true
func isNodeSchedulable(node *api.Node) bool {
	nodeReady := IsNodeConditionSetAsExpected(node, api.NodeReady, true)
	networkReady := IsNodeConditionUnset(node, api.NodeNetworkUnavailable) ||
		IsNodeConditionSetAsExpectedSilent(node, api.NodeNetworkUnavailable, false)
	return !node.Spec.Unschedulable && nodeReady && networkReady
}

// Test whether a fake pod can be scheduled on "node", given its current taints.
func isNodeUntainted(node *api.Node) bool {
	fakePod := &api.Pod{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Pod",
			APIVersion: registered.GroupOrDie(api.GroupName).GroupVersion.String(),
		},
		ObjectMeta: api.ObjectMeta{
			Name:      "fake-not-scheduled",
			Namespace: "fake-not-scheduled",
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				{
					Name:  "fake-not-scheduled",
					Image: "fake-not-scheduled",
				},
			},
		},
	}
	nodeInfo := schedulercache.NewNodeInfo()
	nodeInfo.SetNode(node)
	fit, _, err := predicates.PodToleratesNodeTaints(fakePod, nil, nodeInfo)
	if err != nil {
		Failf("Can't test predicates for node %s: %v", node.Name, err)
		return false
	}
	return fit
}

// GetReadySchedulableNodesOrDie addresses the common use case of getting nodes you can do work on.
// 1) Needs to be schedulable.
// 2) Needs to be ready.
// If EITHER 1 or 2 is not true, most tests will want to ignore the node entirely.
func GetReadySchedulableNodesOrDie(c clientset.Interface) (nodes *api.NodeList) {
	nodes = waitListSchedulableNodesOrDie(c)
	// previous tests may have cause failures of some nodes. Let's skip
	// 'Not Ready' nodes, just in case (there is no need to fail the test).
	FilterNodes(nodes, func(node api.Node) bool {
		return isNodeSchedulable(&node) && isNodeUntainted(&node)
	})
	return nodes
}

func WaitForAllNodesSchedulable(c clientset.Interface, timeout time.Duration) error {
	Logf("Waiting up to %v for all (but %d) nodes to be schedulable", timeout, TestContext.AllowedNotReadyNodes)

	var notSchedulable []*api.Node
	return wait.PollImmediate(30*time.Second, timeout, func() (bool, error) {
		notSchedulable = nil
		opts := api.ListOptions{
			ResourceVersion: "0",
			FieldSelector:   fields.Set{"spec.unschedulable": "false"}.AsSelector(),
		}
		nodes, err := c.Core().Nodes().List(opts)
		if err != nil {
			Logf("Unexpected error listing nodes: %v", err)
			// Ignore the error here - it will be retried.
			return false, nil
		}
		for i := range nodes.Items {
			node := &nodes.Items[i]
			if !isNodeSchedulable(node) {
				notSchedulable = append(notSchedulable, node)
			}
		}
		// Framework allows for <TestContext.AllowedNotReadyNodes> nodes to be non-ready,
		// to make it possible e.g. for incorrect deployment of some small percentage
		// of nodes (which we allow in cluster validation). Some nodes that are not
		// provisioned correctly at startup will never become ready (e.g. when something
		// won't install correctly), so we can't expect them to be ready at any point.
		//
		// However, we only allow non-ready nodes with some specific reasons.
		if len(notSchedulable) > 0 {
			Logf("Unschedulable nodes:")
			for i := range notSchedulable {
				Logf("-> %s Ready=%t Network=%t",
					notSchedulable[i].Name,
					IsNodeConditionSetAsExpected(notSchedulable[i], api.NodeReady, true),
					IsNodeConditionSetAsExpected(notSchedulable[i], api.NodeNetworkUnavailable, false))
			}
		}
		if len(notSchedulable) > TestContext.AllowedNotReadyNodes {
			return false, nil
		}
		return allowedNotReadyReasons(notSchedulable), nil
	})
}

func AddOrUpdateLabelOnNode(c clientset.Interface, nodeName string, labelKey, labelValue string) {
	ExpectNoError(testutils.AddLabelsToNode(c, nodeName, map[string]string{labelKey: labelValue}))
}

func ExpectNodeHasLabel(c clientset.Interface, nodeName string, labelKey string, labelValue string) {
	By("verifying the node has the label " + labelKey + " " + labelValue)
	node, err := c.Core().Nodes().Get(nodeName)
	ExpectNoError(err)
	Expect(node.Labels[labelKey]).To(Equal(labelValue))
}

// RemoveLabelOffNode is for cleaning up labels temporarily added to node,
// won't fail if target label doesn't exist or has been removed.
func RemoveLabelOffNode(c clientset.Interface, nodeName string, labelKey string) {
	By("removing the label " + labelKey + " off the node " + nodeName)
	ExpectNoError(testutils.RemoveLabelOffNode(c, nodeName, []string{labelKey}))

	By("verifying the node doesn't have the label " + labelKey)
	ExpectNoError(testutils.VerifyLabelsRemoved(c, nodeName, []string{labelKey}))
}

func AddOrUpdateTaintOnNode(c clientset.Interface, nodeName string, taint api.Taint) {
	for attempt := 0; attempt < UpdateRetries; attempt++ {
		node, err := c.Core().Nodes().Get(nodeName)
		ExpectNoError(err)

		nodeTaints, err := api.GetTaintsFromNodeAnnotations(node.Annotations)
		ExpectNoError(err)

		var newTaints []api.Taint
		updated := false
		for _, existingTaint := range nodeTaints {
			if taint.MatchTaint(existingTaint) {
				newTaints = append(newTaints, taint)
				updated = true
				continue
			}

			newTaints = append(newTaints, existingTaint)
		}

		if !updated {
			newTaints = append(newTaints, taint)
		}

		taintsData, err := json.Marshal(newTaints)
		ExpectNoError(err)

		if node.Annotations == nil {
			node.Annotations = make(map[string]string)
		}
		node.Annotations[api.TaintsAnnotationKey] = string(taintsData)
		_, err = c.Core().Nodes().Update(node)
		if err != nil {
			if !apierrs.IsConflict(err) {
				ExpectNoError(err)
			} else {
				Logf("Conflict when trying to add/update taint %v to %v", taint, nodeName)
			}
		} else {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func taintExists(taints []api.Taint, taintToFind api.Taint) bool {
	for _, taint := range taints {
		if taint.MatchTaint(taintToFind) {
			return true
		}
	}
	return false
}

func ExpectNodeHasTaint(c clientset.Interface, nodeName string, taint api.Taint) {
	By("verifying the node has the taint " + taint.ToString())
	node, err := c.Core().Nodes().Get(nodeName)
	ExpectNoError(err)

	nodeTaints, err := api.GetTaintsFromNodeAnnotations(node.Annotations)
	ExpectNoError(err)

	if len(nodeTaints) == 0 || !taintExists(nodeTaints, taint) {
		Failf("Failed to find taint %s on node %s", taint.ToString(), nodeName)
	}
}

func deleteTaint(oldTaints []api.Taint, taintToDelete api.Taint) ([]api.Taint, error) {
	newTaints := []api.Taint{}
	found := false
	for _, oldTaint := range oldTaints {
		if oldTaint.MatchTaint(taintToDelete) {
			found = true
			continue
		}
		newTaints = append(newTaints, taintToDelete)
	}

	if !found {
		return nil, fmt.Errorf("taint %s not found.", taintToDelete.ToString())
	}
	return newTaints, nil
}

// RemoveTaintOffNode is for cleaning up taints temporarily added to node,
// won't fail if target taint doesn't exist or has been removed.
func RemoveTaintOffNode(c clientset.Interface, nodeName string, taint api.Taint) {
	By("removing the taint " + taint.ToString() + " off the node " + nodeName)
	for attempt := 0; attempt < UpdateRetries; attempt++ {
		node, err := c.Core().Nodes().Get(nodeName)
		ExpectNoError(err)

		nodeTaints, err := api.GetTaintsFromNodeAnnotations(node.Annotations)
		ExpectNoError(err)
		if len(nodeTaints) == 0 {
			return
		}

		if !taintExists(nodeTaints, taint) {
			return
		}

		newTaints, err := deleteTaint(nodeTaints, taint)
		ExpectNoError(err)
		if len(newTaints) == 0 {
			delete(node.Annotations, api.TaintsAnnotationKey)
		} else {
			taintsData, err := json.Marshal(newTaints)
			ExpectNoError(err)
			node.Annotations[api.TaintsAnnotationKey] = string(taintsData)
		}

		_, err = c.Core().Nodes().Update(node)
		if err != nil {
			if !apierrs.IsConflict(err) {
				ExpectNoError(err)
			} else {
				Logf("Conflict when trying to add/update taint %s to node %v", taint.ToString(), nodeName)
			}
		} else {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	nodeUpdated, err := c.Core().Nodes().Get(nodeName)
	ExpectNoError(err)
	By("verifying the node doesn't have the taint " + taint.ToString())
	taintsGot, err := api.GetTaintsFromNodeAnnotations(nodeUpdated.Annotations)
	ExpectNoError(err)
	if taintExists(taintsGot, taint) {
		Failf("Failed removing taint " + taint.ToString() + " of the node " + nodeName)
	}
}

func ScaleRC(clientset clientset.Interface, ns, name string, size uint, wait bool) error {
	By(fmt.Sprintf("Scaling replication controller %s in namespace %s to %d", name, ns, size))
	scaler, err := kubectl.ScalerFor(api.Kind("ReplicationController"), clientset)
	if err != nil {
		return err
	}
	waitForScale := kubectl.NewRetryParams(5*time.Second, 1*time.Minute)
	waitForReplicas := kubectl.NewRetryParams(5*time.Second, 5*time.Minute)
	if err = scaler.Scale(ns, name, size, nil, waitForScale, waitForReplicas); err != nil {
		return fmt.Errorf("error while scaling RC %s to %d replicas: %v", name, size, err)
	}
	if !wait {
		return nil
	}
	return WaitForRCPodsRunning(clientset, ns, name)
}

// Wait up to 10 minutes for pods to become Running.
func WaitForRCPodsRunning(c clientset.Interface, ns, rcName string) error {
	rc, err := c.Core().ReplicationControllers(ns).Get(rcName)
	if err != nil {
		return err
	}
	selector := labels.SelectorFromSet(labels.Set(rc.Spec.Selector))
	err = testutils.WaitForPodsWithLabelRunning(c, ns, selector)
	if err != nil {
		return fmt.Errorf("Error while waiting for replication controller %s pods to be running: %v", rcName, err)
	}
	return nil
}

func ScaleDeployment(clientset clientset.Interface, ns, name string, size uint, wait bool) error {
	By(fmt.Sprintf("Scaling Deployment %s in namespace %s to %d", name, ns, size))
	scaler, err := kubectl.ScalerFor(extensions.Kind("Deployment"), clientset)
	if err != nil {
		return err
	}
	waitForScale := kubectl.NewRetryParams(5*time.Second, 1*time.Minute)
	waitForReplicas := kubectl.NewRetryParams(5*time.Second, 5*time.Minute)
	if err = scaler.Scale(ns, name, size, nil, waitForScale, waitForReplicas); err != nil {
		return fmt.Errorf("error while scaling Deployment %s to %d replicas: %v", name, size, err)
	}
	if !wait {
		return nil
	}
	return WaitForDeploymentPodsRunning(clientset, ns, name)
}

func WaitForDeploymentPodsRunning(c clientset.Interface, ns, name string) error {
	deployment, err := c.Extensions().Deployments(ns).Get(name)
	if err != nil {
		return err
	}
	selector := labels.SelectorFromSet(labels.Set(deployment.Spec.Selector.MatchLabels))
	err = testutils.WaitForPodsWithLabelRunning(c, ns, selector)
	if err != nil {
		return fmt.Errorf("Error while waiting for Deployment %s pods to be running: %v", name, err)
	}
	return nil
}

// Returns true if all the specified pods are scheduled, else returns false.
func podsWithLabelScheduled(c clientset.Interface, ns string, label labels.Selector) (bool, error) {
	PodStore := testutils.NewPodStore(c, ns, label, fields.Everything())
	defer PodStore.Stop()
	pods := PodStore.List()
	if len(pods) == 0 {
		return false, nil
	}
	for _, pod := range pods {
		if pod.Spec.NodeName == "" {
			return false, nil
		}
	}
	return true, nil
}

// Wait for all matching pods to become scheduled and at least one
// matching pod exists.  Return the list of matching pods.
func WaitForPodsWithLabelScheduled(c clientset.Interface, ns string, label labels.Selector) (pods *api.PodList, err error) {
	err = wait.PollImmediate(Poll, podScheduledBeforeTimeout,
		func() (bool, error) {
			pods, err = WaitForPodsWithLabel(c, ns, label)
			if err != nil {
				return false, err
			}
			for _, pod := range pods.Items {
				if pod.Spec.NodeName == "" {
					return false, nil
				}
			}
			return true, nil
		})
	return pods, err
}

// Wait up to PodListTimeout for getting pods with certain label
func WaitForPodsWithLabel(c clientset.Interface, ns string, label labels.Selector) (pods *api.PodList, err error) {
	for t := time.Now(); time.Since(t) < PodListTimeout; time.Sleep(Poll) {
		options := api.ListOptions{LabelSelector: label}
		pods, err = c.Core().Pods(ns).List(options)
		Expect(err).NotTo(HaveOccurred())
		if len(pods.Items) > 0 {
			break
		}
	}
	if pods == nil || len(pods.Items) == 0 {
		err = fmt.Errorf("Timeout while waiting for pods with label %v", label)
	}
	return
}

// DeleteRCAndPods a Replication Controller and all pods it spawned
func DeleteRCAndPods(clientset clientset.Interface, ns, name string) error {
	By(fmt.Sprintf("deleting replication controller %s in namespace %s", name, ns))
	rc, err := clientset.Core().ReplicationControllers(ns).Get(name)
	if err != nil {
		if apierrs.IsNotFound(err) {
			Logf("RC %s was already deleted: %v", name, err)
			return nil
		}
		return err
	}
	reaper, err := kubectl.ReaperForReplicationController(clientset.Core(), 10*time.Minute)
	if err != nil {
		if apierrs.IsNotFound(err) {
			Logf("RC %s was already deleted: %v", name, err)
			return nil
		}
		return err
	}
	ps, err := podStoreForRC(clientset, rc)
	if err != nil {
		return err
	}
	defer ps.Stop()
	startTime := time.Now()
	err = reaper.Stop(ns, name, 0, nil)
	if apierrs.IsNotFound(err) {
		Logf("RC %s was already deleted: %v", name, err)
		return nil
	}
	if err != nil {
		return fmt.Errorf("error while stopping RC: %s: %v", name, err)
	}
	deleteRCTime := time.Now().Sub(startTime)
	Logf("Deleting RC %s took: %v", name, deleteRCTime)
	err = waitForPodsInactive(ps, 10*time.Millisecond, 10*time.Minute)
	if err != nil {
		return fmt.Errorf("error while waiting for pods to become inactive %s: %v", name, err)
	}
	terminatePodTime := time.Now().Sub(startTime) - deleteRCTime
	Logf("Terminating RC %s pods took: %v", name, terminatePodTime)
	// this is to relieve namespace controller's pressure when deleting the
	// namespace after a test.
	err = waitForPodsGone(ps, 10*time.Second, 10*time.Minute)
	if err != nil {
		return fmt.Errorf("error while waiting for pods gone %s: %v", name, err)
	}
	return nil
}

// DeleteRCAndWaitForGC deletes only the Replication Controller and waits for GC to delete the pods.
func DeleteRCAndWaitForGC(c clientset.Interface, ns, name string) error {
	By(fmt.Sprintf("deleting replication controller %s in namespace %s, will wait for the garbage collector to delete the pods", name, ns))
	rc, err := c.Core().ReplicationControllers(ns).Get(name)
	if err != nil {
		if apierrs.IsNotFound(err) {
			Logf("RC %s was already deleted: %v", name, err)
			return nil
		}
		return err
	}
	ps, err := podStoreForRC(c, rc)
	if err != nil {
		return err
	}
	defer ps.Stop()
	startTime := time.Now()
	falseVar := false
	deleteOption := &api.DeleteOptions{OrphanDependents: &falseVar}
	err = c.Core().ReplicationControllers(ns).Delete(name, deleteOption)
	if err != nil && apierrs.IsNotFound(err) {
		Logf("RC %s was already deleted: %v", name, err)
		return nil
	}
	if err != nil {
		return err
	}
	deleteRCTime := time.Now().Sub(startTime)
	Logf("Deleting RC %s took: %v", name, deleteRCTime)
	var interval, timeout time.Duration
	switch {
	case rc.Spec.Replicas < 100:
		interval = 100 * time.Millisecond
	case rc.Spec.Replicas < 1000:
		interval = 1 * time.Second
	default:
		interval = 10 * time.Second
	}
	if rc.Spec.Replicas < 5000 {
		timeout = 10 * time.Minute
	} else {
		timeout = time.Duration(rc.Spec.Replicas/gcThroughput) * time.Second
		// gcThroughput is pretty strict now, add a bit more to it
		timeout = timeout + 3*time.Minute
	}
	err = waitForPodsInactive(ps, interval, timeout)
	if err != nil {
		return fmt.Errorf("error while waiting for pods to become inactive %s: %v", name, err)
	}
	terminatePodTime := time.Now().Sub(startTime) - deleteRCTime
	Logf("Terminating RC %s pods took: %v", name, terminatePodTime)
	err = waitForPodsGone(ps, interval, 10*time.Minute)
	if err != nil {
		return fmt.Errorf("error while waiting for pods gone %s: %v", name, err)
	}
	return nil
}

// podStoreForRC creates a PodStore that monitors pods belong to the rc. It
// waits until the reflector does a List() before returning.
func podStoreForRC(c clientset.Interface, rc *api.ReplicationController) (*testutils.PodStore, error) {
	labels := labels.SelectorFromSet(rc.Spec.Selector)
	ps := testutils.NewPodStore(c, rc.Namespace, labels, fields.Everything())
	err := wait.Poll(1*time.Second, 2*time.Minute, func() (bool, error) {
		if len(ps.Reflector.LastSyncResourceVersion()) != 0 {
			return true, nil
		}
		return false, nil
	})
	return ps, err
}

// waitForPodsInactive waits until there are no active pods left in the PodStore.
// This is to make a fair comparison of deletion time between DeleteRCAndPods
// and DeleteRCAndWaitForGC, because the RC controller decreases status.replicas
// when the pod is inactvie.
func waitForPodsInactive(ps *testutils.PodStore, interval, timeout time.Duration) error {
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		pods := ps.List()
		for _, pod := range pods {
			if controller.IsPodActive(pod) {
				return false, nil
			}
		}
		return true, nil
	})
}

// waitForPodsGone waits until there are no pods left in the PodStore.
func waitForPodsGone(ps *testutils.PodStore, interval, timeout time.Duration) error {
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		if pods := ps.List(); len(pods) == 0 {
			return true, nil
		}
		return false, nil
	})
}

// Delete a ReplicaSet and all pods it spawned
func DeleteReplicaSet(clientset clientset.Interface, ns, name string) error {
	By(fmt.Sprintf("deleting ReplicaSet %s in namespace %s", name, ns))
	rc, err := clientset.Extensions().ReplicaSets(ns).Get(name)
	if err != nil {
		if apierrs.IsNotFound(err) {
			Logf("ReplicaSet %s was already deleted: %v", name, err)
			return nil
		}
		return err
	}
	reaper, err := kubectl.ReaperFor(extensions.Kind("ReplicaSet"), clientset)
	if err != nil {
		if apierrs.IsNotFound(err) {
			Logf("ReplicaSet %s was already deleted: %v", name, err)
			return nil
		}
		return err
	}
	startTime := time.Now()
	err = reaper.Stop(ns, name, 0, nil)
	if apierrs.IsNotFound(err) {
		Logf("ReplicaSet %s was already deleted: %v", name, err)
		return nil
	}
	deleteRSTime := time.Now().Sub(startTime)
	Logf("Deleting RS %s took: %v", name, deleteRSTime)
	if err == nil {
		err = waitForReplicaSetPodsGone(clientset, rc)
	}
	terminatePodTime := time.Now().Sub(startTime) - deleteRSTime
	Logf("Terminating ReplicaSet %s pods took: %v", name, terminatePodTime)
	return err
}

// waitForReplicaSetPodsGone waits until there are no pods reported under a
// ReplicaSet selector (because the pods have completed termination).
func waitForReplicaSetPodsGone(c clientset.Interface, rs *extensions.ReplicaSet) error {
	return wait.PollImmediate(Poll, 2*time.Minute, func() (bool, error) {
		selector, err := unversioned.LabelSelectorAsSelector(rs.Spec.Selector)
		ExpectNoError(err)
		options := api.ListOptions{LabelSelector: selector}
		if pods, err := c.Core().Pods(rs.Namespace).List(options); err == nil && len(pods.Items) == 0 {
			return true, nil
		}
		return false, nil
	})
}

// Waits for the deployment status to become valid (i.e. max unavailable and max surge aren't violated anymore).
// Note that the status should stay valid at all times unless shortly after a scaling event or the deployment is just created.
// To verify that the deployment status is valid and wait for the rollout to finish, use WaitForDeploymentStatus instead.
func WaitForDeploymentStatusValid(c clientset.Interface, d *extensions.Deployment) error {
	var (
		oldRSs, allOldRSs, allRSs []*extensions.ReplicaSet
		newRS                     *extensions.ReplicaSet
		deployment                *extensions.Deployment
		reason                    string
	)

	err := wait.Poll(Poll, 5*time.Minute, func() (bool, error) {
		var err error
		deployment, err = c.Extensions().Deployments(d.Namespace).Get(d.Name)
		if err != nil {
			return false, err
		}
		oldRSs, allOldRSs, newRS, err = deploymentutil.GetAllReplicaSets(deployment, c)
		if err != nil {
			return false, err
		}
		if newRS == nil {
			// New RC hasn't been created yet.
			reason = "new replica set hasn't been created yet"
			Logf(reason)
			return false, nil
		}
		allRSs = append(oldRSs, newRS)
		// The old/new ReplicaSets need to contain the pod-template-hash label
		for i := range allRSs {
			if !labelsutil.SelectorHasLabel(allRSs[i].Spec.Selector, extensions.DefaultDeploymentUniqueLabelKey) {
				reason = "all replica sets need to contain the pod-template-hash label"
				Logf(reason)
				return false, nil
			}
		}
		totalCreated := deploymentutil.GetReplicaCountForReplicaSets(allRSs)
		maxCreated := deployment.Spec.Replicas + deploymentutil.MaxSurge(*deployment)
		if totalCreated > maxCreated {
			reason = fmt.Sprintf("total pods created: %d, more than the max allowed: %d", totalCreated, maxCreated)
			Logf(reason)
			return false, nil
		}
		minAvailable := deploymentutil.MinAvailable(deployment)
		if deployment.Status.AvailableReplicas < minAvailable {
			reason = fmt.Sprintf("total pods available: %d, less than the min required: %d", deployment.Status.AvailableReplicas, minAvailable)
			Logf(reason)
			return false, nil
		}

		// When the deployment status and its underlying resources reach the desired state, we're done
		if deployment.Status.Replicas == deployment.Spec.Replicas &&
			deployment.Status.UpdatedReplicas == deployment.Spec.Replicas &&
			deployment.Status.AvailableReplicas == deployment.Spec.Replicas {
			return true, nil
		}

		reason = fmt.Sprintf("deployment status: %#v", deployment.Status)
		Logf(reason)

		return false, nil
	})

	if err == wait.ErrWaitTimeout {
		logReplicaSetsOfDeployment(deployment, allOldRSs, newRS)
		logPodsOfDeployment(c, deployment)
		err = fmt.Errorf("%s", reason)
	}
	if err != nil {
		return fmt.Errorf("error waiting for deployment %q status to match expectation: %v", d.Name, err)
	}
	return nil
}

// Waits for the deployment to reach desired state.
// Returns an error if the deployment's rolling update strategy (max unavailable or max surge) is broken at any times.
func WaitForDeploymentStatus(c clientset.Interface, d *extensions.Deployment) error {
	var (
		oldRSs, allOldRSs, allRSs []*extensions.ReplicaSet
		newRS                     *extensions.ReplicaSet
		deployment                *extensions.Deployment
	)

	err := wait.Poll(Poll, 5*time.Minute, func() (bool, error) {
		var err error
		deployment, err = c.Extensions().Deployments(d.Namespace).Get(d.Name)
		if err != nil {
			return false, err
		}
		oldRSs, allOldRSs, newRS, err = deploymentutil.GetAllReplicaSets(deployment, c)
		if err != nil {
			return false, err
		}
		if newRS == nil {
			// New RS hasn't been created yet.
			return false, nil
		}
		allRSs = append(oldRSs, newRS)
		// The old/new ReplicaSets need to contain the pod-template-hash label
		for i := range allRSs {
			if !labelsutil.SelectorHasLabel(allRSs[i].Spec.Selector, extensions.DefaultDeploymentUniqueLabelKey) {
				return false, nil
			}
		}
		totalCreated := deploymentutil.GetReplicaCountForReplicaSets(allRSs)
		maxCreated := deployment.Spec.Replicas + deploymentutil.MaxSurge(*deployment)
		if totalCreated > maxCreated {
			logReplicaSetsOfDeployment(deployment, allOldRSs, newRS)
			logPodsOfDeployment(c, deployment)
			return false, fmt.Errorf("total pods created: %d, more than the max allowed: %d", totalCreated, maxCreated)
		}
		minAvailable := deploymentutil.MinAvailable(deployment)
		if deployment.Status.AvailableReplicas < minAvailable {
			logReplicaSetsOfDeployment(deployment, allOldRSs, newRS)
			logPodsOfDeployment(c, deployment)
			return false, fmt.Errorf("total pods available: %d, less than the min required: %d", deployment.Status.AvailableReplicas, minAvailable)
		}

		// When the deployment status and its underlying resources reach the desired state, we're done
		if deployment.Status.Replicas == deployment.Spec.Replicas &&
			deployment.Status.UpdatedReplicas == deployment.Spec.Replicas {
			return true, nil
		}
		return false, nil
	})

	if err == wait.ErrWaitTimeout {
		logReplicaSetsOfDeployment(deployment, allOldRSs, newRS)
		logPodsOfDeployment(c, deployment)
	}
	if err != nil {
		return fmt.Errorf("error waiting for deployment %q status to match expectation: %v", d.Name, err)
	}
	return nil
}

// WaitForDeploymentUpdatedReplicasLTE waits for given deployment to be observed by the controller and has at least a number of updatedReplicas
func WaitForDeploymentUpdatedReplicasLTE(c clientset.Interface, ns, deploymentName string, minUpdatedReplicas int, desiredGeneration int64) error {
	err := wait.Poll(Poll, 5*time.Minute, func() (bool, error) {
		deployment, err := c.Extensions().Deployments(ns).Get(deploymentName)
		if err != nil {
			return false, err
		}
		if deployment.Status.ObservedGeneration >= desiredGeneration && deployment.Status.UpdatedReplicas >= int32(minUpdatedReplicas) {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("error waiting for deployment %s to have at least %d updpatedReplicas: %v", deploymentName, minUpdatedReplicas, err)
	}
	return nil
}

// WaitForDeploymentRollbackCleared waits for given deployment either started rolling back or doesn't need to rollback.
// Note that rollback should be cleared shortly, so we only wait for 1 minute here to fail early.
func WaitForDeploymentRollbackCleared(c clientset.Interface, ns, deploymentName string) error {
	err := wait.Poll(Poll, 1*time.Minute, func() (bool, error) {
		deployment, err := c.Extensions().Deployments(ns).Get(deploymentName)
		if err != nil {
			return false, err
		}
		// Rollback not set or is kicked off
		if deployment.Spec.RollbackTo == nil {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("error waiting for deployment %s rollbackTo to be cleared: %v", deploymentName, err)
	}
	return nil
}

// WaitForDeploymentRevisionAndImage waits for the deployment's and its new RS's revision and container image to match the given revision and image.
// Note that deployment revision and its new RS revision should be updated shortly, so we only wait for 1 minute here to fail early.
func WaitForDeploymentRevisionAndImage(c clientset.Interface, ns, deploymentName string, revision, image string) error {
	var deployment *extensions.Deployment
	var newRS *extensions.ReplicaSet
	err := wait.Poll(Poll, 1*time.Minute, func() (bool, error) {
		var err error
		deployment, err = c.Extensions().Deployments(ns).Get(deploymentName)
		if err != nil {
			return false, err
		}
		// The new ReplicaSet needs to be non-nil and contain the pod-template-hash label
		newRS, err = deploymentutil.GetNewReplicaSet(deployment, c)
		if err != nil || newRS == nil || !labelsutil.SelectorHasLabel(newRS.Spec.Selector, extensions.DefaultDeploymentUniqueLabelKey) {
			return false, err
		}
		// Check revision of this deployment, and of the new replica set of this deployment
		if deployment.Annotations == nil || deployment.Annotations[deploymentutil.RevisionAnnotation] != revision ||
			newRS.Annotations == nil || newRS.Annotations[deploymentutil.RevisionAnnotation] != revision ||
			deployment.Spec.Template.Spec.Containers[0].Image != image || newRS.Spec.Template.Spec.Containers[0].Image != image {
			return false, nil
		}
		return true, nil
	})
	if err == wait.ErrWaitTimeout {
		logReplicaSetsOfDeployment(deployment, nil, newRS)
	}
	if newRS == nil {
		return fmt.Errorf("deployment %s failed to create new RS: %v", deploymentName, err)
	}
	if err != nil {
		return fmt.Errorf("error waiting for deployment %s (got %s / %s) and new RS %s (got %s / %s) revision and image to match expectation (expected %s / %s): %v", deploymentName, deployment.Annotations[deploymentutil.RevisionAnnotation], deployment.Spec.Template.Spec.Containers[0].Image, newRS.Name, newRS.Annotations[deploymentutil.RevisionAnnotation], newRS.Spec.Template.Spec.Containers[0].Image, revision, image, err)
	}
	return nil
}

func WaitForOverlappingAnnotationMatch(c clientset.Interface, ns, deploymentName, expected string) error {
	return wait.Poll(Poll, 1*time.Minute, func() (bool, error) {
		deployment, err := c.Extensions().Deployments(ns).Get(deploymentName)
		if err != nil {
			return false, err
		}
		if deployment.Annotations[deploymentutil.OverlapAnnotation] == expected {
			return true, nil
		}
		return false, nil
	})
}

// CheckNewRSAnnotations check if the new RS's annotation is as expected
func CheckNewRSAnnotations(c clientset.Interface, ns, deploymentName string, expectedAnnotations map[string]string) error {
	deployment, err := c.Extensions().Deployments(ns).Get(deploymentName)
	if err != nil {
		return err
	}
	newRS, err := deploymentutil.GetNewReplicaSet(deployment, c)
	if err != nil {
		return err
	}
	for k, v := range expectedAnnotations {
		// Skip checking revision annotations
		if k != deploymentutil.RevisionAnnotation && v != newRS.Annotations[k] {
			return fmt.Errorf("Expected new RS annotations = %+v, got %+v", expectedAnnotations, newRS.Annotations)
		}
	}
	return nil
}

func WaitForPodsReady(c clientset.Interface, ns, name string, minReadySeconds int) error {
	label := labels.SelectorFromSet(labels.Set(map[string]string{"name": name}))
	options := api.ListOptions{LabelSelector: label}
	return wait.Poll(Poll, 5*time.Minute, func() (bool, error) {
		pods, err := c.Core().Pods(ns).List(options)
		if err != nil {
			return false, nil
		}
		for _, pod := range pods.Items {
			if !deploymentutil.IsPodAvailable(&pod, int32(minReadySeconds), time.Now()) {
				return false, nil
			}
		}
		return true, nil
	})
}

// Waits for the deployment to clean up old rcs.
func WaitForDeploymentOldRSsNum(c clientset.Interface, ns, deploymentName string, desiredRSNum int) error {
	return wait.Poll(Poll, 5*time.Minute, func() (bool, error) {
		deployment, err := c.Extensions().Deployments(ns).Get(deploymentName)
		if err != nil {
			return false, err
		}
		_, oldRSs, err := deploymentutil.GetOldReplicaSets(deployment, c)
		if err != nil {
			return false, err
		}
		return len(oldRSs) == desiredRSNum, nil
	})
}

func logReplicaSetsOfDeployment(deployment *extensions.Deployment, allOldRSs []*extensions.ReplicaSet, newRS *extensions.ReplicaSet) {
	Logf("Deployment: %+v. Selector = %+v", *deployment, deployment.Spec.Selector)
	for i := range allOldRSs {
		Logf("All old ReplicaSets (%d/%d) of deployment %s: %+v. Selector = %+v", i+1, len(allOldRSs), deployment.Name, *allOldRSs[i], allOldRSs[i].Spec.Selector)
	}
	if newRS != nil {
		Logf("New ReplicaSet of deployment %s: %+v. Selector = %+v", deployment.Name, *newRS, newRS.Spec.Selector)
	} else {
		Logf("New ReplicaSet of deployment %s is nil.", deployment.Name)
	}
}

func WaitForObservedDeployment(c clientset.Interface, ns, deploymentName string, desiredGeneration int64) error {
	return deploymentutil.WaitForObservedDeployment(func() (*extensions.Deployment, error) { return c.Extensions().Deployments(ns).Get(deploymentName) }, desiredGeneration, Poll, 1*time.Minute)
}

func WaitForDeploymentWithCondition(c clientset.Interface, ns, deploymentName, reason string, condType extensions.DeploymentConditionType) error {
	var conditions []extensions.DeploymentCondition
	pollErr := wait.PollImmediate(time.Second, 1*time.Minute, func() (bool, error) {
		deployment, err := c.Extensions().Deployments(ns).Get(deploymentName)
		if err != nil {
			return false, err
		}
		conditions = deployment.Status.Conditions
		cond := deploymentutil.GetDeploymentCondition(deployment.Status, condType)
		return cond != nil && cond.Reason == reason, nil
	})
	if pollErr == wait.ErrWaitTimeout {
		pollErr = fmt.Errorf("deployment %q never updated with the desired condition and reason: %v", deploymentName, conditions)
	}
	return pollErr
}

func logPodsOfDeployment(c clientset.Interface, deployment *extensions.Deployment) {
	minReadySeconds := deployment.Spec.MinReadySeconds
	podList, err := deploymentutil.ListPods(deployment,
		func(namespace string, options api.ListOptions) (*api.PodList, error) {
			return c.Core().Pods(namespace).List(options)
		})
	if err != nil {
		Logf("Failed to list pods of deployment %s: %v", deployment.Name, err)
		return
	}
	if err == nil {
		for _, pod := range podList.Items {
			availability := "not available"
			if deploymentutil.IsPodAvailable(&pod, minReadySeconds, time.Now()) {
				availability = "available"
			}
			Logf("Pod %s is %s: %+v", pod.Name, availability, pod)
		}
	}
}

// Waits for the number of events on the given object to reach a desired count.
func WaitForEvents(c clientset.Interface, ns string, objOrRef runtime.Object, desiredEventsCount int) error {
	return wait.Poll(Poll, 5*time.Minute, func() (bool, error) {
		events, err := c.Core().Events(ns).Search(objOrRef)
		if err != nil {
			return false, fmt.Errorf("error in listing events: %s", err)
		}
		eventsCount := len(events.Items)
		if eventsCount == desiredEventsCount {
			return true, nil
		}
		if eventsCount < desiredEventsCount {
			return false, nil
		}
		// Number of events has exceeded the desired count.
		return false, fmt.Errorf("number of events has exceeded the desired count, eventsCount: %d, desiredCount: %d", eventsCount, desiredEventsCount)
	})
}

// Waits for the number of events on the given object to be at least a desired count.
func WaitForPartialEvents(c clientset.Interface, ns string, objOrRef runtime.Object, atLeastEventsCount int) error {
	return wait.Poll(Poll, 5*time.Minute, func() (bool, error) {
		events, err := c.Core().Events(ns).Search(objOrRef)
		if err != nil {
			return false, fmt.Errorf("error in listing events: %s", err)
		}
		eventsCount := len(events.Items)
		if eventsCount >= atLeastEventsCount {
			return true, nil
		}
		return false, nil
	})
}

type updateDeploymentFunc func(d *extensions.Deployment)

func UpdateDeploymentWithRetries(c clientset.Interface, namespace, name string, applyUpdate updateDeploymentFunc) (deployment *extensions.Deployment, err error) {
	deployments := c.Extensions().Deployments(namespace)
	var updateErr error
	pollErr := wait.Poll(10*time.Millisecond, 1*time.Minute, func() (bool, error) {
		if deployment, err = deployments.Get(name); err != nil {
			return false, err
		}
		// Apply the update, then attempt to push it to the apiserver.
		applyUpdate(deployment)
		if deployment, err = deployments.Update(deployment); err == nil {
			Logf("Updating deployment %s", name)
			return true, nil
		}
		updateErr = err
		return false, nil
	})
	if pollErr == wait.ErrWaitTimeout {
		pollErr = fmt.Errorf("couldn't apply the provided updated to deployment %q: %v", name, updateErr)
	}
	return deployment, pollErr
}

type updateRsFunc func(d *extensions.ReplicaSet)

func UpdateReplicaSetWithRetries(c clientset.Interface, namespace, name string, applyUpdate updateRsFunc) (*extensions.ReplicaSet, error) {
	var rs *extensions.ReplicaSet
	var updateErr error
	pollErr := wait.PollImmediate(10*time.Millisecond, 1*time.Minute, func() (bool, error) {
		var err error
		if rs, err = c.Extensions().ReplicaSets(namespace).Get(name); err != nil {
			return false, err
		}
		// Apply the update, then attempt to push it to the apiserver.
		applyUpdate(rs)
		if rs, err = c.Extensions().ReplicaSets(namespace).Update(rs); err == nil {
			Logf("Updating replica set %q", name)
			return true, nil
		}
		updateErr = err
		return false, nil
	})
	if pollErr == wait.ErrWaitTimeout {
		pollErr = fmt.Errorf("couldn't apply the provided updated to replicaset %q: %v", name, updateErr)
	}
	return rs, pollErr
}

type updateRcFunc func(d *api.ReplicationController)

func UpdateReplicationControllerWithRetries(c clientset.Interface, namespace, name string, applyUpdate updateRcFunc) (*api.ReplicationController, error) {
	var rc *api.ReplicationController
	var updateErr error
	pollErr := wait.PollImmediate(10*time.Millisecond, 1*time.Minute, func() (bool, error) {
		var err error
		if rc, err = c.Core().ReplicationControllers(namespace).Get(name); err != nil {
			return false, err
		}
		// Apply the update, then attempt to push it to the apiserver.
		applyUpdate(rc)
		if rc, err = c.Core().ReplicationControllers(namespace).Update(rc); err == nil {
			Logf("Updating replication controller %q", name)
			return true, nil
		}
		updateErr = err
		return false, nil
	})
	if pollErr == wait.ErrWaitTimeout {
		pollErr = fmt.Errorf("couldn't apply the provided updated to rc %q: %v", name, updateErr)
	}
	return rc, pollErr
}

type updateStatefulSetFunc func(*apps.StatefulSet)

func UpdateStatefulSetWithRetries(c clientset.Interface, namespace, name string, applyUpdate updateStatefulSetFunc) (statefulSet *apps.StatefulSet, err error) {
	statefulSets := c.Apps().StatefulSets(namespace)
	var updateErr error
	pollErr := wait.Poll(10*time.Millisecond, 1*time.Minute, func() (bool, error) {
		if statefulSet, err = statefulSets.Get(name); err != nil {
			return false, err
		}
		// Apply the update, then attempt to push it to the apiserver.
		applyUpdate(statefulSet)
		if statefulSet, err = statefulSets.Update(statefulSet); err == nil {
			Logf("Updating stateful set %s", name)
			return true, nil
		}
		updateErr = err
		return false, nil
	})
	if pollErr == wait.ErrWaitTimeout {
		pollErr = fmt.Errorf("couldn't apply the provided updated to stateful set %q: %v", name, updateErr)
	}
	return statefulSet, pollErr
}

type updateJobFunc func(*batch.Job)

func UpdateJobWithRetries(c clientset.Interface, namespace, name string, applyUpdate updateJobFunc) (job *batch.Job, err error) {
	jobs := c.Batch().Jobs(namespace)
	var updateErr error
	pollErr := wait.PollImmediate(10*time.Millisecond, 1*time.Minute, func() (bool, error) {
		if job, err = jobs.Get(name); err != nil {
			return false, err
		}
		// Apply the update, then attempt to push it to the apiserver.
		applyUpdate(job)
		if job, err = jobs.Update(job); err == nil {
			Logf("Updating job %s", name)
			return true, nil
		}
		updateErr = err
		return false, nil
	})
	if pollErr == wait.ErrWaitTimeout {
		pollErr = fmt.Errorf("couldn't apply the provided updated to job %q: %v", name, updateErr)
	}
	return job, pollErr
}

// NodeAddresses returns the first address of the given type of each node.
func NodeAddresses(nodelist *api.NodeList, addrType api.NodeAddressType) []string {
	hosts := []string{}
	for _, n := range nodelist.Items {
		for _, addr := range n.Status.Addresses {
			// Use the first external IP address we find on the node, and
			// use at most one per node.
			// TODO(roberthbailey): Use the "preferred" address for the node, once
			// such a thing is defined (#2462).
			if addr.Type == addrType {
				hosts = append(hosts, addr.Address)
				break
			}
		}
	}
	return hosts
}

// NodeSSHHosts returns SSH-able host names for all schedulable nodes - this excludes master node.
// It returns an error if it can't find an external IP for every node, though it still returns all
// hosts that it found in that case.
func NodeSSHHosts(c clientset.Interface) ([]string, error) {
	nodelist := waitListSchedulableNodesOrDie(c)

	// TODO(roberthbailey): Use the "preferred" address for the node, once such a thing is defined (#2462).
	hosts := NodeAddresses(nodelist, api.NodeExternalIP)

	// Error if any node didn't have an external IP.
	if len(hosts) != len(nodelist.Items) {
		return hosts, fmt.Errorf(
			"only found %d external IPs on nodes, but found %d nodes. Nodelist: %v",
			len(hosts), len(nodelist.Items), nodelist)
	}

	sshHosts := make([]string, 0, len(hosts))
	for _, h := range hosts {
		sshHosts = append(sshHosts, net.JoinHostPort(h, "22"))
	}
	return sshHosts, nil
}

type SSHResult struct {
	User   string
	Host   string
	Cmd    string
	Stdout string
	Stderr string
	Code   int
}

// SSH synchronously SSHs to a node running on provider and runs cmd. If there
// is no error performing the SSH, the stdout, stderr, and exit code are
// returned.
func SSH(cmd, host, provider string) (SSHResult, error) {
	result := SSHResult{Host: host, Cmd: cmd}

	// Get a signer for the provider.
	signer, err := GetSigner(provider)
	if err != nil {
		return result, fmt.Errorf("error getting signer for provider %s: '%v'", provider, err)
	}

	// RunSSHCommand will default to Getenv("USER") if user == "", but we're
	// defaulting here as well for logging clarity.
	result.User = os.Getenv("KUBE_SSH_USER")
	if result.User == "" {
		result.User = os.Getenv("USER")
	}

	stdout, stderr, code, err := sshutil.RunSSHCommand(cmd, result.User, host, signer)
	result.Stdout = stdout
	result.Stderr = stderr
	result.Code = code

	return result, err
}

func LogSSHResult(result SSHResult) {
	remote := fmt.Sprintf("%s@%s", result.User, result.Host)
	Logf("ssh %s: command:   %s", remote, result.Cmd)
	Logf("ssh %s: stdout:    %q", remote, result.Stdout)
	Logf("ssh %s: stderr:    %q", remote, result.Stderr)
	Logf("ssh %s: exit code: %d", remote, result.Code)
}

func IssueSSHCommandWithResult(cmd, provider string, node *api.Node) (*SSHResult, error) {
	Logf("Getting external IP address for %s", node.Name)
	host := ""
	for _, a := range node.Status.Addresses {
		if a.Type == api.NodeExternalIP {
			host = a.Address + ":22"
			break
		}
	}

	if host == "" {
		return nil, fmt.Errorf("couldn't find external IP address for node %s", node.Name)
	}

	Logf("SSH %q on %s(%s)", cmd, node.Name, host)
	result, err := SSH(cmd, host, provider)
	LogSSHResult(result)

	if result.Code != 0 || err != nil {
		return nil, fmt.Errorf("failed running %q: %v (exit code %d)",
			cmd, err, result.Code)
	}

	return &result, nil
}

func IssueSSHCommand(cmd, provider string, node *api.Node) error {
	result, err := IssueSSHCommandWithResult(cmd, provider, node)
	if result != nil {
		LogSSHResult(*result)
	}

	if result.Code != 0 || err != nil {
		return fmt.Errorf("failed running %q: %v (exit code %d)",
			cmd, err, result.Code)
	}

	return nil
}

// NewHostExecPodSpec returns the pod spec of hostexec pod
func NewHostExecPodSpec(ns, name string) *api.Pod {
	pod := &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				{
					Name:            "hostexec",
					Image:           "gcr.io/google_containers/hostexec:1.2",
					ImagePullPolicy: api.PullIfNotPresent,
				},
			},
			SecurityContext: &api.PodSecurityContext{
				HostNetwork: true,
			},
		},
	}
	return pod
}

// RunHostCmd runs the given cmd in the context of the given pod using `kubectl exec`
// inside of a shell.
func RunHostCmd(ns, name, cmd string) (string, error) {
	return RunKubectl("exec", fmt.Sprintf("--namespace=%v", ns), name, "--", "/bin/sh", "-c", cmd)
}

// RunHostCmdOrDie calls RunHostCmd and dies on error.
func RunHostCmdOrDie(ns, name, cmd string) string {
	stdout, err := RunHostCmd(ns, name, cmd)
	Logf("stdout: %v", stdout)
	ExpectNoError(err)
	return stdout
}

// LaunchHostExecPod launches a hostexec pod in the given namespace and waits
// until it's Running
func LaunchHostExecPod(client clientset.Interface, ns, name string) *api.Pod {
	hostExecPod := NewHostExecPodSpec(ns, name)
	pod, err := client.Core().Pods(ns).Create(hostExecPod)
	ExpectNoError(err)
	err = WaitForPodRunningInNamespace(client, pod)
	ExpectNoError(err)
	return pod
}

// GetSigner returns an ssh.Signer for the provider ("gce", etc.) that can be
// used to SSH to their nodes.
func GetSigner(provider string) (ssh.Signer, error) {
	// Get the directory in which SSH keys are located.
	keydir := filepath.Join(os.Getenv("HOME"), ".ssh")

	// Select the key itself to use. When implementing more providers here,
	// please also add them to any SSH tests that are disabled because of signer
	// support.
	keyfile := ""
	switch provider {
	case "gce", "gke", "kubemark":
		keyfile = "google_compute_engine"
	case "aws":
		// If there is an env. variable override, use that.
		aws_keyfile := os.Getenv("AWS_SSH_KEY")
		if len(aws_keyfile) != 0 {
			return sshutil.MakePrivateKeySignerFromFile(aws_keyfile)
		}
		// Otherwise revert to home dir
		keyfile = "kube_aws_rsa"
	case "vagrant":
		keyfile := os.Getenv("VAGRANT_SSH_KEY")
		if len(keyfile) != 0 {
			return sshutil.MakePrivateKeySignerFromFile(keyfile)
		}
		return nil, fmt.Errorf("VAGRANT_SSH_KEY env variable should be provided")
	default:
		return nil, fmt.Errorf("GetSigner(...) not implemented for %s", provider)
	}
	key := filepath.Join(keydir, keyfile)

	return sshutil.MakePrivateKeySignerFromFile(key)
}

// CheckPodsRunningReady returns whether all pods whose names are listed in
// podNames in namespace ns are running and ready, using c and waiting at most
// timeout.
func CheckPodsRunningReady(c clientset.Interface, ns string, podNames []string, timeout time.Duration) bool {
	return CheckPodsCondition(c, ns, podNames, timeout, testutils.PodRunningReady, "running and ready")
}

// CheckPodsRunningReadyOrSucceeded returns whether all pods whose names are
// listed in podNames in namespace ns are running and ready, or succeeded; use
// c and waiting at most timeout.
func CheckPodsRunningReadyOrSucceeded(c clientset.Interface, ns string, podNames []string, timeout time.Duration) bool {
	return CheckPodsCondition(c, ns, podNames, timeout, testutils.PodRunningReadyOrSucceeded, "running and ready, or succeeded")
}

// CheckPodsCondition returns whether all pods whose names are listed in podNames
// in namespace ns are in the condition, using c and waiting at most timeout.
func CheckPodsCondition(c clientset.Interface, ns string, podNames []string, timeout time.Duration, condition podCondition, desc string) bool {
	np := len(podNames)
	Logf("Waiting up to %v for %d pods to be %s: %s", timeout, np, desc, podNames)
	result := make(chan bool, len(podNames))
	for ix := range podNames {
		// Launch off pod readiness checkers.
		go func(name string) {
			err := waitForPodCondition(c, ns, name, desc, timeout, condition)
			result <- err == nil
		}(podNames[ix])
	}
	// Wait for them all to finish.
	success := true
	// TODO(a-robinson): Change to `for range` syntax and remove logging once we
	// support only Go >= 1.4.
	for _, podName := range podNames {
		if !<-result {
			Logf("Pod %[1]s failed to be %[2]s.", podName, desc)
			success = false
		}
	}
	Logf("Wanted all %d pods to be %s. Result: %t. Pods: %v", np, desc, success, podNames)
	return success
}

// WaitForNodeToBeReady returns whether node name is ready within timeout.
func WaitForNodeToBeReady(c clientset.Interface, name string, timeout time.Duration) bool {
	return WaitForNodeToBe(c, name, api.NodeReady, true, timeout)
}

// WaitForNodeToBeNotReady returns whether node name is not ready (i.e. the
// readiness condition is anything but ready, e.g false or unknown) within
// timeout.
func WaitForNodeToBeNotReady(c clientset.Interface, name string, timeout time.Duration) bool {
	return WaitForNodeToBe(c, name, api.NodeReady, false, timeout)
}

func isNodeConditionSetAsExpected(node *api.Node, conditionType api.NodeConditionType, wantTrue, silent bool) bool {
	// Check the node readiness condition (logging all).
	for _, cond := range node.Status.Conditions {
		// Ensure that the condition type and the status matches as desired.
		if cond.Type == conditionType {
			if (cond.Status == api.ConditionTrue) == wantTrue {
				return true
			} else {
				if !silent {
					Logf("Condition %s of node %s is %v instead of %t. Reason: %v, message: %v",
						conditionType, node.Name, cond.Status == api.ConditionTrue, wantTrue, cond.Reason, cond.Message)
				}
				return false
			}
		}
	}
	if !silent {
		Logf("Couldn't find condition %v on node %v", conditionType, node.Name)
	}
	return false
}

func IsNodeConditionSetAsExpected(node *api.Node, conditionType api.NodeConditionType, wantTrue bool) bool {
	return isNodeConditionSetAsExpected(node, conditionType, wantTrue, false)
}

func IsNodeConditionSetAsExpectedSilent(node *api.Node, conditionType api.NodeConditionType, wantTrue bool) bool {
	return isNodeConditionSetAsExpected(node, conditionType, wantTrue, true)
}

func IsNodeConditionUnset(node *api.Node, conditionType api.NodeConditionType) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == conditionType {
			return false
		}
	}
	return true
}

// WaitForNodeToBe returns whether node "name's" condition state matches wantTrue
// within timeout. If wantTrue is true, it will ensure the node condition status
// is ConditionTrue; if it's false, it ensures the node condition is in any state
// other than ConditionTrue (e.g. not true or unknown).
func WaitForNodeToBe(c clientset.Interface, name string, conditionType api.NodeConditionType, wantTrue bool, timeout time.Duration) bool {
	Logf("Waiting up to %v for node %s condition %s to be %t", timeout, name, conditionType, wantTrue)
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(Poll) {
		node, err := c.Core().Nodes().Get(name)
		if err != nil {
			Logf("Couldn't get node %s", name)
			continue
		}

		if IsNodeConditionSetAsExpected(node, conditionType, wantTrue) {
			return true
		}
	}
	Logf("Node %s didn't reach desired %s condition status (%t) within %v", name, conditionType, wantTrue, timeout)
	return false
}

// Checks whether not-ready nodes can be ignored while checking if all nodes are
// ready (we allow e.g. for incorrect provisioning of some small percentage of nodes
// while validating cluster, and those nodes may never become healthy).
// Currently we allow only for:
// - not present CNI plugins on node
// TODO: we should extend it for other reasons.
func allowedNotReadyReasons(nodes []*api.Node) bool {
	for _, node := range nodes {
		index, condition := api.GetNodeCondition(&node.Status, api.NodeReady)
		if index == -1 ||
			!strings.Contains(condition.Message, "could not locate kubenet required CNI plugins") {
			return false
		}
	}
	return true
}

// Checks whether all registered nodes are ready.
// TODO: we should change the AllNodesReady call in AfterEach to WaitForAllNodesHealthy,
// and figure out how to do it in a configurable way, as we can't expect all setups to run
// default test add-ons.
func AllNodesReady(c clientset.Interface, timeout time.Duration) error {
	Logf("Waiting up to %v for all (but %d) nodes to be ready", timeout, TestContext.AllowedNotReadyNodes)

	var notReady []*api.Node
	err := wait.PollImmediate(Poll, timeout, func() (bool, error) {
		notReady = nil
		// It should be OK to list unschedulable Nodes here.
		nodes, err := c.Core().Nodes().List(api.ListOptions{})
		if err != nil {
			return false, err
		}
		for i := range nodes.Items {
			node := &nodes.Items[i]
			if !IsNodeConditionSetAsExpected(node, api.NodeReady, true) {
				notReady = append(notReady, node)
			}
		}
		// Framework allows for <TestContext.AllowedNotReadyNodes> nodes to be non-ready,
		// to make it possible e.g. for incorrect deployment of some small percentage
		// of nodes (which we allow in cluster validation). Some nodes that are not
		// provisioned correctly at startup will never become ready (e.g. when something
		// won't install correctly), so we can't expect them to be ready at any point.
		//
		// However, we only allow non-ready nodes with some specific reasons.
		if len(notReady) > TestContext.AllowedNotReadyNodes {
			return false, nil
		}
		return allowedNotReadyReasons(notReady), nil
	})

	if err != nil && err != wait.ErrWaitTimeout {
		return err
	}

	if len(notReady) > TestContext.AllowedNotReadyNodes || !allowedNotReadyReasons(notReady) {
		return fmt.Errorf("Not ready nodes: %#v", notReady)
	}
	return nil
}

// checks whether all registered nodes are ready and all required Pods are running on them.
func WaitForAllNodesHealthy(c clientset.Interface, timeout time.Duration) error {
	Logf("Waiting up to %v for all nodes to be ready", timeout)

	var notReady []api.Node
	var missingPodsPerNode map[string][]string
	err := wait.PollImmediate(Poll, timeout, func() (bool, error) {
		notReady = nil
		// It should be OK to list unschedulable Nodes here.
		nodes, err := c.Core().Nodes().List(api.ListOptions{ResourceVersion: "0"})
		if err != nil {
			return false, err
		}
		for _, node := range nodes.Items {
			if !IsNodeConditionSetAsExpected(&node, api.NodeReady, true) {
				notReady = append(notReady, node)
			}
		}
		pods, err := c.Core().Pods(api.NamespaceAll).List(api.ListOptions{ResourceVersion: "0"})
		if err != nil {
			return false, err
		}

		systemPodsPerNode := make(map[string][]string)
		for _, pod := range pods.Items {
			if pod.Namespace == api.NamespaceSystem && pod.Status.Phase == api.PodRunning {
				if pod.Spec.NodeName != "" {
					systemPodsPerNode[pod.Spec.NodeName] = append(systemPodsPerNode[pod.Spec.NodeName], pod.Name)
				}
			}
		}
		missingPodsPerNode = make(map[string][]string)
		for _, node := range nodes.Items {
			if !system.IsMasterNode(&node) {
				for _, requiredPod := range requiredPerNodePods {
					foundRequired := false
					for _, presentPod := range systemPodsPerNode[node.Name] {
						if requiredPod.MatchString(presentPod) {
							foundRequired = true
							break
						}
					}
					if !foundRequired {
						missingPodsPerNode[node.Name] = append(missingPodsPerNode[node.Name], requiredPod.String())
					}
				}
			}
		}
		return len(notReady) == 0 && len(missingPodsPerNode) == 0, nil
	})

	if err != nil && err != wait.ErrWaitTimeout {
		return err
	}

	if len(notReady) > 0 {
		return fmt.Errorf("Not ready nodes: %v", notReady)
	}
	if len(missingPodsPerNode) > 0 {
		return fmt.Errorf("Not running system Pods: %v", missingPodsPerNode)
	}
	return nil

}

// Filters nodes in NodeList in place, removing nodes that do not
// satisfy the given condition
// TODO: consider merging with pkg/client/cache.NodeLister
func FilterNodes(nodeList *api.NodeList, fn func(node api.Node) bool) {
	var l []api.Node

	for _, node := range nodeList.Items {
		if fn(node) {
			l = append(l, node)
		}
	}
	nodeList.Items = l
}

// ParseKVLines parses output that looks like lines containing "<key>: <val>"
// and returns <val> if <key> is found. Otherwise, it returns the empty string.
func ParseKVLines(output, key string) string {
	delim := ":"
	key = key + delim
	for _, line := range strings.Split(output, "\n") {
		pieces := strings.SplitAfterN(line, delim, 2)
		if len(pieces) != 2 {
			continue
		}
		k, v := pieces[0], pieces[1]
		if k == key {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func RestartKubeProxy(host string) error {
	// TODO: Make it work for all providers.
	if !ProviderIs("gce", "gke", "aws") {
		return fmt.Errorf("unsupported provider: %s", TestContext.Provider)
	}
	// kubelet will restart the kube-proxy since it's running in a static pod
	Logf("Killing kube-proxy on node %v", host)
	result, err := SSH("sudo pkill kube-proxy", host, TestContext.Provider)
	if err != nil || result.Code != 0 {
		LogSSHResult(result)
		return fmt.Errorf("couldn't restart kube-proxy: %v", err)
	}
	// wait for kube-proxy to come back up
	sshCmd := "sudo /bin/sh -c 'pgrep kube-proxy | wc -l'"
	err = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
		Logf("Waiting for kubeproxy to come back up with %v on %v", sshCmd, host)
		result, err := SSH(sshCmd, host, TestContext.Provider)
		if err != nil {
			return false, err
		}
		if result.Code != 0 {
			LogSSHResult(result)
			return false, fmt.Errorf("failed to run command, exited %d", result.Code)
		}
		if result.Stdout == "0\n" {
			return false, nil
		}
		Logf("kube-proxy is back up.")
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("kube-proxy didn't recover: %v", err)
	}
	return nil
}

func RestartApiserver(c discovery.ServerVersionInterface) error {
	// TODO: Make it work for all providers.
	if !ProviderIs("gce", "gke", "aws") {
		return fmt.Errorf("unsupported provider: %s", TestContext.Provider)
	}
	if ProviderIs("gce", "aws") {
		return sshRestartMaster()
	}
	// GKE doesn't allow ssh access, so use a same-version master
	// upgrade to teardown/recreate master.
	v, err := c.ServerVersion()
	if err != nil {
		return err
	}
	return masterUpgradeGKE(v.GitVersion[1:]) // strip leading 'v'
}

func sshRestartMaster() error {
	if !ProviderIs("gce", "aws") {
		return fmt.Errorf("unsupported provider: %s", TestContext.Provider)
	}
	var command string
	if ProviderIs("gce") {
		command = "sudo docker ps | grep /kube-apiserver | cut -d ' ' -f 1 | xargs sudo docker kill"
	} else {
		command = "sudo /etc/init.d/kube-apiserver restart"
	}
	Logf("Restarting master via ssh, running: %v", command)
	result, err := SSH(command, GetMasterHost()+":22", TestContext.Provider)
	if err != nil || result.Code != 0 {
		LogSSHResult(result)
		return fmt.Errorf("couldn't restart apiserver: %v", err)
	}
	return nil
}

func WaitForApiserverUp(c clientset.Interface) error {
	for start := time.Now(); time.Since(start) < time.Minute; time.Sleep(5 * time.Second) {
		body, err := c.Core().RESTClient().Get().AbsPath("/healthz").Do().Raw()
		if err == nil && string(body) == "ok" {
			return nil
		}
	}
	return fmt.Errorf("waiting for apiserver timed out")
}

// WaitForClusterSize waits until the cluster has desired size and there is no not-ready nodes in it.
// By cluster size we mean number of Nodes excluding Master Node.
func WaitForClusterSize(c clientset.Interface, size int, timeout time.Duration) error {
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(20 * time.Second) {
		nodes, err := c.Core().Nodes().List(api.ListOptions{FieldSelector: fields.Set{
			"spec.unschedulable": "false",
		}.AsSelector()})
		if err != nil {
			Logf("Failed to list nodes: %v", err)
			continue
		}
		numNodes := len(nodes.Items)

		// Filter out not-ready nodes.
		FilterNodes(nodes, func(node api.Node) bool {
			return IsNodeConditionSetAsExpected(&node, api.NodeReady, true)
		})
		numReady := len(nodes.Items)

		if numNodes == size && numReady == size {
			Logf("Cluster has reached the desired size %d", size)
			return nil
		}
		Logf("Waiting for cluster size %d, current size %d, not ready nodes %d", size, numNodes, numNodes-numReady)
	}
	return fmt.Errorf("timeout waiting %v for cluster size to be %d", timeout, size)
}

func GenerateMasterRegexp(prefix string) string {
	return prefix + "(-...)?"
}

// waitForMasters waits until the cluster has the desired number of ready masters in it.
func WaitForMasters(masterPrefix string, c clientset.Interface, size int, timeout time.Duration) error {
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(20 * time.Second) {
		nodes, err := c.Core().Nodes().List(api.ListOptions{})
		if err != nil {
			Logf("Failed to list nodes: %v", err)
			continue
		}

		// Filter out nodes that are not master replicas
		FilterNodes(nodes, func(node api.Node) bool {
			res, err := regexp.Match(GenerateMasterRegexp(masterPrefix), ([]byte)(node.Name))
			if err != nil {
				Logf("Failed to match regexp to node name: %v", err)
				return false
			}
			return res
		})

		numNodes := len(nodes.Items)

		// Filter out not-ready nodes.
		FilterNodes(nodes, func(node api.Node) bool {
			return IsNodeConditionSetAsExpected(&node, api.NodeReady, true)
		})

		numReady := len(nodes.Items)

		if numNodes == size && numReady == size {
			Logf("Cluster has reached the desired number of masters %d", size)
			return nil
		}
		Logf("Waiting for the number of masters %d, current %d, not ready master nodes %d", size, numNodes, numNodes-numReady)
	}
	return fmt.Errorf("timeout waiting %v for the number of masters to be %d", timeout, size)
}

// GetHostExternalAddress gets the node for a pod and returns the first External
// address. Returns an error if the node the pod is on doesn't have an External
// address.
func GetHostExternalAddress(client clientset.Interface, p *api.Pod) (externalAddress string, err error) {
	node, err := client.Core().Nodes().Get(p.Spec.NodeName)
	if err != nil {
		return "", err
	}
	for _, address := range node.Status.Addresses {
		if address.Type == api.NodeExternalIP {
			if address.Address != "" {
				externalAddress = address.Address
				break
			}
		}
	}
	if externalAddress == "" {
		err = fmt.Errorf("No external address for pod %v on node %v",
			p.Name, p.Spec.NodeName)
	}
	return
}

type extractRT struct {
	http.Header
}

func (rt *extractRT) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.Header = req.Header
	return &http.Response{}, nil
}

// headersForConfig extracts any http client logic necessary for the provided
// config.
func headersForConfig(c *restclient.Config) (http.Header, error) {
	extract := &extractRT{}
	rt, err := restclient.HTTPWrappersForConfig(c, extract)
	if err != nil {
		return nil, err
	}
	if _, err := rt.RoundTrip(&http.Request{}); err != nil {
		return nil, err
	}
	return extract.Header, nil
}

// OpenWebSocketForURL constructs a websocket connection to the provided URL, using the client
// config, with the specified protocols.
func OpenWebSocketForURL(url *url.URL, config *restclient.Config, protocols []string) (*websocket.Conn, error) {
	tlsConfig, err := restclient.TLSConfigFor(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create tls config: %v", err)
	}
	if tlsConfig != nil {
		url.Scheme = "wss"
		if !strings.Contains(url.Host, ":") {
			url.Host += ":443"
		}
	} else {
		url.Scheme = "ws"
		if !strings.Contains(url.Host, ":") {
			url.Host += ":80"
		}
	}
	headers, err := headersForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to load http headers: %v", err)
	}
	cfg, err := websocket.NewConfig(url.String(), "http://localhost")
	if err != nil {
		return nil, fmt.Errorf("failed to create websocket config: %v", err)
	}
	cfg.Header = headers
	cfg.TlsConfig = tlsConfig
	cfg.Protocol = protocols
	return websocket.DialConfig(cfg)
}

// getIngressAddress returns the ips/hostnames associated with the Ingress.
func getIngressAddress(client clientset.Interface, ns, name string) ([]string, error) {
	ing, err := client.Extensions().Ingresses(ns).Get(name)
	if err != nil {
		return nil, err
	}
	addresses := []string{}
	for _, a := range ing.Status.LoadBalancer.Ingress {
		if a.IP != "" {
			addresses = append(addresses, a.IP)
		}
		if a.Hostname != "" {
			addresses = append(addresses, a.Hostname)
		}
	}
	return addresses, nil
}

// WaitForIngressAddress waits for the Ingress to acquire an address.
func WaitForIngressAddress(c clientset.Interface, ns, ingName string, timeout time.Duration) (string, error) {
	var address string
	err := wait.PollImmediate(10*time.Second, timeout, func() (bool, error) {
		ipOrNameList, err := getIngressAddress(c, ns, ingName)
		if err != nil || len(ipOrNameList) == 0 {
			Logf("Waiting for Ingress %v to acquire IP, error %v", ingName, err)
			return false, nil
		}
		address = ipOrNameList[0]
		return true, nil
	})
	return address, err
}

// Looks for the given string in the log of a specific pod container
func LookForStringInLog(ns, podName, container, expectedString string, timeout time.Duration) (result string, err error) {
	return LookForString(expectedString, timeout, func() string {
		return RunKubectlOrDie("logs", podName, container, fmt.Sprintf("--namespace=%v", ns))
	})
}

// Looks for the given string in a file in a specific pod container
func LookForStringInFile(ns, podName, container, file, expectedString string, timeout time.Duration) (result string, err error) {
	return LookForString(expectedString, timeout, func() string {
		return RunKubectlOrDie("exec", podName, "-c", container, fmt.Sprintf("--namespace=%v", ns), "--", "cat", file)
	})
}

// Looks for the given string in the output of a command executed in a specific pod container
func LookForStringInPodExec(ns, podName string, command []string, expectedString string, timeout time.Duration) (result string, err error) {
	return LookForString(expectedString, timeout, func() string {
		// use the first container
		args := []string{"exec", podName, fmt.Sprintf("--namespace=%v", ns), "--"}
		args = append(args, command...)
		return RunKubectlOrDie(args...)
	})
}

// Looks for the given string in the output of fn, repeatedly calling fn until
// the timeout is reached or the string is found. Returns last log and possibly
// error if the string was not found.
func LookForString(expectedString string, timeout time.Duration, fn func() string) (result string, err error) {
	for t := time.Now(); time.Since(t) < timeout; time.Sleep(Poll) {
		result = fn()
		if strings.Contains(result, expectedString) {
			return
		}
	}
	err = fmt.Errorf("Failed to find \"%s\", last result: \"%s\"", expectedString, result)
	return
}

// getSvcNodePort returns the node port for the given service:port.
func getSvcNodePort(client clientset.Interface, ns, name string, svcPort int) (int, error) {
	svc, err := client.Core().Services(ns).Get(name)
	if err != nil {
		return 0, err
	}
	for _, p := range svc.Spec.Ports {
		if p.Port == int32(svcPort) {
			if p.NodePort != 0 {
				return int(p.NodePort), nil
			}
		}
	}
	return 0, fmt.Errorf(
		"No node port found for service %v, port %v", name, svcPort)
}

// GetNodePortURL returns the url to a nodeport Service.
func GetNodePortURL(client clientset.Interface, ns, name string, svcPort int) (string, error) {
	nodePort, err := getSvcNodePort(client, ns, name, svcPort)
	if err != nil {
		return "", err
	}
	// This list of nodes must not include the master, which is marked
	// unschedulable, since the master doesn't run kube-proxy. Without
	// kube-proxy NodePorts won't work.
	var nodes *api.NodeList
	if wait.PollImmediate(Poll, SingleCallTimeout, func() (bool, error) {
		nodes, err = client.Core().Nodes().List(api.ListOptions{FieldSelector: fields.Set{
			"spec.unschedulable": "false",
		}.AsSelector()})
		return err == nil, nil
	}) != nil {
		return "", err
	}
	if len(nodes.Items) == 0 {
		return "", fmt.Errorf("Unable to list nodes in cluster.")
	}
	for _, node := range nodes.Items {
		for _, address := range node.Status.Addresses {
			if address.Type == api.NodeExternalIP {
				if address.Address != "" {
					return fmt.Sprintf("http://%v:%v", address.Address, nodePort), nil
				}
			}
		}
	}
	return "", fmt.Errorf("Failed to find external address for service %v", name)
}

// ScaleRCByLabels scales an RC via ns/label lookup. If replicas == 0 it waits till
// none are running, otherwise it does what a synchronous scale operation would do.
func ScaleRCByLabels(clientset clientset.Interface, ns string, l map[string]string, replicas uint) error {
	listOpts := api.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set(l))}
	rcs, err := clientset.Core().ReplicationControllers(ns).List(listOpts)
	if err != nil {
		return err
	}
	if len(rcs.Items) == 0 {
		return fmt.Errorf("RC with labels %v not found in ns %v", l, ns)
	}
	Logf("Scaling %v RCs with labels %v in ns %v to %v replicas.", len(rcs.Items), l, ns, replicas)
	for _, labelRC := range rcs.Items {
		name := labelRC.Name
		if err := ScaleRC(clientset, ns, name, replicas, false); err != nil {
			return err
		}
		rc, err := clientset.Core().ReplicationControllers(ns).Get(name)
		if err != nil {
			return err
		}
		if replicas == 0 {
			ps, err := podStoreForRC(clientset, rc)
			if err != nil {
				return err
			}
			defer ps.Stop()
			if err = waitForPodsGone(ps, 10*time.Second, 10*time.Minute); err != nil {
				return fmt.Errorf("error while waiting for pods gone %s: %v", name, err)
			}
		} else {
			if err := testutils.WaitForPodsWithLabelRunning(
				clientset, ns, labels.SelectorFromSet(labels.Set(rc.Spec.Selector))); err != nil {
				return err
			}
		}
	}
	return nil
}

// TODO(random-liu): Change this to be a member function of the framework.
func GetPodLogs(c clientset.Interface, namespace, podName, containerName string) (string, error) {
	return getPodLogsInternal(c, namespace, podName, containerName, false)
}

func getPreviousPodLogs(c clientset.Interface, namespace, podName, containerName string) (string, error) {
	return getPodLogsInternal(c, namespace, podName, containerName, true)
}

// utility function for gomega Eventually
func getPodLogsInternal(c clientset.Interface, namespace, podName, containerName string, previous bool) (string, error) {
	logs, err := c.Core().RESTClient().Get().
		Resource("pods").
		Namespace(namespace).
		Name(podName).SubResource("log").
		Param("container", containerName).
		Param("previous", strconv.FormatBool(previous)).
		Do().
		Raw()
	if err != nil {
		return "", err
	}
	if err == nil && strings.Contains(string(logs), "Internal Error") {
		return "", fmt.Errorf("Fetched log contains \"Internal Error\": %q.", string(logs))
	}
	return string(logs), err
}

// EnsureLoadBalancerResourcesDeleted ensures that cloud load balancer resources that were created
// are actually cleaned up.  Currently only implemented for GCE/GKE.
func EnsureLoadBalancerResourcesDeleted(ip, portRange string) error {
	if TestContext.Provider == "gce" || TestContext.Provider == "gke" {
		return ensureGCELoadBalancerResourcesDeleted(ip, portRange)
	}
	return nil
}

func ensureGCELoadBalancerResourcesDeleted(ip, portRange string) error {
	gceCloud, ok := TestContext.CloudConfig.Provider.(*gcecloud.GCECloud)
	if !ok {
		return fmt.Errorf("failed to convert CloudConfig.Provider to GCECloud: %#v", TestContext.CloudConfig.Provider)
	}
	project := TestContext.CloudConfig.ProjectID
	region, err := gcecloud.GetGCERegion(TestContext.CloudConfig.Zone)
	if err != nil {
		return fmt.Errorf("could not get region for zone %q: %v", TestContext.CloudConfig.Zone, err)
	}

	return wait.Poll(10*time.Second, 5*time.Minute, func() (bool, error) {
		service := gceCloud.GetComputeService()
		list, err := service.ForwardingRules.List(project, region).Do()
		if err != nil {
			return false, err
		}
		for ix := range list.Items {
			item := list.Items[ix]
			if item.PortRange == portRange && item.IPAddress == ip {
				Logf("found a load balancer: %v", item)
				return false, nil
			}
		}
		return true, nil
	})
}

// The following helper functions can block/unblock network from source
// host to destination host by manipulating iptable rules.
// This function assumes it can ssh to the source host.
//
// Caution:
// Recommend to input IP instead of hostnames. Using hostnames will cause iptables to
// do a DNS lookup to resolve the name to an IP address, which will
// slow down the test and cause it to fail if DNS is absent or broken.
//
// Suggested usage pattern:
// func foo() {
//	...
//	defer UnblockNetwork(from, to)
//	BlockNetwork(from, to)
//	...
// }
//
func BlockNetwork(from string, to string) {
	Logf("block network traffic from %s to %s", from, to)
	iptablesRule := fmt.Sprintf("OUTPUT --destination %s --jump REJECT", to)
	dropCmd := fmt.Sprintf("sudo iptables --insert %s", iptablesRule)
	if result, err := SSH(dropCmd, from, TestContext.Provider); result.Code != 0 || err != nil {
		LogSSHResult(result)
		Failf("Unexpected error: %v", err)
	}
}

func UnblockNetwork(from string, to string) {
	Logf("Unblock network traffic from %s to %s", from, to)
	iptablesRule := fmt.Sprintf("OUTPUT --destination %s --jump REJECT", to)
	undropCmd := fmt.Sprintf("sudo iptables --delete %s", iptablesRule)
	// Undrop command may fail if the rule has never been created.
	// In such case we just lose 30 seconds, but the cluster is healthy.
	// But if the rule had been created and removing it failed, the node is broken and
	// not coming back. Subsequent tests will run or fewer nodes (some of the tests
	// may fail). Manual intervention is required in such case (recreating the
	// cluster solves the problem too).
	err := wait.Poll(time.Millisecond*100, time.Second*30, func() (bool, error) {
		result, err := SSH(undropCmd, from, TestContext.Provider)
		if result.Code == 0 && err == nil {
			return true, nil
		}
		LogSSHResult(result)
		if err != nil {
			Logf("Unexpected error: %v", err)
		}
		return false, nil
	})
	if err != nil {
		Failf("Failed to remove the iptable REJECT rule. Manual intervention is "+
			"required on host %s: remove rule %s, if exists", from, iptablesRule)
	}
}

func isElementOf(podUID types.UID, pods *api.PodList) bool {
	for _, pod := range pods.Items {
		if pod.UID == podUID {
			return true
		}
	}
	return false
}

func CheckRSHashLabel(rs *extensions.ReplicaSet) error {
	if len(rs.Labels[extensions.DefaultDeploymentUniqueLabelKey]) == 0 ||
		len(rs.Spec.Selector.MatchLabels[extensions.DefaultDeploymentUniqueLabelKey]) == 0 ||
		len(rs.Spec.Template.Labels[extensions.DefaultDeploymentUniqueLabelKey]) == 0 {
		return fmt.Errorf("unexpected RS missing required pod-hash-template: %+v, selector = %+v, template = %+v", rs, rs.Spec.Selector, rs.Spec.Template)
	}
	return nil
}

func CheckPodHashLabel(pods *api.PodList) error {
	invalidPod := ""
	for _, pod := range pods.Items {
		if len(pod.Labels[extensions.DefaultDeploymentUniqueLabelKey]) == 0 {
			if len(invalidPod) == 0 {
				invalidPod = "unexpected pods missing required pod-hash-template:"
			}
			invalidPod = fmt.Sprintf("%s %+v;", invalidPod, pod)
		}
	}
	if len(invalidPod) > 0 {
		return fmt.Errorf("%s", invalidPod)
	}
	return nil
}

// timeout for proxy requests.
const proxyTimeout = 2 * time.Minute

// NodeProxyRequest performs a get on a node proxy endpoint given the nodename and rest client.
func NodeProxyRequest(c clientset.Interface, node, endpoint string) (restclient.Result, error) {
	// proxy tends to hang in some cases when Node is not ready. Add an artificial timeout for this call.
	// This will leak a goroutine if proxy hangs. #22165
	subResourceProxyAvailable, err := ServerVersionGTE(subResourceServiceAndNodeProxyVersion, c.Discovery())
	if err != nil {
		return restclient.Result{}, err
	}
	var result restclient.Result
	finished := make(chan struct{})
	go func() {
		if subResourceProxyAvailable {
			result = c.Core().RESTClient().Get().
				Resource("nodes").
				SubResource("proxy").
				Name(fmt.Sprintf("%v:%v", node, ports.KubeletPort)).
				Suffix(endpoint).
				Do()

		} else {
			result = c.Core().RESTClient().Get().
				Prefix("proxy").
				Resource("nodes").
				Name(fmt.Sprintf("%v:%v", node, ports.KubeletPort)).
				Suffix(endpoint).
				Do()
		}
		finished <- struct{}{}
	}()
	select {
	case <-finished:
		return result, nil
	case <-time.After(proxyTimeout):
		return restclient.Result{}, nil
	}
}

// GetKubeletPods retrieves the list of pods on the kubelet
func GetKubeletPods(c clientset.Interface, node string) (*api.PodList, error) {
	return getKubeletPods(c, node, "pods")
}

// GetKubeletRunningPods retrieves the list of running pods on the kubelet. The pods
// includes necessary information (e.g., UID, name, namespace for
// pods/containers), but do not contain the full spec.
func GetKubeletRunningPods(c clientset.Interface, node string) (*api.PodList, error) {
	return getKubeletPods(c, node, "runningpods")
}

func getKubeletPods(c clientset.Interface, node, resource string) (*api.PodList, error) {
	result := &api.PodList{}
	client, err := NodeProxyRequest(c, node, resource)
	if err != nil {
		return &api.PodList{}, err
	}
	if err = client.Into(result); err != nil {
		return &api.PodList{}, err
	}
	return result, nil
}

// LaunchWebserverPod launches a pod serving http on port 8080 to act
// as the target for networking connectivity checks.  The ip address
// of the created pod will be returned if the pod is launched
// successfully.
func LaunchWebserverPod(f *Framework, podName, nodeName string) (ip string) {
	containerName := fmt.Sprintf("%s-container", podName)
	port := 8080
	pod := &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Name: podName,
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				{
					Name:  containerName,
					Image: "gcr.io/google_containers/porter:cd5cb5791ebaa8641955f0e8c2a9bed669b1eaab",
					Env:   []api.EnvVar{{Name: fmt.Sprintf("SERVE_PORT_%d", port), Value: "foo"}},
					Ports: []api.ContainerPort{{ContainerPort: int32(port)}},
				},
			},
			NodeName:      nodeName,
			RestartPolicy: api.RestartPolicyNever,
		},
	}
	podClient := f.ClientSet.Core().Pods(f.Namespace.Name)
	_, err := podClient.Create(pod)
	ExpectNoError(err)
	ExpectNoError(f.WaitForPodRunning(podName))
	createdPod, err := podClient.Get(podName)
	ExpectNoError(err)
	ip = fmt.Sprintf("%s:%d", createdPod.Status.PodIP, port)
	Logf("Target pod IP:port is %s", ip)
	return
}

// CheckConnectivityToHost launches a pod running wget on the
// specified node to test connectivity to the specified host.  An
// error will be returned if the host is not reachable from the pod.
func CheckConnectivityToHost(f *Framework, nodeName, podName, host string, timeout int) error {
	contName := fmt.Sprintf("%s-container", podName)
	pod := &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Name: podName,
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				{
					Name:    contName,
					Image:   "gcr.io/google_containers/busybox:1.24",
					Command: []string{"wget", fmt.Sprintf("--timeout=%d", timeout), "-s", host},
				},
			},
			NodeName:      nodeName,
			RestartPolicy: api.RestartPolicyNever,
		},
	}
	podClient := f.ClientSet.Core().Pods(f.Namespace.Name)
	_, err := podClient.Create(pod)
	if err != nil {
		return err
	}
	err = WaitForPodSuccessInNamespace(f.ClientSet, podName, f.Namespace.Name)

	if err != nil {
		logs, logErr := GetPodLogs(f.ClientSet, f.Namespace.Name, pod.Name, contName)
		if logErr != nil {
			Logf("Warning: Failed to get logs from pod %q: %v", pod.Name, logErr)
		} else {
			Logf("pod %s/%s \"wget\" logs:\n%s", f.Namespace.Name, pod.Name, logs)
		}
	}

	return err
}

// CoreDump SSHs to the master and all nodes and dumps their logs into dir.
// It shells out to cluster/log-dump.sh to accomplish this.
func CoreDump(dir string) {
	cmd := exec.Command(path.Join(TestContext.RepoRoot, "cluster", "log-dump.sh"), dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		Logf("Error running cluster/log-dump.sh: %v", err)
	}
}

func UpdatePodWithRetries(client clientset.Interface, ns, name string, update func(*api.Pod)) (*api.Pod, error) {
	for i := 0; i < 3; i++ {
		pod, err := client.Core().Pods(ns).Get(name)
		if err != nil {
			return nil, fmt.Errorf("Failed to get pod %q: %v", name, err)
		}
		update(pod)
		pod, err = client.Core().Pods(ns).Update(pod)
		if err == nil {
			return pod, nil
		}
		if !apierrs.IsConflict(err) && !apierrs.IsServerTimeout(err) {
			return nil, fmt.Errorf("Failed to update pod %q: %v", name, err)
		}
	}
	return nil, fmt.Errorf("Too many retries updating Pod %q", name)
}

func GetPodsInNamespace(c clientset.Interface, ns string, ignoreLabels map[string]string) ([]*api.Pod, error) {
	pods, err := c.Core().Pods(ns).List(api.ListOptions{})
	if err != nil {
		return []*api.Pod{}, err
	}
	ignoreSelector := labels.SelectorFromSet(ignoreLabels)
	filtered := []*api.Pod{}
	for _, p := range pods.Items {
		if len(ignoreLabels) != 0 && ignoreSelector.Matches(labels.Set(p.Labels)) {
			continue
		}
		filtered = append(filtered, &p)
	}
	return filtered, nil
}

// RunCmd runs cmd using args and returns its stdout and stderr. It also outputs
// cmd's stdout and stderr to their respective OS streams.
func RunCmd(command string, args ...string) (string, string, error) {
	Logf("Running %s %v", command, args)
	var bout, berr bytes.Buffer
	cmd := exec.Command(command, args...)
	// We also output to the OS stdout/stderr to aid in debugging in case cmd
	// hangs and never returns before the test gets killed.
	//
	// This creates some ugly output because gcloud doesn't always provide
	// newlines.
	cmd.Stdout = io.MultiWriter(os.Stdout, &bout)
	cmd.Stderr = io.MultiWriter(os.Stderr, &berr)
	err := cmd.Run()
	stdout, stderr := bout.String(), berr.String()
	if err != nil {
		return "", "", fmt.Errorf("error running %s %v; got error %v, stdout %q, stderr %q",
			command, args, err, stdout, stderr)
	}
	return stdout, stderr, nil
}

// retryCmd runs cmd using args and retries it for up to SingleCallTimeout if
// it returns an error. It returns stdout and stderr.
func retryCmd(command string, args ...string) (string, string, error) {
	var err error
	stdout, stderr := "", ""
	wait.Poll(Poll, SingleCallTimeout, func() (bool, error) {
		stdout, stderr, err = RunCmd(command, args...)
		if err != nil {
			Logf("Got %v", err)
			return false, nil
		}
		return true, nil
	})
	return stdout, stderr, err
}

// GetPodsScheduled returns a number of currently scheduled and not scheduled Pods.
func GetPodsScheduled(masterNodes sets.String, pods *api.PodList) (scheduledPods, notScheduledPods []api.Pod) {
	for _, pod := range pods.Items {
		if !masterNodes.Has(pod.Spec.NodeName) {
			if pod.Spec.NodeName != "" {
				_, scheduledCondition := api.GetPodCondition(&pod.Status, api.PodScheduled)
				Expect(scheduledCondition != nil).To(Equal(true))
				Expect(scheduledCondition.Status).To(Equal(api.ConditionTrue))
				scheduledPods = append(scheduledPods, pod)
			} else {
				_, scheduledCondition := api.GetPodCondition(&pod.Status, api.PodScheduled)
				Expect(scheduledCondition != nil).To(Equal(true))
				Expect(scheduledCondition.Status).To(Equal(api.ConditionFalse))
				if scheduledCondition.Reason == "Unschedulable" {

					notScheduledPods = append(notScheduledPods, pod)
				}
			}
		}
	}
	return
}

// WaitForStableCluster waits until all existing pods are scheduled and returns their amount.
func WaitForStableCluster(c clientset.Interface, masterNodes sets.String) int {
	timeout := 10 * time.Minute
	startTime := time.Now()

	allPods, err := c.Core().Pods(api.NamespaceAll).List(api.ListOptions{})
	ExpectNoError(err)
	// API server returns also Pods that succeeded. We need to filter them out.
	currentPods := make([]api.Pod, 0, len(allPods.Items))
	for _, pod := range allPods.Items {
		if pod.Status.Phase != api.PodSucceeded && pod.Status.Phase != api.PodFailed {
			currentPods = append(currentPods, pod)
		}

	}
	allPods.Items = currentPods
	scheduledPods, currentlyNotScheduledPods := GetPodsScheduled(masterNodes, allPods)
	for len(currentlyNotScheduledPods) != 0 {
		time.Sleep(2 * time.Second)

		allPods, err := c.Core().Pods(api.NamespaceAll).List(api.ListOptions{})
		ExpectNoError(err)
		scheduledPods, currentlyNotScheduledPods = GetPodsScheduled(masterNodes, allPods)

		if startTime.Add(timeout).Before(time.Now()) {
			Failf("Timed out after %v waiting for stable cluster.", timeout)
			break
		}
	}
	return len(scheduledPods)
}

// GetMasterAndWorkerNodesOrDie will return a list masters and schedulable worker nodes
func GetMasterAndWorkerNodesOrDie(c clientset.Interface) (sets.String, *api.NodeList) {
	nodes := &api.NodeList{}
	masters := sets.NewString()
	all, _ := c.Core().Nodes().List(api.ListOptions{})
	for _, n := range all.Items {
		if system.IsMasterNode(&n) {
			masters.Insert(n.Name)
		} else if isNodeSchedulable(&n) && isNodeUntainted(&n) {
			nodes.Items = append(nodes.Items, n)
		}
	}
	return masters, nodes
}

func CreateFileForGoBinData(gobindataPath, outputFilename string) error {
	data := ReadOrDie(gobindataPath)
	if len(data) == 0 {
		return fmt.Errorf("Failed to read gobindata from %v", gobindataPath)
	}
	fullPath := filepath.Join(TestContext.OutputDir, outputFilename)
	err := os.MkdirAll(filepath.Dir(fullPath), 0777)
	if err != nil {
		return fmt.Errorf("Error while creating directory %v: %v", filepath.Dir(fullPath), err)
	}
	err = ioutil.WriteFile(fullPath, data, 0644)
	if err != nil {
		return fmt.Errorf("Error while trying to write to file %v: %v", fullPath, err)
	}
	return nil
}

func ListNamespaceEvents(c clientset.Interface, ns string) error {
	ls, err := c.Core().Events(ns).List(api.ListOptions{})
	if err != nil {
		return err
	}
	for _, event := range ls.Items {
		glog.Infof("Event(%#v): type: '%v' reason: '%v' %v", event.InvolvedObject, event.Type, event.Reason, event.Message)
	}
	return nil
}

// E2ETestNodePreparer implements testutils.TestNodePreparer interface, which is used
// to create/modify Nodes before running a test.
type E2ETestNodePreparer struct {
	client clientset.Interface
	// Specifies how many nodes should be modified using the given strategy.
	// Only one strategy can be applied to a single Node, so there needs to
	// be at least <sum_of_keys> Nodes in the cluster.
	countToStrategy       []testutils.CountToStrategy
	nodeToAppliedStrategy map[string]testutils.PrepareNodeStrategy
}

func NewE2ETestNodePreparer(client clientset.Interface, countToStrategy []testutils.CountToStrategy) testutils.TestNodePreparer {
	return &E2ETestNodePreparer{
		client:                client,
		countToStrategy:       countToStrategy,
		nodeToAppliedStrategy: make(map[string]testutils.PrepareNodeStrategy),
	}
}

func (p *E2ETestNodePreparer) PrepareNodes() error {
	nodes := GetReadySchedulableNodesOrDie(p.client)
	numTemplates := 0
	for k := range p.countToStrategy {
		numTemplates += k
	}
	if numTemplates > len(nodes.Items) {
		return fmt.Errorf("Can't prepare Nodes. Got more templates than existing Nodes.")
	}
	index := 0
	sum := 0
	for _, v := range p.countToStrategy {
		sum += v.Count
		for ; index < sum; index++ {
			if err := testutils.DoPrepareNode(p.client, &nodes.Items[index], v.Strategy); err != nil {
				glog.Errorf("Aborting node preparation: %v", err)
				return err
			}
			p.nodeToAppliedStrategy[nodes.Items[index].Name] = v.Strategy
		}
	}
	return nil
}

func (p *E2ETestNodePreparer) CleanupNodes() error {
	var encounteredError error
	nodes := GetReadySchedulableNodesOrDie(p.client)
	for i := range nodes.Items {
		var err error
		name := nodes.Items[i].Name
		strategy, found := p.nodeToAppliedStrategy[name]
		if found {
			if err = testutils.DoCleanupNode(p.client, name, strategy); err != nil {
				glog.Errorf("Skipping cleanup of Node: failed update of %v: %v", name, err)
				encounteredError = err
			}
		}
	}
	return encounteredError
}

func CleanupGCEResources(loadBalancerName string) (err error) {
	gceCloud, ok := TestContext.CloudConfig.Provider.(*gcecloud.GCECloud)
	if !ok {
		return fmt.Errorf("failed to convert CloudConfig.Provider to GCECloud: %#v", TestContext.CloudConfig.Provider)
	}
	gceCloud.DeleteFirewall(loadBalancerName)
	gceCloud.DeleteForwardingRule(loadBalancerName)
	gceCloud.DeleteGlobalStaticIP(loadBalancerName)
	hc, _ := gceCloud.GetHttpHealthCheck(loadBalancerName)
	gceCloud.DeleteTargetPool(loadBalancerName, hc)
	return nil
}

// getMaster populates the externalIP, internalIP and hostname fields of the master.
// If any of these is unavailable, it is set to "".
func getMaster(c clientset.Interface) Address {
	master := Address{}

	// Populate the internal IP.
	eps, err := c.Core().Endpoints(api.NamespaceDefault).Get("kubernetes")
	if err != nil {
		Failf("Failed to get kubernetes endpoints: %v", err)
	}
	if len(eps.Subsets) != 1 || len(eps.Subsets[0].Addresses) != 1 {
		Failf("There are more than 1 endpoints for kubernetes service: %+v", eps)
	}
	master.internalIP = eps.Subsets[0].Addresses[0].IP

	// Populate the external IP/hostname.
	url, err := url.Parse(TestContext.Host)
	if err != nil {
		Failf("Failed to parse hostname: %v", err)
	}
	if net.ParseIP(url.Host) != nil {
		// TODO: Check that it is external IP (not having a reserved IP address as per RFC1918).
		master.externalIP = url.Host
	} else {
		master.hostname = url.Host
	}

	return master
}

// GetMasterAddress returns the hostname/external IP/internal IP as appropriate for e2e tests on a particular provider
// which is the address of the interface used for communication with the kubelet.
func GetMasterAddress(c clientset.Interface) string {
	master := getMaster(c)
	switch TestContext.Provider {
	case "gce", "gke":
		return master.externalIP
	case "aws":
		return awsMasterIP
	default:
		Failf("This test is not supported for provider %s and should be disabled", TestContext.Provider)
	}
	return ""
}

// GetNodeExternalIP returns node external IP concatenated with port 22 for ssh
// e.g. 1.2.3.4:22
func GetNodeExternalIP(node *api.Node) string {
	Logf("Getting external IP address for %s", node.Name)
	host := ""
	for _, a := range node.Status.Addresses {
		if a.Type == api.NodeExternalIP {
			host = a.Address + ":22"
			break
		}
	}
	if host == "" {
		Failf("Couldn't get the external IP of host %s with addresses %v", node.Name, node.Status.Addresses)
	}
	return host
}
