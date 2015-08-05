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
	"os"
	"reflect"
	"testing"
)

func TestNewBrickEntry(t *testing.T) {

	size := uint64(10)
	deviceid := "abc"
	nodeid := "def"

	c := NewBrickEntry(size, deviceid, nodeid)
	tests.Assert(t, c.Info.Id != "")
	tests.Assert(t, c.Info.DeviceId == deviceid)
	tests.Assert(t, c.Info.NodeId == nodeid)
	tests.Assert(t, c.Info.Size == size)
}

func TestBrickEntryMarshal(t *testing.T) {
	size := uint64(10)
	deviceid := "abc"
	nodeid := "def"
	m := NewBrickEntry(size, deviceid, nodeid)

	buffer, err := m.Marshal()
	tests.Assert(t, err == nil)
	tests.Assert(t, buffer != nil)
	tests.Assert(t, len(buffer) > 0)

	um := &BrickEntry{}
	err = um.Unmarshal(buffer)
	tests.Assert(t, err == nil)

	tests.Assert(t, reflect.DeepEqual(um, m))
}

func TestNewBrickEntryFromIdNotFound(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Test for ID not found
	err := app.db.View(func(tx *bolt.Tx) error {
		_, err := NewBrickEntryFromId(tx, "123")
		return err
	})
	tests.Assert(t, err == ErrNotFound)

}

func TestNewBrickEntryFromId(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a brick
	b := NewBrickEntry(10, "abc", "def")

	// Save element in database
	err := app.db.Update(func(tx *bolt.Tx) error {
		return b.Save(tx)
	})
	tests.Assert(t, err == nil)

	var brick *BrickEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		var err error
		brick, err = NewBrickEntryFromId(tx, b.Info.Id)
		return err
	})
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(brick, b))

}

func TestNewBrickEntrySaveDelete(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a brick
	b := NewBrickEntry(10, "abc", "def")

	// Save element in database
	err := app.db.Update(func(tx *bolt.Tx) error {
		return b.Save(tx)
	})
	tests.Assert(t, err == nil)

	// Delete entry which has devices
	var brick *BrickEntry
	err = app.db.Update(func(tx *bolt.Tx) error {
		var err error
		brick, err = NewBrickEntryFromId(tx, b.Info.Id)
		if err != nil {
			return err
		}

		err = brick.Delete(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == nil)

	// Check brick has been deleted and is not in db
	err = app.db.View(func(tx *bolt.Tx) error {
		var err error
		brick, err = NewBrickEntryFromId(tx, b.Info.Id)
		return err
	})
	tests.Assert(t, err == ErrNotFound)
}

func TestNewBrickEntryNewInfoResponse(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a brick
	b := NewBrickEntry(10, "abc", "def")

	// Save element in database
	err := app.db.Update(func(tx *bolt.Tx) error {
		return b.Save(tx)
	})
	tests.Assert(t, err == nil)

	var info *BrickInfo
	err = app.db.View(func(tx *bolt.Tx) error {
		brick, err := NewBrickEntryFromId(tx, b.Id())
		if err != nil {
			return err
		}

		info, err = brick.NewInfoResponse(tx)
		return err
	})
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(*info, b.Info))
}
