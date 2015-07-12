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
	//"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	"github.com/heketi/heketi/tests"
	"github.com/heketi/heketi/utils"
	// "io/ioutil"
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

func TestNodeAddBadRequests(t *testing.T) {
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
	r, err := http.Post(ts.URL+"/nodes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == 422)

	// Make a request without hostnames
	request = []byte(`{
		"cluster" : "123",
		"hostname" : {}
    }`)

	// Post bad JSON
	r, err = http.Post(ts.URL+"/nodes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusBadRequest)

	// Make a request with only manage hostname
	request = []byte(`{
		"cluster" : "123",
		"hostnames" : {
			"manage" : [ "manage.hostname.com" ]
		} 
    }`)

	// Post bad JSON
	r, err = http.Post(ts.URL+"/nodes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusBadRequest)

	// Make a request with only storage hostname
	request = []byte(`{
		"cluster" : "123",
		"hostnames" : {
			"storage" : [ "storage.hostname.com" ]
		} 
    }`)

	// Post bad JSON
	r, err = http.Post(ts.URL+"/nodes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusBadRequest)

	// Make a request where the cluster id does not exist
	request = []byte(`{
		"cluster" : "123",
		"hostnames" : {
			"storage" : [ "storage.hostname.com" ],
			"manage" : [ "manage.hostname.com"  ]
		}, 
		"zone" : 10
    }`)

	// Check that it returns that the cluster id is not found
	r, err = http.Post(ts.URL+"/nodes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusNotFound)
}

func TestNodeAdd(t *testing.T) {
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
    }`)

	// Post nothing
	r, err := http.Post(ts.URL+"/clusters", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusCreated)
	tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

	// Read cluster information
	var cluster ClusterInfoResponse
	err = utils.GetJsonFromResponse(r, &cluster)
	tests.Assert(t, err == nil)

	// Create node on this cluster
	request = []byte(fmt.Sprintf(`{
		"cluster" : "%v",
		"hostnames" : {
			"storage" : [ "storage.hostname.com" ],
			"manage" : [ "manage.hostname.com"  ]
		}, 
		"zone" : 1
    }`, cluster.Id))

	// Post nothing
	r, err = http.Post(ts.URL+"/nodes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	location, err := r.Location()
	tests.Assert(t, err == nil)

	// Query queue until finished
	var node NodeInfoResponse
	for {
		r, err = http.Get(location.String())
		tests.Assert(t, err == nil)
		tests.Assert(t, r.StatusCode == http.StatusOK)
		if r.ContentLength <= 0 {
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
	tests.Assert(t, len(node.Id) > 0)
	tests.Assert(t, len(node.Hostnames.Manage) == 1)
	tests.Assert(t, len(node.Hostnames.Storage) == 1)
	tests.Assert(t, node.Hostnames.Manage[0] == "manage.hostname.com")
	tests.Assert(t, node.Hostnames.Storage[0] == "storage.hostname.com")
	tests.Assert(t, node.Zone == 1)
	tests.Assert(t, node.ClusterId == cluster.Id)
	tests.Assert(t, len(node.DevicesInfo) == 0)

	// Check Cluster has node
	r, err = http.Get(ts.URL + "/clusters/" + cluster.Id)
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

	err = utils.GetJsonFromResponse(r, &cluster)
	tests.Assert(t, len(cluster.Nodes) == 1)
	tests.Assert(t, cluster.Nodes[0] == node.Id)

	// Check the data is in the database correctly
	var entry *NodeEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		entry, err = NewNodeEntryFromId(tx, node.Id)
		return err
	})
	tests.Assert(t, err == nil)
	tests.Assert(t, entry != nil)
	tests.Assert(t, entry.Info.Id == node.Id)
	tests.Assert(t, len(entry.Info.Hostnames.Manage) == 1)
	tests.Assert(t, len(entry.Info.Hostnames.Storage) == 1)
	tests.Assert(t, entry.Info.Hostnames.Manage[0] == node.Hostnames.Manage[0])
	tests.Assert(t, entry.Info.Hostnames.Storage[0] == node.Hostnames.Storage[0])
	tests.Assert(t, len(entry.Devices) == 0)
}
