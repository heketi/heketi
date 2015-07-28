//
// Copyright (c) 2015 The heketi Authors
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

//
// Please see https://github.com/heketi/heketi/wiki/API
// for documentation
//
package glusterfs

import (
	"sort"
)

// Storage values in KB
type StorageSize struct {
	Total uint64 `json:"total"`
	Free  uint64 `json:"free"`
	Used  uint64 `json:"used"`
}

type HostAddresses struct {
	Manage  sort.StringSlice `json:"manage"`
	Storage sort.StringSlice `json:"storage"`
}

// Volume

// Brick
type BrickInfo struct {
	Id       string `json:"id"`
	Path     string `json:"path"`
	DeviceId string `json:"device"`
	NodeId   string `json:"node"`

	// Size in KB
	Size uint64 `json:"size"`
}

// Device

type Device struct {
	Name   string `json:"name"`
	Weight int    `json:"weight"`
}

type DeviceAddRequest struct {
	NodeId  string   `json:"node"`
	Devices []Device `json:"devices"`
}

type DeviceInfo struct {
	Device
	Storage StorageSize `json:"storage"`
	Id      string      `json:"id"`
}

type DeviceInfoResponse struct {
	DeviceInfo
	Bricks []BrickInfo `json:"bricks"`
}

// Node
type NodeAddRequest struct {
	Zone      int           `json:"zone"`
	Hostnames HostAddresses `json:"hostnames"`
	ClusterId string        `json:"cluster"`
}

type NodeInfo struct {
	NodeAddRequest
	Id string `json:"id"`
}

type NodeInfoResponse struct {
	NodeInfo
	DevicesInfo []DeviceInfoResponse `json:"devices"`
}

// Cluster
type ClusterInfoResponse struct {
	Id      string           `json:"id"`
	Nodes   sort.StringSlice `json:"nodes"`
	Volumes sort.StringSlice `json:"volumes"`
}

type ClusterListResponse struct {
	Clusters []string `json:"clusters"`
}

// Volume
type VolumeCreateRequest struct {
	// Size in GB
	Size     int      `json:"size"`
	Clusters []string `json:"clusters,omitempty"`
	Name     string   `json:"name"`
	Replica  int      `json:"replica"`
	Snapshot struct {
		Enable bool    `json:"enable"`
		Factor float32 `json:"factor"`
	} `json:"snapshot"`
}

type VolumeInfo struct {
	VolumeCreateRequest
	Id      string `json:"id"`
	Cluster string `json:"cluster"`
	Mount   struct {
		GlusterFS struct {
			MountPoint string            `json:"device"`
			Options    map[string]string `json:"options"`
		} `json:"glusterfs"`
	} `json:"mount"`
}

type VolumeInfoResponse struct {
	VolumeInfo
	Bricks []BrickInfo `json:"bricks"`
}

type VolumeListResponse struct {
	Volumes []string `json:"volumes"`
}

// Constructors

func NewVolumeInfoResponse() *VolumeInfoResponse {

	info := &VolumeInfoResponse{}
	info.Mount.GlusterFS.Options = make(map[string]string)
	info.Bricks = make([]BrickInfo, 0)

	return info
}
