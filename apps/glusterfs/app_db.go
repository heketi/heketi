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

// DbDump ... Creates a JSON output representing the state of DB
func (a *App) DbDump(w http.ResponseWriter, r *http.Request) {
	volEntryList := make([]VolumeEntry, 0)

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

	// Write msg
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(volEntryList); err != nil {
		panic(err)
	}
}
