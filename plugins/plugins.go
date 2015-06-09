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

package plugins

import (
	"github.com/lpabon/heketi/plugins/glusterfs"
	"github.com/lpabon/heketi/plugins/mock"
	"github.com/lpabon/heketi/requests"
)

// Volume interface for plugins
type Plugin interface {
	VolumeCreate(v *requests.VolumeCreateRequest) (*requests.VolumeInfoResp, error)
	VolumeDelete(id uint64) error
	VolumeInfo(id uint64) (*requests.VolumeInfoResp, error)
	VolumeResize(id uint64) (*requests.VolumeInfoResp, error)
	VolumeList() (*requests.VolumeListResponse, error)

	NodeAdd(v *requests.NodeAddRequest) (*requests.NodeInfoResp, error)
	NodeRemove(id uint64) error
	NodeInfo(id uint64) (*requests.NodeInfoResp, error)
	NodeList() (*requests.NodeListResponse, error)
}

func NewPlugin(name string) Plugin {

	switch name {
	case "mock":
		return mock.NewMockPlugin()
	case "glusterfs":
		return glusterfs.NewGlusterFSPlugin()
	default:
		return nil
	}

}
