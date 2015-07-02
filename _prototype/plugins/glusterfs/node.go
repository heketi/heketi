//
// Copyright (c) 2014 The heketi Authors
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

package glusterfs

import (
	"errors"
	"fmt"
	"github.com/heketi/heketi/requests"
	"github.com/heketi/heketi/utils"
	"github.com/heketi/heketi/utils/ssh"
	"github.com/lpabon/godbc"
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

type NodeEntry struct {
	Info requests.NodeInfoResp

	// private
	db *GlusterFSDB
}

func NewNodeEntry(v *requests.NodeAddRequest, db *GlusterFSDB) *NodeEntry {

	node := &NodeEntry{}
	node.Info.Id = utils.GenUUID()
	node.Info.Name = v.Name
	node.Info.Zone = v.Zone
	node.Info.Devices = make(map[string]*requests.DeviceResponse)

	return node
}

func (n *NodeEntry) Copy() *NodeEntry {
	nc := &NodeEntry{}
	*nc = *n
	return nc
}

func (n *NodeEntry) Load(db *GlusterFSDB) {
	n.db = db
}

func (n *NodeEntry) Device(id string) *requests.DeviceResponse {
	return n.Info.Devices[id]
}

func (n *NodeEntry) DeviceAdd(req *requests.DeviceRequest) error {
	// Setup device object
	dev := &requests.DeviceResponse{}
	dev.Name = req.Name
	dev.Weight = req.Weight
	dev.Id = utils.GenUUID()

	// Setup --

	// Just for now, it will work wih https://github.com/lpabon/vagrant-gfsm
	sshexec := ssh.NewSshExecWithKeyFile("vagrant", "insecure_private_key")
	godbc.Check(sshexec != nil)

	commands := []string{
		fmt.Sprintf("sudo pvcreate %v", dev.Name),
		fmt.Sprintf("sudo vgcreate vg_%v %v", dev.Id, dev.Name),
	}

	_, err := sshexec.ConnectAndExec(n.Info.Name+":22", commands, nil)
	if err != nil {
		return err
	}

	// Vg info
	err = n.getVgSizeFromNode(dev)
	if err != nil {
		return err
	}

	// Add to db
	n.Info.Devices[dev.Id] = dev

	return nil
}

func (n *NodeEntry) getVgSizeFromNode(device *requests.DeviceResponse) error {

	// Just for now, it will work wih https://github.com/lpabon/vagrant-gfsm
	sshexec := ssh.NewSshExecWithKeyFile("vagrant", "insecure_private_key")
	godbc.Check(sshexec != nil)

	commands := []string{
		fmt.Sprintf("sudo vgdisplay -c vg_%v", device.Id),
	}

	b, err := sshexec.ConnectAndExec(n.Info.Name+":22", commands, nil)
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

	alloc_extents, err :=
		strconv.ParseUint(vginfo[VGDISPLAY_ALLOCATED_NUMBER_EXTENTS], 10, 64)
	if err != nil {
		return err
	}

	device.Free = free_extents * extent_size
	device.Used = alloc_extents * extent_size
	device.Total, err = strconv.ParseUint(vginfo[VGDISPLAY_SIZE_KB], 10, 64)
	if err != nil {
		return err
	}

	n.Info.Storage.Free += device.Free
	n.Info.Storage.Used += device.Used
	n.Info.Storage.Total += device.Total

	return nil
}
