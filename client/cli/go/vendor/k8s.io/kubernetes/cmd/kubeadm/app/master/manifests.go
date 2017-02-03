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
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/images"
	api "k8s.io/kubernetes/pkg/api/v1"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/blang/semver"
)

// Static pod definitions in golang form are included below so that `kubeadm init` can get going.
const (
	DefaultClusterName     = "kubernetes"
	DefaultCloudConfigPath = "/etc/kubernetes/cloud-config"

	etcd                           = "etcd"
	apiServer                      = "apiserver"
	controllerManager              = "controller-manager"
	scheduler                      = "scheduler"
	proxy                          = "proxy"
	kubeAPIServer                  = "kube-apiserver"
	kubeControllerManager          = "kube-controller-manager"
	kubeScheduler                  = "kube-scheduler"
	kubeProxy                      = "kube-proxy"
	authorizationPolicyFile        = "abac_policy.json"
	authorizationWebhookConfigFile = "webhook_authz.conf"
)

var (
	// Minimum version of kube-apiserver that supports --kubelet-preferred-address-types
	preferredAddressAPIServerMinVersion = semver.MustParse("1.5.0")

	// Minimum version of kube-apiserver that has to have --anonymous-auth=false set
	anonAuthDisableAPIServerMinVersion = semver.MustParse("1.5.0")
)

// WriteStaticPodManifests builds manifest objects based on user provided configuration and then dumps it to disk
// where kubelet will pick and schedule them.
func WriteStaticPodManifests(cfg *kubeadmapi.MasterConfiguration) error {
	volumes := []api.Volume{k8sVolume(cfg)}
	volumeMounts := []api.VolumeMount{k8sVolumeMount()}

	if isCertsVolumeMountNeeded() {
		volumes = append(volumes, certsVolume(cfg))
		volumeMounts = append(volumeMounts, certsVolumeMount())
	}

	if isPkiVolumeMountNeeded() {
		volumes = append(volumes, pkiVolume(cfg))
		volumeMounts = append(volumeMounts, pkiVolumeMount())
	}

	// Prepare static pod specs
	staticPodSpecs := map[string]api.Pod{
		kubeAPIServer: componentPod(api.Container{
			Name:          kubeAPIServer,
			Image:         images.GetCoreImage(images.KubeAPIServerImage, cfg, kubeadmapi.GlobalEnvParams.HyperkubeImage),
			Command:       getAPIServerCommand(cfg, false),
			VolumeMounts:  volumeMounts,
			LivenessProbe: componentProbe(8080, "/healthz"),
			Resources:     componentResources("250m"),
			Env:           getProxyEnvVars(),
		}, volumes...),
		kubeControllerManager: componentPod(api.Container{
			Name:          kubeControllerManager,
			Image:         images.GetCoreImage(images.KubeControllerManagerImage, cfg, kubeadmapi.GlobalEnvParams.HyperkubeImage),
			Command:       getControllerManagerCommand(cfg, false),
			VolumeMounts:  volumeMounts,
			LivenessProbe: componentProbe(10252, "/healthz"),
			Resources:     componentResources("200m"),
			Env:           getProxyEnvVars(),
		}, volumes...),
		kubeScheduler: componentPod(api.Container{
			Name:          kubeScheduler,
			Image:         images.GetCoreImage(images.KubeSchedulerImage, cfg, kubeadmapi.GlobalEnvParams.HyperkubeImage),
			Command:       getSchedulerCommand(cfg, false),
			LivenessProbe: componentProbe(10251, "/healthz"),
			Resources:     componentResources("100m"),
			Env:           getProxyEnvVars(),
		}),
	}

	// Add etcd static pod spec only if external etcd is not configured
	if len(cfg.Etcd.Endpoints) == 0 {
		staticPodSpecs[etcd] = componentPod(api.Container{
			Name: etcd,
			Command: []string{
				"etcd",
				"--listen-client-urls=http://127.0.0.1:2379",
				"--advertise-client-urls=http://127.0.0.1:2379",
				"--data-dir=/var/lib/etcd",
			},
			VolumeMounts:  []api.VolumeMount{certsVolumeMount(), etcdVolumeMount(), k8sVolumeMount()},
			Image:         images.GetCoreImage(images.KubeEtcdImage, cfg, kubeadmapi.GlobalEnvParams.EtcdImage),
			LivenessProbe: componentProbe(2379, "/health"),
			Resources:     componentResources("200m"),
			SecurityContext: &api.SecurityContext{
				SELinuxOptions: &api.SELinuxOptions{
					// TODO: This implies our etcd container is not being restricted by
					// SELinux. This is not optimal and would be nice to adjust in future
					// so it can create and write /var/lib/etcd, but for now this avoids
					// recommending setenforce 0 system-wide.
					Type: "spc_t",
				},
			},
		}, certsVolume(cfg), etcdVolume(cfg), k8sVolume(cfg))
	}

	manifestsPath := path.Join(kubeadmapi.GlobalEnvParams.KubernetesDir, "manifests")
	if err := os.MkdirAll(manifestsPath, 0700); err != nil {
		return fmt.Errorf("failed to create directory %q [%v]", manifestsPath, err)
	}
	for name, spec := range staticPodSpecs {
		filename := path.Join(manifestsPath, name+".json")
		serialized, err := json.MarshalIndent(spec, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal manifest for %q to JSON [%v]", name, err)
		}
		if err := cmdutil.DumpReaderToFile(bytes.NewReader(serialized), filename); err != nil {
			return fmt.Errorf("failed to create static pod manifest file for %q (%q) [%v]", name, filename, err)
		}
	}
	return nil
}

// etcdVolume exposes a path on the host in order to guarantee data survival during reboot.
func etcdVolume(cfg *kubeadmapi.MasterConfiguration) api.Volume {
	return api.Volume{
		Name: "etcd",
		VolumeSource: api.VolumeSource{
			HostPath: &api.HostPathVolumeSource{Path: kubeadmapi.GlobalEnvParams.HostEtcdPath},
		},
	}
}

func etcdVolumeMount() api.VolumeMount {
	return api.VolumeMount{
		Name:      "etcd",
		MountPath: "/var/lib/etcd",
	}
}

func isCertsVolumeMountNeeded() bool {
	// Always return true for now. We may add conditional logic here for images which do not require host mounting /etc/ssl
	// hyperkube for example already has valid ca-certificates installed
	return true
}

// certsVolume exposes host SSL certificates to pod containers.
func certsVolume(cfg *kubeadmapi.MasterConfiguration) api.Volume {
	return api.Volume{
		Name: "certs",
		VolumeSource: api.VolumeSource{
			// TODO(phase1+) make path configurable
			HostPath: &api.HostPathVolumeSource{Path: "/etc/ssl/certs"},
		},
	}
}

func certsVolumeMount() api.VolumeMount {
	return api.VolumeMount{
		Name:      "certs",
		MountPath: "/etc/ssl/certs",
	}
}

func isPkiVolumeMountNeeded() bool {
	// On some systems were we host-mount /etc/ssl/certs, it is also required to mount /etc/pki. This is needed
	// due to symlinks pointing from files in /etc/ssl/certs into /etc/pki/
	if _, err := os.Stat("/etc/pki"); err == nil {
		return true
	}
	return false
}

func pkiVolume(cfg *kubeadmapi.MasterConfiguration) api.Volume {
	return api.Volume{
		Name: "pki",
		VolumeSource: api.VolumeSource{
			// TODO(phase1+) make path configurable
			HostPath: &api.HostPathVolumeSource{Path: "/etc/pki"},
		},
	}
}

func pkiVolumeMount() api.VolumeMount {
	return api.VolumeMount{
		Name:      "pki",
		MountPath: "/etc/pki",
	}
}

func flockVolume() api.Volume {
	return api.Volume{
		Name: "var-lock",
		VolumeSource: api.VolumeSource{
			HostPath: &api.HostPathVolumeSource{Path: "/var/lock"},
		},
	}
}

func flockVolumeMount() api.VolumeMount {
	return api.VolumeMount{
		Name:      "var-lock",
		MountPath: "/var/lock",
		ReadOnly:  false,
	}
}

func k8sVolume(cfg *kubeadmapi.MasterConfiguration) api.Volume {
	return api.Volume{
		Name: "k8s",
		VolumeSource: api.VolumeSource{
			HostPath: &api.HostPathVolumeSource{Path: kubeadmapi.GlobalEnvParams.KubernetesDir},
		},
	}
}

func k8sVolumeMount() api.VolumeMount {
	return api.VolumeMount{
		Name:      "k8s",
		MountPath: "/etc/kubernetes/",
		ReadOnly:  true,
	}
}

func componentResources(cpu string) api.ResourceRequirements {
	return api.ResourceRequirements{
		Requests: api.ResourceList{
			api.ResourceName(api.ResourceCPU): resource.MustParse(cpu),
		},
	}
}

func componentProbe(port int, path string) *api.Probe {
	return &api.Probe{
		Handler: api.Handler{
			HTTPGet: &api.HTTPGetAction{
				Host: "127.0.0.1",
				Path: path,
				Port: intstr.FromInt(port),
			},
		},
		InitialDelaySeconds: 15,
		TimeoutSeconds:      15,
		FailureThreshold:    8,
	}
}

func componentPod(container api.Container, volumes ...api.Volume) api.Pod {
	return api.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      container.Name,
			Namespace: "kube-system",
			Labels:    map[string]string{"component": container.Name, "tier": "control-plane"},
		},
		Spec: api.PodSpec{
			Containers:  []api.Container{container},
			HostNetwork: true,
			Volumes:     volumes,
		},
	}
}

func getComponentBaseCommand(component string) []string {
	if kubeadmapi.GlobalEnvParams.HyperkubeImage != "" {
		return []string{"/hyperkube", component}
	}

	return []string{"kube-" + component}
}

func getCertFilePath(certName string) string {
	return path.Join(kubeadmapi.GlobalEnvParams.HostPKIPath, certName)
}

func getAPIServerCommand(cfg *kubeadmapi.MasterConfiguration, selfHosted bool) []string {
	var command []string

	// self-hosted apiserver needs to wait on a lock
	if selfHosted {
		command = []string{"/usr/bin/flock", "--exclusive", "--timeout=30", "/var/lock/api-server.lock"}
	}

	command = append(getComponentBaseCommand(apiServer),
		"--insecure-bind-address=127.0.0.1",
		"--admission-control=NamespaceLifecycle,LimitRanger,ServiceAccount,PersistentVolumeLabel,DefaultStorageClass,ResourceQuota",
		"--service-cluster-ip-range="+cfg.Networking.ServiceSubnet,
		"--service-account-key-file="+getCertFilePath(kubeadmconstants.APIServerKeyName),
		"--client-ca-file="+getCertFilePath(kubeadmconstants.CACertName),
		"--tls-cert-file="+getCertFilePath(kubeadmconstants.APIServerCertName),
		"--tls-private-key-file="+getCertFilePath(kubeadmconstants.APIServerKeyName),
		"--kubelet-client-certificate="+getCertFilePath(kubeadmconstants.APIServerKubeletClientCertName),
		"--kubelet-client-key="+getCertFilePath(kubeadmconstants.APIServerKubeletClientKeyName),
		"--token-auth-file="+kubeadmapi.GlobalEnvParams.HostPKIPath+"/tokens.csv",
		fmt.Sprintf("--secure-port=%d", cfg.API.Port),
		"--allow-privileged",
		"--storage-backend=etcd3",
	)

	if cfg.AuthorizationMode != "" {
		command = append(command, "--authorization-mode="+cfg.AuthorizationMode)
		switch cfg.AuthorizationMode {
		case "ABAC":
			command = append(command, "--authorization-policy-file="+path.Join(kubeadmapi.GlobalEnvParams.KubernetesDir, authorizationPolicyFile))
		case "Webhook":
			command = append(command, "--authorization-webhook-config-file="+path.Join(kubeadmapi.GlobalEnvParams.KubernetesDir, authorizationWebhookConfigFile))
		}
	}

	// Use first address we are given
	if len(cfg.API.AdvertiseAddresses) > 0 {
		if selfHosted {
			command = append(command, "--advertise-address=$(POD_IP)")
		} else {
			command = append(command, fmt.Sprintf("--advertise-address=%s", cfg.API.AdvertiseAddresses[0]))
		}
	}

	if len(cfg.KubernetesVersion) != 0 {
		// If the k8s version is v1.5-something, this argument is set and makes `kubectl logs` and `kubectl exec`
		// work on bare-metal where hostnames aren't usually resolvable
		// Omit the "v" in the beginning, otherwise semver will fail
		k8sVersion, err := semver.Parse(cfg.KubernetesVersion[1:])

		// If the k8s version is greater than this version, it supports telling it which way it should contact kubelets
		if err == nil && k8sVersion.GTE(preferredAddressAPIServerMinVersion) {
			command = append(command, "--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname")
		}

		// This is a critical "bugfix". Any version above this is vulnerable unless a RBAC/ABAC-authorizer is provided (which kubeadm doesn't for the time being)
		if err == nil && k8sVersion.GTE(anonAuthDisableAPIServerMinVersion) {
			command = append(command, "--anonymous-auth=false")
		}
	}

	// Check if the user decided to use an external etcd cluster
	if len(cfg.Etcd.Endpoints) > 0 {
		command = append(command, fmt.Sprintf("--etcd-servers=%s", strings.Join(cfg.Etcd.Endpoints, ",")))
	} else {
		command = append(command, "--etcd-servers=http://127.0.0.1:2379")
	}

	// Is etcd secured?
	if cfg.Etcd.CAFile != "" {
		command = append(command, fmt.Sprintf("--etcd-cafile=%s", cfg.Etcd.CAFile))
	}
	if cfg.Etcd.CertFile != "" && cfg.Etcd.KeyFile != "" {
		etcdClientFileArg := fmt.Sprintf("--etcd-certfile=%s", cfg.Etcd.CertFile)
		etcdKeyFileArg := fmt.Sprintf("--etcd-keyfile=%s", cfg.Etcd.KeyFile)
		command = append(command, etcdClientFileArg, etcdKeyFileArg)
	}

	if cfg.CloudProvider != "" {
		command = append(command, "--cloud-provider="+cfg.CloudProvider)

		// Only append the --cloud-config option if there's a such file
		if _, err := os.Stat(DefaultCloudConfigPath); err == nil {
			command = append(command, "--cloud-config="+DefaultCloudConfigPath)
		}
	}

	return command
}

func getControllerManagerCommand(cfg *kubeadmapi.MasterConfiguration, selfHosted bool) []string {
	var command []string

	// self-hosted controller-manager needs to wait on a lock
	if selfHosted {
		command = []string{"/usr/bin/flock", "--exclusive", "--timeout=30", "/var/lock/controller-manager.lock"}
	}

	command = append(getComponentBaseCommand(controllerManager),
		"--address=127.0.0.1",
		"--leader-elect",
		"--master=127.0.0.1:8080",
		"--cluster-name="+DefaultClusterName,
		"--root-ca-file="+getCertFilePath(kubeadmconstants.CACertName),
		"--service-account-private-key-file="+getCertFilePath(kubeadmconstants.APIServerKeyName),
		"--cluster-signing-cert-file="+getCertFilePath(kubeadmconstants.CACertName),
		"--cluster-signing-key-file="+getCertFilePath(kubeadmconstants.CAKeyName),
		"--insecure-experimental-approve-all-kubelet-csrs-for-group="+KubeletBootstrapGroup,
	)

	if cfg.CloudProvider != "" {
		command = append(command, "--cloud-provider="+cfg.CloudProvider)

		// Only append the --cloud-config option if there's a such file
		if _, err := os.Stat(DefaultCloudConfigPath); err == nil {
			command = append(command, "--cloud-config="+DefaultCloudConfigPath)
		}
	}

	// Let the controller-manager allocate Node CIDRs for the Pod network.
	// Each node will get a subspace of the address CIDR provided with --pod-network-cidr.
	if cfg.Networking.PodSubnet != "" {
		command = append(command, "--allocate-node-cidrs=true", "--cluster-cidr="+cfg.Networking.PodSubnet)
	}

	return command
}

func getSchedulerCommand(cfg *kubeadmapi.MasterConfiguration, selfHosted bool) []string {
	var command []string

	// self-hosted apiserver needs to wait on a lock
	if selfHosted {
		command = []string{"/usr/bin/flock", "--exclusive", "--timeout=30", "/var/lock/api-server.lock"}
	}

	command = append(getComponentBaseCommand(scheduler),
		"--address=127.0.0.1",
		"--leader-elect",
		"--master=127.0.0.1:8080",
	)

	return command
}

func getProxyCommand(cfg *kubeadmapi.MasterConfiguration) []string {
	return getComponentBaseCommand(proxy)
}

func getProxyEnvVars() []api.EnvVar {
	envs := []api.EnvVar{}
	for _, env := range os.Environ() {
		pos := strings.Index(env, "=")
		if pos == -1 {
			// malformed environment variable, skip it.
			continue
		}
		name := env[:pos]
		value := env[pos+1:]
		if strings.HasSuffix(strings.ToLower(name), "_proxy") && value != "" {
			envVar := api.EnvVar{Name: name, Value: value}
			envs = append(envs, envVar)
		}
	}
	return envs
}

func getSelfHostedAPIServerEnv() []api.EnvVar {
	podIPEnvVar := api.EnvVar{
		Name: "POD_IP",
		ValueFrom: &api.EnvVarSource{
			FieldRef: &api.ObjectFieldSelector{
				FieldPath: "status.podIP",
			},
		},
	}

	return append(getProxyEnvVars(), podIPEnvVar)
}
