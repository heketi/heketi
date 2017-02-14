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

package master

import (
	"encoding/json"
	"fmt"
	"time"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/kubernetes/cmd/kubeadm/app/images"
	"k8s.io/kubernetes/pkg/api/v1"
	extensions "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
)

const apiCallRetryInterval = 500 * time.Millisecond

// TODO: This method shouldn't exist as a standalone function but be integrated into CreateClientFromFile
func createAPIClient(adminKubeconfig *clientcmdapi.Config) (*clientset.Clientset, error) {
	adminClientConfig, err := clientcmd.NewDefaultClientConfig(
		*adminKubeconfig,
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create API client configuration [%v]", err)
	}

	client, err := clientset.NewForConfig(adminClientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client [%v]", err)
	}
	return client, nil
}

func CreateClientFromFile(path string) (*clientset.Clientset, error) {
	adminKubeconfig, err := clientcmd.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load admin kubeconfig [%v]", err)
	}
	return createAPIClient(adminKubeconfig)
}

func CreateClientAndWaitForAPI(file string) (*clientset.Clientset, error) {
	client, err := CreateClientFromFile(file)
	if err != nil {
		return nil, err
	}

	fmt.Println("[apiclient] Created API client, waiting for the control plane to become ready")
	WaitForAPI(client)

	fmt.Println("[apiclient] Waiting for at least one node to register and become ready")
	start := time.Now()
	wait.PollInfinite(apiCallRetryInterval, func() (bool, error) {
		nodeList, err := client.Nodes().List(metav1.ListOptions{})
		if err != nil {
			fmt.Println("[apiclient] Temporarily unable to list nodes (will retry)")
			return false, nil
		}
		if len(nodeList.Items) < 1 {
			return false, nil
		}
		n := &nodeList.Items[0]
		if !v1.IsNodeReady(n) {
			fmt.Println("[apiclient] First node has registered, but is not ready yet")
			return false, nil
		}

		fmt.Printf("[apiclient] First node is ready after %f seconds\n", time.Since(start).Seconds())
		return true, nil
	})

	createDummyDeployment(client)

	return client, nil
}

func standardLabels(n string) map[string]string {
	return map[string]string{
		"component": n, "name": n, "k8s-app": n,
		"kubernetes.io/cluster-service": "true", "tier": "node",
	}
}

func WaitForAPI(client *clientset.Clientset) {
	start := time.Now()
	wait.PollInfinite(apiCallRetryInterval, func() (bool, error) {
		// TODO: use /healthz API instead of this
		cs, err := client.ComponentStatuses().List(metav1.ListOptions{})
		if err != nil {
			if apierrs.IsForbidden(err) {
				fmt.Println("[apiclient] Waiting for API server authorization")
			}
			return false, nil
		}

		// TODO(phase2) must revisit this when we implement HA
		if len(cs.Items) < 3 {
			return false, nil
		}
		for _, item := range cs.Items {
			for _, condition := range item.Conditions {
				if condition.Type != v1.ComponentHealthy {
					fmt.Printf("[apiclient] Control plane component %q is still unhealthy: %#v\n", item.ObjectMeta.Name, item.Conditions)
					return false, nil
				}
			}
		}

		fmt.Printf("[apiclient] All control plane components are healthy after %f seconds\n", time.Since(start).Seconds())
		return true, nil
	})
}

func NewDaemonSet(daemonName string, podSpec v1.PodSpec) *extensions.DaemonSet {
	l := standardLabels(daemonName)
	return &extensions.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: daemonName},
		Spec: extensions.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: l},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: l},
				Spec:       podSpec,
			},
		},
	}
}

func NewService(serviceName string, spec v1.ServiceSpec) *v1.Service {
	l := standardLabels(serviceName)
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   serviceName,
			Labels: l,
		},
		Spec: spec,
	}
}

func NewDeployment(deploymentName string, replicas int32, podSpec v1.PodSpec) *extensions.Deployment {
	l := standardLabels(deploymentName)
	return &extensions.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: deploymentName},
		Spec: extensions.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: l},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: l},
				Spec:       podSpec,
			},
		},
	}
}

// It's safe to do this for alpha, as we don't have HA and there is no way we can get
// more then one node here (TODO(phase1+) use os.Hostname)
func findMyself(client *clientset.Clientset) (*v1.Node, error) {
	nodeList, err := client.Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to list nodes [%v]", err)
	}
	if len(nodeList.Items) < 1 {
		return nil, fmt.Errorf("no nodes found")
	}
	node := &nodeList.Items[0]
	return node, nil
}

func attemptToUpdateMasterRoleLabelsAndTaints(client *clientset.Clientset, schedulable bool) error {
	n, err := findMyself(client)
	if err != nil {
		return err
	}

	n.ObjectMeta.Labels[metav1.NodeLabelKubeadmAlphaRole] = metav1.NodeLabelRoleMaster

	if !schedulable {
		taintsAnnotation, _ := json.Marshal([]v1.Taint{{Key: "dedicated", Value: "master", Effect: "NoSchedule"}})
		n.ObjectMeta.Annotations[v1.TaintsAnnotationKey] = string(taintsAnnotation)
	}

	if _, err := client.Nodes().Update(n); err != nil {
		if apierrs.IsConflict(err) {
			fmt.Println("[apiclient] Temporarily unable to update master node metadata due to conflict (will retry)")
			time.Sleep(apiCallRetryInterval)
			attemptToUpdateMasterRoleLabelsAndTaints(client, schedulable)
		} else {
			return err
		}
	}

	return nil
}

func UpdateMasterRoleLabelsAndTaints(client *clientset.Clientset, schedulable bool) error {
	// TODO(phase1+) use iterate instead of recursion
	err := attemptToUpdateMasterRoleLabelsAndTaints(client, schedulable)
	if err != nil {
		return fmt.Errorf("failed to update master node - [%v]", err)
	}
	return nil
}

func SetMasterTaintTolerations(meta *metav1.ObjectMeta) {
	tolerationsAnnotation, _ := json.Marshal([]v1.Toleration{{Key: "dedicated", Value: "master", Effect: "NoSchedule"}})
	if meta.Annotations == nil {
		meta.Annotations = map[string]string{}
	}
	meta.Annotations[v1.TolerationsAnnotationKey] = string(tolerationsAnnotation)
}

func createDummyDeployment(client *clientset.Clientset) {
	fmt.Println("[apiclient] Creating a test deployment")
	dummyDeployment := NewDeployment("dummy", 1, v1.PodSpec{
		HostNetwork:     true,
		SecurityContext: &v1.PodSecurityContext{},
		Containers: []v1.Container{{
			Name:  "dummy",
			Image: images.GetAddonImage("pause"),
		}},
	})

	wait.PollInfinite(apiCallRetryInterval, func() (bool, error) {
		// TODO: we should check the error, as some cases may be fatal
		if _, err := client.Extensions().Deployments(metav1.NamespaceSystem).Create(dummyDeployment); err != nil {
			fmt.Printf("[apiclient] Failed to create test deployment [%v] (will retry)\n", err)
			return false, nil
		}
		return true, nil
	})

	wait.PollInfinite(apiCallRetryInterval, func() (bool, error) {
		d, err := client.Extensions().Deployments(metav1.NamespaceSystem).Get("dummy", metav1.GetOptions{})
		if err != nil {
			fmt.Printf("[apiclient] Failed to get test deployment [%v] (will retry)\n", err)
			return false, nil
		}
		if d.Status.AvailableReplicas < 1 {
			return false, nil
		}
		return true, nil
	})

	fmt.Println("[apiclient] Test deployment succeeded")

	// TODO: In the future, make sure the ReplicaSet and Pod are garbage collected
	if err := client.Extensions().Deployments(metav1.NamespaceSystem).Delete("dummy", &metav1.DeleteOptions{}); err != nil {
		fmt.Printf("[apiclient] Failed to delete test deployment [%v] (will ignore)\n", err)
	}
}
