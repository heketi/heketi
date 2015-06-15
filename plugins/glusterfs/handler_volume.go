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
	"github.com/heketi/heketi/requests"
	//goon "github.com/shurcooL/go-goon"
	"sync"
)

const (
	KB = 1
	MB = KB * 1024
	GB = MB * 1024
	TB = GB * 1024
)

var (
	BRICK_MIN_SIZE = uint64(1 * GB)
	BRICK_MAX_SIZE = uint64(4 * TB)
	ErrNoSpace     = errors.New("No space")
)

type BrickNode struct {
	node, device string
}

type BrickNodes []BrickNode

// return size of each brick, error
func (m *GlusterFSPlugin) numBricksNeeded(size uint64) (uint64, error) {
	brick_size := size / 2

	if brick_size < BRICK_MIN_SIZE {
		return 0, errors.New("Minimum brick size limit reached.  Out of space.")
	} else if brick_size > BRICK_MAX_SIZE {
		return m.numBricksNeeded(brick_size)
	}

	return brick_size, nil
}

func (m *GlusterFSPlugin) getBrickNodes(brick *Brick, replicas int) BrickNodes {
	// Get info from swift ring

	// Check it has enough space, if not .. go to next device

	nodelist := make(BrickNodes, 0)

	for nodeid, node := range m.db.nodes {
		for deviceid, _ := range node.Info.Devices {
			nodelist = append(nodelist, BrickNode{device: deviceid, node: nodeid})
		}
	}

	return nodelist
}

func (m *GlusterFSPlugin) allocBricks(num_bricks, replicas int, size uint64) ([]*Brick, error) {

	bricks := make([]*Brick, 0)

	// Allocate size for the brick plus the snapshot
	tpsize := uint64(float64(size) * THINP_SNAPSHOT_FACTOR)

	for brick_num := 0; brick_num < num_bricks; brick_num++ {

		var brick *Brick

		brick = NewBrick(size)
		nodelist := m.getBrickNodes(brick, replicas)
		for i := 0; i < replicas; i++ {

			// XXX This is bad, but ok for now
			if i > 0 {
				brick = NewBrick(size)
			}

			// This could be a function
			for enough_space := false; !enough_space; {

				// Could ask for more than just the replicas
				if len(nodelist) < 1 {
					// unable to satisfy request.  Give back the data
					for brick := range bricks {
						bricks[brick].nodedb.Info.Devices[bricks[brick].DeviceId].Used -= tpsize
						bricks[brick].nodedb.Info.Devices[bricks[brick].DeviceId].Free += tpsize

						bricks[brick].nodedb.Info.Storage.Used -= tpsize
						bricks[brick].nodedb.Info.Storage.Free += tpsize
					}
					return nil, ErrNoSpace
				}

				var bricknode BrickNode

				// Should check list size
				bricknode, nodelist = nodelist[len(nodelist)-1], nodelist[:len(nodelist)-1]

				// Probably should be an accessor
				if m.db.nodes[bricknode.node].Info.Devices[bricknode.device].Free > tpsize {
					enough_space = true
					brick.NodeId = bricknode.node
					brick.DeviceId = bricknode.device
					brick.nodedb = m.db.nodes[bricknode.node]

					// This really needs to be cleaned up
					brick.nodedb.Info.Devices[brick.DeviceId].Used += tpsize
					brick.nodedb.Info.Devices[brick.DeviceId].Free -= tpsize

					brick.nodedb.Info.Storage.Used += tpsize
					brick.nodedb.Info.Storage.Free -= tpsize
				}
			}

			// Create a brick object
			bricks = append(bricks, brick)

		}
	}

	return bricks, nil

}

func (m *GlusterFSPlugin) createBricks(bricks []*Brick) error {
	var wg sync.WaitGroup
	for brick := range bricks {
		wg.Add(1)
		go func(b int) {
			defer wg.Done()
			bricks[b].Create()
		}(brick)
	}

	wg.Wait()

	return nil
}

func (m *GlusterFSPlugin) VolumeCreate(v *requests.VolumeCreateRequest) (*requests.VolumeInfoResp, error) {

	m.rwlock.Lock()
	defer m.rwlock.Unlock()

	// Get the nodes and storage for these bricks
	// and Create the bricks
	replica := v.Replica
	if v.Replica == 0 {
		replica = 2
	}

	var bricklist []*Brick
	size := v.Size
	for {
		// Determine number of bricks needed
		brick_size, err := m.numBricksNeeded(size)
		if err != nil {
			return nil, err
		}
		num_bricks := int(v.Size / brick_size)

		// Allocate bricks in the cluster
		bricklist, err = m.allocBricks(num_bricks, replica, brick_size)
		if err == ErrNoSpace {
			size /= 2
			continue
		}
		if err != nil {
			return nil, err
		} else {
			break
		}
	}

	// Create bricks
	err := m.createBricks(bricklist)
	if err != nil {
		return nil, err
	}

	// Create volume object
	volume := NewVolumeDB(v, bricklist, replica)
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

	if v, ok := m.db.volumes[id]; ok {
		v.Destroy()
		delete(m.db.volumes, id)
	} else {
		return errors.New("Volume id not found")
	}

	m.db.Commit()
	return nil
}

func (m *GlusterFSPlugin) VolumeInfo(id string) (*requests.VolumeInfoResp, error) {

	m.rwlock.RLock()
	defer m.rwlock.RUnlock()

	if volume, ok := m.db.volumes[id]; ok {
		return volume.InfoResponse(), nil
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
