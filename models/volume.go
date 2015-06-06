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
	"fmt"
	"net/http"
)

type VolumeManager interface {
	Create(name string) (*VolumeCreateMsg, error)
	Delete(name string) (*VolumeDeleteMsg, error)
	Info(name string) (*VolumeInfoMsg, error)
	List() ([]string, error)
}

type VolumeInfoMsg struct {
	Name string `json:"name"`
	Size uint64 `json:"size"`
}

type VolumeCreateMsg struct {
	Name string `json:"name"`
	Size uint64 `json:"size"`
}

type VolumeDeleteMsg struct {
	Name string `json:"name"`
}

func VolumeListHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "VolumeList\n")
}

func VolumeCreateHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "VolumeCreate\n")
}

func VolumeInfoHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "VolumeInfo\n")
}

func VolumeDeleteHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "VolumeDelete\n")
}
