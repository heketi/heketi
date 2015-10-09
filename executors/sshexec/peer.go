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
	"github.com/lpabon/godbc"
)

func (s *SshExecutor) PeerProbe(host, newnode string) error {

	godbc.Require(host != "")
	godbc.Require(newnode != "")

	logger.Info("Probing: %v -> %v", host, newnode)
	// create the commands
	commands := []string{
		fmt.Sprintf("sudo gluster peer probe %v", newnode),
	}
	_, err := s.sshExec(host, commands, 10)
	if err != nil {
		return err
	}

	return nil
}

func (s *SshExecutor) PeerDetach(host, detachnode string) error {
	godbc.Require(host != "")
	godbc.Require(detachnode != "")

	// create the commands
	logger.Info("Detaching node %v", detachnode)
	commands := []string{
		fmt.Sprintf("sudo gluster peer detach %v", detachnode),
	}
	_, err := s.sshExec(host, commands, 10)
	if err != nil {
		logger.Err(err)
	}

	return nil
}
