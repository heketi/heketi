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
	"github.com/heketi/tests"

	"github.com/heketi/heketi/pkg/glusterfs/api"
)

func TestBasicOperationsCleanup(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		2,    // clusters
		3,    // nodes_per_cluster
		4,    // devices_per_node,
		6*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 3

	vol := NewVolumeEntryFromRequest(req)
	vc := NewVolumeCreateOperation(vol, app.db)
	e := vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		e = MarkPendingOperationsStale(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		return nil
	})

	oc := OperationCleaner{
		db:       app.db,
		executor: app.executor,
		sel:      CleanAll,
	}
	e = oc.Clean()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// assert that pending volume create got cleaned up
	app.db.View(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})
}

func TestOperationsCleanupThreeOps(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		2,    // clusters
		3,    // nodes_per_cluster
		4,    // devices_per_node,
		6*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 3

	// create a volume we can delete later
	vol := NewVolumeEntryFromRequest(req)
	vc := NewVolumeCreateOperation(vol, app.db)
	e := RunOperation(vc, app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})
	dvol := vol

	// create 1st pending op
	vol = NewVolumeEntryFromRequest(req)
	vc = NewVolumeCreateOperation(vol, app.db)
	e = vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// create 2nd pending op
	vol = NewVolumeEntryFromRequest(req)
	vc = NewVolumeCreateOperation(vol, app.db)
	e = vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// create 3rd pending op
	vdel := NewVolumeDeleteOperation(dvol, app.db)
	e = vdel.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 3, "expected len(l) == 3, got:", len(l))
		e = MarkPendingOperationsStale(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		return nil
	})

	oc := OperationCleaner{
		db:       app.db,
		executor: app.executor,
		sel:      CleanAll,
	}
	e = oc.Clean()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.View(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})
}

func TestOperationsCleanupSkipNonLoadable(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		2,    // clusters
		3,    // nodes_per_cluster
		4,    // devices_per_node,
		6*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// create a dummy volume so there's at least one brick on a device
	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 3

	// create a volume we can delete later
	vol := NewVolumeEntryFromRequest(req)
	vc := NewVolumeCreateOperation(vol, app.db)
	e := RunOperation(vc, app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	var deviceId string
	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		dl, e := DeviceList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(dl) > 1, "expected len(dl) > 1, got:", len(dl))
		for _, d := range dl {
			dev, e := NewDeviceEntryFromId(tx, d)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			if len(dev.Bricks) >= 1 {
				deviceId = d
			}
		}
		tests.Assert(t, deviceId != "")
		return nil
	})

	dro := NewDeviceRemoveOperation(deviceId, app.db)
	e = dro.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		e = MarkPendingOperationsStale(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		return nil
	})

	oc := OperationCleaner{
		db:       app.db,
		executor: app.executor,
		sel:      CleanAll,
	}
	e = oc.Clean()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// the non cleanable device remove operation remains
	app.db.View(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})
}

func TestOperationsCleanupVolumeExpand(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		2,    // clusters
		3,    // nodes_per_cluster
		4,    // devices_per_node,
		6*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 3

	// create a volume we can delete later
	vol := NewVolumeEntryFromRequest(req)
	vc := NewVolumeCreateOperation(vol, app.db)
	e := RunOperation(vc, app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})

	ve := NewVolumeExpandOperation(vol, app.db, 50)
	e = ve.Build()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	app.db.Update(func(tx *bolt.Tx) error {
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 1, "expected len(po) == 1, got:", len(po))
		e = MarkPendingOperationsStale(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		return nil
	})

	oc := OperationCleaner{
		db:       app.db,
		executor: app.executor,
		sel:      CleanAll,
	}
	e = oc.Clean()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.View(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got:", len(l))
		vol, e := NewVolumeEntryFromId(tx, vl[0])
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, vol.Info.Size == 1024,
			"expected vol.Info.Size == 1024, got:", vol.Info.Size)
		return nil
	})
}

func TestOperationsCleanupBlockVolumeCreate(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		2,    // clusters
		3,    // nodes_per_cluster
		4,    // devices_per_node,
		6*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	req := &api.BlockVolumeCreateRequest{}
	req.Size = 1024

	vol := NewBlockVolumeEntryFromRequest(req)
	vc := NewBlockVolumeCreateOperation(vol, app.db)
	e := vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		e = MarkPendingOperationsStale(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		return nil
	})

	oc := OperationCleaner{
		db:       app.db,
		executor: app.executor,
		sel:      CleanAll,
	}
	e = oc.Clean()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.View(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})
}

func TestOperationsCleanupBlockVolumeDelete(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		2,    // clusters
		3,    // nodes_per_cluster
		4,    // devices_per_node,
		6*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	req := &api.BlockVolumeCreateRequest{}
	req.Size = 1024

	vol := NewBlockVolumeEntryFromRequest(req)
	vc := NewBlockVolumeCreateOperation(vol, app.db)
	e := RunOperation(vc, app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	vdel := NewBlockVolumeDeleteOperation(vol, app.db)
	e = vdel.Build()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		e = MarkPendingOperationsStale(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		return nil
	})

	oc := OperationCleaner{
		db:       app.db,
		executor: app.executor,
		sel:      CleanAll,
	}
	e = oc.Clean()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.View(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})
}
