// +build ftlarge

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
	"fmt"
	"github.com/heketi/heketi/apps/glusterfs"
	client "github.com/heketi/heketi/client/api/go-client"
	"github.com/heketi/heketi/tests"
	"github.com/heketi/heketi/utils"
	"testing"
)

// These are the settings for the vagrant file
const (

	// The heketi server must be running on the host
	heketiUrl = "http://localhost:8080"

	// VMs
	DISKS = 24
	NODES = 30
)

var (
	// Heketi client
	heketi = client.NewClient(heketiUrl, "admin", "adminkey")
)

func getdisks() []string {

	diskletters := make([]string, DISKS)
	for index, i := 0, []byte("b")[0]; index < DISKS; index, i = index+1, i+1 {
		diskletters[index] = "/dev/vd" + string(i)
	}

	return diskletters
}

func getnodes() []string {
	nodelist := make([]string, NODES)

	for index, ip := 0, 100; index < NODES; index, ip = index+1, ip+1 {
		nodelist[index] = "192.168.10." + fmt.Sprintf("%v", ip)
	}

	return nodelist
}

func setupCluster(t *testing.T) {
	tests.Assert(t, heketi != nil)

	// Create a cluster
	cluster, err := heketi.ClusterCreate()
	tests.Assert(t, err == nil)

	// Add nodes sequentially due to probes
	sg := utils.NewStatusGroup()
	for index, hostname := range getnodes() {
		nodeReq := &glusterfs.NodeAddRequest{}
		nodeReq.ClusterId = cluster.Id
		nodeReq.Hostnames.Manage = []string{hostname}
		nodeReq.Hostnames.Storage = []string{hostname}
		nodeReq.Zone = index % 6

		node, err := heketi.NodeAdd(nodeReq)
		tests.Assert(t, err == nil)

		// Add devices all concurrently
		for _, disk := range getdisks() {
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
	}

	// Wait here for all drives
	err = sg.Result()
	tests.Assert(t, err == nil)
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

		// Delete drives concurrently
		sg := utils.NewStatusGroup()
		for _, node := range clusterInfo.Nodes {

			// Get node information
			nodeInfo, err := heketi.NodeInfo(node)
			tests.Assert(t, err == nil)

			// Delete each device
			for _, device := range nodeInfo.DevicesInfo {
				sg.Add(1)
				go func(id string) {
					defer sg.Done()

					err := heketi.DeviceDelete(id)
					sg.Err(err)

				}(device.Id)
			}
		}
		err = sg.Result()
		tests.Assert(t, err == nil)

		// Delete nodes
		for _, node := range clusterInfo.Nodes {
			err = heketi.NodeDelete(node)
			tests.Assert(t, err == nil)
		}

		// Delete cluster
		err = heketi.ClusterDelete(cluster)
		tests.Assert(t, err == nil)
	}
}

func TestConnection(t *testing.T) {
	err := heketi.Hello()
	tests.Assert(t, err == nil)
}

func TestHeketiTopology(t *testing.T) {
	setupCluster(t)
	defer teardownCluster(t)
}

func testHeketiVolumes(t *testing.T) {

	// Setup the VM storage topology
	setupCluster(t)
	defer teardownCluster(t)

	// Create a volume and delete a few time to test garbage collection
	for i := 0; i < 2; i++ {

		volReq := &glusterfs.VolumeCreateRequest{}
		volReq.Size = 4000
		volReq.Snapshot.Enable = true
		volReq.Snapshot.Factor = 1.5

		volInfo, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil)
		tests.Assert(t, volInfo.Size == 4000)
		tests.Assert(t, volInfo.Mount.GlusterFS.MountPoint != "")
		tests.Assert(t, volInfo.Replica == 2)
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

	simplevol, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil)

	// Create a 4TB volume with 2TB of snapshot space
	// There should be no space
	volReq = &glusterfs.VolumeCreateRequest{}
	volReq.Size = 4096
	volReq.Replica = 3
	volReq.Snapshot.Enable = true
	volReq.Snapshot.Factor = 1.5

	_, err = heketi.VolumeCreate(volReq)
	tests.Assert(t, err != nil)

	// Check there is only one
	volumes, err := heketi.VolumeList()
	tests.Assert(t, err == nil)
	tests.Assert(t, len(volumes.Volumes) == 1)

	// Create a 100G volume with replica 3
	volReq = &glusterfs.VolumeCreateRequest{}
	volReq.Size = 100
	volReq.Replica = 3

	volInfo, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil)
	tests.Assert(t, volInfo.Size == 100)
	tests.Assert(t, volInfo.Mount.GlusterFS.MountPoint != "")
	tests.Assert(t, volInfo.Replica == 3)
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

}
