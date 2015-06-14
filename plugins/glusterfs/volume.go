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

type VolumeDB struct {
	Info    requests.VolumeInfoResp
	Bricks  []*Brick
	Started bool
	Created bool
	Replica int
}

func NewVolumeDB(v *requests.VolumeCreateRequest) *VolumeDB {

	// Save volume information
	vol := &VolumeDB{}
	vol.Replica = v.Replica
	vol.Info.Name = v.Name
	vol.Info.Size = v.Size
	vol.Info.Id = utils.GenUUID()

	return vol
}

func (v *VolumeDB) Destroy() error {

	// Stop glusterfs volume

	// Destroy glusterfs volume

	// Destroy bricks
	for brick := range v.Bricks {
		// :TODO: Log the eror
		v.Bricks[brick].Destroy()
	}

	return nil
}

func (v *VolumeDB) CreateGlusterVolume() error {
	return nil
}
