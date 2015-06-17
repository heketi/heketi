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

// Structs for messages
type VolumeInfoResp struct {
	Name   string      `json:"name"`
	Size   uint64      `json:"size"`
	Id     string      `json:"id"`
	Mount  string      `json:"mount"`
	Plugin interface{} `json:"plugin,omitempty"`
}

type VolumeCreateRequest struct {

	// Name of volume to create
	Name string `json:"name"`

	// Size in KB
	Size uint64 `json:"size"`

	// If possible, number of replicas requested
	Replica int `json:"replica,omitempty"`
}

type VolumeListResponse struct {
	Volumes []VolumeInfoResp `json:"volumes"`
}
