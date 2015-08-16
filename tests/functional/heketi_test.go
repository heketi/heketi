// +build functionaltests

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
package functional

import (
	"bytes"
	"fmt"
	"github.com/heketi/heketi/apps/glusterfs"
	"github.com/heketi/heketi/tests"
	"github.com/heketi/heketi/utils"
	"net/http"
	"testing"
	"time"
)

// This test requires Heketi Demo VMs running with
// the heketi server running on localhost
const (
	heketiUrl = "http://localhost:8080"
	storage0  = "192.168.10.100"
	storage1  = "192.168.10.101"
	storage2  = "192.168.10.102"
	storage3  = "192.168.10.103"
)

var (
	storagevms = []string{
		storage0,
		storage1,
		storage2,
		storage3,
	}

	disks = []string{
		"/dev/sdb",
		"/dev/sdc",
		"/dev/sdd",
		"/dev/sde",

		"/dev/sdf",
		"/dev/sdg",
		"/dev/sdh",
		"/dev/sdi",
	}
)

func TestConnection(t *testing.T) {
	r, err := http.Get(heketiUrl + "/hello")
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusOK)
}

func TestCreateTopology(t *testing.T) {

	// Create a cluster
	r, err := http.Post(heketiUrl+"/clusters",
		"application/json",
		bytes.NewBuffer([]byte(`{}`)))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusCreated)

	// Read JSON response
	var cluster glusterfs.ClusterInfoResponse
	err = utils.GetJsonFromResponse(r, &cluster)
	tests.Assert(t, err == nil)

	// Add nodes
	for index, hostname := range storagevms {
		request := []byte(`{
			"cluster" : "` + cluster.Id + `",
			"hostnames" : {
				"storage" : [ "` + hostname + `" ],
				"manage" : [ "` + hostname + `"  ]
			},
			"zone" : ` + fmt.Sprintf("%v", index%2) + `
	    }`)

		// Create node
		r, err := http.Post(heketiUrl+"/nodes", "application/json", bytes.NewBuffer(request))
		tests.Assert(t, err == nil)
		tests.Assert(t, r.StatusCode == http.StatusAccepted)
		location, err := r.Location()
		tests.Assert(t, err == nil)

		// Query queue until finished
		var node glusterfs.NodeInfoResponse
		for {
			r, err = http.Get(location.String())
			tests.Assert(t, err == nil)
			tests.Assert(t, r.StatusCode == http.StatusOK)
			if r.Header.Get("X-Pending") == "true" {
				time.Sleep(time.Millisecond * 10)
				continue
			} else {
				// Should have node information here
				tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")
				err = utils.GetJsonFromResponse(r, &node)
				tests.Assert(t, err == nil)
				break
			}
		}

		// Add devices
		for _, disk := range disks {
			request := []byte(`{
				"node" : "` + node.Id + `",
				"name" : "` + disk + `",
				"weight": 100
			}`)

			// Add device
			r, err := http.Post(heketiUrl+"/devices", "application/json", bytes.NewBuffer(request))
			tests.Assert(t, err == nil)
			tests.Assert(t, r.StatusCode == http.StatusAccepted)
			location, err := r.Location()
			tests.Assert(t, err == nil)

			// Query queue until finished
			for {
				r, err = http.Get(location.String())
				tests.Assert(t, err == nil)
				if r.Header.Get("X-Pending") == "true" {
					tests.Assert(t, r.StatusCode == http.StatusOK)
					time.Sleep(time.Millisecond * 10)
				} else {
					tests.Assert(t, r.StatusCode == http.StatusNoContent)
					break
				}
			}
		}
	}
}
