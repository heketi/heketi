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
	"github.com/heketi/heketi/executors"
	"github.com/heketi/heketi/utils/ssh"
	"strconv"
	"strings"
)

const (
	VGDISPLAY_SIZE_KB                  = 11
	VGDISPLAY_PHYSICAL_EXTENT_SIZE     = 12
	VGDISPLAY_TOTAL_NUMBER_EXTENTS     = 13
	VGDISPLAY_ALLOCATED_NUMBER_EXTENTS = 14
	VGDISPLAY_FREE_NUMBER_EXTENTS      = 15
)

func (s *SshExecutor) DeviceSetup(host, device, vgid string) (d *executors.DeviceInfo, e error) {

	// Setup ssh session
	exec := ssh.NewSshExecWithKeyFile(logger, s.user, s.private_keyfile)
	if exec == nil {
		return nil, ErrSshPrivateKey
	}

	// Setup commands
	commands := []string{
		fmt.Sprintf("sudo pvcreate %v", device),
		fmt.Sprintf("sudo vgcreate vg_%v %v", vgid, device),
	}

	// Execute command
	_, err := exec.ConnectAndExec(host+":22", commands, 5)
	if err != nil {
		return nil, err
	}

	// Create a cleanup function if anything fails
	defer func() {
		if e != nil {
			s.DeviceTeardown(host, device, vgid)
		}
	}()

	// Vg info
	d = &executors.DeviceInfo{}
	err = s.getVgSizeFromNode(d, host, device, vgid)
	if err != nil {
		return nil, err
	}

	return d, nil
}

func (s *SshExecutor) DeviceTeardown(host, device, vgid string) error {
	// Setup ssh session
	exec := ssh.NewSshExecWithKeyFile(logger, s.user, s.private_keyfile)
	if exec == nil {
		return ErrSshPrivateKey
	}

	// Setup commands
	commands := []string{
		fmt.Sprintf("sudo vgremove vg_%v", vgid),
		fmt.Sprintf("sudo pvremove %v", device),
	}

	// Execute command
	_, err := exec.ConnectAndExec(host+":22", commands, 5)
	if err != nil {
		return err
	}

	return nil
}

func (s *SshExecutor) getVgSizeFromNode(
	d *executors.DeviceInfo,
	host, device, vgid string) error {

	// Setup ssh session
	exec := ssh.NewSshExecWithKeyFile(logger, s.user, s.private_keyfile)
	if exec == nil {
		return ErrSshPrivateKey
	}

	// Setup command
	commands := []string{
		fmt.Sprintf("sudo vgdisplay -c vg_%v", vgid),
	}

	// Execute command
	b, err := exec.ConnectAndExec(host+":22", commands, 5)
	if err != nil {
		return err
	}

	// Example:
	// gfsm:r/w:772:-1:0:0:0:-1:0:4:4:2097135616:4096:511996:0:511996:rJ0bIG-3XNc-NoS0-fkKm-batK-dFyX-xbxHym
	vginfo := strings.Split(b[0], ":")

	// See vgdisplay manpage
	if len(vginfo) < 17 {
		return errors.New("vgdisplay returned an invalid string")
	}

	extent_size, err :=
		strconv.ParseUint(vginfo[VGDISPLAY_PHYSICAL_EXTENT_SIZE], 10, 64)
	if err != nil {
		return err
	}

	free_extents, err :=
		strconv.ParseUint(vginfo[VGDISPLAY_FREE_NUMBER_EXTENTS], 10, 64)
	if err != nil {
		return err
	}

	d.Size = free_extents * extent_size
	logger.Debug("Size of %v in %v is %v", device, host, d.Size)
	return nil
}
