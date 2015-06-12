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

func (m *GlusterFSPlugin) VolumeCreate(v *requests.VolumeCreateRequest) (*requests.VolumeInfoResp, error) {

	m.rwlock.Lock()
	defer m.rwlock.Unlock()

	volume := NewVolumeDB(v)
	err := volume.Create()
	if err != nil {
		return nil, err
	}

	// Save volume information on the DB
	m.db.volumes[volume.Info.Id] = volume

	// Save changes to the DB
	m.db.Commit()

	return &volume.Info, nil
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
		list.Volumes = append(list.Volumes, volume.Info)
	}

	return list, nil
}
