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

func (s *SshExecutor) BrickCreate(host string,
	brick *executors.BrickRequest) (*executors.BrickInfo, error) {

	godbc.Require(brick != nil)
	godbc.Require(host != "")
	godbc.Require(brick.Name != "")
	godbc.Require(brick.Size > 0)
	godbc.Require(brick.TpSize >= brick.Size)
	godbc.Require(brick.VgId != "")

	exec := ssh.NewSshExecWithKeyFile(logger, s.user, s.private_keyfile)
	if exec == nil {
		return nil, ErrSshPrivateKey
	}

	logger.Info("Creating brick on host %v", host)
	commands := []string{

		// Create a directory
		fmt.Sprintf("sudo mkdir /brick_%v", brick.Name),

		// Setup the LV
		fmt.Sprintf("sudo lvcreate -L %vKiB -T vg_%v/tp_%v -V %vKiB -n brick_%v",
			//Thin Pool Size
			brick.TpSize,

			// volume group
			brick.VgId,

			// ThinP name
			brick.Name,

			// Allocation size
			brick.Size,

			// Logical Vol name
			brick.Name),

		// Format
		fmt.Sprintf("sudo mkfs.xfs -i size=512 /dev/vg_%v/brick_%v", brick.VgId, brick.Name),

		// Mount
		fmt.Sprintf("sudo mount /dev/vg_%v/brick_%v /brick_%v",
			brick.VgId, brick.Name, brick.Name),

		// Create a directory inside the formated volume for GlusterFS
		fmt.Sprintf("sudo mkdir /brick_%v/brick", brick.Name),
	}

	// Execute commands
	_, err := exec.ConnectAndExec(host+":22", commands)
	if err != nil {
		return nil, err
	}

	// :TODO: Add to fstab!

	// Save brick location
	b := &executors.BrickInfo{
		Path: fmt.Sprintf("/brick_%v/brick", brick.Name),
	}
	return b, nil
}

func (s *SshExecutor) BrickDestroy(host string,
	brick *executors.BrickRequest) error {

	godbc.Require(brick != nil)
	godbc.Require(host != "")
	godbc.Require(brick.Name != "")
	godbc.Require(brick.VgId != "")

	// Setup ssh session
	exec := ssh.NewSshExecWithKeyFile(logger, s.user, s.private_keyfile)
	if exec == nil {
		return ErrSshPrivateKey
	}

	// Setup commands to delete brick
	commands := []string{
		fmt.Sprintf("sudo umount /brick_%v", brick.Name),
		fmt.Sprintf("sudo lvremove -f vg_%v/tp_%v", brick.VgId, brick.Name),
		fmt.Sprintf("sudo rmdir /brick_%v", brick.Name),
	}

	// Execute commands
	_, err := exec.ConnectAndExec(host+":22", commands)
	if err != nil {
		return err
	}

	// :TODO: Remove from fstab

	return nil
}
