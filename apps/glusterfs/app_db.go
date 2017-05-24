//
// Copyright (c) 2017 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"encoding/json"
	"net/http"

	"github.com/boltdb/bolt"
)

type DbDump struct {
	Clusters []ClusterEntry `json:"clusterentries"`
	Volumes  []VolumeEntry  `json:"volumeentries"`
	Bricks   []BrickEntry   `json:"brickentries"`
	Nodes    []NodeEntry    `json:"nodeentries"`
	Devices  []DeviceEntry  `json:"deviceentries"`
}

// DbDump ... Creates a JSON output representing the state of DB
func (a *App) DbDump(w http.ResponseWriter, r *http.Request) {
	var dump DbDump
	clusterEntryList := make([]ClusterEntry, 0)
	volEntryList := make([]VolumeEntry, 0)
	brickEntryList := make([]BrickEntry, 0)
	nodeEntryList := make([]NodeEntry, 0)
	deviceEntryList := make([]DeviceEntry, 0)

	err := a.db.View(func(tx *bolt.Tx) error {

		volumes, err := VolumeList(tx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		for _, volume := range volumes {
			volEntry, err := NewVolumeEntryFromId(tx, volume)
			if err != nil {
				return err
			}
			logger.Info("%+v", volEntry)
			volEntryList = append(volEntryList, *volEntry)
		}
		return nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	dump.Clusters = clusterEntryList
	dump.Volumes = volEntryList
	dump.Bricks = brickEntryList
	dump.Nodes = nodeEntryList
	dump.Devices = deviceEntryList

	// Write msg
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(dump); err != nil {
		panic(err)
	}
}
