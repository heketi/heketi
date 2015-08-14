//
// Copyright (c) 2015 The heketi Authors
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

package sshexec

import (
	"errors"
	"fmt"
	"github.com/heketi/heketi/utils"
	"github.com/heketi/heketi/utils/ssh"
	"github.com/lpabon/godbc"
	"os"
)

type SshExecutor struct {
	private_keyfile string
	user            string
}

type SshConfig struct {
	PrivateKeyFile string `json:"keyfile"`
	User           string `json:"user"`
}

var (
	logger           = utils.NewLogger("[sshexec]", utils.LEVEL_DEBUG)
	ErrSshPrivateKey = errors.New("Unable to read private key file")
	//ErrSshConnectionRefused = errors.New("Unable to ssh to destination")
)

func NewSshExecutor(config *SshConfig) *SshExecutor {
	godbc.Require(config != nil)

	s := &SshExecutor{}

	// Set configuration
	if config.PrivateKeyFile == "" {
		s.private_keyfile = os.Getenv("HOME") + "/.ssh/id_rsa"
	} else {
		s.private_keyfile = config.PrivateKeyFile
	}

	if config.User == "" {
		s.user = "heketi"
	} else {
		s.user = config.User
	}

	godbc.Ensure(s != nil)
	godbc.Ensure(s.user != "")
	godbc.Ensure(s.private_keyfile != "")

	return s
}

func (s *SshExecutor) PeerProbe(exec_host, newnode string) error {

	godbc.Require(exec_host != "")
	godbc.Require(newnode != "")

	exec := ssh.NewSshExecWithKeyFile(logger, s.user, s.private_keyfile)
	if exec == nil {
		return ErrSshPrivateKey
	}

	logger.Info("Probing: %v -> %v", exec_host, newnode)
	// create the commands
	commands := []string{
		fmt.Sprintf("sudo gluster peer probe %v", newnode),
	}
	_, err := exec.ConnectAndExec(exec_host+":22", commands)
	if err != nil {
		return err
	}

	logger.Info("Probing: %v -> %v", newnode, exec_host)
	// --- Now we need to probe in the other direction.
	commands = []string{
		fmt.Sprintf("sudo gluster peer probe %v", exec_host),
	}
	_, err = exec.ConnectAndExec(newnode+":22", commands)
	if err != nil {
		return err
	}

	return nil
}

func (s *SshExecutor) PeerDetach(exec_host, detachnode string) error {
	godbc.Require(exec_host != "")
	godbc.Require(detachnode != "")

	exec := ssh.NewSshExecWithKeyFile(logger, s.user, s.private_keyfile)
	if exec == nil {
		return ErrSshPrivateKey
	}

	// create the commands
	logger.Info("Detaching node %v", detachnode)
	commands := []string{
		fmt.Sprintf("sudo gluster peer detach %v", detachnode),
	}
	_, err := exec.ConnectAndExec(exec_host+":22", commands)
	if err != nil {
		return err
	}
	return nil
}
