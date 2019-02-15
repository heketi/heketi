//
// Copyright (c) 2019 The heketi Authors
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

	"github.com/heketi/heketi/pkg/glusterfs/api"

	"github.com/boltdb/bolt"
)

var (
	clusterHealth = "ClusterHealth"
)

func checkDBState(db *bolt.DB, info *api.Healthz) {

	var faulty = "faulty"
	//db Read check
	err := db.View(func(tx *bolt.Tx) error {
		_, err := NewDbAttributeEntryFromKey(tx, clusterHealth)
		return err
	})

	if err != nil && err != ErrNotFound {
		info.ClusterHeath = faulty
		info.DbState = faulty
		return
	}
	info.DbState = "read"

	//db write check
	err = db.Update(func(tx *bolt.Tx) error {
		h := NewDbAttributeEntry()
		h.Key = clusterHealth
		return h.Save(tx)
	})
	if err == nil {
		info.DbState = "write"
	}
	return
}

func checkClusterState(health *NodeHealthCache, info *api.Healthz) {

	if len(health.nodes) == 0 {
		return
	}
	totalUnhealthyNodes := 0
	for _, n := range health.nodes {
		if !n.Up {
			totalUnhealthyNodes++
		}
		info.NodesStatus = append(info.NodesStatus, (api.NodeHealthStatus)(*n))
	}
	if totalUnhealthyNodes == len(info.NodesStatus) {
		info.ClusterHeath = "faulty"
	} else {
		info.ClusterHeath = "healthy"
	}
	return
}

func (a *App) Healthz(w http.ResponseWriter, r *http.Request) {
	info := api.Healthz{}
	info.NodesStatus = make([]api.NodeHealthStatus, 0)

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	// db status check
	checkDBState(a.db, &info)

	if MonitorGlusterNodes {
		// cluster status check
		checkClusterState(a.nhealth, &info)
	}

	if err := json.NewEncoder(w).Encode(info); err != nil {
		panic(err)
	}
}
