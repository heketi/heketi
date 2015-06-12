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
	"github.com/lpabon/godbc"
	"github.com/lpabon/heketi/requests"
	"github.com/lpabon/heketi/utils"
	//goon "github.com/shurcooL/go-goon"
)

func createBricks(num_bricks int, size uint64, replicas int) ([]*Brick, error) {
	bricks := make([]*Brick, 0)
	for i := 0; i < replicas*num_bricks; i++ {
		brick := NewBrick(size)
		err := brick.Create()
		if err != nil {
			return nil, err
		}
		bricks = append(bricks, brick)
	}

	return bricks, nil
}

type VolumeDB struct {
	Info    requests.VolumeInfoResp
	Bricks  []*Brick
	Started bool
	Created bool
	Replica int
}

func NewVolumeDB(v *requests.VolumeCreateRequest) *VolumeDB {

	var err error

	// Save volume information
	vol := &VolumeDB{}
	vol.Replica = v.Replica
	vol.Info.Name = v.Name
	vol.Info.Size = v.Size
	vol.Info.Id, err = utils.GenUUID()
	godbc.Check(err != nil)

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

func (v *VolumeDB) Create() error {
	// Get number of bricks we need to satisfy this request
	bricks_num, brick_size, err := v.numBricksNeeded()
	if err != nil {
		return err
	}

	// Get the nodes and storage for these bricks
	// and Create the bricks
	replica := v.Replica
	if v.Replica == 0 {
		replica = 2
	}

	v.Bricks, err = createBricks(bricks_num, brick_size, replica)
	if err != nil {
		return err
	}

	// Setup the glusterfs volume
	err = v.createGlusterVolume()
	if err != nil {
		v.Destroy()
		return err
	}

	return nil

}

// return numbricks, size of each brick, error
func (v *VolumeDB) numBricksNeeded() (int, uint64, error) {
	return 2, v.Info.Size / 2, nil
}

func (v *VolumeDB) createGlusterVolume() error {
	return nil
}
