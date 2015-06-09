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

// Structs for messages
type VolumeInfoResp struct {
	Name string `json:"name"`
	Size uint64 `json:"size"`
	Id   uint64 `json: "id"`
}

type VolumeCreateRequest struct {
	Name string `json:"name"`
	Size uint64 `json:"size"`
}

type VolumeListResponse struct {
	Volumes []VolumeInfoResp `json:"volumes"`
}

type VolumeServer struct {
	plugin Plugin
}

func NewVolumeServer(plugin Plugin) *VolumeServer {
	return &VolumeServer{
		plugin: plugin,
	}

}

func (v *VolumeServer) VolumeRoutes() Routes {

	// Volume REST URLs routes
	var volumeRoutes = Routes{

		Route{"VolumeList", "GET", "/volumes", v.VolumeListHandler},
		Route{"VolumeCreate", "POST", "/volumes", v.VolumeCreateHandler},
		Route{"VolumeInfo", "GET", "/volumes/{id:[0-9]+}", v.VolumeInfoHandler},
		Route{"VolumeDelete", "DELETE", "/volumes/{id:[0-9]+}", v.VolumeDeleteHandler},
	}

	return volumeRoutes
}

// Handlers

func (v *VolumeServer) VolumeListHandler(w http.ResponseWriter, r *http.Request) {

	// Set JSON header
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	// Get list
	list, err := v.plugin.VolumeList()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Write list
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(list); err != nil {
		panic(err)
	}
}

func (v *VolumeServer) VolumeCreateHandler(w http.ResponseWriter, r *http.Request) {
	var request VolumeCreateRequest

	body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1048576))
	if err != nil {
		panic(err)
	}
	if err := r.Body.Close(); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(body, &request); err != nil {
		w.WriteHeader(422) // unprocessable entity
		return
	}

	// Create volume here
	result, err := v.plugin.VolumeCreate(&request)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Send back we created it (as long as we did not fail)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		panic(err)
	}
}

func (v *VolumeServer) VolumeInfoHandler(w http.ResponseWriter, r *http.Request) {

	// Get the id from the URL
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 64)
	if err != nil {
		w.WriteHeader(422) // unprocessable entity
		if err := json.NewEncoder(w).Encode(err); err != nil {
			panic(err)
		}

		return
	}

	// Get info from the plugin
	info, err := v.plugin.VolumeInfo(id)
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

func (v *VolumeServer) VolumeDeleteHandler(w http.ResponseWriter, r *http.Request) {

	// Get the id from the URL
	vars := mux.Vars(r)

	// Get the id from the URL
	id, err := strconv.ParseUint(vars["id"], 10, 64)
	if err != nil {
		w.WriteHeader(422) // unprocessable entity
		return
	}

	err = v.plugin.VolumeDelete(id)
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
