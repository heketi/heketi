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
	"bytes"
	"encoding/gob"
	"errors"
	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/utils"
	"github.com/lpabon/godbc"
	"sort"
)

type DeviceEntry struct {
	Info   DeviceInfo
	Bricks sort.StringSlice
	NodeId string
}

func NewDeviceEntry() *DeviceEntry {
	entry := &DeviceEntry{}
	entry.Bricks = make(sort.StringSlice, 0)

	return entry
}

func NewDeviceEntryFromRequest(req *Device, nodeid string) *DeviceEntry {
	godbc.Require(req != nil)

	device := NewDeviceEntry()
	device.Info.Id = utils.GenUUID()
	device.Info.Name = req.Name
	device.Info.Weight = req.Weight
	device.NodeId = nodeid

	return device
}

func NewDeviceEntryFromId(tx *bolt.Tx, id string) (*DeviceEntry, error) {
	godbc.Require(tx != nil)

	entry := NewDeviceEntry()
	b := tx.Bucket([]byte(BOLTDB_BUCKET_DEVICE))
	if b == nil {
		logger.LogError("Unable to access device bucket")
		err := errors.New("Unable to create device entry")
		return nil, err
	}

	val := b.Get([]byte(id))
	if val == nil {
		return nil, ErrNotFound
	}

	err := entry.Unmarshal(val)
	if err != nil {
		logger.LogError("Unable to unmarshal device: %v", err)
		return nil, err
	}

	return entry, nil
}

func (d *DeviceEntry) Save(tx *bolt.Tx) error {
	godbc.Require(tx != nil)
	godbc.Require(len(d.Info.Id) > 0)

	// Access bucket
	b := tx.Bucket([]byte(BOLTDB_BUCKET_DEVICE))
	if b == nil {
		err := errors.New("Unable to create device entry")
		logger.Err(err)
		return err
	}

	// Save device entry to db
	buffer, err := d.Marshal()
	if err != nil {
		logger.Err(err)
		return err
	}

	// Save data using the id as the key
	err = b.Put([]byte(d.Info.Id), buffer)
	if err != nil {
		logger.Err(err)
		return err
	}

	return nil

}

func (d *DeviceEntry) Delete(tx *bolt.Tx) error {
	godbc.Require(tx != nil)

	// Check if the devices still has drives
	if len(d.Bricks) > 0 {
		logger.Warning("Unable to delete device [%v] because it contains bricks", d.Info.Id)
		return ErrConflict
	}

	b := tx.Bucket([]byte(BOLTDB_BUCKET_DEVICE))
	if b == nil {
		err := errors.New("Unable to access database")
		logger.Err(err)
		return err
	}

	// Delete key
	err := b.Delete([]byte(d.Info.Id))
	if err != nil {
		logger.LogError("Unable to delete container key [%v] in db: %v", d.Info.Id, err.Error())
		return err
	}

	return nil
}

func (d *DeviceEntry) NewInfoResponse(tx *bolt.Tx) (*DeviceInfoResponse, error) {

	godbc.Require(tx != nil)

	info := &DeviceInfoResponse{}
	info.Id = d.Info.Id
	info.Name = d.Info.Name
	info.Weight = d.Info.Weight
	info.Storage = d.Info.Storage

	info.Bricks = make([]Brick, 0)

	/*
	   // Access device information
	   b := tx.Bucket([]byte(BOLTDB_BUCKET_BRI))
	   if b == nil {
	       logger.LogError("Unable to open device bucket")
	       return asdfasdf nil
	   }

	   // Add each drive information
	       for _, driveid := range d.Bricks {
	           entry, err := NewDriveEntryFromId(tx, driveid)
	           godbc.Check(err != ErrNotFound, driveid, d.Bricks)
	           if err != nil {
	               return err
	           }

	           driveinfo, err := entry.NewInfoResponse(tx)
	           if err != nil {
	               return err
	           }
	           info.DeviceInfo = append(info.DeviceInfo, driveinfo)
	       }
	*/

	return info, nil
}

func (d *DeviceEntry) Marshal() ([]byte, error) {
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)
	err := enc.Encode(*d)

	return buffer.Bytes(), err
}

func (d *DeviceEntry) Unmarshal(buffer []byte) error {
	dec := gob.NewDecoder(bytes.NewReader(buffer))
	err := dec.Decode(d)
	if err != nil {
		return err
	}

	// Make sure to setup arrays if nil
	if d.Bricks == nil {
		d.Bricks = make(sort.StringSlice, 0)
	}

	return nil
}

func (d *DeviceEntry) BrickAdd(id string) {
	godbc.Require(!utils.SortedStringHas(d.Bricks, id))

	d.Bricks = append(d.Bricks, id)
	d.Bricks.Sort()
}

func (d *DeviceEntry) BrickDelete(id string) {
	d.Bricks = utils.SortedStringsDelete(d.Bricks, id)
}

func (d *DeviceEntry) StorageSet(amount uint64) {
	d.Info.Storage.Free = amount
	d.Info.Storage.Total = amount
}

func (d *DeviceEntry) StorageAllocate(amount uint64) {
	d.Info.Storage.Free -= amount
	d.Info.Storage.Used += amount
}

func (d *DeviceEntry) StorageFree(amount uint64) {
	d.Info.Storage.Free += amount
	d.Info.Storage.Used -= amount
}
