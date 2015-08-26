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
	"github.com/heketi/heketi/executors"
	"github.com/heketi/heketi/utils/ssh"
	"github.com/lpabon/godbc"
)

func (s *SshExecutor) VolumeCreate(host string,
	volume *executors.VolumeRequest) (*executors.VolumeInfo, error) {

	godbc.Require(volume != nil)
	godbc.Require(host != "")
	godbc.Require(len(volume.Bricks) > 0)
	godbc.Require(volume.Name != "")
	godbc.Require(volume.Replica > 1)

	// Setup ssh key
	exec := ssh.NewSshExecWithKeyFile(logger, s.user, s.private_keyfile)
	if exec == nil {
		return nil, ErrSshPrivateKey
	}

	// Setup volume create command
	// There could many, many bricks which could make the command line
	// too long.  Instead, create the volume first, then add each brick set.
	cmd := fmt.Sprintf("sudo gluster volume create %v replica %v ",
		volume.Name, volume.Replica)
	for _, brick := range volume.Bricks[:volume.Replica] {
		cmd += fmt.Sprintf("%v:%v ", brick.Host, brick.Path)
	}

	// :TODO: Add force for now.  It will allow silly bricks on the same systems
	// to work.  Please remove once we add the intelligent ring
	cmd += " force"

	// Initialize the commands with the create command
	commands := []string{cmd}

	// Now add all the commands to add the bricks
	commands = append(commands, s.createAddBrickCommands(volume, volume.Replica)...)

	// Add command to start the volume
	commands = append(commands, fmt.Sprintf("sudo gluster volume start %v", volume.Name))

	// Execute command
	_, err := exec.ConnectAndExec(host+":22", commands)
	if err != nil {
		return nil, err
	}

	return &executors.VolumeInfo{}, nil
}

func (s *SshExecutor) VolumeExpand(host string,
	volume *executors.VolumeRequest) (*executors.VolumeInfo, error) {

	godbc.Require(volume != nil)
	godbc.Require(host != "")
	godbc.Require(len(volume.Bricks) > 0)
	godbc.Require(volume.Name != "")

	// Setup ssh key
	exec := ssh.NewSshExecWithKeyFile(logger, s.user, s.private_keyfile)
	if exec == nil {
		return nil, ErrSshPrivateKey
	}

	// Setup volume create command
	commands := s.createAddBrickCommands(volume, 0 /* start at the beginning of the brick list */)
	commands = append(commands,
		fmt.Sprintf("sudo gluster volume rebalance %v start", volume.Name))

	// Execute command
	_, err := exec.ConnectAndExec(host+":22", commands)
	if err != nil {
		return nil, err
	}

	return &executors.VolumeInfo{}, nil
}

func (s *SshExecutor) VolumeDestroy(host string, volume string) error {
	godbc.Require(host != "")
	godbc.Require(volume != "")

	// Setup ssh key
	exec := ssh.NewSshExecWithKeyFile(logger, s.user, s.private_keyfile)
	if exec == nil {
		return ErrSshPrivateKey
	}

	// Shutdown volume
	commands := []string{
		// stop gluster volume
		fmt.Sprintf("yes | sudo gluster volume stop %v force", volume),
		fmt.Sprintf("yes | sudo gluster volume delete %v", volume),
	}

	// Execute command
	_, err := exec.ConnectAndExec(host+":22", commands)
	if err != nil {
		return err
	}

	return nil
}

func (s *SshExecutor) createAddBrickCommands(volume *executors.VolumeRequest, start int) []string {

	commands := []string{}
	var cmd string
	for index, brick := range volume.Bricks[start:] {
		if index%(volume.Replica*10) == 0 {
			if cmd != "" {
				commands = append(commands, cmd)
			}
			cmd = fmt.Sprintf("sudo gluster volume add-brick %v ", volume.Name)
		}
		cmd += fmt.Sprintf("%v:%v ", brick.Host, brick.Path)
	}
	commands = append(commands, cmd)

	return commands
}
