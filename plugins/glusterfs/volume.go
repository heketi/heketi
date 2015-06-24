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
	"github.com/heketi/heketi/requests"
	"github.com/heketi/heketi/utils"
	// goon "github.com/shurcooL/go-goon"
	"errors"
	"fmt"
	"github.com/heketi/heketi/utils/ssh"
	"github.com/lpabon/godbc"
	"sync"
)

type VolumeStateResponse struct {
	Bricks  []*Brick `json:"bricks"`
	Started bool     `json:"started"`
	Created bool     `json:"created"`
	Replica int      `json:"replica"`
}

type VolumeEntry struct {
	Info  requests.VolumeInfoResp
	State VolumeStateResponse

	//private
	db *GlusterFSDB
}

func NewVolumeEntry(v *requests.VolumeCreateRequest,
	bricks []*Brick,
	replica int,
	db *GlusterFSDB) *VolumeEntry {

	// Save volume information
	vol := &VolumeEntry{}
	vol.Info.Size = v.Size
	vol.Info.Id = utils.GenUUID()
	vol.State.Bricks = bricks
	vol.State.Replica = replica
	vol.db = db

	if v.Name != "" {
		vol.Info.Name = v.Name
	} else {
		vol.Info.Name = "vol_" + vol.Info.Id
	}

	return vol
}

func (v *VolumeEntry) Copy() *VolumeEntry {

	vc := &VolumeEntry{}
	*vc = *v
	return vc
}

func (v *VolumeEntry) Load(db *GlusterFSDB) {

	for brick := range v.State.Bricks {
		v.State.Bricks[brick].Load(db)
	}
	v.db = db

}

func (v *VolumeEntry) Destroy() error {
	godbc.Require(v.db != nil)

	sshexec := ssh.NewSshExecWithKeyFile("vagrant", "insecure_private_key")
	godbc.Check(sshexec != nil)

	// Get node name
	var nodename string
	err := v.db.Reader(func() error {
		nodename = v.db.nodes[v.State.Bricks[0].NodeId].Info.Name
		return nil
	})
	if err != nil {
		return err
	}

	// Shutdown volume
	commands := []string{
		// stop gluster volume
		fmt.Sprintf("yes | sudo gluster volume stop %v force", v.Info.Name),
		fmt.Sprintf("yes | sudo gluster volume delete %v", v.Info.Name),
	}

	_, err = sshexec.ConnectAndExec(nodename+":22", commands, nil)
	if err != nil {
		return errors.New("Unable to shutdown volume")
	}

	// Destroy bricks
	var wg sync.WaitGroup
	for brick := range v.State.Bricks {
		wg.Add(1)
		go func(b int) {
			defer wg.Done()
			v.State.Bricks[b].Destroy()
		}(brick)
	}
	wg.Wait()

	return nil
}

func (v *VolumeEntry) CreateGlusterVolume() error {

	// Get node name
	var nodename string
	var cmd string

	err := v.db.Reader(func() error {
		nodename = v.db.nodes[v.State.Bricks[0].NodeId].Info.Name

		cmd = fmt.Sprintf("sudo gluster volume create %v replica %v ",
			v.Info.Name, v.State.Replica)
		for brick := range v.State.Bricks {
			cmd += fmt.Sprintf("%v:/gluster/brick_%v/brick ",
				v.db.nodes[v.State.Bricks[brick].NodeId].Info.Name, v.State.Bricks[brick].Id)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Create gluster volume command

	// :TODO: Add force for now.  It will allow silly bricks on the same systems
	// to work.  Please remove once we add the intelligent ring
	cmd += " force"

	// Just for now, it will work wih https://github.com/lpabon/vagrant-gfsm
	sshexec := ssh.NewSshExecWithKeyFile("vagrant", "insecure_private_key")
	godbc.Check(sshexec != nil)

	// Create volume
	commands := []string{
		cmd,
		fmt.Sprintf("sudo gluster volume start %v", v.Info.Name),
	}

	_, err = sshexec.ConnectAndExec(nodename+":22", commands, nil)
	if err != nil {
		return err
	}

	// Setup mount point
	v.Info.Mount = fmt.Sprintf("%v:%v", nodename, v.Info.Name)

	// State
	v.State.Created = true
	v.State.Started = true

	return nil
}
