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
	"github.com/lpabon/heketi/requests"
	//goon "github.com/shurcooL/go-goon"
)

type BrickNode struct {
	node, device string
}

type BrickNodes []BrickNode

// return numbricks, size of each brick, error
func (m *GlusterFSPlugin) numBricksNeeded(size uint64) (int, uint64, error) {
	return 2, size / 2, nil
}

func (m *GlusterFSPlugin) getBrickNodes(brick *Brick, replicas int) BrickNodes {
	// Get info from swift ring

	// Check it has enough space, if not .. go to next device

	nodelist := make(BrickNodes, 0)

	for nodeid, node := range m.db.nodes {
		for deviceid, _ := range node.Info.Devices {
			replicas -= 1
			nodelist = append(nodelist, BrickNode{device: deviceid, node: nodeid})
			if replicas == 0 {
				return nodelist
			}
		}
	}

	return nodelist
}

func (m *GlusterFSPlugin) allocBricks(num_bricks, replicas int, size uint64) ([]*Brick, error) {

	bricks := make([]*Brick, 0)

	for brick_num := 0; brick_num < num_bricks; brick_num++ {

		var brick *Brick

		brick = NewBrick(size)
		nodelist := m.getBrickNodes(brick, replicas)
		for i := 0; i < replicas; i++ {

			// XXX This is bad, but ok for now
			if replicas > 1 {
				brick = NewBrick(size)
			}

			// This could be a function
			for enough_space := false; !enough_space; {

				// Could ask for more than just the replicas
				if len(nodelist) < 1 {
					return nil, errors.New("No space")
				}

				var bricknode BrickNode

				// Should check list size
				bricknode, nodelist = nodelist[len(nodelist)-1], nodelist[:len(nodelist)-1]

				// Probably should be an accessor
				if m.db.nodes[bricknode.node].Info.Devices[bricknode.device].Free > size {
					enough_space = true
					brick.NodeId = bricknode.node
					brick.DeviceId = bricknode.device

					m.db.nodes[bricknode.node].Info.Devices[bricknode.device].Used += size
					m.db.nodes[bricknode.node].Info.Devices[bricknode.device].Free -= size
				}
			}

			// Create a brick object
			bricks = append(bricks, brick)

		}
	}

	return bricks, nil

}

func (m *GlusterFSPlugin) createBricks(bricks []*Brick) error {
	return nil
}

func (m *GlusterFSPlugin) VolumeCreate(v *requests.VolumeCreateRequest) (*requests.VolumeInfoResp, error) {

	m.rwlock.Lock()
	defer m.rwlock.Unlock()

	// Determine number of bricks needed
	bricks_num, brick_size, err := m.numBricksNeeded(v.Size)
	if err != nil {
		return nil, err
	}

	// Get the nodes and storage for these bricks
	// and Create the bricks
	replica := v.Replica
	if v.Replica == 0 {
		replica = 2
	}

	// Allocate bricks in the cluster
	bricks, err := m.allocBricks(bricks_num, replica, brick_size)
	if err != nil {
		return nil, err
	}

	// Create bricks
	err = m.createBricks(bricks)
	if err != nil {
		return nil, err
	}

	// Create volume object
	volume := NewVolumeDB(v, bricks, replica)
	err = volume.CreateGlusterVolume()
	if err != nil {
		return nil, err
	}

	// Save volume information on the DB
	m.db.volumes[volume.Info.Id] = volume

	// Save changes to the DB
	m.db.Commit()

	return volume.InfoResponse(), nil
}

func (m *GlusterFSPlugin) VolumeDelete(id string) error {

	m.rwlock.Lock()
	defer m.rwlock.Unlock()

	if _, ok := m.db.volumes[id]; ok {
		delete(m.db.volumes, id)
	} else {
		return errors.New("Id not found")
	}

	m.db.Commit()
	return nil
}

func (m *GlusterFSPlugin) VolumeInfo(id string) (*requests.VolumeInfoResp, error) {

	m.rwlock.RLock()
	defer m.rwlock.RUnlock()

	if volume, ok := m.db.volumes[id]; ok {
		return &volume.Info, nil
	} else {
		return nil, errors.New("Id not found")
	}
}

func (m *GlusterFSPlugin) VolumeResize(id string) (*requests.VolumeInfoResp, error) {
	return m.VolumeInfo(id)
}

func (m *GlusterFSPlugin) VolumeList() (*requests.VolumeListResponse, error) {

	m.rwlock.RLock()
	defer m.rwlock.RUnlock()

	list := &requests.VolumeListResponse{}
	list.Volumes = make([]requests.VolumeInfoResp, 0)

	for _, volume := range m.db.volumes {
		list.Volumes = append(list.Volumes, *volume.InfoResponse())
	}

	return list, nil
}
