//
// Copyright (c) 2016 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package kubeexec

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/restclient"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/remotecommand"
	"k8s.io/kubernetes/pkg/fields"
	kubeletcmd "k8s.io/kubernetes/pkg/kubelet/server/remotecommand"
	"k8s.io/kubernetes/pkg/labels"

	"github.com/lpabon/godbc"

	"github.com/heketi/heketi/executors/sshexec"
	"github.com/heketi/heketi/pkg/utils"
)

const (
	KubeGlusterFSPodLabelKey = "glusterfs-node"
)

type KubeExecutor struct {
	// Embed all sshexecutor functions
	sshexec.SshExecutor

	// save kube configuration
	config *KubeConfig
}

var (
	logger       = utils.NewLogger("[kubeexec]", utils.LEVEL_DEBUG)
	tokenCreator = tokencmd.RequestToken
)

func setWithEnvVariables(config *KubeConfig) {
	// Check Host e.g. "https://myhost:8443"
	env := os.Getenv("HEKETI_KUBE_APIHOST")
	if "" != env {
		config.Host = env
	}

	// Check certificate file
	env = os.Getenv("HEKETI_KUBE_CERTFILE")
	if "" != env {
		config.CertFile = env
	}

	// Correct values are = y YES yes Yes Y true 1
	// disable are n N no NO No
	env = os.Getenv("HEKETI_KUBE_INSECURE")
	if "" != env {
		env = strings.ToLower(env)
		if env[0] == 'y' || env[0] == '1' {
			config.Insecure = true
		} else if env[0] == 'n' || env[0] == '0' {
			config.Insecure = false
		}
	}

	// User login
	env = os.Getenv("HEKETI_KUBE_USER")
	if "" != env {
		config.User = env
	}

	// Password for user
	env = os.Getenv("HEKETI_KUBE_PASSWORD")
	if "" != env {
		config.Password = env
	}

	// Namespace / Project
	env = os.Getenv("HEKETI_KUBE_NAMESPACE")
	if "" != env {
		config.Namespace = env
	}

	// FSTAB
	env = os.Getenv("HEKETI_FSTAB")
	if "" != env {
		config.Fstab = env
	}

	// Snapshot Limit
	env = os.Getenv("HEKETI_SNAPSHOT_LIMIT")
	if "" != env {
		i, err := strconv.Atoi(env)
		if err == nil {
			config.SnapShotLimit = i
		}
	}

	// Use secret for Auth
	env = os.Getenv("HEKETI_KUBE_USE_SECRET")
	if "" != env {
		env = strings.ToLower(env)
		if env[0] == 'y' || env[0] == '1' {
			config.UseSecrets = true
		} else if env[0] == 'n' || env[0] == '0' {
			config.UseSecrets = false
		}
	}

	env = os.Getenv("HEKETI_KUBE_TOKENFILE")
	if "" != env {
		config.TokenFile = env
	}

	env = os.Getenv("HEKETI_KUBE_NAMESPACEFILE")
	if "" != env {
		config.NamespaceFile = env
	}

	// Determine if Heketi should communicate with Gluster
	// pods deployed by a DaemonSet
	env = os.Getenv("HEKETI_KUBE_GLUSTER_DAEMONSET")
	if "" != env {
		env = strings.ToLower(env)
		if env[0] == 'y' || env[0] == '1' {
			config.GlusterDaemonSet = true
		} else if env[0] == 'n' || env[0] == '0' {
			config.GlusterDaemonSet = false
		}
	}

	// Use POD names
	env = os.Getenv("HEKETI_KUBE_USE_POD_NAMES")
	if "" != env {
		env = strings.ToLower(env)
		if env[0] == 'y' || env[0] == '1' {
			config.UsePodNames = true
		} else if env[0] == 'n' || env[0] == '0' {
			config.UsePodNames = false
		}
	}
}

func NewKubeExecutor(config *KubeConfig) (*KubeExecutor, error) {
	// Override configuration
	setWithEnvVariables(config)

	// Initialize
	k := &KubeExecutor{}
	k.config = config
	k.Throttlemap = make(map[string]chan bool)
	k.RemoteExecutor = k

	if k.config.Fstab == "" {
		k.Fstab = "/etc/fstab"
	} else {
		k.Fstab = config.Fstab
	}

	// Check required values
	if k.config.NamespaceFile != "" {
		var err error
		k.config.Namespace, err = k.readAllLinesFromFile(k.config.NamespaceFile)
		if err != nil {
			return nil, err
		}
	}
	if k.config.Namespace == "" {
		return nil, fmt.Errorf("Namespace must be provided in configuration")
	}

	// Show experimental settings
	if k.config.RebalanceOnExpansion {
		logger.Warning("Rebalance on volume expansion has been enabled.  This is an EXPERIMENTAL feature")
	}

	godbc.Ensure(k != nil)
	godbc.Ensure(k.Fstab != "")

	return k, nil
}

func (k *KubeExecutor) RemoteCommandExecute(host string,
	commands []string,
	timeoutMinutes int) ([]string, error) {

	// Throttle
	k.AccessConnection(host)
	defer k.FreeConnection(host)

	// Execute
	return k.ConnectAndExec(host,
		"pods",
		commands,
		timeoutMinutes)
}

func (k *KubeExecutor) ConnectAndExec(host, resource string,
	commands []string,
	timeoutMinutes int) ([]string, error) {

	// Used to return command output
	buffers := make([]string, len(commands))

	// Create a Kube client configuration
	clientConfig := &restclient.Config{}
	clientConfig.Host = k.config.Host
	clientConfig.CertFile = k.config.CertFile
	clientConfig.Insecure = k.config.Insecure

	// Login
	if k.config.UseSecrets == false &&
		k.config.User != "" &&
		k.config.Password != "" {

		token, err := tokenCreator(clientConfig,
			nil,
			k.config.User,
			k.config.Password)
		if err != nil {
			logger.Err(err)
			return nil, fmt.Errorf("User %v credentials not accepted", k.config.User)
		}
		clientConfig.BearerToken = token
	} else if k.config.UseSecrets {
		var err error
		clientConfig.BearerToken, err = k.readAllLinesFromFile(k.config.TokenFile)
		if err != nil {
			return nil, err
		}
	}

	// Get a client
	conn, err := client.New(clientConfig)
	if err != nil {
		logger.Err(err)
		return nil, fmt.Errorf("Unable to create a client connection")
	}

	// Get pod name
	var podName string
	if k.config.UsePodNames {
		podName = host
	} else if k.config.GlusterDaemonSet {
		podName, err = k.getPodNameFromDaemonSet(conn, host)
	} else {
		podName, err = k.getPodNameByLabel(conn, host)
	}
	if err != nil {
		return nil, err
	}

	for index, command := range commands {

		// Remove any whitespace
		command = strings.Trim(command, " ")

		// SUDO is *not* supported

		// Create REST command
		req := conn.RESTClient.Post().
			Resource(resource).
			Name(podName).
			Namespace(k.config.Namespace).
			SubResource("exec")
		req.VersionedParams(&api.PodExecOptions{
			Command: []string{"/bin/bash", "-c", command},
			Stdout:  true,
			Stderr:  true,
		}, api.ParameterCodec)

		// Create SPDY connection
		exec, err := remotecommand.NewExecutor(clientConfig, "POST", req.URL())
		if err != nil {
			logger.Err(err)
			return nil, fmt.Errorf("Unable to setup a session with %v", podName)
		}

		// Create a buffer to trap session output
		var b bytes.Buffer
		var berr bytes.Buffer

		// Excute command
		err = exec.Stream(remotecommand.StreamOptions{
			SupportedProtocols: kubeletcmd.SupportedStreamingProtocols,
			Stdout:             &b,
			Stderr:             &berr,
		})
		if err != nil {
			logger.LogError("Failed to run command [%v] on %v: Err[%v]: Stdout [%v]: Stderr [%v]",
				command, podName, err, b.String(), berr.String())
			return nil, fmt.Errorf("Unable to execute command on %v: %v", podName, berr.String())
		}
		logger.Debug("Host: %v Pod: %v Command: %v\nResult: %v", host, podName, command, b.String())
		buffers[index] = b.String()

	}

	return buffers, nil
}

func (k *KubeExecutor) RebalanceOnExpansion() bool {
	return k.config.RebalanceOnExpansion
}

func (k *KubeExecutor) SnapShotLimit() int {
	return k.config.SnapShotLimit
}

func (k *KubeExecutor) readAllLinesFromFile(filename string) (string, error) {
	fileBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", logger.LogError("Error reading %v file: %v", filename, err.Error())
	}
	return string(fileBytes), nil
}

func (k *KubeExecutor) getPodNameByLabel(conn *client.Client,
	host string) (string, error) {
	// 'host' is actually the value for the label with a key
	// of 'glusterid'
	selector, err := labels.Parse(KubeGlusterFSPodLabelKey + "==" + host)
	if err != nil {
		logger.Err(err)
		return "", logger.LogError("Unable to get pod with a matching label of %v==%v",
			KubeGlusterFSPodLabelKey, host)
	}

	// Get a list of pods
	pods, err := conn.Pods(k.config.Namespace).List(api.ListOptions{
		LabelSelector: selector,
		FieldSelector: fields.Everything(),
	})
	if err != nil {
		logger.Err(err)
		return "", fmt.Errorf("Failed to get list of pods")
	}

	numPods := len(pods.Items)
	if numPods == 0 {
		// No pods found with that label
		err := fmt.Errorf("No pods with the label '%v=%v' were found",
			KubeGlusterFSPodLabelKey, host)
		logger.Critical(err.Error())
		return "", err

	} else if numPods > 1 {
		// There are more than one pod with the same label
		err := fmt.Errorf("Found %v pods with the sharing the same label '%v=%v'",
			numPods, KubeGlusterFSPodLabelKey, host)
		logger.Critical(err.Error())
		return "", err
	}

	// Get pod name
	return pods.Items[0].ObjectMeta.Name, nil
}

func (k *KubeExecutor) getPodNameFromDaemonSet(conn *client.Client,
	host string) (string, error) {
	// 'host' is actually the value for the label with a key
	// of 'glusterid'
	selector, err := labels.Parse(KubeGlusterFSPodLabelKey)
	if err != nil {
		return "", logger.LogError("Unable to create selector of %v: %v",
			KubeGlusterFSPodLabelKey, err.Error())
	}

	// Get a list of pods
	pods, err := conn.Pods(k.config.Namespace).List(api.ListOptions{
		LabelSelector: selector,
		FieldSelector: fields.Everything(),
	})
	if err != nil {
		logger.Err(err)
		return "", logger.LogError("Failed to get list of pods")
	}

	// Go through the pods looking for the node
	var glusterPod string
	for _, pod := range pods.Items {
		if pod.Spec.NodeName == host {
			glusterPod = pod.ObjectMeta.Name
		}
	}
	if glusterPod == "" {
		return "", logger.LogError("Unable to find a GlusterFS pod on host %v "+
			"with a label key %v", host, KubeGlusterFSPodLabelKey)
	}

	// Get pod name
	return glusterPod, nil
}
