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
	"github.com/heketi/tests"
	"github.com/heketi/utils"
	"os"
	"reflect"
	"testing"
)

func createSampleNodeEntry() *NodeEntry {
	req := &NodeAddRequest{
		ClusterId: "123",
		Hostnames: HostAddresses{
			Manage:  []string{"manage" + utils.GenUUID()[:8]},
			Storage: []string{"storage" + utils.GenUUID()[:8]},
		},
		Zone: 99,
	}

	return NewNodeEntryFromRequest(req)
}

func TestNewNodeEntry(t *testing.T) {

	n := NewNodeEntry()
	tests.Assert(t, n.Info.Id == "")
	tests.Assert(t, n.Info.ClusterId == "")
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

func TestNewNodeEntryMarshal(t *testing.T) {
	req := &NodeAddRequest{
		ClusterId: "123",
		Hostnames: HostAddresses{
			Manage:  []string{"manage"},
			Storage: []string{"storage"},
		},
		Zone: 99,
	}

	n := NewNodeEntryFromRequest(req)
	n.DeviceAdd("abc")
	n.DeviceAdd("def")

	buffer, err := n.Marshal()
	tests.Assert(t, err == nil)
	tests.Assert(t, buffer != nil)
	tests.Assert(t, len(buffer) > 0)

	um := &NodeEntry{}
	err = um.Unmarshal(buffer)
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(n, um))

}

func TestNodeEntryAddDeleteDevices(t *testing.T) {
	n := NewNodeEntry()
	tests.Assert(t, len(n.Devices) == 0)

	n.DeviceAdd("123")
	tests.Assert(t, utils.SortedStringHas(n.Devices, "123"))
	tests.Assert(t, len(n.Devices) == 1)
	n.DeviceAdd("abc")
	tests.Assert(t, utils.SortedStringHas(n.Devices, "123"))
	tests.Assert(t, utils.SortedStringHas(n.Devices, "abc"))
	tests.Assert(t, len(n.Devices) == 2)

	n.DeviceDelete("123")
	tests.Assert(t, !utils.SortedStringHas(n.Devices, "123"))
	tests.Assert(t, utils.SortedStringHas(n.Devices, "abc"))
	tests.Assert(t, len(n.Devices) == 1)

	n.DeviceDelete("ccc")
	tests.Assert(t, !utils.SortedStringHas(n.Devices, "123"))
	tests.Assert(t, utils.SortedStringHas(n.Devices, "abc"))
	tests.Assert(t, len(n.Devices) == 1)
}

func TestNodeEntryRegister(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a node
	req := &NodeAddRequest{
		ClusterId: "123",
		Hostnames: HostAddresses{
			Manage:  []string{"manage"},
			Storage: []string{"storage"},
		},
		Zone: 99,
	}
	n := NewNodeEntryFromRequest(req)

	// Register node
	err := app.db.Update(func(tx *bolt.Tx) error {
		err := n.Register(tx)
		tests.Assert(t, err == nil)

		return err
	})
	tests.Assert(t, err == nil)

	// Should not be able to register again
	err = app.db.Update(func(tx *bolt.Tx) error {
		err := n.Register(tx)
		tests.Assert(t, err != nil)

		return err
	})
	tests.Assert(t, err != nil)

	// Create a new node on *different* cluster
	req = &NodeAddRequest{
		ClusterId: "abc",
		Hostnames: HostAddresses{
			// Same name as previous
			Manage:  []string{"manage"},
			Storage: []string{"storage"},
		},
		Zone: 99,
	}
	diff_cluster_n := NewNodeEntryFromRequest(req)

	// Should not be able to register diff_cluster_n
	err = app.db.Update(func(tx *bolt.Tx) error {
		err := diff_cluster_n.Register(tx)
		tests.Assert(t, err != nil)

		return err
	})
	tests.Assert(t, err != nil)

	// Add a new node
	req = &NodeAddRequest{
		ClusterId: "3",
		Hostnames: HostAddresses{
			Manage:  []string{"manage2"},
			Storage: []string{"storage2"},
		},
		Zone: 99,
	}
	n2 := NewNodeEntryFromRequest(req)

	// Register n2 node
	err = app.db.Update(func(tx *bolt.Tx) error {
		err := n2.Register(tx)
		tests.Assert(t, err == nil)

		return err
	})
	tests.Assert(t, err == nil)

	// Remove n
	err = app.db.Update(func(tx *bolt.Tx) error {
		err := n.Deregister(tx)
		tests.Assert(t, err == nil)

		return err
	})
	tests.Assert(t, err == nil)

	// Register n node again
	err = app.db.Update(func(tx *bolt.Tx) error {
		err := n.Register(tx)
		tests.Assert(t, err == nil)

		return err
	})
	tests.Assert(t, err == nil)

}
func TestNewNodeEntryFromIdNotFound(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
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

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a node
	req := &NodeAddRequest{
		ClusterId: "123",
		Hostnames: HostAddresses{
			Manage:  []string{"manage"},
			Storage: []string{"storage"},
		},
		Zone: 99,
	}

	n := NewNodeEntryFromRequest(req)
	n.DeviceAdd("abc")
	n.DeviceAdd("def")

	// Save element in database
	err := app.db.Update(func(tx *bolt.Tx) error {
		return n.Save(tx)
	})
	tests.Assert(t, err == nil)

	var node *NodeEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		var err error
		node, err = NewNodeEntryFromId(tx, n.Info.Id)
		if err != nil {
			return err
		}
		return nil

	})
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(node, n))

}

func TestNewNodeEntrySaveDelete(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a node
	req := &NodeAddRequest{
		ClusterId: "123",
		Hostnames: HostAddresses{
			Manage:  []string{"manage"},
			Storage: []string{"storage"},
		},
		Zone: 99,
	}

	n := NewNodeEntryFromRequest(req)
	n.DeviceAdd("abc")
	n.DeviceAdd("def")

	// Save element in database
	err := app.db.Update(func(tx *bolt.Tx) error {
		return n.Save(tx)
	})
	tests.Assert(t, err == nil)

	var node *NodeEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		var err error
		node, err = NewNodeEntryFromId(tx, n.Info.Id)
		if err != nil {
			return err
		}
		return nil

	})
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(node, n))

	// Delete entry which has devices
	err = app.db.Update(func(tx *bolt.Tx) error {
		var err error
		node, err = NewNodeEntryFromId(tx, n.Info.Id)
		if err != nil {
			return err
		}

		err = node.Delete(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == ErrConflict)

	// Delete devices in node
	node.DeviceDelete("abc")
	node.DeviceDelete("def")
	tests.Assert(t, len(node.Devices) == 0)
	err = app.db.Update(func(tx *bolt.Tx) error {
		return node.Save(tx)
	})
	tests.Assert(t, err == nil)

	// Now try to delete the node
	err = app.db.Update(func(tx *bolt.Tx) error {
		var err error
		node, err = NewNodeEntryFromId(tx, n.Info.Id)
		if err != nil {
			return err
		}

		err = node.Delete(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == nil)

	// Check node has been deleted and is not in db
	err = app.db.View(func(tx *bolt.Tx) error {
		var err error
		node, err = NewNodeEntryFromId(tx, n.Info.Id)
		if err != nil {
			return err
		}
		return nil

	})
	tests.Assert(t, err == ErrNotFound)
}

func TestNewNodeEntryNewInfoResponse(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a node
	req := &NodeAddRequest{
		ClusterId: "123",
		Hostnames: HostAddresses{
			Manage:  []string{"manage"},
			Storage: []string{"storage"},
		},
		Zone: 99,
	}

	n := NewNodeEntryFromRequest(req)

	// Save element in database
	err := app.db.Update(func(tx *bolt.Tx) error {
		return n.Save(tx)
	})
	tests.Assert(t, err == nil)

	var info *NodeInfoResponse
	err = app.db.View(func(tx *bolt.Tx) error {
		node, err := NewNodeEntryFromId(tx, n.Info.Id)
		if err != nil {
			return err
		}

		info, err = node.NewInfoReponse(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == nil)

	tests.Assert(t, info.ClusterId == n.Info.ClusterId)
	tests.Assert(t, info.Id == n.Info.Id)
	tests.Assert(t, info.Zone == n.Info.Zone)
	tests.Assert(t, len(info.Hostnames.Manage) == 1)
	tests.Assert(t, len(info.Hostnames.Storage) == 1)
	tests.Assert(t, reflect.DeepEqual(info.Hostnames.Manage, n.Info.Hostnames.Manage))
	tests.Assert(t, reflect.DeepEqual(info.Hostnames.Storage, n.Info.Hostnames.Storage))
}
