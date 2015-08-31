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
	"fmt"
	"github.com/heketi/heketi/utils/ssh"
	"github.com/lpabon/godbc"
)

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
	_, err := exec.ConnectAndExec(exec_host+":22", commands, 5)
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
	_, err := exec.ConnectAndExec(exec_host+":22", commands, 5)
	if err != nil {
		logger.Err(err)
	}
	return nil
}
