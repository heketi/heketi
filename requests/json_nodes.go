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

package requests

// Storage values in KB
type StorageSize struct {
	Total uint64 `json:"total"`
	Free  uint64 `json:"free"`
	Used  uint64 `json:"used"`
}

type LvmVolumeGroup struct {
	Name string `json:"name"`
	StorageSize
}

// Structs for messages
type NodeInfoResp struct {
	Name    string      `json:"hostname"`
	Id      string      `json:"id"`
	Zone    string      `json:"zone"`
	Storage StorageSize `json:"storage"`

	// -- optional values --
	VolumeGroups []LvmVolumeGroup `json:"volumegroups,omitempty"`
}

type Storage struct {
	Disks        []string `json:"disks,omitempty"`
	VolumeGroups []string `json:"vgs,omitempty"`
}

type NodeAddRequest struct {
	Name string `json:"name"`
	Zone string `json:"zone"`

	// ----- Optional Values ------
	// When Adding Disks
	Storage
}

type NodeListResponse struct {
	Nodes []NodeInfoResp `json:"nodes"`
}
