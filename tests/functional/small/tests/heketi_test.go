// +build ftsmall

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
	"github.com/heketi/heketi/apps/glusterfs"
	client "github.com/heketi/heketi/client/api/go-client"
	"github.com/heketi/tests"
	"github.com/heketi/utils"
	"net/http"
	"testing"
)

// These are the settings for the vagrant file
const (

	// The heketi server must be running on the host
	heketiUrl = "http://localhost:8080"

	// VMs
	storage0 = "192.168.10.100"
	storage1 = "192.168.10.101"
	storage2 = "192.168.10.102"
	storage3 = "192.168.10.103"
)

var (
	// Heketi client
	heketi = client.NewClientNoAuth(heketiUrl)

	// Storage systems
	storagevms = []string{
		storage0,
		storage1,
		storage2,
		storage3,
	}

	// Disks on each system
	disks = []string{
		"/dev/vdb",
		"/dev/vdc",
		"/dev/vdd",
		"/dev/vde",

		"/dev/vdf",
		"/dev/vdg",
		"/dev/vdh",
		"/dev/vdi",
	}
)

func setupCluster(t *testing.T) {
	tests.Assert(t, heketi != nil)

	// Create a cluster
	cluster, err := heketi.ClusterCreate()
	tests.Assert(t, err == nil)

	// Add nodes
	for index, hostname := range storagevms {
		nodeReq := &glusterfs.NodeAddRequest{}
		nodeReq.ClusterId = cluster.Id
		nodeReq.Hostnames.Manage = []string{hostname}
		nodeReq.Hostnames.Storage = []string{hostname}
		nodeReq.Zone = index%2 + 1

		node, err := heketi.NodeAdd(nodeReq)
		tests.Assert(t, err == nil)

		// Add devices
		sg := utils.NewStatusGroup()
		for _, disk := range disks {
			sg.Add(1)
			go func(d string) {
				defer sg.Done()

				driveReq := &glusterfs.DeviceAddRequest{}
				driveReq.Name = d
				driveReq.Weight = 100
				driveReq.NodeId = node.Id

				err := heketi.DeviceAdd(driveReq)
				sg.Err(err)
			}(disk)
		}

		err = sg.Result()
		tests.Assert(t, err == nil)
	}
}

func teardownCluster(t *testing.T) {
	clusters, err := heketi.ClusterList()
	tests.Assert(t, err == nil)

	for _, cluster := range clusters.Clusters {

		clusterInfo, err := heketi.ClusterInfo(cluster)
		tests.Assert(t, err == nil)

		// Delete volumes in this cluster
		for _, volume := range clusterInfo.Volumes {
			err := heketi.VolumeDelete(volume)
			tests.Assert(t, err == nil)
		}

		// Delete nodes
		for _, node := range clusterInfo.Nodes {

			// Get node information
			nodeInfo, err := heketi.NodeInfo(node)
			tests.Assert(t, err == nil)

			// Delete each device
			sg := utils.NewStatusGroup()
			for _, device := range nodeInfo.DevicesInfo {
				sg.Add(1)
				go func(id string) {
					defer sg.Done()

					err := heketi.DeviceDelete(id)
					sg.Err(err)

				}(device.Id)
			}
			err = sg.Result()
			tests.Assert(t, err == nil)

			// Delete node
			err = heketi.NodeDelete(node)
			tests.Assert(t, err == nil)
		}

		// Delete cluster
		err = heketi.ClusterDelete(cluster)
		tests.Assert(t, err == nil)
	}
}

func TestConnection(t *testing.T) {
	r, err := http.Get(heketiUrl + "/hello")
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusOK)
}

func TestHeketiVolumes(t *testing.T) {

	// Setup the VM storage topology
	teardownCluster(t)
	setupCluster(t)

	// Create a volume and delete a few time to test garbage collection
	for i := 0; i < 2; i++ {

		volReq := &glusterfs.VolumeCreateRequest{}
		volReq.Size = 4000
		volReq.Snapshot.Enable = true
		volReq.Snapshot.Factor = 1.5
		volReq.Durability.Type = client.VOLUME_CREATE_DURABILITY_TYPE_REPLICATE

		volInfo, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil)
		tests.Assert(t, volInfo.Size == 4000)
		tests.Assert(t, volInfo.Mount.GlusterFS.MountPoint != "")
		tests.Assert(t, volInfo.Name != "")

		volumes, err := heketi.VolumeList()
		tests.Assert(t, err == nil)
		tests.Assert(t, len(volumes.Volumes) == 1)
		tests.Assert(t, volumes.Volumes[0] == volInfo.Id)

		err = heketi.VolumeDelete(volInfo.Id)
		tests.Assert(t, err == nil)
	}

	// Create a 1TB volume
	volReq := &glusterfs.VolumeCreateRequest{}
	volReq.Size = 1024
	volReq.Snapshot.Enable = true
	volReq.Snapshot.Factor = 1.5
	volReq.Durability.Type = client.VOLUME_CREATE_DURABILITY_TYPE_REPLICATE

	simplevol, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil)

	// Create a 12TB volume with 6TB of snapshot space
	// There should be no space
	volReq = &glusterfs.VolumeCreateRequest{}
	volReq.Size = 12 * 1024
	volReq.Snapshot.Enable = true
	volReq.Snapshot.Factor = 1.5
	volReq.Durability.Type = client.VOLUME_CREATE_DURABILITY_TYPE_REPLICATE

	_, err = heketi.VolumeCreate(volReq)
	tests.Assert(t, err != nil)

	// Check there is only one
	volumes, err := heketi.VolumeList()
	tests.Assert(t, err == nil)
	tests.Assert(t, len(volumes.Volumes) == 1)

	// Create a 100G volume with replica 3
	volReq = &glusterfs.VolumeCreateRequest{}
	volReq.Size = 100
	volReq.Durability.Type = client.VOLUME_CREATE_DURABILITY_TYPE_REPLICATE
	volReq.Durability.Replicate.Replica = 3

	volInfo, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil)
	tests.Assert(t, volInfo.Size == 100)
	tests.Assert(t, volInfo.Mount.GlusterFS.MountPoint != "")
	tests.Assert(t, volInfo.Name != "")
	tests.Assert(t, len(volInfo.Bricks) == 6)

	// Check there are two volumes
	volumes, err = heketi.VolumeList()
	tests.Assert(t, err == nil)
	tests.Assert(t, len(volumes.Volumes) == 2)

	// Expand volume
	volExpReq := &glusterfs.VolumeExpandRequest{}
	volExpReq.Size = 2000

	volInfo, err = heketi.VolumeExpand(simplevol.Id, volExpReq)
	tests.Assert(t, err == nil)
	tests.Assert(t, volInfo.Size == simplevol.Size+2000)

	// Delete volume
	err = heketi.VolumeDelete(volInfo.Id)
	tests.Assert(t, err == nil)
}
