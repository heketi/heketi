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
	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/tests"
	"github.com/heketi/heketi/utils"
	"os"
	"reflect"
	"testing"
)

func TestNewDeviceEntry(t *testing.T) {

	d := NewDeviceEntry()
	tests.Assert(t, d != nil)
	tests.Assert(t, d.Info.Id == "")
	tests.Assert(t, d.Info.Name == "")
	tests.Assert(t, d.Info.Weight == 0)
	tests.Assert(t, d.Info.Storage.Free == 0)
	tests.Assert(t, d.Info.Storage.Total == 0)
	tests.Assert(t, d.Info.Storage.Used == 0)
	tests.Assert(t, d.Bricks != nil)
	tests.Assert(t, len(d.Bricks) == 0)

}

func TestNewDeviceEntryFromRequest(t *testing.T) {
	req := &Device{
		Name:   "dev",
		Weight: 123,
	}

	d := NewDeviceEntryFromRequest(req, "123")
	tests.Assert(t, d != nil)
	tests.Assert(t, d.Info.Id != "")
	tests.Assert(t, d.Info.Name == req.Name)
	tests.Assert(t, d.Info.Weight == req.Weight)
	tests.Assert(t, d.Info.Storage.Free == 0)
	tests.Assert(t, d.Info.Storage.Total == 0)
	tests.Assert(t, d.Info.Storage.Used == 0)
	tests.Assert(t, d.NodeId == "123")
	tests.Assert(t, d.Bricks != nil)
	tests.Assert(t, len(d.Bricks) == 0)

}

func TestNewDeviceEntryMarshal(t *testing.T) {
	req := &Device{
		Name:   "dev",
		Weight: 123,
	}
	nodeid := "abc"

	d := NewDeviceEntryFromRequest(req, nodeid)
	d.Info.Storage.Free = 10
	d.Info.Storage.Total = 100
	d.Info.Storage.Used = 1000
	d.BrickAdd("abc")
	d.BrickAdd("def")

	buffer, err := d.Marshal()
	tests.Assert(t, err == nil)
	tests.Assert(t, buffer != nil)
	tests.Assert(t, len(buffer) > 0)

	um := &DeviceEntry{}
	err = um.Unmarshal(buffer)
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(um, d))

}

func TestDeviceEntryAddDeleteBricks(t *testing.T) {
	d := NewDeviceEntry()
	tests.Assert(t, len(d.Bricks) == 0)

	d.BrickAdd("123")
	tests.Assert(t, utils.SortedStringHas(d.Bricks, "123"))
	tests.Assert(t, len(d.Bricks) == 1)
	d.BrickAdd("abc")
	tests.Assert(t, utils.SortedStringHas(d.Bricks, "123"))
	tests.Assert(t, utils.SortedStringHas(d.Bricks, "abc"))
	tests.Assert(t, len(d.Bricks) == 2)

	d.BrickDelete("123")
	tests.Assert(t, !utils.SortedStringHas(d.Bricks, "123"))
	tests.Assert(t, utils.SortedStringHas(d.Bricks, "abc"))
	tests.Assert(t, len(d.Bricks) == 1)

	d.BrickDelete("ccc")
	tests.Assert(t, !utils.SortedStringHas(d.Bricks, "123"))
	tests.Assert(t, utils.SortedStringHas(d.Bricks, "abc"))
	tests.Assert(t, len(d.Bricks) == 1)
}

func TestNewDeviceEntryFromIdNotFound(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Patch dbfilename so that it is restored at the end of the tests
	defer tests.Patch(&dbfilename, tmpfile).Restore()

	// Create the app
	app := NewApp()
	defer app.Close()

	// Test for ID not found
	err := app.db.View(func(tx *bolt.Tx) error {
		_, err := NewDeviceEntryFromId(tx, "123")
		return err
	})
	tests.Assert(t, err == ErrNotFound)

}

func TestNewDeviceEntryFromId(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Patch dbfilename so that it is restored at the end of the tests
	defer tests.Patch(&dbfilename, tmpfile).Restore()

	// Create the app
	app := NewApp()
	defer app.Close()

	// Create a device
	req := &Device{
		Name:   "dev",
		Weight: 123,
	}
	nodeid := "abc"

	d := NewDeviceEntryFromRequest(req, nodeid)
	d.Info.Storage.Free = 10
	d.Info.Storage.Total = 100
	d.Info.Storage.Used = 1000
	d.BrickAdd("abc")
	d.BrickAdd("def")

	// Save element in database
	err := app.db.Update(func(tx *bolt.Tx) error {
		return d.Save(tx)
	})
	tests.Assert(t, err == nil)

	var device *DeviceEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		var err error
		device, err = NewDeviceEntryFromId(tx, d.Info.Id)
		if err != nil {
			return err
		}
		return nil

	})
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(device, d))
}

func TestNewDeviceEntrySaveDelete(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Patch dbfilename so that it is restored at the end of the tests
	defer tests.Patch(&dbfilename, tmpfile).Restore()

	// Create the app
	app := NewApp()
	defer app.Close()

	// Create a device
	req := &Device{
		Name:   "dev",
		Weight: 123,
	}
	nodeid := "abc"

	d := NewDeviceEntryFromRequest(req, nodeid)
	d.Info.Storage.Free = 10
	d.Info.Storage.Total = 100
	d.Info.Storage.Used = 1000
	d.BrickAdd("abc")
	d.BrickAdd("def")

	// Save element in database
	err := app.db.Update(func(tx *bolt.Tx) error {
		return d.Save(tx)
	})
	tests.Assert(t, err == nil)

	var device *DeviceEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		var err error
		device, err = NewDeviceEntryFromId(tx, d.Info.Id)
		if err != nil {
			return err
		}
		return nil

	})
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(device, d))

	// Delete entry which has devices
	err = app.db.Update(func(tx *bolt.Tx) error {
		var err error
		device, err = NewDeviceEntryFromId(tx, d.Info.Id)
		if err != nil {
			return err
		}

		err = device.Delete(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == ErrConflict)

	// Delete devices in device
	device.BrickDelete("abc")
	device.BrickDelete("def")
	tests.Assert(t, len(device.Bricks) == 0)
	err = app.db.Update(func(tx *bolt.Tx) error {
		return device.Save(tx)
	})
	tests.Assert(t, err == nil)

	// Now try to delete the device
	err = app.db.Update(func(tx *bolt.Tx) error {
		var err error
		device, err = NewDeviceEntryFromId(tx, d.Info.Id)
		if err != nil {
			return err
		}

		err = device.Delete(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == nil)

	// Check device has been deleted and is not in db
	err = app.db.View(func(tx *bolt.Tx) error {
		var err error
		device, err = NewDeviceEntryFromId(tx, d.Info.Id)
		if err != nil {
			return err
		}
		return nil

	})
	tests.Assert(t, err == ErrNotFound)
}

func TestNewDeviceEntryNewInfoResponse(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Patch dbfilename so that it is restored at the end of the tests
	defer tests.Patch(&dbfilename, tmpfile).Restore()

	// Create the app
	app := NewApp()
	defer app.Close()

	// Create a device
	req := &Device{
		Name:   "dev",
		Weight: 123,
	}
	nodeid := "abc"

	d := NewDeviceEntryFromRequest(req, nodeid)
	d.Info.Storage.Free = 10
	d.Info.Storage.Total = 100
	d.Info.Storage.Used = 1000
	d.BrickAdd("abc")
	d.BrickAdd("def")

	// Save element in database
	err := app.db.Update(func(tx *bolt.Tx) error {
		return d.Save(tx)
	})
	tests.Assert(t, err == nil)

	var info *DeviceInfoResponse
	err = app.db.View(func(tx *bolt.Tx) error {
		device, err := NewDeviceEntryFromId(tx, d.Info.Id)
		if err != nil {
			return err
		}

		info, err = device.NewInfoResponse(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == nil)
	tests.Assert(t, info.Id == d.Info.Id)
	tests.Assert(t, info.Name == d.Info.Name)
	tests.Assert(t, info.Weight == d.Info.Weight)
	tests.Assert(t, reflect.DeepEqual(info.Storage, d.Info.Storage))
	tests.Assert(t, len(info.Bricks) == 0)
}

func TestDeviceEntryStorage(t *testing.T) {
	d := NewDeviceEntry()

	tests.Assert(t, d.Info.Storage.Free == 0)
	tests.Assert(t, d.Info.Storage.Total == 0)
	tests.Assert(t, d.Info.Storage.Used == 0)

	d.StorageSet(1000)
	tests.Assert(t, d.Info.Storage.Free == 1000)
	tests.Assert(t, d.Info.Storage.Total == 1000)
	tests.Assert(t, d.Info.Storage.Used == 0)

	d.StorageSet(2000)
	tests.Assert(t, d.Info.Storage.Free == 2000)
	tests.Assert(t, d.Info.Storage.Total == 2000)
	tests.Assert(t, d.Info.Storage.Used == 0)

	d.StorageAllocate(1000)
	tests.Assert(t, d.Info.Storage.Free == 1000)
	tests.Assert(t, d.Info.Storage.Total == 2000)
	tests.Assert(t, d.Info.Storage.Used == 1000)

	d.StorageAllocate(500)
	tests.Assert(t, d.Info.Storage.Free == 500)
	tests.Assert(t, d.Info.Storage.Total == 2000)
	tests.Assert(t, d.Info.Storage.Used == 1500)

	d.StorageFree(500)
	tests.Assert(t, d.Info.Storage.Free == 1000)
	tests.Assert(t, d.Info.Storage.Total == 2000)
	tests.Assert(t, d.Info.Storage.Used == 1000)

	d.StorageFree(1000)
	tests.Assert(t, d.Info.Storage.Free == 2000)
	tests.Assert(t, d.Info.Storage.Total == 2000)
	tests.Assert(t, d.Info.Storage.Used == 0)
}
