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
	"strings"
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

func setupCluster(t *testing.T) {
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
				time.Sleep(time.Millisecond * 500)
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
					time.Sleep(time.Millisecond * 500)
				} else {
					tests.Assert(t, r.StatusCode == http.StatusNoContent)
					break
				}
			}
		}

	}
}

func teardownCluster(t *testing.T) {
	// Delete each volumes, device, node, then finally the cluster
	r, err := http.Get(heketiUrl + "/clusters")
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

	var clusters glusterfs.ClusterListResponse
	err = utils.GetJsonFromResponse(r, &clusters)

	for _, cluster := range clusters.Clusters {

		// Get the list of nodes and volumes in this cluster
		r, err := http.Get(heketiUrl + "/clusters/" + cluster)
		tests.Assert(t, r.StatusCode == http.StatusOK)
		tests.Assert(t, err == nil)
		tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

		var clusterInfo glusterfs.ClusterInfoResponse
		err = utils.GetJsonFromResponse(r, &clusterInfo)
		tests.Assert(t, err == nil)

		// Delete volumes in this cluster
		for _, volume := range clusterInfo.Volumes {
			req, err := http.NewRequest("DELETE", heketiUrl+"/volumes/"+volume, nil)
			tests.Assert(t, err == nil)
			r, err := http.DefaultClient.Do(req)
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
					time.Sleep(time.Second)
				} else {
					// Check that it was removed correctly
					tests.Assert(t, r.StatusCode == http.StatusNoContent)
					break
				}
			}
		}

		// Delete nodes
		for _, node := range clusterInfo.Nodes {

			// Get node information
			r, err := http.Get(heketiUrl + "/nodes/" + node)
			tests.Assert(t, r.StatusCode == http.StatusOK)
			tests.Assert(t, err == nil)
			tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

			var nodeInfo glusterfs.NodeInfoResponse
			err = utils.GetJsonFromResponse(r, &nodeInfo)
			tests.Assert(t, err == nil)

			// Delete each device
			for _, device := range nodeInfo.DevicesInfo {
				req, err := http.NewRequest("DELETE", heketiUrl+"/devices/"+device.Id, nil)
				tests.Assert(t, err == nil)
				r, err := http.DefaultClient.Do(req)
				tests.Assert(t, err == nil)
				tests.Assert(t, r.StatusCode == http.StatusAccepted)
				location, err := r.Location()
				tests.Assert(t, err == nil)

				// Query queue until finished
				for {
					r, err := http.Get(location.String())
					tests.Assert(t, err == nil)
					if r.Header.Get("X-Pending") == "true" {
						tests.Assert(t, r.StatusCode == http.StatusOK)
						time.Sleep(time.Second)
					} else {
						// Check that it was removed correctly
						tests.Assert(t, r.StatusCode == http.StatusNoContent)
						break
					}
				}
			}

			// Delete node
			req, err := http.NewRequest("DELETE", heketiUrl+"/nodes/"+node, nil)
			tests.Assert(t, err == nil)
			r, err = http.DefaultClient.Do(req)
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
					time.Sleep(time.Second)
				} else {
					// Check that it was removed correctly
					tests.Assert(t, r.StatusCode == http.StatusNoContent)
					break
				}
			}
		}

		// Delete cluster
		req, err := http.NewRequest("DELETE", heketiUrl+"/clusters/"+cluster, nil)
		tests.Assert(t, err == nil)
		r, err = http.DefaultClient.Do(req)
		tests.Assert(t, err == nil)
		tests.Assert(t, r.StatusCode == http.StatusOK)
	}
}

func expandVolume(t *testing.T, size int, id string) *glusterfs.VolumeInfoResponse {

	request := []byte(`{
        "expand_size" : ` + fmt.Sprintf("%v", size) + `
    }`)

	// Send request
	r, err := http.Post(heketiUrl+"/volumes/"+id+"/expand",
		"application/json",
		bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	location, err := r.Location()
	tests.Assert(t, err == nil)

	// Query queue until finished
	var info glusterfs.VolumeInfoResponse
	for {
		r, err := http.Get(location.String())
		tests.Assert(t, err == nil)
		tests.Assert(t, r.StatusCode == http.StatusOK)
		if r.Header.Get("X-Pending") == "true" {
			time.Sleep(time.Second)
			continue
		} else {
			err = utils.GetJsonFromResponse(r, &info)
			tests.Assert(t, err == nil)
			break
		}
	}

	return &info
}

func createVolume(t *testing.T, request []byte) *glusterfs.VolumeInfoResponse {
	r, err := http.Post(heketiUrl+"/volumes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	location, err := r.Location()
	tests.Assert(t, err == nil)

	// Query queue until finished
	var volInfo glusterfs.VolumeInfoResponse
	for {
		r, err = http.Get(location.String())
		tests.Assert(t, err == nil)
		tests.Assert(t, r.StatusCode == http.StatusOK)
		if r.Header.Get("X-Pending") == "true" {
			time.Sleep(time.Second)
		} else {
			// We have volume information
			tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")
			err = utils.GetJsonFromResponse(r, &volInfo)
			tests.Assert(t, err == nil)
			break
		}
	}

	return &volInfo
}

func deleteVolume(t *testing.T, id string) {
	req, err := http.NewRequest("DELETE", heketiUrl+"/volumes/"+id, nil)
	tests.Assert(t, err == nil)
	r, err := http.DefaultClient.Do(req)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	location, err := r.Location()
	tests.Assert(t, err == nil)

	// Query queue until finished
	for {
		r, err := http.Get(location.String())
		tests.Assert(t, err == nil)
		if r.Header.Get("X-Pending") == "true" {
			tests.Assert(t, r.StatusCode == http.StatusOK)
			time.Sleep(time.Second)
		} else {
			// Check that it was removed correctly
			tests.Assert(t, r.StatusCode == http.StatusNoContent)
			break
		}
	}
}

func listVolumes(t *testing.T) *glusterfs.VolumeListResponse {
	// Get list of volumes
	r, err := http.Get(heketiUrl + "/volumes")
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

	// Read response
	var volumes glusterfs.VolumeListResponse
	err = utils.GetJsonFromResponse(r, &volumes)
	tests.Assert(t, err == nil)

	return &volumes

}

func TestConnection(t *testing.T) {
	r, err := http.Get(heketiUrl + "/hello")
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusOK)
}

func TestHeketiVolumes(t *testing.T) {

	// Setup the VM storage topology
	setupCluster(t)
	defer teardownCluster(t)

	// Create a volume and delete a few time to test garbage collection
	for i := 0; i < 2; i++ {
		// Create a 4TB volume with snapshot space
		request := []byte(`{
			"size" : 4000,
			"snapshot" : {
				"enable" : true,
				"factor" : 1.5
			}
		}`)

		volInfo := createVolume(t, request)
		tests.Assert(t, volInfo.Size == 4000)
		tests.Assert(t, volInfo.Mount.GlusterFS.MountPoint != "")
		tests.Assert(t, volInfo.Replica == 2)
		tests.Assert(t, volInfo.Name != "")

		volumes := listVolumes(t)
		tests.Assert(t, len(volumes.Volumes) == 1)
		tests.Assert(t, volumes.Volumes[0] == volInfo.Id)

		deleteVolume(t, volInfo.Id)
	}

	// Create a 1TB volume
	request := []byte(`{
			"size" : 1024,
			"snapshot" : {
				"enable" : true,
				"factor" : 1.5
			}
		}`)
	simplevol := createVolume(t, request)

	// Create a 4TB volume with 2TB of snapshot space
	// There should be no space
	request = []byte(`{
			"size" : 4096,
			"replica" : 3,
			"snapshot" : {
				"enable" : true,
				"factor" : 1.5
			}
		}`)

	r, err := http.Post(heketiUrl+"/volumes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	location, err := r.Location()
	tests.Assert(t, err == nil)

	// Query queue until finished
	for {
		r, err := http.Get(location.String())
		tests.Assert(t, err == nil)
		if r.Header.Get("X-Pending") == "true" {
			tests.Assert(t, r.StatusCode == http.StatusOK)
			time.Sleep(time.Second)
		} else {
			tests.Assert(t, r.StatusCode == http.StatusInternalServerError)
			s, err := utils.GetStringFromResponse(r)
			tests.Assert(t, err == nil)
			tests.Assert(t, strings.Contains(s, "No space"))
			break
		}
	}

	// Check there is only one
	volumes := listVolumes(t)
	tests.Assert(t, len(volumes.Volumes) == 1)

	// Create a 100G volume with replica 3
	// There should be no space
	request = []byte(`{
			"size" : 100,
			"replica" : 3
		}`)
	volInfo := createVolume(t, request)

	tests.Assert(t, volInfo.Size == 100)
	tests.Assert(t, volInfo.Mount.GlusterFS.MountPoint != "")
	tests.Assert(t, volInfo.Replica == 3)
	tests.Assert(t, volInfo.Name != "")
	tests.Assert(t, len(volInfo.Bricks) == 6)

	// Check there are two volumes
	volumes = listVolumes(t)
	tests.Assert(t, len(volumes.Volumes) == 2)

	// Expand volume
	volInfo = expandVolume(t, 2000, simplevol.Id)
	tests.Assert(t, volInfo.Size == simplevol.Size+2000)
	simplevol = volInfo

}
