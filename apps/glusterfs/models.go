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
type Brick struct {
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

type DeviceInfoResponse struct {
	Device
	Storage StorageSize `json:"storage"`
	Id      string      `json:"id"`
	Bricks  []Brick     `json:"bricks"`
}

// Node
type NodeAddRequest struct {
	Zone      int           `json:"zone"`
	Hostnames HostAddresses `json:"hostnames"`
	ClusterId string        `json:"cluster"`
}

type NodeInfoResponse struct {
	NodeAddRequest
	Id          string               `json:"id"`
	Storage     StorageSize          `json:"storage"`
	DevicesInfo []DeviceInfoResponse `json:"devices"`
}

// Cluster
type ClusterInfoResponse struct {
	Id      string           `json:"id"`
	Nodes   sort.StringSlice `json:"nodes"`
	Volumes sort.StringSlice `json:"volumes"`
	Storage StorageSize      `json:"storage"`
}

type ClusterListResponse struct {
	Clusters []string `json:"clusters"`
}
