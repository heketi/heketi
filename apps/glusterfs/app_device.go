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

func (a *App) DeviceAdd(w http.ResponseWriter, r *http.Request) {

	var msg DeviceAddRequest
	err := utils.GetJsonFromRequest(r, &msg)
	if err != nil {
		http.Error(w, "request unable to be parsed", 422)
		return
	}

	// Check the message has devices
	if len(msg.Devices) <= 0 {
		http.Error(w, "no devices added", http.StatusBadRequest)
		return
	}

	// Check the node is in the db
	err = a.db.View(func(tx *bolt.Tx) error {
		_, err := NewNodeEntryFromId(tx, msg.NodeId)
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
	logger.Info("Adding devices %+v to node %v", msg.Devices, msg.NodeId)

	// Add device in an asynchronous function
	a.asyncManager.AsyncHttpRedirectFunc(w, r, func() (string, error) {

		sg := utils.NewStatusGroup()
		for index := range msg.Devices {
			sg.Add(1)

			// Add each drive
			go func(dev *Device) {
				defer sg.Done()

				device := NewDeviceEntryFromRequest(dev, msg.NodeId)

				// Pretend work
				time.Sleep(1 * time.Second)
				// :TODO: Fake work
				device.StorageSet(10 * TB)

				err := a.db.Update(func(tx *bolt.Tx) error {
					node, err := NewNodeEntryFromId(tx, msg.NodeId)
					if err != nil {
						return err
					}

					// Add device to node
					node.DeviceAdd(device.Info.Id)

					// Commit
					err = node.Save(tx)
					if err != nil {
						return err
					}

					// Save drive
					err = device.Save(tx)
					if err != nil {
						return err
					}

					return nil

				})
				if err != nil {
					sg.Err(err)
				}

				logger.Info("Added device %v", dev.Name)

			}(&msg.Devices[index])
		}

		// Done
		// Returning a null string instructs the async manager
		// to return http status of 204 (No Content)
		return "", sg.Result()
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

	// Get info from db
	err := a.db.Update(func(tx *bolt.Tx) error {

		// Access device entry
		device, err := NewDeviceEntryFromId(tx, id)
		if err == ErrNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		// Access node entry
		node, err := NewNodeEntryFromId(tx, device.NodeId)
		if err == ErrNotFound {
			http.Error(w, "Node id does not exist", http.StatusInternalServerError)
			logger.Critical(
				"Node id %v pointed to by device %v, but it is not in the db",
				device.NodeId,
				device.Info.Id)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		// Delete device from node
		node.DeviceDelete(device.Info.Id)

		// Save node
		node.Save(tx)

		// Delete device from db
		err = device.Delete(tx)
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

	// Show that the key has been deleted
	logger.Info("Deleted node [%s]", id)

	// Write msg
	w.WriteHeader(http.StatusOK)
}
