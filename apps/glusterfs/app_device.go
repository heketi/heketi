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
)

func (a *App) DeviceAdd(w http.ResponseWriter, r *http.Request) {

	var msg DeviceAddRequest
	err := utils.GetJsonFromRequest(r, &msg)
	if err != nil {
		http.Error(w, "request unable to be parsed", 422)
		return
	}

	// Check the message has devices
	if msg.Name == "" {
		http.Error(w, "no devices added", http.StatusBadRequest)
		return
	}

	// Check the node is in the db
	var node *NodeEntry
	err = a.db.View(func(tx *bolt.Tx) error {
		var err error
		node, err = NewNodeEntryFromId(tx, msg.NodeId)
		if err == ErrNotFound {
			http.Error(w, "Node id does not exist", http.StatusNotFound)
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

	// Log the devices are being added
	logger.Info("Adding device %v to node %v", msg.Name, msg.NodeId)

	// Add device in an asynchronous function
	a.asyncManager.AsyncHttpRedirectFunc(w, r, func() (seeOtherUrl string, e error) {

		// Create device entry
		device := NewDeviceEntryFromRequest(&msg)

		// Setup device on node
		info, err := a.executor.DeviceSetup(node.ManageHostName(),
			device.Info.Name, device.Info.Id)
		if err != nil {
			return "", err
		}

		// Create an entry for the device and set the size
		device.StorageSet(info.Size)

		// Setup garbage collector on error
		defer func() {
			if e != nil {
				a.executor.DeviceTeardown(node.ManageHostName(),
					device.Info.Name,
					device.Info.Id)
			}
		}()

		// Save on db
		err = a.db.Update(func(tx *bolt.Tx) error {

			nodeEntry, err := NewNodeEntryFromId(tx, msg.NodeId)
			if err != nil {
				return err
			}

			// Add device to node
			nodeEntry.DeviceAdd(device.Info.Id)

			clusterEntry, err := NewClusterEntryFromId(tx, nodeEntry.Info.ClusterId)
			if err != nil {
				return err
			}

			// Commit
			err = nodeEntry.Save(tx)
			if err != nil {
				return err
			}

			// Save drive
			err = device.Save(tx)
			if err != nil {
				return err
			}

			// Add to allocator
			err = a.allocator.AddDevice(clusterEntry, nodeEntry, device)
			if err != nil {
				return err
			}

			return nil

		})
		if err != nil {
			return "", err
		}

		logger.Info("Added device %v", msg.Name)

		// Done
		// Returning a null string instructs the async manager
		// to return http status of 204 (No Content)
		return "", nil
	})

}

func (a *App) DeviceInfo(w http.ResponseWriter, r *http.Request) {

	// Get device id from URL
	vars := mux.Vars(r)
	id := vars["id"]

	// Get device information
	var info *DeviceInfoResponse
	err := a.db.View(func(tx *bolt.Tx) error {
		entry, err := NewDeviceEntryFromId(tx, id)
		if err == ErrNotFound {
			http.Error(w, "Id not found", http.StatusNotFound)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		info, err = entry.NewInfoResponse(tx)
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

func (a *App) DeviceDelete(w http.ResponseWriter, r *http.Request) {
	// Get the id from the URL
	vars := mux.Vars(r)
	id := vars["id"]

	// Check request
	var (
		device  *DeviceEntry
		node    *NodeEntry
		cluster *ClusterEntry
	)
	err := a.db.View(func(tx *bolt.Tx) error {
		var err error
		// Access device entry
		device, err = NewDeviceEntryFromId(tx, id)
		if err == ErrNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return err
		} else if err != nil {
			logger.Err(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		// Check if we can delete the device
		if !device.IsDeleteOk() {
			http.Error(w, ErrConflict.Error(), http.StatusConflict)
			return ErrConflict
		}

		// Access node entry
		node, err = NewNodeEntryFromId(tx, device.NodeId)
		if err != nil {
			logger.Err(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		// Save cluster to update allocator
		cluster, err = NewClusterEntryFromId(tx, node.Info.ClusterId)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return
	}

	// Delete device
	logger.Info("Deleting device %v on node %v", device.Info.Id, device.NodeId)
	a.asyncManager.AsyncHttpRedirectFunc(w, r, func() (string, error) {

		// Teardown device
		err := a.executor.DeviceTeardown(node.ManageHostName(),
			device.Info.Name, device.Info.Id)
		if err != nil {
			return "", err
		}

		// Remove device from allocator
		err = a.allocator.RemoveDevice(cluster, node, device)
		if err != nil {
			return "", err
		}

		// Get info from db
		err = a.db.Update(func(tx *bolt.Tx) error {

			// Access node entry
			node, err := NewNodeEntryFromId(tx, device.NodeId)
			if err == ErrNotFound {
				logger.Critical(
					"Node id %v pointed to by device %v, but it is not in the db",
					device.NodeId,
					device.Info.Id)
				return err
			} else if err != nil {
				logger.Err(err)
				return err
			}

			// Delete device from node
			node.DeviceDelete(device.Info.Id)

			// Save node
			node.Save(tx)

			// Delete device from db
			err = device.Delete(tx)
			if err != nil {
				logger.Err(err)
				return err
			}

			return nil

		})
		if err != nil {
			return "", err
		}

		// Show that the key has been deleted
		logger.Info("Deleted node [%s]", id)

		return "", nil
	})

}
