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
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/utils"
)

// GeoReplicationStatus is the handler returning the geo-replication session
// status
func (a *App) GeoReplicationStatus(w http.ResponseWriter, r *http.Request) {
	logger.Debug("In GeoReplicationStatus")

	var node *NodeEntry
	var err error

	err = a.db.View(func(tx *bolt.Tx) error {
		clusters, err := ClusterList(tx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		if len(clusters) == 0 {
			http.Error(w, fmt.Sprintf("No clusters configured"), http.StatusBadRequest)
			logger.LogError("No clusters configured")
			return ErrNotFound
		}

		cluster, err := NewClusterEntryFromId(tx, clusters[0])
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		if len(cluster.Info.Nodes) == 0 {
			errMsg := "No clusters configured"
			http.Error(w, fmt.Sprintf(errMsg), http.StatusBadRequest)
			logger.LogError(errMsg)
			return ErrNotFound
		}

		//use first node of the first cluster to get status
		node, err = NewNodeEntryFromId(tx, cluster.Info.Nodes[0])
		if err == ErrNotFound {
			http.Error(w, "Node Id not found", http.StatusNotFound)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		return nil
	})
	if err != nil {
		return
	}

	resp, err := node.NewGeoReplicationStatusResponse(a.executor)
	if err != nil {
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		panic(err)
	}
}

// GeoReplicationVolumeStatus is the handler returning the geo-replication session
// status for a specific volume
func (a *App) GeoReplicationVolumeStatus(w http.ResponseWriter, r *http.Request) {
	logger.Debug("In GeoReplicationVolumeStatus")

	vars := mux.Vars(r)
	id := vars["id"]

	var volume *VolumeEntry
	var host string
	var err error

	err = a.db.View(func(tx *bolt.Tx) error {
		volume, err = NewVolumeEntryFromId(tx, id)
		if err == ErrNotFound {
			http.Error(w, "Volume Id not found", http.StatusNotFound)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		cluster, err := NewClusterEntryFromId(tx, volume.Info.Cluster)
		if err == ErrNotFound {
			http.Error(w, "Cluster Id not found", http.StatusNotFound)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		node, err := NewNodeEntryFromId(tx, cluster.Info.Nodes[0])
		if err == ErrNotFound {
			http.Error(w, "Node Id not found", http.StatusNotFound)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		host = node.ManageHostName()

		return nil
	})
	if err != nil {
		return
	}

	resp, err := volume.NewGeoReplicationStatusResponse(a.executor, host)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		logger.LogError("Failed to get geo-replication status: %s", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		panic(err)
	}
}

// GeoReplicationPostHandler is the handler for managing a geo-replication session
// It covers create, config, start, stop, pause, resume and delete
func (a *App) GeoReplicationPostHandler(w http.ResponseWriter, r *http.Request) {
	logger.Debug("In VolumeGeoReplication")

	vars := mux.Vars(r)
	id := vars["id"]

	var volume *VolumeEntry
	var host string
	var err error

	var msg api.GeoReplicationRequest
	if err := utils.GetJsonFromRequest(r, &msg); err != nil {
		http.Error(w, "request unable to be parsed", http.StatusUnprocessableEntity)
		return
	}
	logger.Debug("Msg: %v", msg)

	switch {
	case msg.SlaveHost == "":
		errMsg := "Slave host not defined"
		http.Error(w, errMsg, http.StatusBadRequest)
		logger.LogError(errMsg)
		return
	case msg.SlaveVolume == "":
		errMsg := "Slave volume not defined"
		http.Error(w, errMsg, http.StatusBadRequest)
		logger.LogError(errMsg)
		return
	}

	err = a.db.View(func(tx *bolt.Tx) error {
		volume, err = NewVolumeEntryFromId(tx, id)
		if err == ErrNotFound {
			http.Error(w, "Volume Id not found", http.StatusNotFound)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		cluster, err := NewClusterEntryFromId(tx, volume.Info.Cluster)
		if err == ErrNotFound {
			http.Error(w, "Cluster Id not found", http.StatusNotFound)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		node, err := NewNodeEntryFromId(tx, cluster.Info.Nodes[0])
		if err == ErrNotFound {
			http.Error(w, "Node Id not found", http.StatusNotFound)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		host = node.ManageHostName()

		return nil
	})
	if err != nil {
		return
	}

	// Perform GeoReplication action on volume in an asynchronous function
	a.asyncManager.AsyncHttpRedirectFunc(w, r, func() (string, error) {
		if err := volume.GeoReplicationAction(a.db, a.executor, host, msg); err != nil {
			return "", err
		}

		if msg.Action == api.GeoReplicationActionDelete {
			return "/georeplication", nil
		}

		return "/volumes/" + volume.Info.Id + "/georeplication", nil
	})

}
