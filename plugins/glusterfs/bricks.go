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
	THINP_SNAPSHOT_FACTOR = 1.25
)

type Brick struct {
	Id       string `json:"id"`
	Path     string `json:"path"`
	NodeId   string `json:"node_id"`
	DeviceId string `json:"device_id"`
	Size     uint64 `json:"size"`

	// private
	nodedb *NodeEntry
}

func NewBrick(size uint64) *Brick {
	return &Brick{
		Id:   utils.GenUUID(),
		Size: size,
	}
}

func (b *Brick) Load(db *GlusterFSDB) {
	b.nodedb = db.nodes[b.NodeId]
}

func (b *Brick) Create() error {
	godbc.Require(b.nodedb != nil)
	godbc.Require(b.DeviceId != "")

	// Just for now, it will work wih https://github.com/lpabon/vagrant-gfsm
	sshexec := ssh.NewSshExecWithKeyFile("vagrant", "insecure_private_key")
	godbc.Check(sshexec != nil)

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

	_, err := sshexec.ConnectAndExec(b.nodedb.Info.Name+":22", commands, nil)
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

	// Just for now, it will work wih https://github.com/lpabon/vagrant-gfsm
	sshexec := ssh.NewSshExecWithKeyFile("vagrant", "insecure_private_key")
	godbc.Check(sshexec != nil)

	commands := []string{
		fmt.Sprintf("sudo umount /gluster/brick_%v", b.Id),
		fmt.Sprintf("sudo lvremove -f vg_%v/tp_%v", b.DeviceId, b.Id),
		fmt.Sprintf("sudo rmdir /gluster/brick_%v", b.Id),
	}

	_, err := sshexec.ConnectAndExec(b.nodedb.Info.Name+":22", commands, nil)
	if err != nil {
		return err
	}

	// SSH into node and create brick
	return nil

}
