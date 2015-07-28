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
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func init() {
	// turn off logging
	logger.SetLevel(utils.LEVEL_NOLOG)
}

func TestVolumeCreateBadJson(t *testing.T) {
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

	// VolumeCreate JSON Request
	request := []byte(`{
        asdfsdf
    }`)

	// Send request
	r, err := http.Post(ts.URL+"/volumes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == 422)
}

func TestVolumeCreateNoTopology(t *testing.T) {
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

	// VolumeCreate JSON Request
	request := []byte(`{
        "size" : 100
    }`)

	// Send request
	r, err := http.Post(ts.URL+"/volumes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusBadRequest)
}

func TestVolumeCreateSmallSize(t *testing.T) {
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

	// VolumeCreate JSON Request
	request := []byte(`{
        "size" : 0
    }`)

	// Send request
	r, err := http.Post(ts.URL+"/volumes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusBadRequest)
	body, err := ioutil.ReadAll(io.LimitReader(r.Body, r.ContentLength))
	tests.Assert(t, err == nil)
	r.Body.Close()
	tests.Assert(t, strings.Contains(string(body), "Invalid volume size"))
}

func TestVolumeCreateBadClusters(t *testing.T) {
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

	// Create a cluster
	// Setup database
	err := setupSampleDbWithTopology(app.db,
		1,    // clusters
		10,   // nodes_per_cluster
		10,   // devices_per_node,
		5*TB, // disksize)
	)
	tests.Assert(t, err == nil)

	// VolumeCreate JSON Request
	request := []byte(`{
        "size" : 10,
        "clusters" : [
            "bad"
        ]
    }`)

	// Send request
	r, err := http.Post(ts.URL+"/volumes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusBadRequest)
	body, err := ioutil.ReadAll(io.LimitReader(r.Body, r.ContentLength))
	tests.Assert(t, err == nil)
	r.Body.Close()
	tests.Assert(t, strings.Contains(string(body), "Cluster id bad not found"))
}

func TestVolumeCreateBadSnapshotFactor(t *testing.T) {
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

	// Create JSON with missing factor
	request := []byte(`{
        "size" : 100,
        "snapshot" : {
            "enable" : true
        }
    }`)

	// Send request
	r, err := http.Post(ts.URL+"/volumes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusBadRequest)
	body, err := ioutil.ReadAll(io.LimitReader(r.Body, r.ContentLength))
	tests.Assert(t, err == nil)
	r.Body.Close()
	tests.Assert(t, strings.Contains(string(body), "Invalid snapshot factor"))

	// Create JSON with large invalid factor
	request = []byte(`{
        "size" : 100,
        "snapshot" : {
            "enable" : true,
            "factor" : 101
        }
    }`)

	// Send request
	r, err = http.Post(ts.URL+"/volumes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusBadRequest)
	body, err = ioutil.ReadAll(io.LimitReader(r.Body, r.ContentLength))
	tests.Assert(t, err == nil)
	r.Body.Close()
	tests.Assert(t, strings.Contains(string(body), "Invalid snapshot factor"))

	// Create JSON with small invalid factor
	request = []byte(`{
        "size" : 100,
        "snapshot" : {
            "enable" : true,
            "factor" : 0.1
        }
    }`)

	// Send request
	r, err = http.Post(ts.URL+"/volumes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusBadRequest)
	body, err = ioutil.ReadAll(io.LimitReader(r.Body, r.ContentLength))
	tests.Assert(t, err == nil)
	r.Body.Close()
	tests.Assert(t, strings.Contains(string(body), "Invalid snapshot factor"))

}

func TestVolumeCreate(t *testing.T) {
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

	// Setup database
	err := setupSampleDbWithTopology(app.db,
		1,    // clusters
		10,   // nodes_per_cluster
		10,   // devices_per_node,
		5*TB, // disksize)
	)
	tests.Assert(t, err == nil)

	// VolumeCreate JSON Request
	request := []byte(`{
        "size" : 100
    }`)

	// Send request
	r, err := http.Post(ts.URL+"/volumes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	location, err := r.Location()
	tests.Assert(t, err == nil)

	// Query queue until finished
	var info VolumeInfoResponse
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
			err = utils.GetJsonFromResponse(r, &info)
			tests.Assert(t, err == nil)
			break
		}
	}
	tests.Assert(t, info.Id != "")
	tests.Assert(t, info.Cluster != "")
	tests.Assert(t, len(info.Bricks) == 2*DEFAULT_REPLICA) // Only two 50GB bricks needed
	tests.Assert(t, info.Bricks[0].Size == 50*GB)
	tests.Assert(t, info.Bricks[1].Size == 50*GB)
	tests.Assert(t, info.Name == "vol_"+info.Id)
	tests.Assert(t, info.Replica == DEFAULT_REPLICA)
	tests.Assert(t, info.Snapshot.Enable == false)
	tests.Assert(t, info.Snapshot.Factor == 1)
}

func TestVolumeInfoIdNotFound(t *testing.T) {
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

	// Now that we have some data in the database, we can
	// make a request for the clutser list
	r, err := http.Get(ts.URL + "/volumes/12345")
	tests.Assert(t, r.StatusCode == http.StatusNotFound)
	tests.Assert(t, err == nil)
}

func TestVolumeInfo(t *testing.T) {
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

	// Setup database
	err := setupSampleDbWithTopology(app.db,
		1,    // clusters
		10,   // nodes_per_cluster
		10,   // devices_per_node,
		5*TB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create a volume
	v := createSampleVolumeEntry(100)
	tests.Assert(t, v != nil)
	err = app.db.Update(func(tx *bolt.Tx) error {
		return v.Save(tx)
	})
	tests.Assert(t, err == nil)
	err = v.Create(app.db)
	tests.Assert(t, err == nil)

	// Now that we have some data in the database, we can
	// make a request for the clutser list
	r, err := http.Get(ts.URL + "/volumes/" + v.Info.Id)
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

	// Read response
	var msg VolumeInfoResponse
	err = utils.GetJsonFromResponse(r, &msg)
	tests.Assert(t, err == nil)

	tests.Assert(t, msg.Id == v.Info.Id)
	tests.Assert(t, msg.Cluster == v.Info.Cluster)
	tests.Assert(t, msg.Name == v.Info.Name)
	tests.Assert(t, msg.Replica == v.Info.Replica)
	tests.Assert(t, msg.Size == v.Info.Size)
	tests.Assert(t, reflect.DeepEqual(msg.Snapshot, v.Info.Snapshot))
	for _, brick := range msg.Bricks {
		tests.Assert(t, utils.SortedStringHas(v.Bricks, brick.Id))
	}
}

func TestVolumeListEmpty(t *testing.T) {
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

	// Get volumes, there should be none
	r, err := http.Get(ts.URL + "/volumes")
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

	// Read response
	var msg VolumeListResponse
	err = utils.GetJsonFromResponse(r, &msg)
	tests.Assert(t, err == nil)
	tests.Assert(t, len(msg.Volumes) == 0)
}

func TestVolumeList(t *testing.T) {
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

	// Create some volumes
	numvolumes := 10
	err := app.db.Update(func(tx *bolt.Tx) error {

		for i := 0; i < numvolumes; i++ {
			v := createSampleVolumeEntry(100)
			err := v.Save(tx)
			if err != nil {
				return err
			}
		}

		return nil

	})
	tests.Assert(t, err == nil)

	// Get volumes, there should be none
	r, err := http.Get(ts.URL + "/volumes")
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

	// Read response
	var msg VolumeListResponse
	err = utils.GetJsonFromResponse(r, &msg)
	tests.Assert(t, err == nil)
	tests.Assert(t, len(msg.Volumes) == numvolumes)

	// Check that all the volumes are in the database
	err = app.db.View(func(tx *bolt.Tx) error {
		for _, id := range msg.Volumes {
			_, err := NewVolumeEntryFromId(tx, id)
			if err != nil {
				return err
			}
		}

		return nil
	})
	tests.Assert(t, err == nil)

}

/*

func TestVolumeList(t *testing.T) {
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

    // Save some objects in the database
    numvolumes := 5
    err := app.db.Update(func(tx *bolt.Tx) error {
        b := tx.Bucket([]byte(BOLTDB_BUCKET_CLUSTER))
        if b == nil {
            return errors.New("Unable to open bucket")
        }

        for i := 0; i < numvolumes; i++ {
            var entry VolumeEntry

            entry.Info.Id = fmt.Sprintf("%v", 5000+i)
            buffer, err := entry.Marshal()
            if err != nil {
                return err
            }

            err = b.Put([]byte(entry.Info.Id), buffer)
            if err != nil {
                return err
            }
        }

        return nil

    })
    tests.Assert(t, err == nil)

    // Now that we have some data in the database, we can
    // make a request for the clutser list
    r, err := http.Get(ts.URL + "/volumes")
    tests.Assert(t, r.StatusCode == http.StatusOK)
    tests.Assert(t, err == nil)
    tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

    // Read response
    var msg VolumeListResponse
    err = utils.GetJsonFromResponse(r, &msg)
    tests.Assert(t, err == nil)

    // Thanks to BoltDB they come back in order
    mockid := 5000 // This is the mock id value we set above
    for _, id := range msg.Volumes {
        tests.Assert(t, id == fmt.Sprintf("%v", mockid))
        mockid++
    }
}

func TestVolumeInfoIdNotFound(t *testing.T) {
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

    // Now that we have some data in the database, we can
    // make a request for the clutser list
    r, err := http.Get(ts.URL + "/volumes/12345")
    tests.Assert(t, r.StatusCode == http.StatusNotFound)
    tests.Assert(t, err == nil)
}

func TestVolumeInfo(t *testing.T) {
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

    // Create a new VolumeInfo
    entry := NewVolumeEntry()
    entry.Info.Id = "123"
    for _, node := range []string{"a1", "a2", "a3"} {
        entry.NodeAdd(node)
    }
    for _, vol := range []string{"b1", "b2", "b3"} {
        entry.VolumeAdd(vol)
    }

    // Save the info in the database
    err := app.db.Update(func(tx *bolt.Tx) error {
        b := tx.Bucket([]byte(BOLTDB_BUCKET_CLUSTER))
        if b == nil {
            return errors.New("Unable to open bucket")
        }

        buffer, err := entry.Marshal()
        if err != nil {
            return err
        }

        err = b.Put([]byte(entry.Info.Id), buffer)
        if err != nil {
            return err
        }

        return nil

    })
    tests.Assert(t, err == nil)

    // Now that we have some data in the database, we can
    // make a request for the clutser list
    r, err := http.Get(ts.URL + "/volumes/" + "123")
    tests.Assert(t, r.StatusCode == http.StatusOK)
    tests.Assert(t, err == nil)
    tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

    // Read response
    var msg VolumeInfoResponse
    err = utils.GetJsonFromResponse(r, &msg)
    tests.Assert(t, err == nil)

    // Check values are equal
    tests.Assert(t, entry.Info.Id == msg.Id)
    tests.Assert(t, entry.Info.Volumes[0] == msg.Volumes[0])
    tests.Assert(t, entry.Info.Volumes[1] == msg.Volumes[1])
    tests.Assert(t, entry.Info.Volumes[2] == msg.Volumes[2])
    tests.Assert(t, entry.Info.Nodes[0] == msg.Nodes[0])
    tests.Assert(t, entry.Info.Nodes[1] == msg.Nodes[1])
    tests.Assert(t, entry.Info.Nodes[2] == msg.Nodes[2])
}

func TestVolumeDelete(t *testing.T) {
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

    // Create an entry with volumes and nodes
    entries := make([]*VolumeEntry, 0)
    entry := NewVolumeEntry()
    entry.Info.Id = "a1"
    for _, node := range []string{"a1", "a2", "a3"} {
        entry.NodeAdd(node)
    }
    for _, vol := range []string{"b1", "b2", "b3"} {
        entry.VolumeAdd(vol)
    }
    entries = append(entries, entry)

    // Create an entry with only volumes
    entry = NewVolumeEntry()
    entry.Info.Id = "a2"
    for _, vol := range []string{"b1", "b2", "b3"} {
        entry.VolumeAdd(vol)
    }
    entries = append(entries, entry)

    // Create an entry with only nodes
    entry = NewVolumeEntry()
    entry.Info.Id = "a3"
    for _, node := range []string{"a1", "a2", "a3"} {
        entry.NodeAdd(node)
    }
    entries = append(entries, entry)

    // Create an empty entry
    entry = NewVolumeEntry()
    entry.Info.Id = "000"
    entries = append(entries, entry)

    // Save the info in the database
    err := app.db.Update(func(tx *bolt.Tx) error {
        b := tx.Bucket([]byte(BOLTDB_BUCKET_CLUSTER))
        if b == nil {
            return errors.New("Unable to open bucket")
        }

        for _, entry := range entries {
            buffer, err := entry.Marshal()
            if err != nil {
                return err
            }

            err = b.Put([]byte(entry.Info.Id), buffer)
            if err != nil {
                return err
            }
        }

        return nil

    })
    tests.Assert(t, err == nil)

    // Check that we cannot delete a cluster with elements
    req, err := http.NewRequest("DELETE", ts.URL+"/volumes/"+"a1", nil)
    tests.Assert(t, err == nil)
    r, err := http.DefaultClient.Do(req)
    tests.Assert(t, err == nil)
    tests.Assert(t, r.StatusCode == http.StatusConflict)

    // Check that we cannot delete a cluster with volumes
    req, err = http.NewRequest("DELETE", ts.URL+"/volumes/"+"a2", nil)
    tests.Assert(t, err == nil)
    r, err = http.DefaultClient.Do(req)
    tests.Assert(t, err == nil)
    tests.Assert(t, r.StatusCode == http.StatusConflict)

    // Check that we cannot delete a cluster with nodes
    req, err = http.NewRequest("DELETE", ts.URL+"/volumes/"+"a3", nil)
    tests.Assert(t, err == nil)
    r, err = http.DefaultClient.Do(req)
    tests.Assert(t, err == nil)
    tests.Assert(t, r.StatusCode == http.StatusConflict)

    // Delete cluster with no elements
    req, err = http.NewRequest("DELETE", ts.URL+"/volumes/"+"000", nil)
    tests.Assert(t, err == nil)
    r, err = http.DefaultClient.Do(req)
    tests.Assert(t, err == nil)
    tests.Assert(t, r.StatusCode == http.StatusOK)

    // Check database still has a1,a2, and a3, but not '000'
    err = app.db.View(func(tx *bolt.Tx) error {
        b := tx.Bucket([]byte(BOLTDB_BUCKET_CLUSTER))
        if b == nil {
            return errors.New("Unable to open bucket")
        }

        // Check that the ids are still in the database
        for _, id := range []string{"a1", "a2", "a3"} {
            buffer := b.Get([]byte(id))
            if buffer == nil {
                return errors.New(fmt.Sprintf("Id %v not found", id))
            }
        }

        // Check that the id 000 is no longer in the database
        buffer := b.Get([]byte("000"))
        if buffer != nil {
            return errors.New(fmt.Sprintf("Id 000 still in database and was deleted"))
        }

        return nil

    })
    tests.Assert(t, err == nil, err)

}
*/
