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
	"errors"
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

	// Create a node entry
	node := NewNodeEntry()

	// Add node entry into the db
	err = a.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BOLTDB_BUCKET_CLUSTER))
		if b == nil {
			logger.LogError("Unable to access cluster bucket")
			err := errors.New("Unable to access database")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		// Check if the cluster specified exists
		if b.Get([]byte(msg.ClusterId)) == nil {
			http.Error(w, "Cluster id does not exist", http.StatusNotFound)
			return ErrNotFound
		}

		b = tx.Bucket([]byte(BOLTDB_BUCKET_NODE))
		if b == nil {
			logger.LogError("Unable to access node bucket")
			err := errors.New("Unable to create node entry")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		// Setup node entry
		node.Info.Id = utils.GenUUID()
		node.Info.ClusterId = msg.ClusterId
		node.Info.Hostnames = msg.Hostnames
		node.Info.Zone = msg.Zone

		// Save node entry to db
		buffer, err := node.Marshal()
		if err != nil {
			logger.LogError("Unable to marshal node entry: %v", node)
			http.Error(w, "Unable to create cluster", http.StatusInternalServerError)
			return err
		}
		err = b.Put([]byte(node.Info.Id), buffer)
		if err != nil {
			logger.LogError("Unable to save node entry")
			http.Error(w, "Unable to create cluster", http.StatusInternalServerError)
			return err
		}

		return nil

	})
	if err != nil {
		return
	}

	logger.Info("Adding node %v", node.Info.Hostnames.Manage[0])
	a.asyncManager.AsyncHttpRedirectFunc(w, r, func() (string, error) {
		time.Sleep(65 * time.Second)
		logger.Info("Redirect to %v", "/nodes/"+node.Info.Id)
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
		b := tx.Bucket([]byte(BOLTDB_BUCKET_NODE))
		if b == nil {
			logger.LogError("Unable to access node bucket")
			err := errors.New("Unable to create node entry")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		entry := NewNodeEntry()
		val := b.Get([]byte(id))
		if val == nil {
			http.Error(w, "Id not found", http.StatusNotFound)
			return ErrNotFound
		}

		err := entry.Unmarshal(val)
		if err != nil {
			logger.LogError("Unable to unmarshal node: %v", err)
			http.Error(w, "Unable to access node information", http.StatusInternalServerError)
			return err
		}

		info = entry.InfoReponse()

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

	logger.Info("Added node %v", id)
}
