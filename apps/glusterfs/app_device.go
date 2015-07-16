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
	//"encoding/json"
	//"errors"
	"github.com/boltdb/bolt"
	//"github.com/gorilla/mux"
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

				// Pretend work
				time.Sleep(1 * time.Second)

				device := NewDeviceEntryFromRequest(dev)
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
