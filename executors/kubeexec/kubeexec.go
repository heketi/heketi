//
// Copyright (c) 2016 The heketi Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package kubeexec

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/restclient"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/remotecommand"

	"github.com/lpabon/godbc"

	"github.com/heketi/heketi/executors/sshexec"
	"github.com/heketi/utils"
)

type KubernetesClient interface {
}

type KubernetesRemoteCommand interface {
}

type KubernetesRemoteCommandStream interface {
}

type KubeConfig struct {
	Host      string `json:"host"`
	Sudo      bool   `json:"sudo"`
	CertFile  string `json:"cert"`
	Insecure  bool   `json:"insecure"`
	User      string `json:"user"`
	Password  string `json:"password"`
	Namespace string `json:"namespace"`
	Fstab     string `json:"fstab"`
}

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
	if k.config.Namespace == "" {
		return nil, fmt.Errorf("Namespace must be provided in configuration")
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
		k.config.Namespace,
		"pods",
		commands,
		timeoutMinutes)
}

func (k *KubeExecutor) ConnectAndExec(host, namespace, resource string,
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
	token, err := tokenCreator(clientConfig,
		nil,
		k.config.User,
		k.config.Password)
	if err != nil {
		logger.Err(err)
		return nil, fmt.Errorf("User %v credentials not accepted", k.config.User)
	}
	clientConfig.BearerToken = token

	// Get a client
	conn, err := client.New(clientConfig)
	if err != nil {
		logger.Err(err)
		return nil, fmt.Errorf("Unable to create a client connection")
	}

	for index, command := range commands {

		// Remove any whitespace
		command = strings.Trim(command, " ")

		// Determine if we should use sudo
		if k.config.Sudo {
			command = "sudo " + command
		}

		// Create REST command
		req := conn.RESTClient.Post().
			Resource(resource).
			Name(host).
			Namespace(namespace).
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
			return nil, fmt.Errorf("Unable to setup a session with %v", host)
		}

		// Create a buffer to trap session output
		var b bytes.Buffer
		var berr bytes.Buffer

		// Excute command
		err = exec.Stream(nil, nil, &b, &berr, false)
		if err != nil {
			logger.LogError("Failed to run command [%v] on %v: Err[%v]: Stdout [%v]: Stderr [%v]",
				command, host, err, b.String(), berr.String())
			return nil, fmt.Errorf("Unable to execute command on %v", host)
		}
		logger.Debug("Host: %v Command: %v\nResult: %v", host, command, b.String())
		buffers[index] = b.String()

	}

	return buffers, nil
}
