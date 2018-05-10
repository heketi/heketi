//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"bytes"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	client "github.com/heketi/heketi/client/api/go-client"
	"github.com/heketi/heketi/pkg/utils"
	"github.com/heketi/tests"
)

func TestAppBadConfigData(t *testing.T) {
	data := []byte(`{ bad json }`)
	app := NewApp(bytes.NewBuffer(data))
	tests.Assert(t, app == nil)

	data = []byte(`{}`)
	app = NewApp(bytes.NewReader(data))
	tests.Assert(t, app == nil)

	data = []byte(`{
		"glusterfs" : {}
		}`)
	app = NewApp(bytes.NewReader(data))
	tests.Assert(t, app == nil)
}

func TestAppUnknownExecutorInConfig(t *testing.T) {
	data := []byte(`{
		"glusterfs" : {
			"executor" : "unknown value here"
		}
		}`)
	app := NewApp(bytes.NewReader(data))
	tests.Assert(t, app == nil)
}

func TestAppUnknownAllocatorInConfig(t *testing.T) {
	data := []byte(`{
		"glusterfs" : {
			"allocator" : "unknown value here"
		}
		}`)
	app := NewApp(bytes.NewReader(data))
	tests.Assert(t, app == nil)
}

func TestAppBadDbLocation(t *testing.T) {
	data := []byte(`{
		"glusterfs" : {
			"db" : "/badlocation"
		}
	}`)
	app := NewApp(bytes.NewReader(data))
	tests.Assert(t, app == nil)
}

func TestAppAdvsettings(t *testing.T) {

	dbfile := tests.Tempfile()
	defer os.Remove(dbfile)
	os.Setenv("HEKETI_EXECUTOR", "mock")
	defer os.Unsetenv("HEKETI_EXECUTOR")
	os.Setenv("HEKETI_DB_PATH", dbfile)
	defer os.Unsetenv("HEKETI_DB_PATH")

	data := []byte(`{
		"glusterfs" : {
			"executor" : "crazyexec",
			"allocator" : "simple",
			"db" : "/path/to/nonexistent/heketi.db",
			"brick_max_size_gb" : 1024,
			"brick_min_size_gb" : 4,
			"max_bricks_per_volume" : 33
		}
	}`)

	bmax, bmin, bnum := BrickMaxSize, BrickMinSize, BrickMaxNum
	defer func() {
		BrickMaxSize, BrickMinSize, BrickMaxNum = bmax, bmin, bnum
	}()

	app := NewApp(bytes.NewReader(data))
	defer app.Close()
	tests.Assert(t, app != nil)
	tests.Assert(t, app.conf.Executor == "mock")
	tests.Assert(t, app.conf.DBfile == dbfile)
	tests.Assert(t, BrickMaxNum == 33)
	tests.Assert(t, BrickMaxSize == 1*TB)
	tests.Assert(t, BrickMinSize == 4*GB)
}

func TestAppLogLevel(t *testing.T) {
	dbfile := tests.Tempfile()
	defer os.Remove(dbfile)

	levels := []string{
		"none",
		"critical",
		"error",
		"warning",
		"info",
		"debug",
	}

	logger.SetLevel(utils.LEVEL_DEBUG)
	for _, level := range levels {
		data := []byte(`{
			"glusterfs" : {
				"executor" : "mock",
				"allocator" : "simple",
				"db" : "` + dbfile + `",
				"loglevel" : "` + level + `"
			}
		}`)

		app := NewApp(bytes.NewReader(data))
		tests.Assert(t, app != nil, level, string(data))

		switch level {
		case "none":
			tests.Assert(t, logger.Level() == utils.LEVEL_NOLOG)
		case "critical":
			tests.Assert(t, logger.Level() == utils.LEVEL_CRITICAL)
		case "error":
			tests.Assert(t, logger.Level() == utils.LEVEL_ERROR)
		case "warning":
			tests.Assert(t, logger.Level() == utils.LEVEL_WARNING)
		case "info":
			tests.Assert(t, logger.Level() == utils.LEVEL_INFO)
		case "debug":
			tests.Assert(t, logger.Level() == utils.LEVEL_DEBUG)
		}
		app.Close()
	}

	// Test that an unknown value does not change the loglevel
	logger.SetLevel(utils.LEVEL_NOLOG)
	data := []byte(`{
			"glusterfs" : {
				"executor" : "mock",
				"allocator" : "simple",
				"db" : "` + dbfile + `",
				"loglevel" : "blah"
			}
		}`)

	app := NewApp(bytes.NewReader(data))
	defer app.Close()
	tests.Assert(t, app != nil)
	tests.Assert(t, logger.Level() == utils.LEVEL_NOLOG)
}

func TestAppReadOnlyDb(t *testing.T) {

	dbfile := tests.Tempfile()
	defer os.Remove(dbfile)

	// First, create a db
	data := []byte(`{
		"glusterfs": {
			"executor" : "mock",
			"db" : "` + dbfile + `"
		}
	}`)
	app := NewApp(bytes.NewReader(data))
	tests.Assert(t, app != nil)
	tests.Assert(t, app.dbReadOnly == false)
	app.Close()

	// Now open it again here.  This will force NewApp()
	// to be unable to open RW.
	db, err := bolt.Open(dbfile, 0666, &bolt.Options{
		ReadOnly: true,
	})
	tests.Assert(t, err == nil, err)
	tests.Assert(t, db != nil)

	// Now open it again and notice how it opened
	app = NewApp(bytes.NewReader(data))
	defer app.Close()
	tests.Assert(t, app != nil)
	tests.Assert(t, app.dbReadOnly == true)
}

func TestAppPathNotFound(t *testing.T) {
	dbfile := tests.Tempfile()
	defer os.Remove(dbfile)

	app := NewTestApp(dbfile)
	tests.Assert(t, app != nil)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Setup a new client
	c := client.NewClientNoAuth(ts.URL)

	// Test paths which do not match the hexadecimal id
	_, err := c.ClusterInfo("xxx")
	tests.Assert(t, strings.Contains(err.Error(), "Invalid path or request"))

	_, err = c.NodeInfo("xxx")
	tests.Assert(t, strings.Contains(err.Error(), "Invalid path or request"))

	_, err = c.VolumeInfo("xxx")
	tests.Assert(t, strings.Contains(err.Error(), "Invalid path or request"))
}

func TestAppBlockSettings(t *testing.T) {

	dbfile := tests.Tempfile()
	defer os.Remove(dbfile)
	os.Setenv("HEKETI_EXECUTOR", "mock")
	defer os.Unsetenv("HEKETI_EXECUTOR")
	os.Setenv("HEKETI_DB_PATH", dbfile)
	defer os.Unsetenv("HEKETI_DB_PATH")

	data := []byte(`{
		"glusterfs" : {
			"executor" : "crazyexec",
			"allocator" : "simple",
			"db" : "/path/to/nonexistent/heketi.db",
			"auto_create_block_hosting_volume" : true,
			"block_hosting_volume_size" : 500
		}
	}`)

	blockauto, blocksize := CreateBlockHostingVolumes, BlockHostingVolumeSize
	defer func() {
		CreateBlockHostingVolumes, BlockHostingVolumeSize = blockauto, blocksize
	}()

	app := NewApp(bytes.NewReader(data))
	defer app.Close()
	tests.Assert(t, app != nil)
	tests.Assert(t, app.conf.Executor == "mock")
	tests.Assert(t, app.conf.DBfile == dbfile)
	tests.Assert(t, CreateBlockHostingVolumes == true)
	tests.Assert(t, BlockHostingVolumeSize == 500)
}

func TestCannotStartWhenPendingOperations(t *testing.T) {
	dbfile := tests.Tempfile()
	defer os.Remove(dbfile)

	// create a app that will only be used to set up the test
	app := NewTestApp(dbfile)
	tests.Assert(t, app != nil)

	// populate the db with a "dummy" pending op entry. this should
	// trigger a panic the next time an app is instantiated
	err := app.db.Update(func(tx *bolt.Tx) error {
		op := NewPendingOperationEntry(NEW_ID)
		op.Type = OperationCreateVolume
		op.Save(tx)
		return nil
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	app.Close()

	defer func() {
		// check that we (a) panicked (b) had the right error message
		r := recover()
		tests.Assert(t, r != nil, "expected r != nil, got:", r)
		tests.Assert(t,
			strings.Contains(r.(error).Error(), "pending operations are present"),
			`expected "pending operations are present" in r.Error(), got:`,
			r.(error).Error())
	}()
	// now creating a new app should panic
	app = NewTestApp(dbfile)

	t.Fatalf("Test should not reach this line")
}

func TestCanStartWhenPendingOperationsIgnored(t *testing.T) {
	dbfile := tests.Tempfile()
	defer os.Remove(dbfile)

	// create a app that will only be used to set up the test
	app := NewTestApp(dbfile)
	tests.Assert(t, app != nil)

	// populate the db with a "dummy" pending op entry.
	// without the environment var we're setting later
	// this would trigger a panic
	err := app.db.Update(func(tx *bolt.Tx) error {
		op := NewPendingOperationEntry(NEW_ID)
		op.Type = OperationCreateVolume
		op.Save(tx)
		return nil
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	app.Close()

	// now creating a new app should NOT panic
	os.Setenv("HEKETI_IGNORE_STALE_OPERATIONS", "1")
	defer os.Unsetenv("HEKETI_IGNORE_STALE_OPERATIONS")
	app = NewTestApp(dbfile)
	tests.Assert(t, app != nil, "expected app != nil, got:", app)
}
