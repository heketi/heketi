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

package utils

import (
	"fmt"
	"math"
	"os"
	"sync"
	"time"

	"k8s.io/kubernetes/pkg/api"
	apierrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/uuid"
	"k8s.io/kubernetes/pkg/util/workqueue"

	"github.com/golang/glog"
)

const (
	// String used to mark pod deletion
	nonExist = "NonExist"
)

type RCConfig struct {
	Client         clientset.Interface
	Image          string
	Command        []string
	Name           string
	Namespace      string
	PollInterval   time.Duration
	Timeout        time.Duration
	PodStatusFile  *os.File
	Replicas       int
	CpuRequest     int64 // millicores
	CpuLimit       int64 // millicores
	MemRequest     int64 // bytes
	MemLimit       int64 // bytes
	ReadinessProbe *api.Probe
	DNSPolicy      *api.DNSPolicy

	// Env vars, set the same for every pod.
	Env map[string]string

	// Extra labels added to every pod.
	Labels map[string]string

	// Node selector for pods in the RC.
	NodeSelector map[string]string

	// Ports to declare in the container (map of name to containerPort).
	Ports map[string]int
	// Ports to declare in the container as host and container ports.
	HostPorts map[string]int

	Volumes      []api.Volume
	VolumeMounts []api.VolumeMount

	// Pointer to a list of pods; if non-nil, will be set to a list of pods
	// created by this RC by RunRC.
	CreatedPods *[]*api.Pod

	// Maximum allowable container failures. If exceeded, RunRC returns an error.
	// Defaults to replicas*0.1 if unspecified.
	MaxContainerFailures *int

	// If set to false starting RC will print progress, otherwise only errors will be printed.
	Silent bool

	// If set this function will be used to print log lines instead of glog.
	LogFunc func(fmt string, args ...interface{})
	// If set those functions will be used to gather data from Nodes - in integration tests where no
	// kubelets are running those variables should be nil.
	NodeDumpFunc      func(c clientset.Interface, nodeNames []string, logFunc func(fmt string, args ...interface{}))
	ContainerDumpFunc func(c clientset.Interface, ns string, logFunc func(ftm string, args ...interface{}))
}

func (rc *RCConfig) RCConfigLog(fmt string, args ...interface{}) {
	if rc.LogFunc != nil {
		rc.LogFunc(fmt, args...)
	}
	glog.Infof(fmt, args...)
}

type DeploymentConfig struct {
	RCConfig
}

type ReplicaSetConfig struct {
	RCConfig
}

// podInfo contains pod information useful for debugging e2e tests.
type podInfo struct {
	oldHostname string
	oldPhase    string
	hostname    string
	phase       string
}

// PodDiff is a map of pod name to podInfos
type PodDiff map[string]*podInfo

// Print formats and prints the give PodDiff.
func (p PodDiff) String(ignorePhases sets.String) string {
	ret := ""
	for name, info := range p {
		if ignorePhases.Has(info.phase) {
			continue
		}
		if info.phase == nonExist {
			ret += fmt.Sprintf("Pod %v was deleted, had phase %v and host %v\n", name, info.oldPhase, info.oldHostname)
			continue
		}
		phaseChange, hostChange := false, false
		msg := fmt.Sprintf("Pod %v ", name)
		if info.oldPhase != info.phase {
			phaseChange = true
			if info.oldPhase == nonExist {
				msg += fmt.Sprintf("in phase %v ", info.phase)
			} else {
				msg += fmt.Sprintf("went from phase: %v -> %v ", info.oldPhase, info.phase)
			}
		}
		if info.oldHostname != info.hostname {
			hostChange = true
			if info.oldHostname == nonExist || info.oldHostname == "" {
				msg += fmt.Sprintf("assigned host %v ", info.hostname)
			} else {
				msg += fmt.Sprintf("went from host: %v -> %v ", info.oldHostname, info.hostname)
			}
		}
		if phaseChange || hostChange {
			ret += msg + "\n"
		}
	}
	return ret
}

// Diff computes a PodDiff given 2 lists of pods.
func Diff(oldPods []*api.Pod, curPods []*api.Pod) PodDiff {
	podInfoMap := PodDiff{}

	// New pods will show up in the curPods list but not in oldPods. They have oldhostname/phase == nonexist.
	for _, pod := range curPods {
		podInfoMap[pod.Name] = &podInfo{hostname: pod.Spec.NodeName, phase: string(pod.Status.Phase), oldHostname: nonExist, oldPhase: nonExist}
	}

	// Deleted pods will show up in the oldPods list but not in curPods. They have a hostname/phase == nonexist.
	for _, pod := range oldPods {
		if info, ok := podInfoMap[pod.Name]; ok {
			info.oldHostname, info.oldPhase = pod.Spec.NodeName, string(pod.Status.Phase)
		} else {
			podInfoMap[pod.Name] = &podInfo{hostname: nonExist, phase: nonExist, oldHostname: pod.Spec.NodeName, oldPhase: string(pod.Status.Phase)}
		}
	}
	return podInfoMap
}

// RunDeployment Launches (and verifies correctness) of a Deployment
// and will wait for all pods it spawns to become "Running".
// It's the caller's responsibility to clean up externally (i.e. use the
// namespace lifecycle for handling Cleanup).
func RunDeployment(config DeploymentConfig) error {
	err := config.create()
	if err != nil {
		return err
	}
	return config.start()
}

func (config *DeploymentConfig) create() error {
	deployment := &extensions.Deployment{
		ObjectMeta: api.ObjectMeta{
			Name: config.Name,
		},
		Spec: extensions.DeploymentSpec{
			Replicas: int32(config.Replicas),
			Selector: &unversioned.LabelSelector{
				MatchLabels: map[string]string{
					"name": config.Name,
				},
			},
			Template: api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Labels: map[string]string{"name": config.Name},
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{
							Name:    config.Name,
							Image:   config.Image,
							Command: config.Command,
							Ports:   []api.ContainerPort{{ContainerPort: 80}},
						},
					},
				},
			},
		},
	}

	config.applyTo(&deployment.Spec.Template)

	_, err := config.Client.Extensions().Deployments(config.Namespace).Create(deployment)
	if err != nil {
		return fmt.Errorf("Error creating deployment: %v", err)
	}
	config.RCConfigLog("Created deployment with name: %v, namespace: %v, replica count: %v", deployment.Name, config.Namespace, deployment.Spec.Replicas)
	return nil
}

// RunReplicaSet launches (and verifies correctness) of a ReplicaSet
// and waits until all the pods it launches to reach the "Running" state.
// It's the caller's responsibility to clean up externally (i.e. use the
// namespace lifecycle for handling Cleanup).
func RunReplicaSet(config ReplicaSetConfig) error {
	err := config.create()
	if err != nil {
		return err
	}
	return config.start()
}

func (config *ReplicaSetConfig) create() error {
	rs := &extensions.ReplicaSet{
		ObjectMeta: api.ObjectMeta{
			Name: config.Name,
		},
		Spec: extensions.ReplicaSetSpec{
			Replicas: int32(config.Replicas),
			Selector: &unversioned.LabelSelector{
				MatchLabels: map[string]string{
					"name": config.Name,
				},
			},
			Template: api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Labels: map[string]string{"name": config.Name},
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{
							Name:    config.Name,
							Image:   config.Image,
							Command: config.Command,
							Ports:   []api.ContainerPort{{ContainerPort: 80}},
						},
					},
				},
			},
		},
	}

	config.applyTo(&rs.Spec.Template)

	_, err := config.Client.Extensions().ReplicaSets(config.Namespace).Create(rs)
	if err != nil {
		return fmt.Errorf("Error creating replica set: %v", err)
	}
	config.RCConfigLog("Created replica set with name: %v, namespace: %v, replica count: %v", rs.Name, config.Namespace, rs.Spec.Replicas)
	return nil
}

// RunRC Launches (and verifies correctness) of a Replication Controller
// and will wait for all pods it spawns to become "Running".
// It's the caller's responsibility to clean up externally (i.e. use the
// namespace lifecycle for handling Cleanup).
func RunRC(config RCConfig) error {
	err := config.create()
	if err != nil {
		return err
	}
	return config.start()
}

func (config *RCConfig) create() error {
	dnsDefault := api.DNSDefault
	if config.DNSPolicy == nil {
		config.DNSPolicy = &dnsDefault
	}
	rc := &api.ReplicationController{
		ObjectMeta: api.ObjectMeta{
			Name: config.Name,
		},
		Spec: api.ReplicationControllerSpec{
			Replicas: int32(config.Replicas),
			Selector: map[string]string{
				"name": config.Name,
			},
			Template: &api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Labels: map[string]string{"name": config.Name},
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{
							Name:           config.Name,
							Image:          config.Image,
							Command:        config.Command,
							Ports:          []api.ContainerPort{{ContainerPort: 80}},
							ReadinessProbe: config.ReadinessProbe,
						},
					},
					DNSPolicy:    *config.DNSPolicy,
					NodeSelector: config.NodeSelector,
				},
			},
		},
	}

	config.applyTo(rc.Spec.Template)

	_, err := config.Client.Core().ReplicationControllers(config.Namespace).Create(rc)
	if err != nil {
		return fmt.Errorf("Error creating replication controller: %v", err)
	}
	config.RCConfigLog("Created replication controller with name: %v, namespace: %v, replica count: %v", rc.Name, config.Namespace, rc.Spec.Replicas)
	return nil
}

func (config *RCConfig) applyTo(template *api.PodTemplateSpec) {
	if config.Env != nil {
		for k, v := range config.Env {
			c := &template.Spec.Containers[0]
			c.Env = append(c.Env, api.EnvVar{Name: k, Value: v})
		}
	}
	if config.Labels != nil {
		for k, v := range config.Labels {
			template.ObjectMeta.Labels[k] = v
		}
	}
	if config.NodeSelector != nil {
		template.Spec.NodeSelector = make(map[string]string)
		for k, v := range config.NodeSelector {
			template.Spec.NodeSelector[k] = v
		}
	}
	if config.Ports != nil {
		for k, v := range config.Ports {
			c := &template.Spec.Containers[0]
			c.Ports = append(c.Ports, api.ContainerPort{Name: k, ContainerPort: int32(v)})
		}
	}
	if config.HostPorts != nil {
		for k, v := range config.HostPorts {
			c := &template.Spec.Containers[0]
			c.Ports = append(c.Ports, api.ContainerPort{Name: k, ContainerPort: int32(v), HostPort: int32(v)})
		}
	}
	if config.CpuLimit > 0 || config.MemLimit > 0 {
		template.Spec.Containers[0].Resources.Limits = api.ResourceList{}
	}
	if config.CpuLimit > 0 {
		template.Spec.Containers[0].Resources.Limits[api.ResourceCPU] = *resource.NewMilliQuantity(config.CpuLimit, resource.DecimalSI)
	}
	if config.MemLimit > 0 {
		template.Spec.Containers[0].Resources.Limits[api.ResourceMemory] = *resource.NewQuantity(config.MemLimit, resource.DecimalSI)
	}
	if config.CpuRequest > 0 || config.MemRequest > 0 {
		template.Spec.Containers[0].Resources.Requests = api.ResourceList{}
	}
	if config.CpuRequest > 0 {
		template.Spec.Containers[0].Resources.Requests[api.ResourceCPU] = *resource.NewMilliQuantity(config.CpuRequest, resource.DecimalSI)
	}
	if config.MemRequest > 0 {
		template.Spec.Containers[0].Resources.Requests[api.ResourceMemory] = *resource.NewQuantity(config.MemRequest, resource.DecimalSI)
	}
	if len(config.Volumes) > 0 {
		template.Spec.Volumes = config.Volumes
	}
	if len(config.VolumeMounts) > 0 {
		template.Spec.Containers[0].VolumeMounts = config.VolumeMounts
	}
}

type RCStartupStatus struct {
	Expected              int
	Terminating           int
	Running               int
	RunningButNotReady    int
	Waiting               int
	Pending               int
	Unknown               int
	Inactive              int
	FailedContainers      int
	Created               []*api.Pod
	ContainerRestartNodes sets.String
}

func (s *RCStartupStatus) String(name string) string {
	return fmt.Sprintf("%v Pods: %d out of %d created, %d running, %d pending, %d waiting, %d inactive, %d terminating, %d unknown, %d runningButNotReady ",
		name, len(s.Created), s.Expected, s.Running, s.Pending, s.Waiting, s.Inactive, s.Terminating, s.Unknown, s.RunningButNotReady)
}

func ComputeRCStartupStatus(pods []*api.Pod, expected int) RCStartupStatus {
	startupStatus := RCStartupStatus{
		Expected:              expected,
		Created:               make([]*api.Pod, 0, expected),
		ContainerRestartNodes: sets.NewString(),
	}
	for _, p := range pods {
		if p.DeletionTimestamp != nil {
			startupStatus.Terminating++
			continue
		}
		startupStatus.Created = append(startupStatus.Created, p)
		if p.Status.Phase == api.PodRunning {
			ready := false
			for _, c := range p.Status.Conditions {
				if c.Type == api.PodReady && c.Status == api.ConditionTrue {
					ready = true
					break
				}
			}
			if ready {
				// Only count a pod is running when it is also ready.
				startupStatus.Running++
			} else {
				startupStatus.RunningButNotReady++
			}
			for _, v := range FailedContainers(p) {
				startupStatus.FailedContainers = startupStatus.FailedContainers + v.Restarts
				startupStatus.ContainerRestartNodes.Insert(p.Spec.NodeName)
			}
		} else if p.Status.Phase == api.PodPending {
			if p.Spec.NodeName == "" {
				startupStatus.Waiting++
			} else {
				startupStatus.Pending++
			}
		} else if p.Status.Phase == api.PodSucceeded || p.Status.Phase == api.PodFailed {
			startupStatus.Inactive++
		} else if p.Status.Phase == api.PodUnknown {
			startupStatus.Unknown++
		}
	}
	return startupStatus
}

func (config *RCConfig) start() error {
	// Don't force tests to fail if they don't care about containers restarting.
	var maxContainerFailures int
	if config.MaxContainerFailures == nil {
		maxContainerFailures = int(math.Max(1.0, float64(config.Replicas)*.01))
	} else {
		maxContainerFailures = *config.MaxContainerFailures
	}

	label := labels.SelectorFromSet(labels.Set(map[string]string{"name": config.Name}))

	PodStore := NewPodStore(config.Client, config.Namespace, label, fields.Everything())
	defer PodStore.Stop()

	interval := config.PollInterval
	if interval <= 0 {
		interval = 10 * time.Second
	}
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	oldPods := make([]*api.Pod, 0)
	oldRunning := 0
	lastChange := time.Now()
	for oldRunning != config.Replicas {
		time.Sleep(interval)

		pods := PodStore.List()
		startupStatus := ComputeRCStartupStatus(pods, config.Replicas)

		pods = startupStatus.Created
		if config.CreatedPods != nil {
			*config.CreatedPods = pods
		}
		if !config.Silent {
			config.RCConfigLog(startupStatus.String(config.Name))
		}

		if config.PodStatusFile != nil {
			fmt.Fprintf(config.PodStatusFile, "%d, running, %d, pending, %d, waiting, %d, inactive, %d, unknown, %d, runningButNotReady\n", startupStatus.Running, startupStatus.Pending, startupStatus.Waiting, startupStatus.Inactive, startupStatus.Unknown, startupStatus.RunningButNotReady)
		}

		if startupStatus.FailedContainers > maxContainerFailures {
			if config.NodeDumpFunc != nil {
				config.NodeDumpFunc(config.Client, startupStatus.ContainerRestartNodes.List(), config.RCConfigLog)
			}
			if config.ContainerDumpFunc != nil {
				// Get the logs from the failed containers to help diagnose what caused them to fail
				config.ContainerDumpFunc(config.Client, config.Namespace, config.RCConfigLog)
			}
			return fmt.Errorf("%d containers failed which is more than allowed %d", startupStatus.FailedContainers, maxContainerFailures)
		}
		if len(pods) < len(oldPods) || len(pods) > config.Replicas {
			// This failure mode includes:
			// kubelet is dead, so node controller deleted pods and rc creates more
			//	- diagnose by noting the pod diff below.
			// pod is unhealthy, so replication controller creates another to take its place
			//	- diagnose by comparing the previous "2 Pod states" lines for inactive pods
			errorStr := fmt.Sprintf("Number of reported pods for %s changed: %d vs %d", config.Name, len(pods), len(oldPods))
			config.RCConfigLog("%v, pods that changed since the last iteration:", errorStr)
			config.RCConfigLog(Diff(oldPods, pods).String(sets.NewString()))
			return fmt.Errorf(errorStr)
		}

		if len(pods) > len(oldPods) || startupStatus.Running > oldRunning {
			lastChange = time.Now()
		}
		oldPods = pods
		oldRunning = startupStatus.Running

		if time.Since(lastChange) > timeout {
			break
		}
	}

	if oldRunning != config.Replicas {
		// List only pods from a given replication controller.
		options := api.ListOptions{LabelSelector: label}
		if pods, err := config.Client.Core().Pods(api.NamespaceAll).List(options); err == nil {

			for _, pod := range pods.Items {
				config.RCConfigLog("Pod %s\t%s\t%s\t%s", pod.Name, pod.Spec.NodeName, pod.Status.Phase, pod.DeletionTimestamp)
			}
		} else {
			config.RCConfigLog("Can't list pod debug info: %v", err)
		}
		return fmt.Errorf("Only %d pods started out of %d", oldRunning, config.Replicas)
	}
	return nil
}

// Simplified version of RunRC, that does not create RC, but creates plain Pods.
// Optionally waits for pods to start running (if waitForRunning == true).
// The number of replicas must be non-zero.
func StartPods(c clientset.Interface, replicas int, namespace string, podNamePrefix string,
	pod api.Pod, waitForRunning bool, logFunc func(fmt string, args ...interface{})) error {
	// no pod to start
	if replicas < 1 {
		panic("StartPods: number of replicas must be non-zero")
	}
	startPodsID := string(uuid.NewUUID()) // So that we can label and find them
	for i := 0; i < replicas; i++ {
		podName := fmt.Sprintf("%v-%v", podNamePrefix, i)
		pod.ObjectMeta.Name = podName
		pod.ObjectMeta.Labels["name"] = podName
		pod.ObjectMeta.Labels["startPodsID"] = startPodsID
		pod.Spec.Containers[0].Name = podName
		_, err := c.Core().Pods(namespace).Create(&pod)
		if err != nil {
			return err
		}
	}
	logFunc("Waiting for running...")
	if waitForRunning {
		label := labels.SelectorFromSet(labels.Set(map[string]string{"startPodsID": startPodsID}))
		err := WaitForPodsWithLabelRunning(c, namespace, label)
		if err != nil {
			return fmt.Errorf("Error waiting for %d pods to be running - probably a timeout: %v", replicas, err)
		}
	}
	return nil
}

// Wait up to 10 minutes for all matching pods to become Running and at least one
// matching pod exists.
func WaitForPodsWithLabelRunning(c clientset.Interface, ns string, label labels.Selector) error {
	running := false
	PodStore := NewPodStore(c, ns, label, fields.Everything())
	defer PodStore.Stop()
waitLoop:
	for start := time.Now(); time.Since(start) < 10*time.Minute; time.Sleep(5 * time.Second) {
		pods := PodStore.List()
		if len(pods) == 0 {
			continue waitLoop
		}
		for _, p := range pods {
			if p.Status.Phase != api.PodRunning {
				continue waitLoop
			}
		}
		running = true
		break
	}
	if !running {
		return fmt.Errorf("Timeout while waiting for pods with labels %q to be running", label.String())
	}
	return nil
}

type CountToStrategy struct {
	Count    int
	Strategy PrepareNodeStrategy
}

type TestNodePreparer interface {
	PrepareNodes() error
	CleanupNodes() error
}

type PrepareNodeStrategy interface {
	PreparePatch(node *api.Node) []byte
	CleanupNode(node *api.Node) *api.Node
}

type TrivialNodePrepareStrategy struct{}

func (*TrivialNodePrepareStrategy) PreparePatch(*api.Node) []byte {
	return []byte{}
}

func (*TrivialNodePrepareStrategy) CleanupNode(node *api.Node) *api.Node {
	nodeCopy := *node
	return &nodeCopy
}

type LabelNodePrepareStrategy struct {
	labelKey   string
	labelValue string
}

func NewLabelNodePrepareStrategy(labelKey string, labelValue string) *LabelNodePrepareStrategy {
	return &LabelNodePrepareStrategy{
		labelKey:   labelKey,
		labelValue: labelValue,
	}
}

func (s *LabelNodePrepareStrategy) PreparePatch(*api.Node) []byte {
	labelString := fmt.Sprintf("{\"%v\":\"%v\"}", s.labelKey, s.labelValue)
	patch := fmt.Sprintf(`{"metadata":{"labels":%v}}`, labelString)
	return []byte(patch)
}

func (s *LabelNodePrepareStrategy) CleanupNode(node *api.Node) *api.Node {
	objCopy, err := api.Scheme.DeepCopy(*node)
	if err != nil {
		return &api.Node{}
	}
	nodeCopy, ok := (objCopy).(*api.Node)
	if !ok {
		return &api.Node{}
	}
	if node.Labels != nil && len(node.Labels[s.labelKey]) != 0 {
		delete(nodeCopy.Labels, s.labelKey)
	}
	return nodeCopy
}

func DoPrepareNode(client clientset.Interface, node *api.Node, strategy PrepareNodeStrategy) error {
	var err error
	patch := strategy.PreparePatch(node)
	if len(patch) == 0 {
		return nil
	}
	for attempt := 0; attempt < retries; attempt++ {
		if _, err = client.Core().Nodes().Patch(node.Name, api.MergePatchType, []byte(patch)); err == nil {
			return nil
		}
		if !apierrs.IsConflict(err) {
			return fmt.Errorf("Error while applying patch %v to Node %v: %v", string(patch), node.Name, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("To many conflicts when applying patch %v to Node %v", string(patch), node.Name)
}

func DoCleanupNode(client clientset.Interface, nodeName string, strategy PrepareNodeStrategy) error {
	for attempt := 0; attempt < retries; attempt++ {
		node, err := client.Core().Nodes().Get(nodeName)
		if err != nil {
			return fmt.Errorf("Skipping cleanup of Node: failed to get Node %v: %v", nodeName, err)
		}
		updatedNode := strategy.CleanupNode(node)
		if api.Semantic.DeepEqual(node, updatedNode) {
			return nil
		}
		if _, err = client.Core().Nodes().Update(updatedNode); err == nil {
			return nil
		}
		if !apierrs.IsConflict(err) {
			return fmt.Errorf("Error when updating Node %v: %v", nodeName, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("To many conflicts when trying to cleanup Node %v", nodeName)
}

type TestPodCreateStrategy func(client clientset.Interface, namespace string, podCount int) error

type CountToPodStrategy struct {
	Count    int
	Strategy TestPodCreateStrategy
}

type TestPodCreatorConfig map[string][]CountToPodStrategy

func NewTestPodCreatorConfig() *TestPodCreatorConfig {
	config := make(TestPodCreatorConfig)
	return &config
}

func (c *TestPodCreatorConfig) AddStrategy(
	namespace string, podCount int, strategy TestPodCreateStrategy) {
	(*c)[namespace] = append((*c)[namespace], CountToPodStrategy{Count: podCount, Strategy: strategy})
}

type TestPodCreator struct {
	Client clientset.Interface
	// namespace -> count -> strategy
	Config *TestPodCreatorConfig
}

func NewTestPodCreator(client clientset.Interface, config *TestPodCreatorConfig) *TestPodCreator {
	return &TestPodCreator{
		Client: client,
		Config: config,
	}
}

func (c *TestPodCreator) CreatePods() error {
	for ns, v := range *(c.Config) {
		for _, countToStrategy := range v {
			if err := countToStrategy.Strategy(c.Client, ns, countToStrategy.Count); err != nil {
				return err
			}
		}
	}
	return nil
}

func MakePodSpec() api.PodSpec {
	return api.PodSpec{
		Containers: []api.Container{{
			Name:  "pause",
			Image: "kubernetes/pause",
			Ports: []api.ContainerPort{{ContainerPort: 80}},
			Resources: api.ResourceRequirements{
				Limits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("100m"),
					api.ResourceMemory: resource.MustParse("500Mi"),
				},
				Requests: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("100m"),
					api.ResourceMemory: resource.MustParse("500Mi"),
				},
			},
		}},
	}
}

func makeCreatePod(client clientset.Interface, namespace string, podTemplate *api.Pod) error {
	var err error
	for attempt := 0; attempt < retries; attempt++ {
		if _, err := client.Core().Pods(namespace).Create(podTemplate); err == nil {
			return nil
		}
		glog.Errorf("Error while creating pod, maybe retry: %v", err)
	}
	return fmt.Errorf("Terminal error while creating pod, won't retry: %v", err)
}

func createPod(client clientset.Interface, namespace string, podCount int, podTemplate *api.Pod) error {
	var createError error
	lock := sync.Mutex{}
	createPodFunc := func(i int) {
		if err := makeCreatePod(client, namespace, podTemplate); err != nil {
			lock.Lock()
			defer lock.Unlock()
			createError = err
		}
	}

	if podCount < 30 {
		workqueue.Parallelize(podCount, podCount, createPodFunc)
	} else {
		workqueue.Parallelize(30, podCount, createPodFunc)
	}
	return createError
}

func createController(client clientset.Interface, controllerName, namespace string, podCount int, podTemplate *api.Pod) error {
	rc := &api.ReplicationController{
		ObjectMeta: api.ObjectMeta{
			Name: controllerName,
		},
		Spec: api.ReplicationControllerSpec{
			Replicas: int32(podCount),
			Selector: map[string]string{"name": controllerName},
			Template: &api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Labels: map[string]string{"name": controllerName},
				},
				Spec: podTemplate.Spec,
			},
		},
	}
	var err error
	for attempt := 0; attempt < retries; attempt++ {
		if _, err := client.Core().ReplicationControllers(namespace).Create(rc); err == nil {
			return nil
		}
		glog.Errorf("Error while creating rc, maybe retry: %v", err)
	}
	return fmt.Errorf("Terminal error while creating rc, won't retry: %v", err)
}

func NewCustomCreatePodStrategy(podTemplate *api.Pod) TestPodCreateStrategy {
	return func(client clientset.Interface, namespace string, podCount int) error {
		return createPod(client, namespace, podCount, podTemplate)
	}
}

func NewSimpleCreatePodStrategy() TestPodCreateStrategy {
	basePod := &api.Pod{
		ObjectMeta: api.ObjectMeta{
			GenerateName: "simple-pod-",
		},
		Spec: MakePodSpec(),
	}
	return NewCustomCreatePodStrategy(basePod)
}

func NewSimpleWithControllerCreatePodStrategy(controllerName string) TestPodCreateStrategy {
	return func(client clientset.Interface, namespace string, podCount int) error {
		basePod := &api.Pod{
			ObjectMeta: api.ObjectMeta{
				GenerateName: controllerName + "-pod-",
				Labels:       map[string]string{"name": controllerName},
			},
			Spec: MakePodSpec(),
		}
		if err := createController(client, controllerName, namespace, podCount, basePod); err != nil {
			return err
		}
		return createPod(client, namespace, podCount, basePod)
	}
}
