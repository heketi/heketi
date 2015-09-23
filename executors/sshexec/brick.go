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
	"github.com/lpabon/godbc"
)

const (
	rootMountPoint = "/var/lib/heketi/mounts"
)

// Return the mount point for the brick
func (s *SshExecutor) brickMountPoint(brick *executors.BrickRequest) string {
	return rootMountPoint + "/" +
		s.vgName(brick.VgId) + "/" +
		s.brickName(brick.Name)
}

// Device node for the lvm volume
func (s *SshExecutor) devnode(brick *executors.BrickRequest) string {
	return "/dev/" + s.vgName(brick.VgId) +
		"/" + s.brickName(brick.Name)
}

func (s *SshExecutor) BrickCreate(host string,
	brick *executors.BrickRequest) (*executors.BrickInfo, error) {

	godbc.Require(brick != nil)
	godbc.Require(host != "")
	godbc.Require(brick.Name != "")
	godbc.Require(brick.Size > 0)
	godbc.Require(brick.TpSize >= brick.Size)
	godbc.Require(brick.VgId != "")

	// Create mountpoint name
	mountpoint := s.brickMountPoint(brick)

	// Create command set to execute on the node
	commands := []string{

		// Create a directory
		fmt.Sprintf("sudo mkdir -p %v", mountpoint),

		// Setup the LV
		fmt.Sprintf("sudo lvcreate --poolmetadatasize %vK -c 256K -L %vK -T %v/%v -V %vK -n %v",
			// MetadataSize
			brick.PoolMetadataSize,

			//Thin Pool Size
			brick.TpSize,

			// volume group
			s.vgName(brick.VgId),

			// ThinP name
			s.tpName(brick.Name),

			// Allocation size
			brick.Size,

			// Logical Vol name
			s.brickName(brick.Name)),

		// Format
		fmt.Sprintf("sudo mkfs.xfs -i size=512 -n size=8192 %v", s.devnode(brick)),

		// Fstab
		fmt.Sprintf("echo \"%v %v xfs rw,inode64,noatime,nouuid 1 2\" | sudo tee -a /etc/fstab > /dev/null ",
			s.devnode(brick),
			mountpoint),

		// Mount
		fmt.Sprintf("sudo mount %v %v", s.devnode(brick), mountpoint),

		// Create a directory inside the formated volume for GlusterFS
		fmt.Sprintf("sudo mkdir %v/brick", mountpoint),
	}

	// Execute commands
	_, err := s.sshExec(host, commands, 10)
	if err != nil {
		// Cleanup
		s.BrickDestroy(host, brick)
		return nil, err
	}

	// Save brick location
	b := &executors.BrickInfo{
		Path: fmt.Sprintf("%v/brick", mountpoint),
	}
	return b, nil
}

func (s *SshExecutor) BrickDestroy(host string,
	brick *executors.BrickRequest) error {

	godbc.Require(brick != nil)
	godbc.Require(host != "")
	godbc.Require(brick.Name != "")
	godbc.Require(brick.VgId != "")

	// Try to unmount first
	commands := []string{
		fmt.Sprintf("sudo umount %v", s.brickMountPoint(brick)),
	}
	_, err := s.sshExec(host, commands, 5)
	if err != nil {
		logger.Err(err)
	}

	// Now try to remove the LV
	commands = []string{
		fmt.Sprintf("sudo lvremove -f %v/%v", s.vgName(brick.VgId), s.tpName(brick.Name)),
	}
	_, err = s.sshExec(host, commands, 5)
	if err != nil {
		logger.Err(err)
	}

	// Now cleanup the mount point
	commands = []string{
		fmt.Sprintf("sudo rmdir %v", s.brickMountPoint(brick)),
	}
	_, err = s.sshExec(host, commands, 5)
	if err != nil {
		logger.Err(err)
	}

	// Remove from fstab
	commands = []string{
		fmt.Sprintf("sudo sed -i.save '/%v/d' /etc/fstab", s.brickName(brick.Name)),
	}
	_, err = s.sshExec(host, commands, 5)
	if err != nil {
		logger.Err(err)
	}

	return nil
}
