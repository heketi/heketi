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
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	"github.com/heketi/heketi/executors"
	"github.com/heketi/heketi/executors/kubeexec"
	"github.com/heketi/heketi/executors/mockexec"
	"github.com/heketi/heketi/executors/sshexec"
	"github.com/heketi/heketi/pkg/utils"
	"github.com/heketi/rest"
)

const (
	ASYNC_ROUTE                    = "/queue"
	BOLTDB_BUCKET_CLUSTER          = "CLUSTER"
	BOLTDB_BUCKET_NODE             = "NODE"
	BOLTDB_BUCKET_VOLUME           = "VOLUME"
	BOLTDB_BUCKET_DEVICE           = "DEVICE"
	BOLTDB_BUCKET_BRICK            = "BRICK"
	BOLTDB_BUCKET_BLOCKVOLUME      = "BLOCKVOLUME"
	BOLTDB_BUCKET_DBATTRIBUTE      = "DBATTRIBUTE"
	DB_CLUSTER_HAS_FILE_BLOCK_FLAG = "DB_CLUSTER_HAS_FILE_BLOCK_FLAG"
)

var (
	logger     = utils.NewLogger("[heketi]", utils.LEVEL_INFO)
	dbfilename = "heketi.db"
)

type App struct {
	asyncManager *rest.AsyncHttpManager
	db           *bolt.DB
	dbReadOnly   bool
	executor     executors.Executor
	_allocator   Allocator
	conf         *GlusterFSConfig

	// For testing only.  Keep access to the object
	// not through the interface
	xo *mockexec.MockExecutor
}

// Use for tests only
func NewApp(configIo io.Reader) *App {
	var err error
	app := &App{}

	// Load configuration file
	app.conf = loadConfiguration(configIo)
	if app.conf == nil {
		return nil
	}

	// Set values mentioned in environmental variable
	app.setFromEnvironmentalVariable()

	// Setup loglevel
	err = app.setLogLevel(app.conf.Loglevel)
	if err != nil {
		// just log that the log level was bad, it never failed
		// anything in previous versions
		logger.Err(err)
	}

	// Setup asynchronous manager
	app.asyncManager = rest.NewAsyncHttpManager(ASYNC_ROUTE)

	// Setup executor
	switch app.conf.Executor {
	case "mock":
		app.xo, err = mockexec.NewMockExecutor()
		app.executor = app.xo
	case "kube", "kubernetes":
		app.executor, err = kubeexec.NewKubeExecutor(&app.conf.KubeConfig)
	case "ssh", "":
		app.executor, err = sshexec.NewSshExecutor(&app.conf.SshConfig)
	default:
		return nil
	}
	if err != nil {
		logger.Err(err)
		return nil
	}
	logger.Info("Loaded %v executor", app.conf.Executor)

	// Set db is set in the configuration file
	if app.conf.DBfile != "" {
		dbfilename = app.conf.DBfile
	}

	// Setup database
	app.db, err = OpenDB(dbfilename, false)
	if err != nil {
		logger.LogError("Unable to open database: %v. Retrying using read only mode", err)

		// Try opening as read-only
		app.db, err = OpenDB(dbfilename, true)
		if err != nil {
			logger.LogError("Unable to open database: %v", err)
			return nil
		}
		app.dbReadOnly = true
	} else {
		err = app.db.Update(func(tx *bolt.Tx) error {
			err := initializeBuckets(tx)
			if err != nil {
				logger.LogError("Unable to initialize buckets")
				return err
			}

			// Handle Upgrade Changes
			err = UpgradeDB(tx)
			if err != nil {
				logger.LogError("Unable to Upgrade Changes")
				return err
			}

			return nil

		})
		if err != nil {
			logger.Err(err)
			return nil
		}
	}

	// Abort the application if there are pending operations in the db.
	// In the immediate future we need to prevent incomplete operations
	// from piling up in the db. If there are any pending ops in the db
	// (meaning heketi was uncleanly terminated during the op) we are
	// simply going to refuse to start and provide offline tooling to
	// repair the situation. In the long term we may gain the ability to
	// auto-rollback or even try to resume some operations.
	if HasPendingOperations(app.db) {
		e := errors.New(
			"Heketi was terminated while performing one or more operations." +
				" Server may refuse to start as long as pending operations" +
				" are present in the db.")
		logger.Err(e)
		logger.Info(
			"Please refer to the Heketi troubleshooting documentation for more" +
				" information on how to resolve this issue.")
		if !app.conf.IgnoreStaleOperations {
			logger.Warning("Server refusing to start.")
			panic(e)
		}
		logger.Warning("Ignoring stale pending operations." +
			"Server will be running with incomplete/inconsistent state in DB.")
	}

	// Set advanced settings
	app.setAdvSettings()

	// Set block settings
	app.setBlockSettings()

	// Show application has loaded
	logger.Info("GlusterFS Application Loaded")

	return app
}

func (a *App) setLogLevel(level string) error {
	switch level {
	case "none":
		logger.SetLevel(utils.LEVEL_NOLOG)
	case "critical":
		logger.SetLevel(utils.LEVEL_CRITICAL)
	case "error":
		logger.SetLevel(utils.LEVEL_ERROR)
	case "warning":
		logger.SetLevel(utils.LEVEL_WARNING)
	case "info":
		logger.SetLevel(utils.LEVEL_INFO)
	case "debug":
		logger.SetLevel(utils.LEVEL_DEBUG)
	default:
		return fmt.Errorf("invalid log level: %s", level)
	}
	return nil
}

func (a *App) setFromEnvironmentalVariable() {
	var err error

	// environment variable overrides file config
	env := os.Getenv("HEKETI_EXECUTOR")
	if env != "" {
		a.conf.Executor = env
	}

	env = os.Getenv("HEKETI_GLUSTERAPP_LOGLEVEL")
	if env != "" {
		a.conf.Loglevel = env
	}

	env = os.Getenv("HEKETI_IGNORE_STALE_OPERATIONS")
	if env != "" {
		a.conf.IgnoreStaleOperations, err = strconv.ParseBool(env)
		if err != nil {
			logger.LogError("Error: While parsing HEKETI_IGNORE_STALE_OPERATIONS as bool: %v", err)
		}
	}

	env = os.Getenv("HEKETI_AUTO_CREATE_BLOCK_HOSTING_VOLUME")
	if "" != env {
		a.conf.CreateBlockHostingVolumes, err = strconv.ParseBool(env)
		if err != nil {
			logger.LogError("Error: Parse bool in Create Block Hosting Volumes: %v", err)
		}
	}

	env = os.Getenv("HEKETI_BLOCK_HOSTING_VOLUME_SIZE")
	if "" != env {
		a.conf.BlockHostingVolumeSize, err = strconv.Atoi(env)
		if err != nil {
			logger.LogError("Error: Atoi in Block Hosting Volume Size: %v", err)
		}
	}
}

func (a *App) setAdvSettings() {
	if a.conf.BrickMaxNum != 0 {
		logger.Info("Adv: Max bricks per volume set to %v", a.conf.BrickMaxNum)

		// From volume_entry.go
		BrickMaxNum = a.conf.BrickMaxNum
	}
	if a.conf.BrickMaxSize != 0 {
		logger.Info("Adv: Max brick size %v GB", a.conf.BrickMaxSize)

		// From volume_entry.go
		// Convert to KB
		BrickMaxSize = uint64(a.conf.BrickMaxSize) * 1024 * 1024
	}
	if a.conf.BrickMinSize != 0 {
		logger.Info("Adv: Min brick size %v GB", a.conf.BrickMinSize)

		// From volume_entry.go
		// Convert to KB
		BrickMinSize = uint64(a.conf.BrickMinSize) * 1024 * 1024
	}
}

func (a *App) setBlockSettings() {
	if a.conf.CreateBlockHostingVolumes != false {
		logger.Info("Block: Auto Create Block Hosting Volume set to %v", a.conf.CreateBlockHostingVolumes)

		// switch to auto creation of block hosting volumes
		CreateBlockHostingVolumes = a.conf.CreateBlockHostingVolumes
	}
	if a.conf.BlockHostingVolumeSize > 0 {
		logger.Info("Block: New Block Hosting Volume size %v GB", a.conf.BlockHostingVolumeSize)

		// Should be in GB as this is input for block hosting volume create
		BlockHostingVolumeSize = a.conf.BlockHostingVolumeSize
	}
}

// Register Routes
func (a *App) SetRoutes(router *mux.Router) error {

	routes := rest.Routes{

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
			Name:        "ClusterSetFlags",
			Method:      "POST",
			Pattern:     "/clusters/{id:[A-Fa-f0-9]+}/flags",
			HandlerFunc: a.ClusterSetFlags},
		rest.Route{
			Name:        "ClusterInfo",
			Method:      "GET",
			Pattern:     "/clusters/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.ClusterInfo},
		rest.Route{
			Name:        "ClusterList",
			Method:      "GET",
			Pattern:     "/clusters",
			HandlerFunc: a.ClusterList},
		rest.Route{
			Name:        "ClusterDelete",
			Method:      "DELETE",
			Pattern:     "/clusters/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.ClusterDelete},

		// Node
		rest.Route{
			Name:        "NodeAdd",
			Method:      "POST",
			Pattern:     "/nodes",
			HandlerFunc: a.NodeAdd},
		rest.Route{
			Name:        "NodeInfo",
			Method:      "GET",
			Pattern:     "/nodes/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.NodeInfo},
		rest.Route{
			Name:        "NodeDelete",
			Method:      "DELETE",
			Pattern:     "/nodes/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.NodeDelete},
		rest.Route{
			Name:        "NodeSetState",
			Method:      "POST",
			Pattern:     "/nodes/{id:[A-Fa-f0-9]+}/state",
			HandlerFunc: a.NodeSetState},

		// Devices
		rest.Route{
			Name:        "DeviceAdd",
			Method:      "POST",
			Pattern:     "/devices",
			HandlerFunc: a.DeviceAdd},
		rest.Route{
			Name:        "DeviceInfo",
			Method:      "GET",
			Pattern:     "/devices/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.DeviceInfo},
		rest.Route{
			Name:        "DeviceDelete",
			Method:      "DELETE",
			Pattern:     "/devices/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.DeviceDelete},
		rest.Route{
			Name:        "DeviceSetState",
			Method:      "POST",
			Pattern:     "/devices/{id:[A-Fa-f0-9]+}/state",
			HandlerFunc: a.DeviceSetState},
		rest.Route{
			Name:        "DeviceResync",
			Method:      "GET",
			Pattern:     "/devices/{id:[A-Fa-f0-9]+}/resync",
			HandlerFunc: a.DeviceResync},

		// Volume
		rest.Route{
			Name:        "VolumeCreate",
			Method:      "POST",
			Pattern:     "/volumes",
			HandlerFunc: a.VolumeCreate},
		rest.Route{
			Name:        "VolumeInfo",
			Method:      "GET",
			Pattern:     "/volumes/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.VolumeInfo},
		rest.Route{
			Name:        "VolumeExpand",
			Method:      "POST",
			Pattern:     "/volumes/{id:[A-Fa-f0-9]+}/expand",
			HandlerFunc: a.VolumeExpand},
		rest.Route{
			Name:        "VolumeDelete",
			Method:      "DELETE",
			Pattern:     "/volumes/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.VolumeDelete},
		rest.Route{
			Name:        "VolumeList",
			Method:      "GET",
			Pattern:     "/volumes",
			HandlerFunc: a.VolumeList},

		// BlockVolumes
		rest.Route{
			Name:        "BlockVolumeCreate",
			Method:      "POST",
			Pattern:     "/blockvolumes",
			HandlerFunc: a.BlockVolumeCreate},
		rest.Route{
			Name:        "BlockVolumeInfo",
			Method:      "GET",
			Pattern:     "/blockvolumes/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.BlockVolumeInfo},
		rest.Route{
			Name:        "BlockVolumeDelete",
			Method:      "DELETE",
			Pattern:     "/blockvolumes/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.BlockVolumeDelete},
		rest.Route{
			Name:        "BlockVolumeList",
			Method:      "GET",
			Pattern:     "/blockvolumes",
			HandlerFunc: a.BlockVolumeList},

		// Backup
		rest.Route{
			Name:        "Backup",
			Method:      "GET",
			Pattern:     "/backup/db",
			HandlerFunc: a.Backup},

		// Db
		rest.Route{
			Name:        "DbDump",
			Method:      "GET",
			Pattern:     "/db/dump",
			HandlerFunc: a.DbDump},

		// Logging
		rest.Route{
			Name:        "GetLogLevel",
			Method:      "GET",
			Pattern:     "/internal/logging",
			HandlerFunc: a.GetLogLevel},
		rest.Route{
			Name:        "SetLogLevel",
			Method:      "POST",
			Pattern:     "/internal/logging",
			HandlerFunc: a.SetLogLevel},
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

	// Set default error handler
	router.NotFoundHandler = http.HandlerFunc(a.NotFoundHandler)

	return nil
}

func (a *App) Close() {

	// Close the DB
	a.db.Close()
	logger.Info("Closed")
}

func (a *App) Backup(w http.ResponseWriter, r *http.Request) {
	err := a.db.View(func(tx *bolt.Tx) error {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="heketi.db"`)
		w.Header().Set("Content-Length", strconv.Itoa(int(tx.Size())))
		_, err := tx.WriteTo(w)
		return err
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *App) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	logger.Warning("Invalid path or request %v", r.URL.Path)
	http.Error(w, "Invalid path or request", http.StatusNotFound)
}
