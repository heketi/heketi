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

// Structs for messages
type NodeAddRequest struct {
	Name string `json:"name"`
	Zone int    `json:"zone"`
}

type NodeInfoResp struct {
	Name    string                     `json:"name"`
	Zone    int                        `json:"zone"`
	Id      string                     `json:"id"`
	Storage StorageSize                `json:"storage"`
	Devices map[string]*DeviceResponse `json:"devices"`
	Plugin  interface{}                `json:"plugin,omitempty"`
}

type NodeListResponse struct {
	Nodes []NodeInfoResp `json:"nodes"`
}

type DeviceRequest struct {
	Name   string `json:"name"`
	Weight int    `json:"weight"`
}

type DeviceResponse struct {
	DeviceRequest
	StorageSize
	Id string `json:"id"`
}

type DeviceAddRequest struct {
	Devices []DeviceRequest `json:"devices"`
}
