package glusterfs

import (
	"os"
	"strings"
	"testing"

	"github.com/heketi/heketi/executors"
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

func TestVolumeCreatePendingBrickMissing(t *testing.T) {
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

	app.db.Update(func(tx *bolt.Tx) error {
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		b, e := NewBrickEntryFromId(tx, bl[0])
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		e = b.Delete(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		return nil
	})

	// now that the brick list in the db is broken Exec/Finalize/Rollback
	// will return errors

	e = vc.Exec(app.executor)
	tests.Assert(t, e != nil, "expected e != nil, got", e)

	e = vc.Finalize()
	tests.Assert(t, e != nil, "expected e != nil, got", e)

	e = vc.Rollback(app.executor)
	tests.Assert(t, e != nil, "expected e != nil, got", e)
}

func TestVolumeCreateOperationBasics(t *testing.T) {
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
	vol.Info.Id = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	vc := NewVolumeCreateOperation(vol, app.db)

	tests.Assert(t, vc.Id() == vc.op.Id,
		"expected vc.Id() == vc.op.Id, got:", vc.Id(), vc.op.Id)
	tests.Assert(t, vc.Label() == "Create Volume",
		`expected vc.Label() == "Volume Create", got:`, vc.Label())
	tests.Assert(t, vc.ResourceUrl() == "/volumes/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		`expected vc.ResourceUrl() == "/volumes/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", got:`,
		vc.ResourceUrl())
}

func TestVolumeDeleteOperation(t *testing.T) {
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

	// first we need to create a volume to delete
	vol := NewVolumeEntryFromRequest(req)
	vc := NewVolumeCreateOperation(vol, app.db)

	e := vc.Build(app.Allocator())
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	e = vc.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	e = vc.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	app.db.View(func(tx *bolt.Tx) error {
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 3, "expected len(bl) == 3, got:", len(bl))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 0, "expected len(po) == 0, got:", len(po))
		return nil
	})

	vd := NewVolumeDeleteOperation(vol, app.db)
	e = vd.Build(app.Allocator())
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	app.db.View(func(tx *bolt.Tx) error {
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 3, "expected len(bl) == 3, got:", len(bl))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 1, "expected len(po) == 1, got:", len(po))
		return nil
	})

	e = vd.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	e = vd.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	app.db.View(func(tx *bolt.Tx) error {
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 0, "expected len(bl) == 0, got:", len(bl))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 0, "expected len(po) == 0, got:", len(po))
		return nil
	})
}

func TestVolumeExpandOperation(t *testing.T) {
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

	// first we need to create a volume to delete
	vol := NewVolumeEntryFromRequest(req)
	vc := NewVolumeCreateOperation(vol, app.db)

	e := vc.Build(app.Allocator())
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	e = vc.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	e = vc.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	app.db.View(func(tx *bolt.Tx) error {
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 3, "expected len(bl) == 3, got:", len(bl))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 0, "expected len(po) == 0, got:", len(po))
		return nil
	})

	ve := NewVolumeExpandOperation(vol, app.db, 100)
	e = ve.Build(app.Allocator())
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	app.db.View(func(tx *bolt.Tx) error {
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 6, "expected len(bl) == 6, got:", len(bl))
		pcount := 0
		for _, id := range bl {
			b, e := NewBrickEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			if b.Pending.Id != "" {
				pcount++
			}
		}
		tests.Assert(t, pcount == 3, "expected len(bl) == 3, got:", pcount)
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 1, "expected len(po) == 1, got:", len(po))
		return nil
	})

	e = ve.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	e = ve.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	app.db.View(func(tx *bolt.Tx) error {
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 6, "expected len(bl) == 6, got:", len(bl))
		pcount := 0
		for _, id := range bl {
			b, e := NewBrickEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			if b.Pending.Id != "" {
				pcount++
			}
		}
		tests.Assert(t, pcount == 0, "expected len(bl) == 0, got:", pcount)
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 0, "expected len(po) == 0, got:", len(po))
		return nil
	})
}

func TestBlockVolumeCreateOperation(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		2*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	req := &api.BlockVolumeCreateRequest{}
	req.Size = 1024

	vol := NewBlockVolumeEntryFromRequest(req)
	vc := NewBlockVolumeCreateOperation(vol, app.db)

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

	// verify there is one pending op, volume and some bricks
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
		return nil
	})

	e = vc.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	e = vc.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify the volume and bricks exist but no pending op
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
		return nil
	})
}

func TestBlockVolumeCreateOperationExistingHostVol(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		3*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// first we create a volume to host the block volume

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 2048
	vreq.Block = true
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3

	vol := NewVolumeEntryFromRequest(vreq)
	vc := NewVolumeCreateOperation(vol, app.db)

	e := vc.Build(app.Allocator())
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	e = vc.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	e = vc.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	app.db.View(func(tx *bolt.Tx) error {
		bvl, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvl) == 0, "expected len(bvl) == 0, got", len(bvl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 3, "expected len(bl) == 3, got:", len(bl))
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got:", len(vl))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 0, "expected len(po) == 0, got:", len(po))
		return nil
	})

	breq := &api.BlockVolumeCreateRequest{}
	breq.Size = 1024

	bvol := NewBlockVolumeEntryFromRequest(breq)
	bco := NewBlockVolumeCreateOperation(bvol, app.db)

	e = bco.Build(app.Allocator())
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	// at this point we shouldn't have a new volume or bricks,
	// just a pending op for the block volume itself
	app.db.View(func(tx *bolt.Tx) error {
		bvl, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvl) == 1, "expected len(bvl) == 1, got", len(bvl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 3, "expected len(bl) == 3, got:", len(bl))
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got:", len(vl))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 1, "expected len(po) == 1, got:", len(po))
		return nil
	})

	e = bco.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	e = bco.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	// the block volume is there but the pending op is gone
	app.db.View(func(tx *bolt.Tx) error {
		bvl, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvl) == 1, "expected len(bvl) == 1, got", len(bvl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 3, "expected len(bl) == 3, got:", len(bl))
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got:", len(vl))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 0, "expected len(po) == 0, got:", len(po))
		return nil
	})
}

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

	err = d.SetState(app.db, app.executor, app.Allocator(), api.EntryStateOffline)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	dro := NewDeviceRemoveOperation(d.Info.Id, app.Allocator(), app.db)
	err = dro.Build(app.Allocator())
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
		err = v.Create(app.db, app.executor, app.Allocator())
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

	err = d.SetState(app.db, app.executor, app.Allocator(), api.EntryStateOffline)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	dro := NewDeviceRemoveOperation(d.Info.Id, app.Allocator(), app.db)
	err = dro.Build(app.Allocator())
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
		err = v.Create(app.db, app.executor, app.Allocator())
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

	err = d.SetState(app.db, app.executor, app.Allocator(), api.EntryStateOffline)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	dro := NewDeviceRemoveOperation(d.Info.Id, app.Allocator(), app.db)
	err = dro.Build(app.Allocator())
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
		err = v.Create(app.db, app.executor, app.Allocator())
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
	err = vc.Build(app.Allocator())
	tests.Assert(t, err == nil, "expected e == nil, got", err)
	// we should have one pending operation
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})

	err = d.SetState(app.db, app.executor, app.Allocator(), api.EntryStateOffline)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	dro := NewDeviceRemoveOperation(d.Info.Id, app.Allocator(), app.db)
	err = dro.Build(app.Allocator())
	tests.Assert(t, err == ErrConflict, "expected err == ErrConflict, got:", err)

	// we should have one pending operation (the volume create)
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})
}
