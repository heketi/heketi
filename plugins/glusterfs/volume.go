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
}

func NewVolumeEntry(v *requests.VolumeCreateRequest, bricks []*Brick, replica int) *VolumeEntry {

	// Save volume information
	vol := &VolumeEntry{}
	vol.Info.Size = v.Size
	vol.Info.Id = utils.GenUUID()
	vol.State.Bricks = bricks
	vol.State.Replica = replica

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

}

func (v *VolumeEntry) Destroy() error {
	sshexec := ssh.NewSshExecWithKeyFile("vagrant", "insecure_private_key")
	godbc.Check(sshexec != nil)

	commands := []string{
		// stop gluster volume
		fmt.Sprintf("yes | sudo gluster volume stop %v force", v.Info.Name),
		fmt.Sprintf("yes | sudo gluster volume delete %v", v.Info.Name),
	}

	_, err := sshexec.ConnectAndExec(v.State.Bricks[0].nodedb.Info.Name+":22", commands, nil)
	if err != nil {
		return err
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

	// Update storage status
	tpsize := uint64(float64(v.State.Bricks[0].Size) * THINP_SNAPSHOT_FACTOR)
	for brick := range v.State.Bricks {
		v.State.Bricks[brick].nodedb.Info.Devices[v.State.Bricks[brick].DeviceId].Used -= tpsize
		v.State.Bricks[brick].nodedb.Info.Devices[v.State.Bricks[brick].DeviceId].Free += tpsize

		v.State.Bricks[brick].nodedb.Info.Storage.Used -= tpsize
		v.State.Bricks[brick].nodedb.Info.Storage.Free += tpsize
	}

	return nil
}

func (v *VolumeEntry) InfoResponse() *requests.VolumeInfoResp {
	return &v.Copy().Info
}

func (v *VolumeEntry) CreateGlusterVolume() error {

	// Create gluster volume
	cmd := fmt.Sprintf("sudo gluster volume create %v replica %v ",
		v.Info.Name, v.State.Replica)
	for brick := range v.State.Bricks {
		cmd += fmt.Sprintf("%v:/gluster/brick_%v/brick ",
			v.State.Bricks[brick].nodedb.Info.Name, v.State.Bricks[brick].Id)
	}

	// :TODO: Add force for now.  It will allow silly bricks on the same systems
	// to work.  Please remove once we add the intelligent ring
	cmd += " force"

	// Just for now, it will work wih https://github.com/lpabon/vagrant-gfsm
	sshexec := ssh.NewSshExecWithKeyFile("vagrant", "insecure_private_key")
	godbc.Check(sshexec != nil)

	commands := []string{
		cmd,
		fmt.Sprintf("sudo gluster volume start %v", v.Info.Name),
	}

	_, err := sshexec.ConnectAndExec(v.State.Bricks[0].nodedb.Info.Name+":22", commands, nil)
	if err != nil {
		return err
	}

	// Setup mount point
	v.Info.Mount = fmt.Sprintf("%v:%v", v.State.Bricks[0].nodedb.Info.Name, v.Info.Name)

	// State
	v.State.Created = true
	v.State.Started = true

	return nil
}
