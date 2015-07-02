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

package mock

import (
	"errors"
	"github.com/heketi/heketi/requests"
	"github.com/heketi/heketi/utils"
)

type Volume struct {
	volume *requests.VolumeInfoResp
}

func (m *MockPlugin) VolumeCreate(v *requests.VolumeCreateRequest) (*requests.VolumeInfoResp, error) {

	var err error

	info := &requests.VolumeInfoResp{}
	info.Name = v.Name
	info.Size = v.Size
	info.Id = utils.GenUUID()
	if err != nil {
		return nil, err
	}

	volume := &Volume{
		volume: info,
	}

	m.db.volumes[info.Id] = volume

	return m.VolumeInfo(info.Id)
}

func (m *MockPlugin) VolumeDelete(id string) error {

	if _, ok := m.db.volumes[id]; ok {
		delete(m.db.volumes, id)
		return nil
	} else {
		return errors.New("Id not found")
	}
}

func (m *MockPlugin) VolumeInfo(id string) (*requests.VolumeInfoResp, error) {
	if volume, ok := m.db.volumes[id]; ok {
		info := &requests.VolumeInfoResp{}
		*info = *volume.volume
		return info, nil
	} else {
		return nil, errors.New("Id not found")
	}
}

func (m *MockPlugin) VolumeResize(id string) (*requests.VolumeInfoResp, error) {
	return m.VolumeInfo(id)
}

func (m *MockPlugin) VolumeList() (*requests.VolumeListResponse, error) {
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
