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
	"github.com/gorilla/mux"
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
func (a *App) SetRoutes(router *mux.Router) error {

	routes := rest.Routes{

		// HelloWorld
		rest.Route{
			Name:        "Hello",
			Method:      "GET",
			Pattern:     "/hello",
			HandlerFunc: a.Hello},

		// Asynchronous Manager
		rest.Route{
			Name:        "Async",
			Method:      "GET",
			Pattern:     ASYNC_ROUTE + "/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.asyncManager.HandlerStatus},

		// Cluster
		rest.Route{
			Name:        "ClusterCreate",
			Method:      "POST",
			Pattern:     "/clusters",
			HandlerFunc: a.ClusterCreate},
		rest.Route{
			Name:        "ClusterInfo",
			Method:      "GET",
			Pattern:     "/clusters/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.NotImplemented},
		rest.Route{
			Name:        "ClusterList",
			Method:      "GET",
			Pattern:     "/clusters",
			HandlerFunc: a.ClusterList},
		rest.Route{
			Name:        "ClusterDelete",
			Method:      "DELETE",
			Pattern:     "/clusters/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.NotImplemented},

		// Node
		rest.Route{
			Name:        "NodeAdd",
			Method:      "POST",
			Pattern:     "/nodes",
			HandlerFunc: a.NotImplemented},
		rest.Route{
			Name:        "NodeInfo",
			Method:      "GET",
			Pattern:     "/nodes/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.NotImplemented},
		rest.Route{
			Name:        "NodeDelete",
			Method:      "DELETE",
			Pattern:     "/nodes/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.NotImplemented},

		// Devices
		rest.Route{
			Name:        "DeviceAdd",
			Method:      "POST",
			Pattern:     "/devices",
			HandlerFunc: a.NotImplemented},
		rest.Route{
			Name:        "DeviceInfo",
			Method:      "GET",
			Pattern:     "/devices/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.NotImplemented},
		rest.Route{
			Name:        "DeviceDelete",
			Method:      "DELETE",
			Pattern:     "/devices/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.NotImplemented},

		// Volume
		rest.Route{
			Name:        "VolumeCreate",
			Method:      "POST",
			Pattern:     "/volumes",
			HandlerFunc: a.NotImplemented},
		rest.Route{
			Name:        "VolumeInfo",
			Method:      "GET",
			Pattern:     "/volumes/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.NotImplemented},
		rest.Route{
			Name:        "VolumeExpand",
			Method:      "POST",
			Pattern:     "/volumes/{id:[A-Fa-f0-9]+}/expand",
			HandlerFunc: a.NotImplemented},
		rest.Route{
			Name:        "VolumeDelete",
			Method:      "DELETE",
			Pattern:     "/volumes/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.NotImplemented},
		rest.Route{
			Name:        "VolumeList",
			Method:      "GET",
			Pattern:     "/volumes",
			HandlerFunc: a.NotImplemented},
	}

	// Register all routes from the App
	for _, route := range routes {

		// Add routes from the table
		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(route.HandlerFunc)

	}

	return nil

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
