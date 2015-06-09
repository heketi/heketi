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
)

type Volume struct {
	volume *requests.VolumeInfoResp
}

func (m *GlusterFSPlugin) VolumeCreate(v *requests.VolumeCreateRequest) (*requests.VolumeInfoResp, error) {
	m.db.current_id++

	info := &requests.VolumeInfoResp{}
	info.Name = v.Name
	info.Size = v.Size
	info.Id = m.db.current_id

	volume := &Volume{
		volume: info,
	}

	m.db.volumes[m.db.current_id] = volume

	return m.VolumeInfo(info.Id)
}

func (m *GlusterFSPlugin) VolumeDelete(id uint64) error {

	if _, ok := m.db.volumes[id]; ok {
		delete(m.db.volumes, id)
		return nil
	} else {
		return errors.New("Id not found")
	}
}

func (m *GlusterFSPlugin) VolumeInfo(id uint64) (*requests.VolumeInfoResp, error) {
	if volume, ok := m.db.volumes[id]; ok {
		info := &requests.VolumeInfoResp{}
		*info = *volume.volume
		return info, nil
	} else {
		return nil, errors.New("Id not found")
	}
}

func (m *GlusterFSPlugin) VolumeResize(id uint64) (*requests.VolumeInfoResp, error) {
	return m.VolumeInfo(id)
}

func (m *GlusterFSPlugin) VolumeList() (*requests.VolumeListResponse, error) {
	list := &requests.VolumeListResponse{}
	list.Volumes = make([]requests.VolumeInfoResp, 0)

	for id, _ := range m.db.volumes {
		info, err := m.VolumeInfo(id)
		if err != nil {
			return nil, err
		}
		list.Volumes = append(list.Volumes, *info)
	}

	return list, nil
}
