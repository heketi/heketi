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
	"github.com/lpabon/heketi/requests"
	"github.com/lpabon/heketi/utils"
	//goon "github.com/shurcooL/go-goon"
)

type VolumeStateResponse struct {
	Bricks  []*Brick `json:"bricks"`
	Started bool     `json:"started"`
	Created bool     `json:"created"`
	Replica int      `json:"replica"`
}

type VolumeDB struct {
	Info  requests.VolumeInfoResp
	State VolumeStateResponse
}

func NewVolumeDB(v *requests.VolumeCreateRequest, bricks []*Brick, replica int) *VolumeDB {

	// Save volume information
	vol := &VolumeDB{}
	vol.Info.Name = v.Name
	vol.Info.Size = v.Size
	vol.Info.Id = utils.GenUUID()
	vol.State.Bricks = bricks
	vol.State.Replica = replica

	return vol
}

func (v *VolumeDB) Destroy() error {

	// Stop glusterfs volume

	// Destroy glusterfs volume

	// Destroy bricks
	for brick := range v.State.Bricks {
		// :TODO: Log the eror
		v.State.Bricks[brick].Destroy()
	}

	return nil
}

func (v *VolumeDB) InfoResponse() *requests.VolumeInfoResp {
	info := &requests.VolumeInfoResp{}
	*info = v.Info
	info.Plugin = v.State
	return info
}

func (v *VolumeDB) CreateGlusterVolume() error {
	v.State.Created = true
	v.State.Started = true
	return nil
}
