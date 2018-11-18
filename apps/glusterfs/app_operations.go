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
	"encoding/json"
	"net/http"

	"github.com/heketi/heketi/pkg/glusterfs/api"

	"github.com/boltdb/bolt"
)

func (a *App) OperationsInfo(w http.ResponseWriter, r *http.Request) {
	info := &api.OperationsInfo{}

	err := a.db.View(func(tx *bolt.Tx) error {
		ops, err := PendingOperationList(tx)
		if err != nil {
			return err
		}
		info.Total = uint64(len(ops))
		m, err := PendingOperationStateCount(tx)
		if err != nil {
			return err
		}
		info.New = uint64(m[NewOperation])
		info.Stale = uint64(m[StaleOperation])
		return nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	info.InFlight = a.optracker.Get()

	// Write msg
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(info); err != nil {
		panic(err)
	}
}

func (a *App) PendingOperationList(w http.ResponseWriter, r *http.Request) {
	p := &api.PendingOperationListResponse{}

	err := a.db.View(func(tx *bolt.Tx) error {
		ops, err := PendingOperationList(tx)
		if err != nil {
			return err
		}
		p.PendingOperations = make([]api.PendingOperationInfo, len(ops))
		for i, pid := range ops {
			pop, err := NewPendingOperationEntryFromId(tx, pid)
			if err != nil {
				return err
			}
			p.PendingOperations[i] = pop.ToInfo()
		}
		return nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write msg
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(p); err != nil {
		panic(err)
	}
}
