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
	"github.com/heketi/heketi/executors/sshexec"
)

type KubeConfig struct {
	sshexec.CLICommandConfig
	Host      string `json:"host"`
	CertFile  string `json:"cert"`
	Insecure  bool   `json:"insecure"`
	User      string `json:"user"`
	Password  string `json:"password"`
	Token     string `json:"token"`
	Namespace string `json:"namespace"`
	// Use Secrets to get the Bearerkey
	UseSecrets bool `json:"use_secrets"`

	TokenFile     string `json:"token_file"`
	NamespaceFile string `json:"namespace_file"`

	// Use POD name instead of using label
	// to access POD
	UsePodNames bool `json:"use_pod_names"`
}
