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
	"encoding/json"
	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	"github.com/heketi/heketi/utils"
	"net/http"
	"time"
)

func (a *App) NodeAdd(w http.ResponseWriter, r *http.Request) {
	var msg NodeAddRequest

	err := utils.GetJsonFromRequest(r, &msg)
	if err != nil {
		http.Error(w, "request unable to be parsed", 422)
		return
	}

	// Check information in JSON request
	if len(msg.Hostnames.Manage) == 0 {
		http.Error(w, "Manage hostname missing", http.StatusBadRequest)
		return
	}
	if len(msg.Hostnames.Storage) == 0 {
		http.Error(w, "Storage hostname missing", http.StatusBadRequest)
		return
	}

	// Create a node entry
	node := NewNodeEntryFromRequest(&msg)

	// Add node entry into the db
	err = a.db.Update(func(tx *bolt.Tx) error {
		cluster, err := NewClusterEntryFromId(tx, msg.ClusterId)
		if err == ErrNotFound {
			http.Error(w, "Cluster id does not exist", http.StatusNotFound)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		// Add node to cluster
		cluster.NodeAdd(node.Info.Id)

		// Save cluster
		err = cluster.Save(tx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		// Save node
		err = node.Save(tx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		return nil

	})
	if err != nil {
		return
	}

	logger.Info("Adding node %v", node.Info.Hostnames.Manage[0])
	a.asyncManager.AsyncHttpRedirectFunc(w, r, func() (string, error) {
		time.Sleep(1 * time.Second)
		logger.Info("Added node " + node.Info.Id)
		return "/nodes/" + node.Info.Id, nil
	})
}

func (a *App) NodeInfo(w http.ResponseWriter, r *http.Request) {

	// Get node id from URL
	vars := mux.Vars(r)
	id := vars["id"]

	// Get Node information
	var info *NodeInfoResponse
	err := a.db.View(func(tx *bolt.Tx) error {
		entry, err := NewNodeEntryFromId(tx, id)
		if err == ErrNotFound {
			http.Error(w, "Id not found", http.StatusNotFound)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		info, err = entry.NewInfoReponse(tx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		return nil
	})
	if err != nil {
		return
	}

	// Write msg
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(info); err != nil {
		panic(err)
	}

}

func (a *App) NodeDelete(w http.ResponseWriter, r *http.Request) {
	// Get the id from the URL
	vars := mux.Vars(r)
	id := vars["id"]

	// Get info from db
	err := a.db.Update(func(tx *bolt.Tx) error {

		// Access node entry
		node, err := NewNodeEntryFromId(tx, id)
		if err == ErrNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		// Get Cluster
		cluster, err := NewClusterEntryFromId(tx, node.Info.ClusterId)
		if err == ErrNotFound {
			http.Error(w, "Cluster id does not exist", http.StatusInternalServerError)
			logger.Critical("Cluster id %v is expected be in db. Pointed to by node %v",
				node.Info.ClusterId,
				node.Info.Id)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}
		cluster.NodeDelete(node.Info.Id)

		// Save cluster
		cluster.Save(tx)

		// Delete node from db
		err = node.Delete(tx)
		if err == ErrConflict {
			http.Error(w, err.Error(), http.StatusConflict)
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

	a.asyncManager.AsyncHttpRedirectFunc(w, r, func() (string, error) {
		time.Sleep(1 * time.Second)
		// Show that the key has been deleted
		logger.Info("Deleted node [%s]", id)
		return "", nil
	})
}
