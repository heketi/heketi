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
	"github.com/heketi/heketi/executors"
	"github.com/heketi/tests"
	"github.com/heketi/utils"
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

	// Create the app
	app := NewTestApp(tmpfile)
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

	// Make a request with no device
	request = []byte(`{
        "node" : "123"
    }`)

	// Post bad JSON
	r, err = http.Post(ts.URL+"/devices", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusBadRequest)

	// Make a request with unknown node
	request = []byte(`{
        "node" : "123",
        "name" : "/dev/fake"
    }`)

	// Post unknown node
	r, err = http.Post(ts.URL+"/devices", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusNotFound)

}

func TestDeviceAddDelete(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Add Cluster then a Node on the cluster
	// node
	cluster := NewClusterEntryFromRequest()
	nodereq := &NodeAddRequest{
		ClusterId: cluster.Info.Id,
		Hostnames: HostAddresses{
			Manage:  []string{"manage"},
			Storage: []string{"storage"},
		},
		Zone: 99,
	}
	node := NewNodeEntryFromRequest(nodereq)
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

	// Create a request to a device
	request := []byte(`{
        "node" : "` + node.Info.Id + `",
        "name" : "/dev/fake1"
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
		} else {
			tests.Assert(t, r.StatusCode == http.StatusNoContent)
			break
		}
	}

	// Add the same device.  It should conflict
	r, err = http.Post(ts.URL+"/devices", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusConflict)

	// Add a second device
	request = []byte(`{
        "node" : "` + node.Info.Id + `",
        "name" : "/dev/fake2"
    }`)

	// Add device using POST
	r, err = http.Post(ts.URL+"/devices", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	location, err = r.Location()
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
	tests.Assert(t, len(val.Bricks) == 0)

	val, ok = devicemap["/dev/fake2"]
	tests.Assert(t, ok)
	tests.Assert(t, val.Info.Name == "/dev/fake2")
	tests.Assert(t, len(val.Bricks) == 0)

	// Add some bricks to check if delete conflicts works
	fakeid := devicemap["/dev/fake1"].Info.Id
	err = app.db.Update(func(tx *bolt.Tx) error {
		device, err := NewDeviceEntryFromId(tx, fakeid)
		if err != nil {
			return err
		}

		device.BrickAdd("123")
		device.BrickAdd("456")
		return device.Save(tx)
	})
	tests.Assert(t, err == nil)

	// Now delete device and check for conflict
	req, err := http.NewRequest("DELETE", ts.URL+"/devices/"+fakeid, nil)
	tests.Assert(t, err == nil)
	r, err = http.DefaultClient.Do(req)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusConflict)

	// Check the db is still intact
	err = app.db.View(func(tx *bolt.Tx) error {
		device, err := NewDeviceEntryFromId(tx, fakeid)
		if err != nil {
			return err
		}

		node, err = NewNodeEntryFromId(tx, device.NodeId)
		if err != nil {
			return err
		}

		return nil
	})
	tests.Assert(t, err == nil)
	tests.Assert(t, utils.SortedStringHas(node.Devices, fakeid))

	// Node delete bricks from the device
	err = app.db.Update(func(tx *bolt.Tx) error {
		device, err := NewDeviceEntryFromId(tx, fakeid)
		if err != nil {
			return err
		}

		device.BrickDelete("123")
		device.BrickDelete("456")
		return device.Save(tx)
	})
	tests.Assert(t, err == nil)

	// Delete device
	req, err = http.NewRequest("DELETE", ts.URL+"/devices/"+fakeid, nil)
	tests.Assert(t, err == nil)
	r, err = http.DefaultClient.Do(req)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	location, err = r.Location()
	tests.Assert(t, err == nil)

	// Wait for deletion
	for {
		r, err := http.Get(location.String())
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

	// Check db
	err = app.db.View(func(tx *bolt.Tx) error {
		_, err := NewDeviceEntryFromId(tx, fakeid)
		return err
	})
	tests.Assert(t, err == ErrNotFound)

	// Check node does not have the device
	err = app.db.View(func(tx *bolt.Tx) error {
		node, err = NewNodeEntryFromId(tx, node.Info.Id)
		return err
	})
	tests.Assert(t, err == nil)
	tests.Assert(t, !utils.SortedStringHas(node.Devices, fakeid))

	// Check the registration of the device has been removed,
	// and the device can be added again
	request = []byte(`{
        "node" : "` + node.Info.Id + `",
        "name" : "/dev/fake1"
    }`)
	r, err = http.Post(ts.URL+"/devices", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	location, err = r.Location()
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

func TestDeviceAddCleansUp(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Add Cluster then a Node on the cluster
	// node
	cluster := NewClusterEntryFromRequest()
	nodereq := &NodeAddRequest{
		ClusterId: cluster.Info.Id,
		Hostnames: HostAddresses{
			Manage:  []string{"manage"},
			Storage: []string{"storage"},
		},
		Zone: 99,
	}
	node := NewNodeEntryFromRequest(nodereq)
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

	// Mock the device setup to return an error, which will
	// cause the cleanup.
	deviceSetupFn := app.xo.MockDeviceSetup
	app.xo.MockDeviceSetup = func(host, device, vgid string) (*executors.DeviceInfo, error) {
		return nil, ErrDbAccess
	}

	// Create a request to a device
	request := []byte(`{
        "node" : "` + node.Info.Id + `",
        "name" : "/dev/fake1"
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
		} else {
			tests.Assert(t, r.StatusCode != http.StatusNoContent)
			break
		}
	}

	// Let's reset the mocked function
	app.xo.MockDeviceSetup = deviceSetupFn

	// Now it should work
	// Add device using POST
	r, err = http.Post(ts.URL+"/devices", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	location, err = r.Location()
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

func TestDeviceInfoIdNotFound(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
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

	// Create the app
	app := NewTestApp(tmpfile)
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
	tests.Assert(t, info.Storage.Free == device.Info.Storage.Free)
	tests.Assert(t, info.Storage.Used == device.Info.Storage.Used)
	tests.Assert(t, info.Storage.Total == device.Info.Storage.Total)

}

func TestDeviceDeleteErrors(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
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
	device.NodeId = "def"
	device.StorageSet(10000)
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

	// Delete device without a node there.. that's probably a really
	// bad situation
	req, err = http.NewRequest("DELETE", ts.URL+"/devices/"+device.Info.Id, nil)
	tests.Assert(t, err == nil)
	r, err = http.DefaultClient.Do(req)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusInternalServerError)
}
