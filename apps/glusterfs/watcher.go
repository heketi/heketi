//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"sync"
	"time"

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/executors"
	wdb "github.com/heketi/heketi/pkg/db"
)

//Do i really need to get read /write lock for this varaible?
type nodesStatus struct {
	Status map[string]bool
	Mutex  sync.RWMutex
}

var NodesStatuses nodesStatus

func init() {

	NodesStatuses.Status = make(map[string]bool)
}

func NodeWatcher(db wdb.RODB, e executors.Executor) {
	var status = make(map[string]bool)
	err := db.View(func(tx *bolt.Tx) error {
		nodes, err := NodeList(tx)
		if err != nil {
			return err
		}
		//to watch the node and update in memory

		for _, n := range nodes {
			if _, ok := status[n]; !ok {
				status[n] = false
			}
			var newNode *NodeEntry
			newNode, err = NewNodeEntryFromId(tx, n)

			if err != nil {
				//pass on to next node
				continue

			}

			// should we check status of nodes disabled in db
			//Ignore if the node is not online
			if !newNode.isOnline() {
				continue
			}
			err = e.GlusterdCheck(newNode.ManageHostName())
			if err != nil {
				logger.Warning("Glusterd not running in %v", newNode.ManageHostName())
				continue
			}
			status[n] = true

		}
		return nil
	})
	if err == nil {
		NodesStatuses.Mutex.Lock()
		defer NodesStatuses.Mutex.Unlock()
		NodesStatuses.Status = status
	}

}

//func to start watcher

func StartNodeWatcher(db wdb.RODB, e executors.Executor) {
	// ticker := time.NewTicker(1 * time.Minute)
	// select {
	// case <-ticker.C:
	// 	NodeWatcher(db, e)
	// }
	for {
		NodeWatcher(db, e)
		_ = <-time.After(time.Minute)
	}

}
