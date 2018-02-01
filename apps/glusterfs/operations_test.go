package glusterfs

import (
	"os"
	"testing"

	"github.com/heketi/heketi/pkg/glusterfs/api"

	"github.com/boltdb/bolt"
	"github.com/heketi/tests"
)

func TestVolumeCreatePendingCreatedCleared(t *testing.T) {
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

	// verify that there are no volumes, bricks or pending operations
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 0, "expected len(vl) == 0, got", len(vl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 0, "expected len(bl) == 0, got", len(bl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))
		return nil
	})

	e := vc.Build(app.Allocator())
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify volumes, bricks, & pending ops exist
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got", len(vl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 3, "expected len(bl) == 3, got", len(bl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 1, "expected len(pol) == 1, got", len(pol))

		for _, vid := range vl {
			v, e := NewVolumeEntryFromId(tx, vid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, v.Pending.Id == pol[0],
				"expected v.Pending.Id == pol[0], got:", v.Pending.Id, pol[0])
		}
		for _, bid := range bl {
			b, e := NewBrickEntryFromId(tx, bid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, b.Pending.Id == pol[0],
				"expected b.Pending.Id == pol[0], got:", b.Pending.Id, pol[0])
		}
		return nil
	})

	e = vc.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	e = vc.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify volumes & bricks exist but pending is gone
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got", len(vl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 3, "expected len(bl) == 3, got", len(bl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))

		for _, vid := range vl {
			v, e := NewVolumeEntryFromId(tx, vid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, v.Pending.Id == "",
				`expected v.Pending.Id == "", got:`, v.Pending.Id)
		}
		for _, bid := range bl {
			b, e := NewBrickEntryFromId(tx, bid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, b.Pending.Id == "",
				`expected b.Pending.Id == "", got:`, b.Pending.Id)
		}
		return nil
	})
}

func TestVolumeCreatePendingRollback(t *testing.T) {
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

	// verify that there are no volumes, bricks or pending operations
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 0, "expected len(vl) == 0, got", len(vl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 0, "expected len(bl) == 0, got", len(bl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))
		return nil
	})

	e := vc.Build(app.Allocator())
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify volumes, bricks, & pending ops exist
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got", len(vl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 3, "expected len(bl) == 3, got", len(bl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 1, "expected len(pol) == 1, got", len(pol))

		for _, vid := range vl {
			v, e := NewVolumeEntryFromId(tx, vid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, v.Pending.Id == pol[0],
				"expected v.Pending.Id == pol[0], got:", v.Pending.Id, pol[0])
		}
		for _, bid := range bl {
			b, e := NewBrickEntryFromId(tx, bid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, b.Pending.Id == pol[0],
				"expected b.Pending.Id == pol[0], got:", b.Pending.Id, pol[0])
		}
		return nil
	})

	e = vc.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	e = vc.Rollback(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify that there are no volumes, bricks or pending operations
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 0, "expected len(vl) == 0, got", len(vl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 0, "expected len(bl) == 0, got", len(bl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))
		return nil
	})
}

func TestVolumeCreatePendingNoSpace(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		2,    // clusters
		3,    // nodes_per_cluster
		4,    // devices_per_node,
		1*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	req := &api.VolumeCreateRequest{}
	req.Size = 1024 * 5
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 3

	vol := NewVolumeEntryFromRequest(req)
	vc := NewVolumeCreateOperation(vol, app.db)

	// verify that there are no volumes, bricks or pending operations
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 0, "expected len(vl) == 0, got", len(vl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 0, "expected len(bl) == 0, got", len(bl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))
		return nil
	})

	e := vc.Build(app.Allocator())
	// verify that we failed to allocate due to lack of space
	tests.Assert(t, e == ErrNoSpace, "expected e == ErrNoSpace, got", e)

	// verify no volumes, bricks or pending ops in db
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 0, "expected len(vl) == 0, got", len(vl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 0, "expected len(bl) == 0, got", len(bl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))
		return nil
	})
}
