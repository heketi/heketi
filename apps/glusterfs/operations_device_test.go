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
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/heketi/heketi/v10/executors"
	"github.com/heketi/heketi/v10/pkg/glusterfs/api"

	"github.com/boltdb/bolt"
	"github.com/heketi/tests"
)

func TestDeviceRemoveOperationEmpty(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		3,    // devices_per_node,
		8*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// grab a device
	var d *DeviceEntry
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
			break
		}
		return nil
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = d.SetState(app.db, app.executor, api.StateRequest{State: api.EntryStateOffline})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	dro := NewDeviceRemoveOperation(d.Info.Id, app.db, api.HealCheckEnable)
	err = dro.Build()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// because there are no bricks on this device it can be disabled
	// instantly and there are no pending ops for it in the db
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})

	err = dro.Exec(app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = dro.Finalize()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}

func TestDeviceRemoveOperationWithBricks(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		3,    // devices_per_node,
		8*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	// create two volumes
	for i := 0; i < 5; i++ {
		v := NewVolumeEntryFromRequest(vreq)
		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// grab a devices that has bricks
	var d *DeviceEntry
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
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = d.SetState(app.db, app.executor, api.StateRequest{State: api.EntryStateOffline})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	dro := NewDeviceRemoveOperation(d.Info.Id, app.db, api.HealCheckEnable)
	err = dro.Build()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// because there were bricks on this device it needs to perform
	// a full "operation cycle"
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})

	app.xo.MockVolumeInfo = func(host string, volume string) (*executors.Volume, error) {
		return mockVolumeInfoFromDb(app.db, volume)
	}
	app.xo.MockHealInfo = func(host string, volume string) (*executors.HealInfo, error) {
		return mockHealStatusFromDb(app.db, volume)
	}

	err = dro.Exec(app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// operation is not over. we should still have a pending op
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})

	err = dro.Finalize()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// operation is over. we should _not_ have a pending op now
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})

	// update d from db
	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	// our d should be w/o bricks and be in failed state
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(d.Bricks) == 0,
		"expected len(d.Bricks) == 0, got:", len(d.Bricks))
	tests.Assert(t, d.State == api.EntryStateFailed)
}

func TestDeviceRemoveOperationTooFewDevices(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		8*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	// create two volumes
	for i := 0; i < 5; i++ {
		v := NewVolumeEntryFromRequest(vreq)
		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// grab a devices that has bricks
	var d *DeviceEntry
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
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = d.SetState(app.db, app.executor, api.StateRequest{State: api.EntryStateOffline})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	dro := NewDeviceRemoveOperation(d.Info.Id, app.db, api.HealCheckEnable)
	err = dro.Build()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// because there were bricks on this device it needs to perform
	// a full "operation cycle"
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})

	app.xo.MockVolumeInfo = func(host string, volume string) (*executors.Volume, error) {
		return mockVolumeInfoFromDb(app.db, volume)
	}
	app.xo.MockHealInfo = func(host string, volume string) (*executors.HealInfo, error) {
		return mockHealStatusFromDb(app.db, volume)
	}

	err = dro.Exec(app.executor)
	tests.Assert(t, strings.Contains(err.Error(), ErrNoReplacement.Error()),
		"expected strings.Contains(err.Error(), ErrNoReplacement.Error()), got:",
		err.Error())

	// operation is not over. we should still have a pending op
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})

	err = dro.Rollback(app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// operation is over. we should _not_ have a pending op now
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})

	// update d from db
	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	// our d should be in the original state because the exec failed
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(d.Bricks) > 0,
		"expected len(d.Bricks) > 0, got:", len(d.Bricks))
	tests.Assert(t, d.State == api.EntryStateOffline)
}

func TestDeviceRemoveOperationOtherPendingOps(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		8*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	// create two volumes
	for i := 0; i < 4; i++ {
		v := NewVolumeEntryFromRequest(vreq)
		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// grab a devices that has bricks
	var d *DeviceEntry
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
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// now start a volume create operation but don't finish it
	vol := NewVolumeEntryFromRequest(vreq)
	vc := NewVolumeCreateOperation(vol, app.db)
	err = vc.Build()
	tests.Assert(t, err == nil, "expected e == nil, got", err)
	// we should have one pending operation
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})

	err = d.SetState(app.db, app.executor, api.StateRequest{State: api.EntryStateOffline})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	dro := NewDeviceRemoveOperation(d.Info.Id, app.db, api.HealCheckEnable)
	err = dro.Build()
	tests.Assert(t, err == ErrConflict, "expected err == ErrConflict, got:", err)

	// we should have one pending operation (the volume create)
	app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})
}

// TestDeviceRemoveOperationMultipleRequests tests that
// the system fails gracefully if a remove device request
// comes in while an existing operation is already in progress.
func TestDeviceRemoveOperationMultipleRequests(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		8*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	// create volumes
	for i := 0; i < 4; i++ {
		v := NewVolumeEntryFromRequest(vreq)
		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// grab a device that has bricks
	var d *DeviceEntry
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
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = d.SetState(app.db, app.executor, api.StateRequest{State: api.EntryStateOffline})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// perform the build step of one remove operation
	dro := NewDeviceRemoveOperation(d.Info.Id, app.db, api.HealCheckEnable)
	err = dro.Build()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// perform the build step of a 2nd remove operation
	// we can "fake' it this way in a test because the transactions
	// that cover the Build steps are effectively serializing
	// these actions.
	dro2 := NewDeviceRemoveOperation(d.Info.Id, app.db, api.HealCheckEnable)
	err = dro2.Build()
	tests.Assert(t, err == ErrConflict, "expected err == ErrConflict, got:", err)

	// we should have one pending operation (the device remove)
	app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})

}

func TestBrickEvictOperation(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		3,    // devices_per_node,
		8*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	v := NewVolumeEntryFromRequest(vreq)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	var b *BrickEntry
	app.db.View(func(tx *bolt.Tx) error {
		bl, err := BrickList(tx)
		tests.Assert(t, err == nil)
		tests.Assert(t, len(bl) == 3)
		b, err = NewBrickEntryFromId(tx, bl[0])
		tests.Assert(t, err == nil)
		return nil
	})

	beo := NewBrickEvictOperation(b.Info.Id, app.db, api.HealCheckEnable)
	err = beo.Build()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		bl, err := BrickList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		// because gluster is gluster, we can't allocate a brick until
		// the exec step. we will only see 3 bricks after build
		tests.Assert(t, len(bl) == 3, "expected len(l) == 1, got:", len(l))
		return nil
	})

	app.xo.MockVolumeInfo = func(host string, volume string) (*executors.Volume, error) {
		return mockVolumeInfoFromDb(app.db, volume)
	}
	app.xo.MockHealInfo = func(host string, volume string) (*executors.HealInfo, error) {
		return mockHealStatusFromDb(app.db, volume)
	}

	err = beo.Exec(app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// operation is not over. we should still have a pending op
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		bl, err := BrickList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		// the new brick should be in the db
		tests.Assert(t, len(bl) == 4, "expected len(l) == 1, got:", len(l))
		// the new brick should be pending
		pc := 0
		for _, brickId := range bl {
			b, err := NewBrickEntryFromId(tx, brickId)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			if b.Pending.Id != "" {
				logger.Info("Pending Brick: %v", b.Id())
				pc += 1
			}
		}
		tests.Assert(t, pc == 2, "expected 2 pending bricks, got:", pc)
		return nil
	})

	err = beo.Finalize()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// operation is over. we should _not_ have a pending op now
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))

		bl, err := BrickList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		// the new brick should be in the db, and old is gone
		tests.Assert(t, len(bl) == 3, "expected len(l) == 1, got:", len(l))
		// the new brick should be pending
		pc := 0
		for _, brickId := range bl {
			b, err := NewBrickEntryFromId(tx, brickId)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			if b.Pending.Id != "" {
				logger.Info("Pending Brick: %v", b.Id())
				pc += 1
			}
		}
		tests.Assert(t, pc == 0, "expected 0 pending bricks, got:", pc)
		return nil
	})

	// TODO : assert: old brick is gone, new brick in place
}

func TestBrickEvictOperationOneAtATime(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		3,    // devices_per_node,
		8*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	v := NewVolumeEntryFromRequest(vreq)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	var b1, b2 *BrickEntry
	app.db.View(func(tx *bolt.Tx) error {
		bl, err := BrickList(tx)
		tests.Assert(t, err == nil)
		tests.Assert(t, len(bl) == 3)
		b1, err = NewBrickEntryFromId(tx, bl[0])
		tests.Assert(t, err == nil)
		b2, err = NewBrickEntryFromId(tx, bl[1])
		tests.Assert(t, err == nil)
		return nil
	})

	beo1 := NewBrickEvictOperation(b1.Info.Id, app.db, api.HealCheckEnable)
	err = beo1.Build()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	beo2 := NewBrickEvictOperation(b2.Info.Id, app.db, api.HealCheckEnable)
	err = beo2.Build()
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
	tests.Assert(t, strings.Contains(err.Error(), "pending"),
		"expected 'pedning' in error, got:", err)

	beo3 := NewBrickEvictOperation(b1.Info.Id, app.db, api.HealCheckEnable)
	err = beo3.Build()
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
	tests.Assert(t, strings.Contains(err.Error(), "pending"),
		"expected 'pedning' in error, got:", err)
}

func TestBrickEvictOperationError(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		3,    // devices_per_node,
		8*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	v := NewVolumeEntryFromRequest(vreq)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	var b *BrickEntry
	app.db.View(func(tx *bolt.Tx) error {
		bl, err := BrickList(tx)
		tests.Assert(t, err == nil)
		tests.Assert(t, len(bl) == 3)
		b, err = NewBrickEntryFromId(tx, bl[0])
		tests.Assert(t, err == nil)
		return nil
	})

	app.xo.MockVolumeInfo = func(host string, volume string) (*executors.Volume, error) {
		return nil, fmt.Errorf("blarf")
	}
	app.xo.MockHealInfo = func(host string, volume string) (*executors.HealInfo, error) {
		return mockHealStatusFromDb(app.db, volume)
	}

	beo := NewBrickEvictOperation(b.Info.Id, app.db, api.HealCheckEnable)
	err = RunOperation(beo, app.executor)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)

	// operation is over. we should _not_ have a pending op now
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))

		bl, err := BrickList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(bl) == 3, "expected len(l) == 1, got:", len(l))
		pc := 0
		for _, brickId := range bl {
			b, err := NewBrickEntryFromId(tx, brickId)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			if b.Pending.Id != "" {
				logger.Info("Pending Brick: %v", b.Id())
				pc += 1
			}
		}
		tests.Assert(t, pc == 0, "expected 0 pending bricks, got:", pc)
		return nil
	})
}

func TestBrickEvictOperationErrorAfterBrick(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		3,    // devices_per_node,
		8*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	v := NewVolumeEntryFromRequest(vreq)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	var b *BrickEntry
	app.db.View(func(tx *bolt.Tx) error {
		bl, err := BrickList(tx)
		tests.Assert(t, err == nil)
		tests.Assert(t, len(bl) == 3)
		b, err = NewBrickEntryFromId(tx, bl[0])
		tests.Assert(t, err == nil)
		return nil
	})

	app.xo.MockVolumeInfo = func(host string, volume string) (*executors.Volume, error) {
		return mockVolumeInfoFromDb(app.db, volume)
	}
	app.xo.MockHealInfo = func(host string, volume string) (*executors.HealInfo, error) {
		return mockHealStatusFromDb(app.db, volume)
	}
	app.xo.MockVolumeReplaceBrick = func(host string, volume string, oldBrick *executors.BrickInfo, newBrick *executors.BrickInfo) error {
		return fmt.Errorf("Whoopsie")
	}

	beo := NewBrickEvictOperation(b.Info.Id, app.db, api.HealCheckEnable)
	err = RunOperation(beo, app.executor)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)

	// operation is over. we should _not_ have a pending op now
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))

		bl, err := BrickList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(bl) == 3, "expected len(l) == 1, got:", len(l))
		pc := 0
		for _, brickId := range bl {
			b, err := NewBrickEntryFromId(tx, brickId)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			if b.Pending.Id != "" {
				logger.Info("Pending Brick: %v", b.Id())
				pc += 1
			}
		}
		tests.Assert(t, pc == 0, "expected 0 pending bricks, got:", pc)
		return nil
	})
}

func TestBrickEvictOperationInvalidExec(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		3,    // devices_per_node,
		8*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	v := NewVolumeEntryFromRequest(vreq)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	var b *BrickEntry
	app.db.View(func(tx *bolt.Tx) error {
		bl, err := BrickList(tx)
		tests.Assert(t, err == nil)
		tests.Assert(t, len(bl) == 3)
		b, err = NewBrickEntryFromId(tx, bl[0])
		tests.Assert(t, err == nil)
		return nil
	})

	beo := NewBrickEvictOperation(b.Info.Id, app.db, api.HealCheckEnable)
	err = beo.Build()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	var pop *PendingOperationEntry
	err = app.db.Update(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		bl, err := BrickList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		// because gluster is gluster, we can't allocate a brick until
		// the exec step. we will only see 3 bricks after build
		tests.Assert(t, len(bl) == 3, "expected len(l) == 1, got:", len(l))

		// going to intentionally muddle the operation in order to
		// trigger an error check
		pop, err = NewPendingOperationEntryFromId(tx, l[0])
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, pop.Type == OperationBrickEvict)
		tests.Assert(t, len(pop.Actions) == 1)
		tests.Assert(t, pop.Actions[0].Change == OpDeleteBrick)
		pop.Actions = append(pop.Actions,
			PendingOperationAction{Change: OpAddBrick, Id: "FOOBAR"})
		err = pop.Save(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		return nil
	})
	o, err := LoadOperation(app.db, pop)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	beo = o.(*BrickEvictOperation)

	app.xo.MockVolumeInfo = func(host string, volume string) (*executors.Volume, error) {
		return mockVolumeInfoFromDb(app.db, volume)
	}
	app.xo.MockHealInfo = func(host string, volume string) (*executors.HealInfo, error) {
		return mockHealStatusFromDb(app.db, volume)
	}

	err = beo.Exec(app.executor)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
}

func TestBrickEvictOperationErrorNotReplaced(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		3,    // devices_per_node,
		8*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	v := NewVolumeEntryFromRequest(vreq)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	var (
		bl []string
		b  *BrickEntry
	)
	app.db.View(func(tx *bolt.Tx) error {
		var err error
		bl, err = BrickList(tx)
		tests.Assert(t, err == nil)
		tests.Assert(t, len(bl) == 3)
		b, err = NewBrickEntryFromId(tx, bl[0])
		tests.Assert(t, err == nil)
		return nil
	})

	app.xo.MockVolumeInfo = func(host string, volume string) (*executors.Volume, error) {
		// this volume info must never return the "new" brick in
		// order to convince the mock to act like a volume info that
		// didn't have its brick replaced in gluster
		m, err := mockVolumeInfoFromDb(app.db, volume)
		if err != nil {
			return m, err
		}
		bx := []executors.Brick{}
		for _, bi := range m.Bricks.BrickList {
			for _, brickId := range bl {
				if strings.Contains(bi.Name, brickId) {
					bx = append(bx, bi)
				}
			}
		}
		m.Bricks.BrickList = bx
		return m, nil
	}
	app.xo.MockHealInfo = func(host string, volume string) (*executors.HealInfo, error) {
		return mockHealStatusFromDb(app.db, volume)
	}
	app.xo.MockVolumeReplaceBrick = func(host string, volume string, oldBrick *executors.BrickInfo, newBrick *executors.BrickInfo) error {
		return fmt.Errorf("Whoopsie")
	}

	beo := NewBrickEvictOperation(b.Info.Id, app.db, api.HealCheckEnable)
	err = RunOperation(beo, app.executor)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)

	// operation is over. we should _not_ have a pending op now
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))

		bl, err := BrickList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(bl) == 3, "expected len(l) == 1, got:", len(l))
		pc := 0
		for _, brickId := range bl {
			b, err := NewBrickEntryFromId(tx, brickId)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			if b.Pending.Id != "" {
				logger.Info("Pending Brick: %v", b.Id())
				pc += 1
			}
		}
		tests.Assert(t, pc == 0, "expected 0 pending bricks, got:", pc)
		return nil
	})
}

func TestDeviceRemoveOperationChildOpError(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		3,    // devices_per_node,
		8*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	// create two volumes
	for i := 0; i < 5; i++ {
		v := NewVolumeEntryFromRequest(vreq)
		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// grab a devices that has bricks
	var d *DeviceEntry
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
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = d.SetState(app.db, app.executor, api.StateRequest{State: api.EntryStateOffline})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	dro := NewDeviceRemoveOperation(d.Info.Id, app.db, api.HealCheckEnable)
	err = dro.Build()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// because there were bricks on this device it needs to perform
	// a full "operation cycle"
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})

	app.xo.MockVolumeInfo = func(host string, volume string) (*executors.Volume, error) {
		return mockVolumeInfoFromDb(app.db, volume)
	}
	app.xo.MockHealInfo = func(host string, volume string) (*executors.HealInfo, error) {
		return mockHealStatusFromDb(app.db, volume)
	}
	app.xo.MockVolumeReplaceBrick = func(host string, volume string, oldBrick *executors.BrickInfo, newBrick *executors.BrickInfo) error {
		panic("ARGLE")
	}

	err = func() (x error) {
		defer func() {
			if r := recover(); r != nil {
				x = fmt.Errorf("panicked")
			}
		}()
		x = dro.Exec(app.executor)
		t.Fatalf("Test should not reach this line")
		return x
	}()
	tests.Assert(t, err != nil, "expected err != nil, got:", err)

	// exec failed. check pending ops status
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		// because we panicked within the child operation we
		// should have two pending ops
		tests.Assert(t, len(l) == 2, "expected len(l) == 2, got:", len(l))

		var popRemove, popEvict *PendingOperationEntry
		for _, id := range l {
			p, err := NewPendingOperationEntryFromId(tx, id)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			switch p.Type {
			case OperationRemoveDevice:
				popRemove = p
			case OperationBrickEvict:
				popEvict = p
			default:
				t.Fatalf("Invalid pending operation type for this test")
			}
		}

		var chAction PendingOperationAction
		for _, a := range popRemove.Actions {
			if a.Change == OpChildOperation {
				chAction = a
			}
		}
		tests.Assert(t, chAction.Id != "")
		tests.Assert(t, chAction.Id == popEvict.Id,
			"expected chAction.Id == popEvict.Id, got:",
			chAction.Id, popEvict.Id)

		return nil
	})
}

func TestDeviceRemoveOperationChildOpCleanup(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		3,    // devices_per_node,
		8*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	// create two volumes
	for i := 0; i < 5; i++ {
		v := NewVolumeEntryFromRequest(vreq)
		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// grab a devices that has bricks
	var d *DeviceEntry
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
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = d.SetState(app.db, app.executor, api.StateRequest{State: api.EntryStateOffline})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	dro := NewDeviceRemoveOperation(d.Info.Id, app.db, api.HealCheckEnable)
	err = dro.Build()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// because there were bricks on this device it needs to perform
	// a full "operation cycle"
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})

	app.xo.MockVolumeInfo = func(host string, volume string) (*executors.Volume, error) {
		return mockVolumeInfoFromDb(app.db, volume)
	}
	app.xo.MockHealInfo = func(host string, volume string) (*executors.HealInfo, error) {
		return mockHealStatusFromDb(app.db, volume)
	}
	app.xo.MockVolumeReplaceBrick = func(host string, volume string, oldBrick *executors.BrickInfo, newBrick *executors.BrickInfo) error {
		panic("ARGLE")
	}

	err = func() (x error) {
		defer func() {
			if r := recover(); r != nil {
				x = fmt.Errorf("panicked")
			}
		}()
		x = dro.Exec(app.executor)
		t.Fatalf("Test should not reach this line")
		return x
	}()
	tests.Assert(t, err != nil, "expected err != nil, got:", err)

	// exec failed. check pending ops status
	var popRemove, popEvict *PendingOperationEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		// because we panicked within the child operation we
		// should have two pending ops
		tests.Assert(t, len(l) == 2, "expected len(l) == 2, got:", len(l))

		for _, id := range l {
			p, err := NewPendingOperationEntryFromId(tx, id)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			switch p.Type {
			case OperationRemoveDevice:
				popRemove = p
			case OperationBrickEvict:
				popEvict = p
			default:
				t.Fatalf("Invalid pending operation type for this test")
			}
		}
		return nil
	})
	_ = popRemove
	_ = popEvict

	err = dro.Clean(app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = dro.CleanDone()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})
}
