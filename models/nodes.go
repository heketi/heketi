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

package models

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
)

// Node interface for plugins
type NodeManager interface {
	Add(v NodeAddRequest) (*NodeInfoResp, error)
	Remove(id uint64) error
	Info(id uint64) (*NodeInfoResp, error)
	List() (*NodeListResponse, error)
}

type StorageSize struct {
	Total uint64 `json:"total"`
	Free  uint64 `json:"free"`
	Used  uint64 `json:"used"`
}

type LvmVolumeGroup struct {
	Name string      `json:"name"`
	Size StorageSize `json:"storage"`
}

// Structs for messages
type NodeInfoResp struct {
	Name    string      `json:"hostname"`
	Id      uint64      `json: "id"`
	Zone    string      `json:"zone"`
	Storage StorageSize `json:"storage"`

	// -- optional values --
	VolumeGroups []LvmVolumeGroup `json:"volumegroups,omitempty"`
}

type NodeLvm struct {
	VolumeGroup string `json:"volumegroup"`
}

type NodeAddRequest struct {
	Name string `json:"name"`
	Zone string `json:"zone"`

	// ----- Optional Values ------

	// When Adding VGs
	Lvm NodeLvm `json:"lvm,omitempty"`
}

type NodeListResponse struct {
	Nodes []NodeInfoResp `json:"nodes"`
}

// Node REST URLs routes
var nodeRoutes = Routes{

	Route{"NodeList", "GET", "/nodes", NodeListHandler},
	Route{"NodeAdd", "POST", "/nodes", NodeAddHandler},
	Route{"NodeInfo", "GET", "/nodes/{volid:[0-9]+}", NodeInfoHandler},
	Route{"NodeDelete", "DELETE", "/nodes/{volid:[0-9]+}", NodeDeleteHandler},
}

func NodeRoutes() Routes {
	return nodeRoutes
}

// Handlers

func NodeListHandler(w http.ResponseWriter, r *http.Request) {

	// Sample - MOCK
	msg := NodeListResponse{
		Nodes: []NodeInfoResp{
			NodeInfoResp{
				Name: "node1",
				Zone: "1",
				Id:   0,
			},
			NodeInfoResp{
				Name: "node2",
				Zone: "2",
				Id:   1,
				Storage: StorageSize{
					Total: 1000,
					Free:  800,
					Used:  200,
				},
				VolumeGroups: []LvmVolumeGroup{
					LvmVolumeGroup{
						Name: "vg_group1",
						Size: StorageSize{
							Total: 1000,
							Free:  100,
							Used:  900,
						},
					},
					LvmVolumeGroup{
						Name: "vg_group2",
						Size: StorageSize{
							Total: 2000,
							Free:  200,
							Used:  2000 - 200,
						},
					},
				},
			},
		},
	}

	// Set JSON header
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	// Write msg
	if err := json.NewEncoder(w).Encode(msg); err != nil {

		// Bad error
		w.WriteHeader(http.StatusInternalServerError)
		// log
	} else {
		// Everything is OK
		w.WriteHeader(http.StatusOK)
	}
}

func NodeAddHandler(w http.ResponseWriter, r *http.Request) {
	var msg NodeAddRequest

	body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1048576))
	if err != nil {
		panic(err)
	}
	if err := r.Body.Close(); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(body, &msg); err != nil {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(422) // unprocessable entity
		if err := json.NewEncoder(w).Encode(err); err != nil {
			panic(err)
		}
	}

	// Add node here
	v := NodeInfoResp{
		Name: msg.Name,
		Zone: msg.Zone,
		Id:   1234,
	}

	// Send back we created it (as long as we did not fail)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		panic(err)
	}
}

func NodeInfoHandler(w http.ResponseWriter, r *http.Request) {

	// Set JSON header
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	// Get the id from the URL
	vars := mux.Vars(r)
	volid, err := strconv.ParseUint(vars["volid"], 10, 64)
	if err != nil {
		w.WriteHeader(422) // unprocessable entity
		if err := json.NewEncoder(w).Encode(err); err != nil {
			panic(err)
		}
	}

	// sample node information
	msg := NodeInfoResp{
		Name: "infonode",
		Id:   volid,
	}

	// Write msg
	if err := json.NewEncoder(w).Encode(msg); err != nil {

		// Bad error
		w.WriteHeader(http.StatusInternalServerError)
		// log
	} else {
		// Everything is OK
		w.WriteHeader(http.StatusOK)
	}
}

func NodeDeleteHandler(w http.ResponseWriter, r *http.Request) {

	// Get the id from the URL
	vars := mux.Vars(r)

	// Get the id from the URL
	volid, err := strconv.ParseUint(vars["volid"], 10, 64)
	if err != nil {
		w.WriteHeader(422) // unprocessable entity
		if err := json.NewEncoder(w).Encode(err); err != nil {
			panic(err)
		}
	}

	// Delete here, and send the correct status code in case of failure
	w.Header().Add("X-Heketi-Deleted", fmt.Sprintf("%v", volid))

	// Send back we created it (as long as we did not fail)
	w.WriteHeader(http.StatusOK)
}
