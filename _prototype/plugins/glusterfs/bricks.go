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
	"fmt"
	"github.com/heketi/heketi/utils"
	"github.com/heketi/heketi/utils/ssh"
	"github.com/lpabon/godbc"
)

const (

	// :TODO: This should be saved on the brick object so that on upgrades
	// or changes it still has the correct older value
	THINP_SNAPSHOT_FACTOR = 1.25
)

type Brick struct {
	Id       string `json:"id"`
	Path     string `json:"path"`
	NodeId   string `json:"node_id"`
	DeviceId string `json:"device_id"`
	Size     uint64 `json:"size"`

	// private
	db *GlusterFSDB
}

func NewBrick(size uint64, db *GlusterFSDB) *Brick {
	return &Brick{
		Id:   utils.GenUUID(),
		Size: size,
		db:   db,
	}
}

func (b *Brick) Load(db *GlusterFSDB) {
	b.db = db
}

func (b *Brick) Create() error {
	godbc.Require(b.db != nil)
	godbc.Require(b.DeviceId != "")

	// Just for now, it will work wih https://github.com/lpabon/vagrant-gfsm
	sshexec := ssh.NewSshExecWithKeyFile("vagrant", "insecure_private_key")
	godbc.Check(sshexec != nil)

	var nodename string
	err := b.db.Reader(func() error {
		nodename = b.db.nodes[b.NodeId].Info.Name
		return nil
	})

	commands := []string{
		fmt.Sprintf("sudo lvcreate -L %vKiB -T vg_%v/tp_%v -V %vKiB -n brick_%v",
			//Thin Pool Size
			uint64(float64(b.Size)*THINP_SNAPSHOT_FACTOR),

			// volume group
			b.DeviceId,

			// ThinP name
			b.Id,

			// Volume size
			b.Size,

			// Logical Vol name
			b.Id),
		fmt.Sprintf("sudo mkfs.xfs -i size=512 /dev/vg_%v/brick_%v", b.DeviceId, b.Id),
		fmt.Sprintf("sudo mkdir /gluster/brick_%v", b.Id),
		fmt.Sprintf("sudo mount /dev/vg_%v/brick_%v /gluster/brick_%v",
			b.DeviceId, b.Id, b.Id),
		fmt.Sprintf("sudo mkdir /gluster/brick_%v/brick", b.Id),
	}

	_, err = sshexec.ConnectAndExec(nodename+":22", commands, nil)
	if err != nil {
		return err
	}

	// SSH into node and create brick
	b.Path = fmt.Sprintf("/gluster/brick_%v", b.Id)
	return nil
}

func (b *Brick) Destroy() error {
	godbc.Require(b.NodeId != "")
	godbc.Require(b.Path != "")
	godbc.Require(b.db != nil)

	// Just for now, it will work wih https://github.com/lpabon/vagrant-gfsm
	sshexec := ssh.NewSshExecWithKeyFile("vagrant", "insecure_private_key")
	godbc.Check(sshexec != nil)

	// Get node name
	var nodename string
	err := b.db.Reader(func() error {
		nodename = b.db.nodes[b.NodeId].Info.Name
		return nil
	})

	// Delete brick storage
	commands := []string{
		fmt.Sprintf("sudo umount /gluster/brick_%v", b.Id),
		fmt.Sprintf("sudo lvremove -f vg_%v/tp_%v", b.DeviceId, b.Id),
		fmt.Sprintf("sudo rmdir /gluster/brick_%v", b.Id),
	}

	_, err = sshexec.ConnectAndExec(nodename+":22", commands, nil)
	if err != nil {
		return err
	}

	err = b.FreeStorage()

	return err
}

func (b *Brick) FreeStorage() error {
	// Add storage back
	return b.db.Writer(func() error {
		tpsize := uint64(float64(b.Size) * THINP_SNAPSHOT_FACTOR)
		b.db.nodes[b.NodeId].Info.Devices[b.DeviceId].Used -= tpsize
		b.db.nodes[b.NodeId].Info.Devices[b.DeviceId].Free += tpsize

		b.db.nodes[b.NodeId].Info.Storage.Used -= tpsize
		b.db.nodes[b.NodeId].Info.Storage.Free += tpsize

		return nil
	})
}

func (b *Brick) AllocateStorage(nodeid, deviceid string) error {
	// Add storage back
	b.NodeId = nodeid
	b.DeviceId = deviceid
	return b.db.Writer(func() error {
		tpsize := uint64(float64(b.Size) * THINP_SNAPSHOT_FACTOR)
		b.db.nodes[b.NodeId].Info.Devices[b.DeviceId].Used += tpsize
		b.db.nodes[b.NodeId].Info.Devices[b.DeviceId].Free -= tpsize

		b.db.nodes[b.NodeId].Info.Storage.Used += tpsize
		b.db.nodes[b.NodeId].Info.Storage.Free -= tpsize

		return nil
	})
}
