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

package glusterfs

import (
	"bytes"
	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	"github.com/heketi/heketi/tests"
	"github.com/heketi/heketi/utils"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func init() {
	// turn off logging
	logger.SetLevel(utils.LEVEL_NOLOG)
}

func TestDeviceAddBadRequests(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Patch dbfilename so that it is restored at the end of the tests
	defer tests.Patch(&dbfilename, tmpfile).Restore()

	// Create the app
	app := NewApp()
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// ClusterCreate JSON Request
	request := []byte(`{
        bad json
    }`)

	// Post bad JSON
	r, err := http.Post(ts.URL+"/devices", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == 422)

	// Make a request with no devices
	request = []byte(`{
        "node" : "123",
        "devices" : []
    }`)

	// Post bad JSON
	r, err = http.Post(ts.URL+"/devices", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusBadRequest)

	// Make a request with unknown node
	request = []byte(`{
        "node" : "123",
        "devices" : [
            {
                "name" : "/dev/fake",
                "weight" : 20
            }
        ]
    }`)

	// Post bad JSON
	r, err = http.Post(ts.URL+"/devices", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusNotFound)

}

func TestDeviceAdd(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Patch dbfilename so that it is restored at the end of the tests
	defer tests.Patch(&dbfilename, tmpfile).Restore()

	// Create the app
	app := NewApp()
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Add Cluster then a Node on the cluster
	// node
	cluster := NewClusterEntryFromRequest()
	req := &NodeAddRequest{
		ClusterId: cluster.Info.Id,
		Hostnames: HostAddresses{
			Manage:  []string{"manage"},
			Storage: []string{"storage"},
		},
		Zone: 99,
	}
	node := NewNodeEntryFromRequest(req)
	cluster.NodeAdd(node.Info.Id)

	// Save information in the db
	err := app.db.Update(func(tx *bolt.Tx) error {
		err := cluster.Save(tx)
		if err != nil {
			return err
		}

		err = node.Save(tx)
		if err != nil {
			return err
		}
		return nil
	})
	tests.Assert(t, err == nil)

	// Create a request to add four devices
	request := []byte(`{
        "node" : "` + node.Info.Id + `",
        "devices" : [
            {
                "name" : "/dev/fake1",
                "weight" : 10
            },
            {
                "name" : "/dev/fake2",
                "weight" : 20
            },
            {
                "name" : "/dev/fake3",
                "weight" : 30
            },
            {
                "name" : "/dev/fake4",
                "weight" : 40
            }
        ]
    }`)

	// Add device using POST
	r, err := http.Post(ts.URL+"/devices", "application/json", bytes.NewBuffer(request))
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
			continue
		} else {
			tests.Assert(t, r.StatusCode == http.StatusNoContent)
			break
		}
	}

	// Check db to make sure devices where added
	devicemap := make(map[string]*DeviceEntry)
	err = app.db.View(func(tx *bolt.Tx) error {
		node, err = NewNodeEntryFromId(tx, node.Info.Id)
		if err != nil {
			return err
		}

		for _, id := range node.Devices {
			device, err := NewDeviceEntryFromId(tx, id)
			if err != nil {
				return err
			}
			devicemap[device.Info.Name] = device
		}

		return nil
	})
	tests.Assert(t, err == nil)

	val, ok := devicemap["/dev/fake1"]
	tests.Assert(t, ok)
	tests.Assert(t, val.Info.Name == "/dev/fake1")
	tests.Assert(t, val.Info.Weight == 10)
	tests.Assert(t, len(val.Bricks) == 0)

	val, ok = devicemap["/dev/fake2"]
	tests.Assert(t, ok)
	tests.Assert(t, val.Info.Name == "/dev/fake2")
	tests.Assert(t, val.Info.Weight == 20)
	tests.Assert(t, len(val.Bricks) == 0)

	val, ok = devicemap["/dev/fake3"]
	tests.Assert(t, ok)
	tests.Assert(t, val.Info.Name == "/dev/fake3")
	tests.Assert(t, val.Info.Weight == 30)
	tests.Assert(t, len(val.Bricks) == 0)

	val, ok = devicemap["/dev/fake4"]
	tests.Assert(t, ok)
	tests.Assert(t, val.Info.Name == "/dev/fake4")
	tests.Assert(t, val.Info.Weight == 40)
	tests.Assert(t, len(val.Bricks) == 0)
}

func TestDeviceInfoIdNotFound(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Patch dbfilename so that it is restored at the end of the tests
	defer tests.Patch(&dbfilename, tmpfile).Restore()

	// Create the app
	app := NewApp()
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Get unknown device id
	r, err := http.Get(ts.URL + "/devices/123456789")
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusNotFound)

}

func TestDeviceInfo(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Patch dbfilename so that it is restored at the end of the tests
	defer tests.Patch(&dbfilename, tmpfile).Restore()

	// Create the app
	app := NewApp()
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Create a device to save in the db
	device := NewDeviceEntry()
	device.Info.Id = "abc"
	device.Info.Name = "/dev/fake1"
	device.Info.Weight = 101
	device.NodeId = "def"
	device.StorageSet(10000)
	device.StorageAllocate(1000)

	// Save device in the db
	err := app.db.Update(func(tx *bolt.Tx) error {
		return device.Save(tx)
	})
	tests.Assert(t, err == nil)

	// Get device information
	r, err := http.Get(ts.URL + "/devices/" + device.Info.Id)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

	var info DeviceInfoResponse
	err = utils.GetJsonFromResponse(r, &info)
	tests.Assert(t, info.Id == device.Info.Id)
	tests.Assert(t, info.Name == device.Info.Name)
	tests.Assert(t, info.Weight == device.Info.Weight)
	tests.Assert(t, info.Storage.Free == device.Info.Storage.Free)
	tests.Assert(t, info.Storage.Used == device.Info.Storage.Used)
	tests.Assert(t, info.Storage.Total == device.Info.Storage.Total)

}

/*
func TestDeviceDeleteErrors(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Patch dbfilename so that it is restored at the end of the tests
	defer tests.Patch(&dbfilename, tmpfile).Restore()

	// Create the app
	app := NewApp()
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Create a device to save in the db
	device := NewDeviceEntry()
	device.Info.Id = "abc"
	device.Info.ClusterId = "123"
	device.Info.Hostnames.Manage = sort.StringSlice{"manage.system"}
	device.Info.Hostnames.Storage = sort.StringSlice{"storage.system"}
	device.Info.Zone = 10
	device.StorageAdd(10000)
	device.StorageAllocate(1000)

	// Save device in the db
	err := app.db.Update(func(tx *bolt.Tx) error {
		return device.Save(tx)
	})
	tests.Assert(t, err == nil)

	// Delete unknown id
	req, err := http.NewRequest("DELETE", ts.URL+"/devices/123", nil)
	tests.Assert(t, err == nil)
	r, err := http.DefaultClient.Do(req)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusNotFound)

	// Delete device without a cluster there.. that's probably a really
	// bad situation
	req, err = http.NewRequest("DELETE", ts.URL+"/devices/"+device.Info.Id, nil)
	tests.Assert(t, err == nil)
	r, err = http.DefaultClient.Do(req)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusNotFound)

}

*/
