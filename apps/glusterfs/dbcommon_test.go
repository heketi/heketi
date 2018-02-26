//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"os"
	"testing"

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/utils"
	"github.com/heketi/tests"
)

func TestDeleteBricksWithEmptyPath(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		5*TB, // disksize
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	var nodeEntry *NodeEntry
	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	// create a few volumes
	for i := 0; i < 15; i++ {
		v := NewVolumeEntryFromRequest(vreq)
		err = v.Create(app.db, app.executor, app.Allocator())
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// grab a device that has bricks
	var d *DeviceEntry
	var newbrick *BrickEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		dl, err := DeviceList(tx)
		if err != nil {
			return err
		}
		for _, id := range dl {
			d, err = NewDeviceEntryFromId(tx, id)
			if err != nil {
				return err
			}
			if len(d.Bricks) > 0 {
				return nil
			}
		}
		t.Fatalf("should have at least one device with bricks")
		return nil
	})

	// At this point, we have 15 legal bricks on each device and we have
	// made note of a device(node and cluster in corollary) where we will
	// create 25 bricks with empty path.

	// CASE1: use "all" bool to delete empty path bricks from all clusters
	// ====================================================================
	// create bricks in device and set the path empty
	// save device and brick to db
	for i := 0; i < 25; i++ {
		newbrick = d.NewBrickEntry(102400, 1, 2000, utils.GenUUID())
		newbrick.Info.Path = ""
		d.BrickAdd(newbrick.Id())
		err = app.db.Update(func(tx *bolt.Tx) error {
			err = d.Save(tx)
			tests.Assert(t, err == nil)
			return newbrick.Save(tx)
		})
		tests.Assert(t, err == nil)
	}
	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(d.Bricks) == 40,
		"expected len(d.Bricks) == 40, got:", len(d.Bricks))

	// now delete bricks with empty path
	err = DeleteBricksWithEmptyPath(app.db, true, []string{}, []string{}, []string{}, true)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(d.Bricks) == 15,
		"expected len(d.Bricks) == 15, got:", len(d.Bricks))

	// CASE2: delete empty path bricks from this device
	// ====================================================================
	// create bricks in device and set the path empty
	// save device and brick to db
	for i := 0; i < 25; i++ {
		newbrick = d.NewBrickEntry(102400, 1, 2000, utils.GenUUID())
		newbrick.Info.Path = ""
		d.BrickAdd(newbrick.Id())
		err = app.db.Update(func(tx *bolt.Tx) error {
			err = d.Save(tx)
			tests.Assert(t, err == nil)
			return newbrick.Save(tx)
		})
		tests.Assert(t, err == nil)
	}
	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(d.Bricks) == 40,
		"expected len(d.Bricks) == 40, got:", len(d.Bricks))

	err = DeleteBricksWithEmptyPath(app.db, false, []string{}, []string{}, []string{d.Info.Id}, true)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(d.Bricks) == 15,
		"expected len(d.Bricks) == 15, got:", len(d.Bricks))

	// CASE3: delete empty path bricks from a node
	// ====================================================================
	// create bricks in device and set the path empty
	// save device and brick to db
	for i := 0; i < 25; i++ {
		newbrick = d.NewBrickEntry(102400, 1, 2000, utils.GenUUID())
		newbrick.Info.Path = ""
		d.BrickAdd(newbrick.Id())
		err = app.db.Update(func(tx *bolt.Tx) error {
			err = d.Save(tx)
			tests.Assert(t, err == nil)
			return newbrick.Save(tx)
		})
		tests.Assert(t, err == nil)
	}
	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(d.Bricks) == 40,
		"expected len(d.Bricks) == 40, got:", len(d.Bricks))

	err = DeleteBricksWithEmptyPath(app.db, false, []string{}, []string{d.NodeId, d.NodeId}, []string{}, true)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(d.Bricks) == 15,
		"expected len(d.Bricks) == 15, got:", len(d.Bricks))

	// CASE4: delete empty path bricks from a cluster
	// ====================================================================
	// create bricks in device and set the path empty
	// save device and brick to db
	for i := 0; i < 25; i++ {
		newbrick = d.NewBrickEntry(102400, 1, 2000, utils.GenUUID())
		newbrick.Info.Path = ""
		d.BrickAdd(newbrick.Id())
		err = app.db.Update(func(tx *bolt.Tx) error {
			err = d.Save(tx)
			tests.Assert(t, err == nil)
			return newbrick.Save(tx)
		})
		tests.Assert(t, err == nil)
	}
	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(d.Bricks) == 40,
		"expected len(d.Bricks) == 40, got:", len(d.Bricks))

	err = app.db.View(func(tx *bolt.Tx) error {
		nodeEntry, err = NewNodeEntryFromId(tx, d.NodeId)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = DeleteBricksWithEmptyPath(app.db, false, []string{nodeEntry.Info.ClusterId}, []string{d.NodeId}, []string{}, true)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(d.Bricks) == 15,
		"expected len(d.Bricks) == 15, got:", len(d.Bricks))
}
