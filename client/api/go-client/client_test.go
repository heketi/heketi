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

package client

import (
	"fmt"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"github.com/heketi/heketi/apps/glusterfs"
	"github.com/heketi/heketi/middleware"
	"github.com/heketi/tests"
	"github.com/heketi/utils"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
)

const (
	TEST_ADMIN_KEY = "adminkey"
)

func setupHeketiServer(app *glusterfs.App) *httptest.Server {
	router := mux.NewRouter()
	app.SetRoutes(router)
	n := negroni.New()

	jwtconfig := &middleware.JwtAuthConfig{}
	jwtconfig.Admin.PrivateKey = TEST_ADMIN_KEY
	jwtconfig.User.PrivateKey = "userkey"

	// Setup middleware
	n.Use(middleware.NewJwtAuth(jwtconfig))
	n.UseFunc(app.Auth)
	n.UseHandler(router)

	// Create server
	return httptest.NewServer(n)
}

func TestClientCluster(t *testing.T) {
	db := tests.Tempfile()
	defer os.Remove(db)

	// Create the app
	app := glusterfs.NewTestApp(db)
	defer app.Close()

	// Setup the server
	ts := setupHeketiServer(app)
	defer ts.Close()

	// Create cluster with unknown user
	c := NewClient(ts.URL, "asdf", "")
	tests.Assert(t, c != nil)
	cluster, err := c.ClusterCreate()
	tests.Assert(t, err != nil)
	tests.Assert(t, cluster == nil)

	// Create cluster with bad password
	c = NewClient(ts.URL, "admin", "badpassword")
	tests.Assert(t, c != nil)
	cluster, err = c.ClusterCreate()
	tests.Assert(t, err != nil)
	tests.Assert(t, cluster == nil)

	// Create cluster correctly
	c = NewClient(ts.URL, "admin", TEST_ADMIN_KEY)
	tests.Assert(t, c != nil)
	cluster, err = c.ClusterCreate()
	tests.Assert(t, err == nil)
	tests.Assert(t, cluster.Id != "")
	tests.Assert(t, len(cluster.Nodes) == 0)
	tests.Assert(t, len(cluster.Volumes) == 0)

	// Request bad id
	info, err := c.ClusterInfo("bad")
	tests.Assert(t, err != nil)
	tests.Assert(t, info == nil)

	// Get information about the client
	info, err = c.ClusterInfo(cluster.Id)
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(info, cluster))

	// Get a list of clusters
	list, err := c.ClusterList()
	tests.Assert(t, err == nil)
	tests.Assert(t, len(list.Clusters) == 1)
	tests.Assert(t, list.Clusters[0] == info.Id)

	// Delete non-existent cluster
	err = c.ClusterDelete("badid")
	tests.Assert(t, err != nil)

	// Delete current cluster
	err = c.ClusterDelete(info.Id)
	tests.Assert(t, err == nil)
}

func TestClientNode(t *testing.T) {
	db := tests.Tempfile()
	defer os.Remove(db)

	// Create the app
	app := glusterfs.NewTestApp(db)
	defer app.Close()

	// Setup the server
	ts := setupHeketiServer(app)
	defer ts.Close()

	// Create cluster
	c := NewClient(ts.URL, "admin", TEST_ADMIN_KEY)
	tests.Assert(t, c != nil)
	cluster, err := c.ClusterCreate()
	tests.Assert(t, err == nil)
	tests.Assert(t, cluster.Id != "")
	tests.Assert(t, len(cluster.Nodes) == 0)
	tests.Assert(t, len(cluster.Volumes) == 0)

	// Add node to unknown cluster
	nodeReq := &glusterfs.NodeAddRequest{}
	nodeReq.ClusterId = "badid"
	nodeReq.Hostnames.Manage = []string{"manage"}
	nodeReq.Hostnames.Storage = []string{"storage"}
	nodeReq.Zone = 10
	_, err = c.NodeAdd(nodeReq)
	tests.Assert(t, err != nil)

	// Create node request packet
	nodeReq.ClusterId = cluster.Id
	node, err := c.NodeAdd(nodeReq)
	tests.Assert(t, err == nil)
	tests.Assert(t, node.Zone == nodeReq.Zone)
	tests.Assert(t, node.Id != "")
	tests.Assert(t, reflect.DeepEqual(nodeReq.Hostnames, node.Hostnames))
	tests.Assert(t, len(node.DevicesInfo) == 0)

	// Info on invalid id
	info, err := c.NodeInfo("badid")
	tests.Assert(t, err != nil)
	tests.Assert(t, info == nil)

	// Get node info
	info, err = c.NodeInfo(node.Id)
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(info, node))

	// Delete invalid node
	err = c.NodeDelete("badid")
	tests.Assert(t, err != nil)

	// Can't delete cluster with a node
	err = c.ClusterDelete(cluster.Id)
	tests.Assert(t, err != nil)

	// Delete node
	err = c.NodeDelete(node.Id)
	tests.Assert(t, err == nil)

	// Delete cluster
	err = c.ClusterDelete(cluster.Id)
	tests.Assert(t, err == nil)

}

func TestClientDevice(t *testing.T) {
	db := tests.Tempfile()
	defer os.Remove(db)

	// Create the app
	app := glusterfs.NewTestApp(db)
	defer app.Close()

	// Setup the server
	ts := setupHeketiServer(app)
	defer ts.Close()

	// Create cluster
	c := NewClient(ts.URL, "admin", TEST_ADMIN_KEY)
	tests.Assert(t, c != nil)
	cluster, err := c.ClusterCreate()
	tests.Assert(t, err == nil)

	// Create node request packet
	nodeReq := &glusterfs.NodeAddRequest{}
	nodeReq.ClusterId = cluster.Id
	nodeReq.Hostnames.Manage = []string{"manage"}
	nodeReq.Hostnames.Storage = []string{"storage"}
	nodeReq.Zone = 10

	// Create node
	node, err := c.NodeAdd(nodeReq)
	tests.Assert(t, err == nil)

	// Create a device request
	deviceReq := &glusterfs.DeviceAddRequest{}
	deviceReq.Name = "sda"
	deviceReq.Weight = 100
	deviceReq.NodeId = node.Id

	// Create device
	err = c.DeviceAdd(deviceReq)
	tests.Assert(t, err == nil)

	// Get node information
	info, err := c.NodeInfo(node.Id)
	tests.Assert(t, err == nil)
	tests.Assert(t, len(info.DevicesInfo) == 1)
	tests.Assert(t, len(info.DevicesInfo[0].Bricks) == 0)
	tests.Assert(t, info.DevicesInfo[0].Name == deviceReq.Name)
	tests.Assert(t, info.DevicesInfo[0].Weight == deviceReq.Weight)
	tests.Assert(t, info.DevicesInfo[0].Id != "")

	// Get info from an unknown id
	_, err = c.DeviceInfo("badid")
	tests.Assert(t, err != nil)

	// Get device information
	deviceInfo, err := c.DeviceInfo(info.DevicesInfo[0].Id)
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(*deviceInfo, info.DevicesInfo[0]))

	// Try to delete node, and will not until we delete the device
	err = c.NodeDelete(node.Id)
	tests.Assert(t, err != nil)

	// Delete unknown device
	err = c.DeviceDelete("badid")
	tests.Assert(t, err != nil)

	// Delete device
	err = c.DeviceDelete(deviceInfo.Id)
	tests.Assert(t, err == nil)

	// Delete node
	err = c.NodeDelete(node.Id)
	tests.Assert(t, err == nil)

	// Delete cluster
	err = c.ClusterDelete(cluster.Id)
	tests.Assert(t, err == nil)

}

func TestClientVolume(t *testing.T) {
	db := tests.Tempfile()
	defer os.Remove(db)

	// Create the app
	app := glusterfs.NewTestApp(db)
	defer app.Close()

	// Setup the server
	ts := setupHeketiServer(app)
	defer ts.Close()

	// Create cluster
	c := NewClient(ts.URL, "admin", TEST_ADMIN_KEY)
	tests.Assert(t, c != nil)
	cluster, err := c.ClusterCreate()
	tests.Assert(t, err == nil)

	// Create node request packet
	for n := 0; n < 4; n++ {
		nodeReq := &glusterfs.NodeAddRequest{}
		nodeReq.ClusterId = cluster.Id
		nodeReq.Hostnames.Manage = []string{"manage" + fmt.Sprintf("%v", n)}
		nodeReq.Hostnames.Storage = []string{"storage" + fmt.Sprintf("%v", n)}
		nodeReq.Zone = n + 1

		// Create node
		node, err := c.NodeAdd(nodeReq)
		tests.Assert(t, err == nil)

		// Create a device request
		sg := utils.NewStatusGroup()
		for i := 0; i < 50; i++ {
			sg.Add(1)
			go func() {
				defer sg.Done()

				deviceReq := &glusterfs.DeviceAddRequest{}
				deviceReq.Name = "sd" + utils.GenUUID()[:8]
				deviceReq.Weight = 100
				deviceReq.NodeId = node.Id

				// Create device
				err := c.DeviceAdd(deviceReq)
				sg.Err(err)

			}()
		}
		tests.Assert(t, sg.Result() == nil)
	}

	// Get list of volumes
	list, err := c.VolumeList()
	tests.Assert(t, err == nil)
	tests.Assert(t, len(list.Volumes) == 0)

	// Create a volume
	volumeReq := &glusterfs.VolumeCreateRequest{}
	volumeReq.Size = 10
	volume, err := c.VolumeCreate(volumeReq)
	tests.Assert(t, err == nil)
	tests.Assert(t, volume.Id != "")
	tests.Assert(t, volume.Size == volumeReq.Size)

	// Get list of volumes
	list, err = c.VolumeList()
	tests.Assert(t, err == nil)
	tests.Assert(t, len(list.Volumes) == 1)
	tests.Assert(t, list.Volumes[0] == volume.Id)

	// Get info on incorrect id
	info, err := c.VolumeInfo("badid")
	tests.Assert(t, err != nil)

	// Get info
	info, err = c.VolumeInfo(volume.Id)
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(info, volume))

	// Expand volume with a bad id
	expandReq := &glusterfs.VolumeExpandRequest{}
	expandReq.Size = 10
	volumeInfo, err := c.VolumeExpand("badid", expandReq)
	tests.Assert(t, err != nil)

	// Expand volume
	volumeInfo, err = c.VolumeExpand(volume.Id, expandReq)
	tests.Assert(t, err == nil)
	tests.Assert(t, volumeInfo.Size == 20)

	// Delete bad id
	err = c.VolumeDelete("badid")
	tests.Assert(t, err != nil)

	// Delete volume
	err = c.VolumeDelete(volume.Id)
	tests.Assert(t, err == nil)

	clusterInfo, err := c.ClusterInfo(cluster.Id)
	for _, nodeid := range clusterInfo.Nodes {
		// Get node information
		nodeInfo, err := c.NodeInfo(nodeid)
		tests.Assert(t, err == nil)

		// Delete all devices
		sg := utils.NewStatusGroup()
		for index := range nodeInfo.DevicesInfo {
			sg.Add(1)
			go func(i int) {
				defer sg.Done()
				sg.Err(c.DeviceDelete(nodeInfo.DevicesInfo[i].Id))
			}(index)
		}
		err = sg.Result()
		tests.Assert(t, err == nil, err)

		// Delete node
		err = c.NodeDelete(nodeid)
		tests.Assert(t, err == nil)

	}

	// Delete cluster
	err = c.ClusterDelete(cluster.Id)
	tests.Assert(t, err == nil)

}
