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

// TODO: Replace panic() calls with correct returns to the caller

package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
)

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

type NodeServer struct {
	plugin Plugin
}

// Handlers
func NewNodeServer(plugin Plugin) *NodeServer {
	return &NodeServer{
		plugin: plugin,
	}
}

func (n *NodeServer) NodeRoutes() Routes {

	// Node REST URLs routes
	var nodeRoutes = Routes{
		Route{"NodeList", "GET", "/nodes", n.NodeListHandler},
		Route{"NodeAdd", "POST", "/nodes", n.NodeAddHandler},
		Route{"NodeInfo", "GET", "/nodes/{id:[0-9]+}", n.NodeInfoHandler},
		Route{"NodeDelete", "DELETE", "/nodes/{id:[0-9]+}", n.NodeDeleteHandler},
	}

	return nodeRoutes
}

func (n *NodeServer) NodeListHandler(w http.ResponseWriter, r *http.Request) {

	// Get list
	list, err := n.plugin.NodeList()

	// Must be a server error if we could not get a list
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Write msg
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(list); err != nil {
		panic(err)
	}
}

func (n *NodeServer) NodeAddHandler(w http.ResponseWriter, r *http.Request) {
	var msg NodeAddRequest

	body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1048576))
	if err != nil {
		panic(err)
	}
	if err := r.Body.Close(); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(body, &msg); err != nil {
		w.WriteHeader(422) // unprocessable entity
		return
	}

	// Add node here
	info, err := n.plugin.NodeAdd(&msg)

	// :TODO:
	// Depending on the error returned here,
	// we should return the correct error code
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Send back we created it (as long as we did not fail)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(info); err != nil {
		panic(err)
	}
}

func (n *NodeServer) NodeInfoHandler(w http.ResponseWriter, r *http.Request) {

	// Get the id from the URL
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 64)
	if err != nil {
		w.WriteHeader(422) // unprocessable entity
		return
	}

	// Call plugin
	info, err := n.plugin.NodeInfo(id)
	if err != nil {
		// Let's guess here and pretend that it failed because
		// it was not found.
		// There probably should be a table of err to http status codes
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Write msg
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(info); err != nil {
		panic(err)
	}
}

func (n *NodeServer) NodeDeleteHandler(w http.ResponseWriter, r *http.Request) {

	// Get the id from the URL
	vars := mux.Vars(r)

	// Get the id from the URL
	id, err := strconv.ParseUint(vars["id"], 10, 64)
	if err != nil {
		w.WriteHeader(422) // unprocessable entity
		return
	}

	// Remove node
	err = n.plugin.NodeRemove(id)
	if err != nil {
		// Let's guess here and pretend that it failed because
		// it was not found.
		// There probably should be a table of err to http status codes
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Delete here, and send the correct status code in case of failure
	w.Header().Add("X-Heketi-Deleted", fmt.Sprintf("%v", id))

	// Send back we created it (as long as we did not fail)
	w.WriteHeader(http.StatusOK)
}
