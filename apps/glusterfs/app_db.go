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
	"strings"

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

		logger.Info("rtalurlogs: starting volume bucket")

		// Volume Bucket
		volumes, err := VolumeList(tx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		for _, volume := range volumes {
			logger.Info("rtalurlogs: adding volume entry %v", volume)
			volEntry, err := NewVolumeEntryFromId(tx, volume)
			if err != nil {
				return err
			}
			volEntryList = append(volEntryList, *volEntry)
		}

		// Brick Bucket
		logger.Info("rtalurlogs: starting brick bucket")
		bricks, err := BrickList(tx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		for _, brick := range bricks {
			logger.Info("rtalurlogs: adding brick entry %v", brick)
			brickEntry, err := NewBrickEntryFromId(tx, brick)
			if err != nil {
				return err
			}
			brickEntryList = append(brickEntryList, *brickEntry)
		}

		// Cluster Bucket
		logger.Info("rtalurlogs: starting cluster bucket")
		clusters, err := ClusterList(tx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		for _, cluster := range clusters {
			logger.Info("rtalurlogs: adding cluster entry %v", cluster)
			clusterEntry, err := NewClusterEntryFromId(tx, cluster)
			if err != nil {
				return err
			}
			clusterEntryList = append(clusterEntryList, *clusterEntry)
		}

		// Node Bucket
		logger.Info("rtalurlogs: starting node bucket")
		nodes, err := NodeList(tx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		for _, node := range nodes {
			logger.Info("rtalurlogs: adding node entry %v", node)
			if strings.HasPrefix(node, "MANAGE") || strings.HasPrefix(node, "STORAGE") {
				logger.Info("rtalurlogs, ignoring registry key")
			} else {
				nodeEntry, err := NewNodeEntryFromId(tx, node)
				if err != nil {
					return err
				}
				nodeEntryList = append(nodeEntryList, *nodeEntry)
			}
		}

		// Device Bucket
		logger.Info("rtalurlogs: starting device bucket")
		devices, err := DeviceList(tx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		for _, device := range devices {
			logger.Info("rtalurlogs: adding device entry %v", device)
			if strings.HasPrefix(device, "DEVICE") {
				logger.Info("rtalurlogs, ignoring registry key")
			} else {
				deviceEntry, err := NewDeviceEntryFromId(tx, device)
				if err != nil {
					return err
				}
				deviceEntryList = append(deviceEntryList, *deviceEntry)
			}
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
