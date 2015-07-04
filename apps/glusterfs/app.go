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
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/rest"
	"github.com/heketi/heketi/utils"
	"net/http"
	"time"
)

const (
	ASYNC_ROUTE           = "/queue"
	BOLTDB_BUCKET_CLUSTER = "CLUSTER"
	BOLTDB_BUCKET_NODE    = "NODE"
	BOLTDB_BUCKET_VOLUME  = "VOLUME"
	BOLTDB_BUCKET_DEVICE  = "DEVICE"
	BOLTDB_BUCKET_BRICK   = "BRICK"
)

var (
	logger     = utils.NewLogger("[heketi]", utils.LEVEL_DEBUG)
	dbfilename = "heketi.db"
)

type App struct {
	asyncManager *rest.AsyncHttpManager
	db           *bolt.DB
}

func NewApp() *App {
	app := &App{}

	// Setup asynchronous manager
	app.asyncManager = rest.NewAsyncHttpManager(ASYNC_ROUTE)

	// Setup BoltDB database
	var err error
	app.db, err = bolt.Open(dbfilename, 0600, &bolt.Options{Timeout: 3 * time.Second})
	if err != nil {
		logger.Error("Unable to open database")
		return nil
	}

	err = app.db.Update(func(tx *bolt.Tx) error {
		// Create Cluster Bucket
		_, err := tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_CLUSTER))
		if err != nil {
			logger.Error("Unable to create cluster bucket in DB")
			return err
		}

		// Create Node Bucket
		_, err = tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_NODE))
		if err != nil {
			logger.Error("Unable to create cluster bucket in DB")
			return err
		}

		// Create Volume Bucket
		_, err = tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_VOLUME))
		if err != nil {
			logger.Error("Unable to create cluster bucket in DB")
			return err
		}

		// Create Device Bucket
		_, err = tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_DEVICE))
		if err != nil {
			logger.Error("Unable to create cluster bucket in DB")
			return err
		}

		// Create Brick Bucket
		_, err = tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_BRICK))
		if err != nil {
			logger.Error("Unable to create cluster bucket in DB")
			return err
		}

		return nil

	})
	if err != nil {
		logger.Err(err)
		return nil
	}

	logger.Info("GlusterFS Application Loaded")

	return app
}

// Register Routes
func (a *App) GetRoutes() rest.Routes {

	return rest.Routes{

		// HelloWorld
		rest.Route{"Hello", "GET", "/hello", a.Hello},

		// Asynchronous Manager
		rest.Route{"Async", "GET", ASYNC_ROUTE + "/{id:[A-Fa-f0-9]+}", a.asyncManager.HandlerStatus},

		// Cluster
		rest.Route{"ClusterCreate", "POST", "/clusters", a.ClusterCreate},
		rest.Route{"ClusterInfo", "GET", "/clusters/{id:[A-Fa-f0-9]+}", a.NotImplemented},
		rest.Route{"ClusterList", "GET", "/clusters", a.ClusterList},
		rest.Route{"ClusterDelete", "DELETE", "/clusters/{id:[A-Fa-f0-9]+}", a.NotImplemented},

		// Node
		rest.Route{"NodeAdd", "POST", "/nodes", a.NotImplemented},
		rest.Route{"NodeInfo", "GET", "/nodes/{id:[A-Fa-f0-9]+}", a.NotImplemented},
		rest.Route{"NodeDelete", "DELETE", "/nodes/{id:[A-Fa-f0-9]+}", a.NotImplemented},

		// Devices
		rest.Route{"DeviceAdd", "POST", "/devices", a.NotImplemented},
		rest.Route{"DeviceInfo", "GET", "/devices/{id:[A-Fa-f0-9]+}", a.NotImplemented},
		rest.Route{"DeviceDelete", "DELETE", "/devices/{id:[A-Fa-f0-9]+}", a.NotImplemented},

		// Volume
		rest.Route{"VolumeCreate", "POST", "/volumes", a.NotImplemented},
		rest.Route{"VolumeInfo", "GET", "/volumes/{id:[A-Fa-f0-9]+}", a.NotImplemented},
		rest.Route{"VolumeExpand", "POST", "/volumes/{id:[A-Fa-f0-9]+}/expand", a.NotImplemented},
		rest.Route{"VolumeDelete", "DELETE", "/volumes/{id:[A-Fa-f0-9]+}", a.NotImplemented},
		rest.Route{"VolumeList", "GET", "/volumes", a.NotImplemented},
	}
}

func (a *App) Close() {

	// Close the DB
	a.db.Close()
	logger.Info("Closed")
}

func (a *App) Hello(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "HelloWorld from GlusterFS Application")
}

func (a *App) NotImplemented(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Function not yet supported", http.StatusNotImplemented)
}
