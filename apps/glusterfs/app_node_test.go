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
	// "fmt"
	//"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	"github.com/heketi/heketi/tests"
	"github.com/heketi/heketi/utils"
	// "io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
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

	// Post bad JSON
	r, err = http.Post(ts.URL+"/nodes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusNotFound, *r)
}
