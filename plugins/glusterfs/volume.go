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
	"github.com/lpabon/heketi/requests"
	"github.com/lpabon/heketi/utils"
	//goon "github.com/shurcooL/go-goon"
)

type Brick struct {
	Id     string
	Path   string
	NodeId string
	Online bool
	Size   uint64
}

type VolumeDB struct {
	Volume *requests.VolumeInfoResp
	Bricks []*Brick
}

func numBricks(v *requests.VolumeCreateRequest) (int, uint64, error) {
	return 2, v.Size / 2, nil
}

func createBricks(num_bricks int, size uint64, replicas int) ([]*Brick, error) {
	bricks := make([]*Brick, 0)
	for i := 0; i < replicas*num_bricks; i++ {
		id, _ := utils.GenUUID()
		brick := &Brick{
			Id:     id,
			Path:   fmt.Sprintf("/fake/path/%v", id),
			NodeId: "asdf",
			Online: true,
			Size:   size,
		}
		bricks = append(bricks, brick)
	}

	return bricks, nil
}

func createGlusterVolume(bricks []*Brick, replica int) error {
	return nil
}

func (m *GlusterFSPlugin) VolumeCreate(v *requests.VolumeCreateRequest) (*requests.VolumeInfoResp, error) {

	// Get number of bricks we need to satisfy this request
	bricks_num, brick_size, err := numBricks(v)
	if err != nil {
		return nil, err
	}

	// Get the nodes and storage for these bricks
	// and Create the bricks
	bricks, err := createBricks(bricks_num, brick_size, 2)
	if err != nil {
		return nil, err
	}

	// Setup the glusterfs volume
	err = createGlusterVolume(bricks, 2)
	if err != nil {
		return nil, err
	}

	m.rwlock.Lock()
	defer m.rwlock.Unlock()

	// Save volume information
	info := &requests.VolumeInfoResp{}
	info.Name = v.Name
	info.Size = v.Size
	info.Id, err = utils.GenUUID()
	if err != nil {
		return nil, err
	}

	volume := &VolumeDB{
		Volume: info,
		Bricks: bricks,
	}

	m.db.volumes[info.Id] = volume

	m.db.Commit()

	return volume.Volume, nil
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
		return volume.Volume, nil
	} else {
		return nil, errors.New("Id not found")
	}
}

func (m *GlusterFSPlugin) VolumeResize(id string) (*requests.VolumeInfoResp, error) {
	m.rwlock.RLock()
	defer m.rwlock.RUnlock()

	return m.VolumeInfo(id)
}

func (m *GlusterFSPlugin) VolumeList() (*requests.VolumeListResponse, error) {

	m.rwlock.RLock()
	defer m.rwlock.RUnlock()

	list := &requests.VolumeListResponse{}
	list.Volumes = make([]requests.VolumeInfoResp, 0)

	for _, info := range m.db.volumes {
		list.Volumes = append(list.Volumes, *info.Volume)
	}

	return list, nil
}
