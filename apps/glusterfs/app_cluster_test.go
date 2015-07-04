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
	"encoding/gob"
	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/tests"
	"github.com/heketi/heketi/utils"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func init() {
	// turn off logging
	logger.SetLevel(utils.LEVEL_NOLOG)
}

func TestClusterCreateEmptyRequest(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Patch dbfilename so that it is restored at the end of the tests
	defer tests.Patch(&dbfilename, tmpfile).Restore()

	// Create the app
	app := NewApp()
	defer app.Close()

	// Setup the server
	ts := httptest.NewServer(http.HandlerFunc(app.ClusterCreate))
	defer ts.Close()

	r, err := http.Get(ts.URL)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusCreated)

	// Read JSON
	var msg ClusterInfoResponse
	err = utils.GetJsonFromResponse(r, &msg)
	tests.Assert(t, err == nil)

	// Test JSON
	tests.Assert(t, msg.Id == msg.Name)
	tests.Assert(t, len(msg.Nodes) == 0)
	tests.Assert(t, len(msg.Volumes) == 0)

	// Check that the data on the database is recorded correctly
	var entrybytes []byte
	err = app.db.View(func(tx *bolt.Tx) error {
		entrybytes = tx.Bucket([]byte(BOLTDB_BUCKET_CLUSTER)).Get([]byte(msg.Id))
		return nil
	})
	tests.Assert(t, err == nil)

	// Unmarshal
	var entry ClusterEntry
	dec := gob.NewDecoder(bytes.NewReader(entrybytes))
	err = dec.Decode(&entry)
	tests.Assert(t, err == nil)

	// Make sure they entries are euqal
	tests.Assert(t, entry.Info.Id == msg.Id)
	tests.Assert(t, entry.Info.Name == msg.Name)
	tests.Assert(t, len(entry.Info.Volumes) == 0)
	tests.Assert(t, len(entry.Info.Nodes) == 0)
}

func TestClusterCreateWithName(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Patch dbfilename so that it is restored at the end of the tests
	defer tests.Patch(&dbfilename, tmpfile).Restore()

	// Create the app
	app := NewApp()
	defer app.Close()

	// Setup the server
	ts := httptest.NewServer(http.HandlerFunc(app.ClusterCreate))
	defer ts.Close()

	// ClusterCreate JSON Request
	request := []byte(`{
        "name" : "test_name"
    }`)

	// Request
	r, err := http.Post(ts.URL, "application/json", bytes.NewBuffer(request))
	tests.Assert(t, r.StatusCode == http.StatusCreated)
	tests.Assert(t, err == nil)

	// Read JSON
	var msg ClusterInfoResponse
	err = utils.GetJsonFromResponse(r, &msg)
	tests.Assert(t, err == nil)

	// Test JSON
	tests.Assert(t, msg.Id != msg.Name)
	tests.Assert(t, "test_name" == msg.Name)
	tests.Assert(t, len(msg.Nodes) == 0)
	tests.Assert(t, len(msg.Volumes) == 0)

	// Check that the data on the database is recorded correctly
	var entrybytes []byte
	err = app.db.View(func(tx *bolt.Tx) error {
		entrybytes = tx.Bucket([]byte(BOLTDB_BUCKET_CLUSTER)).Get([]byte(msg.Id))
		return nil
	})
	tests.Assert(t, err == nil)

	// Unmarshal
	var entry ClusterEntry
	dec := gob.NewDecoder(bytes.NewReader(entrybytes))
	err = dec.Decode(&entry)
	tests.Assert(t, err == nil)

	// Make sure they entries are euqal
	tests.Assert(t, entry.Info.Id == msg.Id)
	tests.Assert(t, entry.Info.Name == msg.Name)
	tests.Assert(t, len(entry.Info.Volumes) == 0)
	tests.Assert(t, len(entry.Info.Nodes) == 0)
}
