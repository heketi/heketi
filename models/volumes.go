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

// Volume REST URLs routes
var volumeRoutes = Routes{

	Route{"VolumeList", "GET", "/volumes", VolumeListHandler},
	Route{"VolumeCreate", "POST", "/volumes", VolumeCreateHandler},
	Route{"VolumeInfo", "GET", "/volumes/{volid:[0-9]+}", VolumeInfoHandler},
	Route{"VolumeDelete", "DELETE", "/volumes/{volid:[0-9]+}", VolumeDeleteHandler},
}

func VolumeRoutes() Routes {
	return volumeRoutes
}

// Handlers

func VolumeListHandler(w http.ResponseWriter, r *http.Request) {

	// Sample - MOCK
	msg := VolumeListResponse{
		Volumes: []VolumeInfoResp{
			VolumeInfoResp{
				Name: "volume1",
				Size: 1234,
				Id:   0,
			},
			VolumeInfoResp{
				Name: "volume2",
				Size: 12345,
				Id:   1,
			},
			VolumeInfoResp{
				Name: "volume3",
				Size: 1234567,
				Id:   2,
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

func VolumeCreateHandler(w http.ResponseWriter, r *http.Request) {
	var msg VolumeCreateRequest

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

	// Create volume here
	v := VolumeInfoResp{
		Name: msg.Name,
		Size: msg.Size,
		Id:   1234,
	}

	// Send back we created it (as long as we did not fail)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		panic(err)
	}
}

func VolumeInfoHandler(w http.ResponseWriter, r *http.Request) {

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

	// sample volume information
	msg := VolumeInfoResp{
		Name: "infovolume",
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

func VolumeDeleteHandler(w http.ResponseWriter, r *http.Request) {

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
