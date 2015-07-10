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
	//"github.com/heketi/heketi/utils"
	"os"
	"testing"
)

func TestNewNodeEntry(t *testing.T) {

	n := NewNodeEntry()
	tests.Assert(t, n.Info.Id == "")
	tests.Assert(t, n.Info.ClusterId == "")
	tests.Assert(t, n.Info.DevicesInfo == nil)
	tests.Assert(t, len(n.Devices) == 0)
	tests.Assert(t, n.Devices != nil)
}

func TestNewNodeEntryFromRequest(t *testing.T) {
	req := &NodeAddRequest{
		ClusterId: "123",
		Hostnames: HostAddresses{
			Manage:  []string{"manage"},
			Storage: []string{"storage"},
		},
		Zone: 99,
	}

	n := NewNodeEntryFromRequest(req)
	tests.Assert(t, n != nil)
	tests.Assert(t, n.Info.ClusterId == req.ClusterId)
	tests.Assert(t, n.Info.Zone == req.Zone)
	tests.Assert(t, len(n.Info.Id) > 0)
	tests.Assert(t, len(n.Info.Hostnames.Manage) == len(req.Hostnames.Manage))
	tests.Assert(t, len(n.Info.Hostnames.Storage) == len(req.Hostnames.Storage))
	tests.Assert(t, n.Info.Hostnames.Manage[0] == req.Hostnames.Manage[0])
	tests.Assert(t, n.Info.Hostnames.Storage[0] == req.Hostnames.Storage[0])

}

func TestNewNodeEntryFromIdNotFound(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Patch dbfilename so that it is restored at the end of the tests
	defer tests.Patch(&dbfilename, tmpfile).Restore()

	// Create the app
	app := NewApp()
	defer app.Close()

	// Test for ID not found
	err := app.db.View(func(tx *bolt.Tx) error {
		_, err := NewNodeEntryFromId(tx, "123")
		return err
	})
	tests.Assert(t, err == ErrNotFound)

}

func TestNewNodeEntryFromId(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Patch dbfilename so that it is restored at the end of the tests
	defer tests.Patch(&dbfilename, tmpfile).Restore()

	// Create the app
	app := NewApp()
	defer app.Close()

	// Test for ID not found
	err := app.db.View(func(tx *bolt.Tx) error {
		_, err := NewNodeEntryFromId(tx, "123")
		return err
	})
	tests.Assert(t, err == ErrNotFound)
}
