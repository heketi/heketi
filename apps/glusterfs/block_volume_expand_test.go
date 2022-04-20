//
// Copyright (c) 2020 The heketi Authors
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
	"testing"

	"github.com/heketi/heketi/v10/pkg/glusterfs/api"

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/v10/executors"
	"github.com/heketi/tests"
)

func TestBlockVolumeExpandOperationFinalize(t *testing.T) {
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

	bvolReq := &api.BlockVolumeCreateRequest{}
	bvolReq.Size = 100 // Initial size

	bvol := NewBlockVolumeEntryFromRequest(bvolReq)
	bvc := NewBlockVolumeCreateOperation(bvol, app.db)

	// verify,
	// there are no volumes, bricks or pending operations
	app.db.View(func(tx *bolt.Tx) error {
		vols, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vols) == 0, "expected len(vols) == 0, got", len(vols))
		bricks, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bricks) == 0, "expected len(bricks) == 0, got", len(bricks))
		pendingOps, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pendingOps) == 0, "expected len(pendingOps) == 0, got", len(pendingOps))
		return nil
	})

	e := bvc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)
	e = bvc.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)
	e = bvc.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify,
	// the block volume, block hosting volume and bricks exist
	// there are no pending ops
	// the size, usable size of block volume and FreeSize on block hosting volume
	app.db.View(func(tx *bolt.Tx) error {
		bvols, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvols) == 1, "expected len(bvols) == 1, got", len(bvols))
		vols, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vols) == 1, "expected len(vols) == 1, got", len(vols))
		bricks, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bricks) == 3, "expected len(bricks) == 3, got", len(bricks))
		pendingOps, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pendingOps) == 0, "expected len(pendingOps) == 0, got", len(pendingOps))
		for _, id := range vols {
			v, e := NewVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			// free size = block hosting volume size - 2% reserved size - block volume size
			expectedVolumeFreeSize := v.Info.Size - (v.Info.Size * 2 / 100) - 100
			tests.Assert(t, v.Info.BlockInfo.FreeSize == expectedVolumeFreeSize,
				"expected free size == ", expectedVolumeFreeSize, " got:", v.Info.BlockInfo.FreeSize)
		}
		for _, id := range bvols {
			bv, e := NewBlockVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, bv.Info.Size == 100, "expected block volume size == 100, got:", bv.Info.Size)
			tests.Assert(t, bv.Info.UsableSize == 100, "expected block volume usable size == 100, got:", bv.Info.UsableSize)
		}
		return nil
	})

	bve := NewBlockVolumeExpandOperation(bvol.Info.Id, app.db, 150) // New size

	e = bve.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify,
	// we have a pending op for the expand
	// as the Build is done, block hosting volume size should have got modified
	app.db.View(func(tx *bolt.Tx) error {
		bvols, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvols) == 1, "expected len(bvols) == 1, got", len(bvols))
		vols, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vols) == 1, "expected len(vols) == 1, got", len(vols))
		bricks, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bricks) == 3, "expected len(bricks) == 3, got", len(bricks))
		pendingOps, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pendingOps) == 1, "expected len(pendingOps) == 1, got", len(pendingOps))
		for _, id := range vols {
			v, e := NewVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			// free size = block hosting volume size - 2% reserved size - block volume size
			expectedVolumeFreeSize := v.Info.Size - (v.Info.Size * 2 / 100) - 150
			tests.Assert(t, v.Info.BlockInfo.FreeSize == expectedVolumeFreeSize,
				"expected free size == ", expectedVolumeFreeSize, " got:", v.Info.BlockInfo.FreeSize)
		}
		for _, id := range bvols {
			bv, e := NewBlockVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, bv.Info.Size == 100, "expected block volume size == 100, got:", bv.Info.Size)
			tests.Assert(t, bv.Info.UsableSize == 100, "expected block volume usable size == 100, got:", bv.Info.UsableSize)
		}
		return nil
	})

	e = bve.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	e = bve.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify,
	// the pending op is gone
	// new block volume size and usable size is effective
	app.db.View(func(tx *bolt.Tx) error {
		bvols, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvols) == 1, "expected len(bvols) == 0, got", len(bvols))
		vols, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vols) == 1, "expected len(vols) == 1, got", len(vols))
		bricks, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bricks) == 3, "expected len(bricks) == 3, got", len(bricks))
		pendingOps, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pendingOps) == 0, "expected len(pendingOps) == 0, got", len(pendingOps))
		for _, id := range vols {
			v, e := NewVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			// free size = block hosting volume size - 2% reserved size - block volume size
			expectedVolumeFreeSize := v.Info.Size - (v.Info.Size * 2 / 100) - 150
			tests.Assert(t, v.Info.BlockInfo.FreeSize == expectedVolumeFreeSize,
				"expected free size == ", expectedVolumeFreeSize, " got:", v.Info.BlockInfo.FreeSize)
		}
		for _, id := range bvols {
			bv, e := NewBlockVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, bv.Info.Size == 150, "expected block volume size == 150, got:", bv.Info.Size)
			tests.Assert(t, bv.Info.UsableSize == 150, "expected block volume usable size == 150, got:", bv.Info.UsableSize)
		}
		return nil
	})
}

func TestBlockVolumeExpandOperationBuildFailed(t *testing.T) {
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

	bvolReq := &api.BlockVolumeCreateRequest{}
	bvolReq.Size = 100

	bvol := NewBlockVolumeEntryFromRequest(bvolReq)
	bvc := NewBlockVolumeCreateOperation(bvol, app.db)

	// verify,
	// there are no volumes, bricks or pending operations
	app.db.View(func(tx *bolt.Tx) error {
		vols, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vols) == 0, "expected len(vols) == 0, got", len(vols))
		bricks, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bricks) == 0, "expected len(bricks) == 0, got", len(bricks))
		pendingOps, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pendingOps) == 0, "expected len(pendingOps) == 0, got", len(pendingOps))
		return nil
	})

	e := bvc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)
	e = bvc.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)
	e = bvc.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify,
	// the block volume, block hosting volume and bricks exist
	// there are no pending ops
	// the size of block volume and FreeSize on block hosting volume
	app.db.View(func(tx *bolt.Tx) error {
		bvols, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvols) == 1, "expected len(bvols) == 1, got", len(bvols))
		vols, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vols) == 1, "expected len(vols) == 1, got", len(vols))
		bricks, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bricks) == 3, "expected len(bricks) == 3, got", len(bricks))
		pendingOps, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pendingOps) == 0, "expected len(pendingOps) == 0, got", len(pendingOps))
		for _, id := range vols {
			v, e := NewVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			// free size = block hosting volume size - 2% reserved size - block volume size
			expectedVolumeFreeSize := v.Info.Size - (v.Info.Size * 2 / 100) - 100
			tests.Assert(t, v.Info.BlockInfo.FreeSize == expectedVolumeFreeSize,
				"expected free size == ", expectedVolumeFreeSize, " got:", v.Info.BlockInfo.FreeSize)
		}
		for _, id := range bvols {
			bv, e := NewBlockVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, bv.Info.Size == 100, "expected block volume size == 100, got:", bv.Info.Size)
		}
		return nil
	})

	// request a size larger than the BlockHostingVolumeSize
	bve := NewBlockVolumeExpandOperation(bvol.Info.Id, app.db, 1100)

	e = bve.Build()
	tests.Assert(t, e != nil, "expected e != nil, got", e)
	tests.Assert(t, e == ErrNoSpace, "expected e == ErrNoSpace', got", e)

	// request a size same as current block volume size
	bve = NewBlockVolumeExpandOperation(bvol.Info.Id, app.db, 100)

	e = bve.Build()
	err_str := "Requested new-size 100 is same as current block volume size 100, nothing to be done."
	tests.Assert(t, e != nil, "expected e != nil, got", e)
	tests.Assert(t, e.Error() == err_str,
		"expected '", err_str, "', got '", e.Error(), "'")

	// try shrink, request a size less than current block volume size
	bve = NewBlockVolumeExpandOperation(bvol.Info.Id, app.db, 50)

	e = bve.Build()
	err_str = "Requested new-size 50 is less than current block volume size 100, shrinking is not allowed."
	tests.Assert(t, e != nil, "expected e != nil, got", e)
	tests.Assert(t, e.Error() == err_str,
		"expected '", err_str, "', got '", e.Error(), "'")
}

func TestBlockVolumeExpandOperationRollbackGbCliFailedCompletely(t *testing.T) {
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

	bvolReq := &api.BlockVolumeCreateRequest{}
	bvolReq.Size = 100 // Initial size

	bvol := NewBlockVolumeEntryFromRequest(bvolReq)
	bvc := NewBlockVolumeCreateOperation(bvol, app.db)

	// verify,
	// there are no volumes, bricks or pending operations
	app.db.View(func(tx *bolt.Tx) error {
		vols, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vols) == 0, "expected len(vols) == 0, got", len(vols))
		bricks, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bricks) == 0, "expected len(bricks) == 0, got", len(bricks))
		pendingOps, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pendingOps) == 0, "expected len(pendingOps) == 0, got", len(pendingOps))
		return nil
	})

	e := bvc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)
	e = bvc.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)
	e = bvc.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify,
	// the block volume, block hosting volume and bricks exist
	// there are no pending ops
	// the size, usable size of block volume and FreeSize on block hosting volume
	app.db.View(func(tx *bolt.Tx) error {
		bvols, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvols) == 1, "expected len(bvols) == 1, got", len(bvols))
		vols, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vols) == 1, "expected len(vols) == 1, got", len(vols))
		bricks, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bricks) == 3, "expected len(bricks) == 3, got", len(bricks))
		pendingOps, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pendingOps) == 0, "expected len(pendingOps) == 0, got", len(pendingOps))
		for _, id := range vols {
			v, e := NewVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			// free size = block hosting volume size - 2% reserved size - block volume size
			expectedVolumeFreeSize := v.Info.Size - (v.Info.Size * 2 / 100) - 100
			tests.Assert(t, v.Info.BlockInfo.FreeSize == expectedVolumeFreeSize,
				"expected free size == ", expectedVolumeFreeSize, " got:", v.Info.BlockInfo.FreeSize)
		}
		for _, id := range bvols {
			bv, e := NewBlockVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, bv.Info.Size == 100, "expected block volume size == 100, got:", bv.Info.Size)
			tests.Assert(t, bv.Info.UsableSize == 100, "expected block volume usable size == 100, got:", bv.Info.UsableSize)
		}
		return nil
	})

	bve := NewBlockVolumeExpandOperation(bvol.Info.Id, app.db, 150) // New size

	e = bve.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify,
	// we have a pending op for the expand
	// as the Build is done, block hosting volume size should have got modified
	var volume *VolumeEntry
	var blockvolume *BlockVolumeEntry
	app.db.View(func(tx *bolt.Tx) error {
		bvols, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvols) == 1, "expected len(bvols) == 1, got", len(bvols))
		vols, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vols) == 1, "expected len(vols) == 1, got", len(vols))
		bricks, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bricks) == 3, "expected len(bricks) == 3, got", len(bricks))
		pendingOps, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pendingOps) == 1, "expected len(pendingOps) == 1, got", len(pendingOps))
		for _, id := range vols {
			volume, e = NewVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			// free size = block hosting volume size - 2% reserved size - block volume size
			expectedVolumeFreeSize := volume.Info.Size - (volume.Info.Size * 2 / 100) - 150
			tests.Assert(t, volume.Info.BlockInfo.FreeSize == expectedVolumeFreeSize,
				"expected free size == ", expectedVolumeFreeSize, " got:", volume.Info.BlockInfo.FreeSize)
		}
		for _, id := range bvols {
			blockvolume, e = NewBlockVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, blockvolume.Info.Size == 100, "expected block volume size == 100, got:", blockvolume.Info.Size)
			tests.Assert(t, blockvolume.Info.UsableSize == 100, "expected block volume usable size == 100, got:", blockvolume.Info.UsableSize)
		}
		return nil
	})

	// pretend Exec failed
	app.xo.MockBlockVolumeExpand = func(host string, blockHostingVolumeName string,
		blockVolumeName string, newSize int) error {
		return fmt.Errorf("Failed to expand block volume")
	}

	e = bve.Exec(app.executor)
	tests.Assert(t, e != nil, "expected e != nil, got", e)

	// pretend that we have called gluster-block cli and got some info
	// assumption, gluster-block failed way before expanding the backend file
	app.xo.MockBlockVolumeInfo = func(host string, blockhostingvolume string,
		blockVolumeName string) (*executors.BlockVolumeInfo, error) {
		var blockVolumeInfo executors.BlockVolumeInfo

		blockVolumeInfo.BlockHosts = []string{"FakeHost1", "FakeHost2", "FakeHost3"}
		blockVolumeInfo.GlusterNode = "Fake GlusterNode"
		blockVolumeInfo.GlusterVolumeName = volume.Info.Name
		blockVolumeInfo.Hacount = blockvolume.Info.Hacount
		blockVolumeInfo.Iqn = "Fake Iqn"
		blockVolumeInfo.Name = blockvolume.Info.Name
		blockVolumeInfo.Size = 100       // original/old size
		blockVolumeInfo.UsableSize = 100 // original/old size
		blockVolumeInfo.Username = "Fake Username"
		blockVolumeInfo.Password = "Fake Password"

		return &blockVolumeInfo, nil
	}

	e = bve.Rollback(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify,
	// there are no pending ops
	// block hosting volume free size is reset
	// block volume size, usable size is same as Initial
	app.db.View(func(tx *bolt.Tx) error {
		bvols, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvols) == 1, "expected len(bvols) == 0, got", len(bvols))
		vols, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vols) == 1, "expected len(vols) == 1, got", len(vols))
		bricks, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bricks) == 3, "expected len(bricks) == 3, got", len(bricks))
		pendingOps, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pendingOps) == 0, "expected len(pendingOps) == 0, got", len(pendingOps))
		for _, id := range vols {
			v, e := NewVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			// free size = block hosting volume size - 2% reserved size - block volume size
			expectedVolumeFreeSize := v.Info.Size - (v.Info.Size * 2 / 100) - 100
			tests.Assert(t, v.Info.BlockInfo.FreeSize == expectedVolumeFreeSize,
				"expected free size == ", expectedVolumeFreeSize, " got:", v.Info.BlockInfo.FreeSize)
		}
		for _, id := range bvols {
			bv, e := NewBlockVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, bv.Info.Size == 100, "expected block volume size == 100, got:", bv.Info.Size)
			tests.Assert(t, bv.Info.UsableSize == 100, "expected block volume usable size == 100, got:", bv.Info.UsableSize)
		}
		return nil
	})
}

func TestBlockVolumeExpandOperationRollbackGbCliFailedPartially(t *testing.T) {
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

	bvolReq := &api.BlockVolumeCreateRequest{}
	bvolReq.Size = 100 // Initial size

	bvol := NewBlockVolumeEntryFromRequest(bvolReq)
	bvc := NewBlockVolumeCreateOperation(bvol, app.db)

	// verify,
	// there are no volumes, bricks or pending operations
	app.db.View(func(tx *bolt.Tx) error {
		vols, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vols) == 0, "expected len(vols) == 0, got", len(vols))
		bricks, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bricks) == 0, "expected len(bricks) == 0, got", len(bricks))
		pendingOps, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pendingOps) == 0, "expected len(pendingOps) == 0, got", len(pendingOps))
		return nil
	})

	e := bvc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)
	e = bvc.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)
	e = bvc.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify,
	// the block volume, block hosting volume and bricks exist
	// there are no pending ops
	// the size, usable size of block volume and FreeSize on block hosting volume
	app.db.View(func(tx *bolt.Tx) error {
		bvols, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvols) == 1, "expected len(bvols) == 1, got", len(bvols))
		vols, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vols) == 1, "expected len(vols) == 1, got", len(vols))
		bricks, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bricks) == 3, "expected len(bricks) == 3, got", len(bricks))
		pendingOps, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pendingOps) == 0, "expected len(pendingOps) == 0, got", len(pendingOps))
		for _, id := range vols {
			v, e := NewVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			// free size = block hosting volume size - 2% reserved size - block volume size
			expectedVolumeFreeSize := v.Info.Size - (v.Info.Size * 2 / 100) - 100
			tests.Assert(t, v.Info.BlockInfo.FreeSize == expectedVolumeFreeSize,
				"expected free size == ", expectedVolumeFreeSize, " got:", v.Info.BlockInfo.FreeSize)
		}
		for _, id := range bvols {
			bv, e := NewBlockVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, bv.Info.Size == 100, "expected block volume size == 100, got:", bv.Info.Size)
			tests.Assert(t, bv.Info.UsableSize == 100, "expected block volume usable size == 100, got:", bv.Info.UsableSize)
		}
		return nil
	})

	bve := NewBlockVolumeExpandOperation(bvol.Info.Id, app.db, 150) // New size

	e = bve.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify,
	// we have a pending op for the expand
	// as the Build is done, block hosting volume size should have got modified
	var volume *VolumeEntry
	var blockvolume *BlockVolumeEntry
	app.db.View(func(tx *bolt.Tx) error {
		bvols, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvols) == 1, "expected len(bvols) == 1, got", len(bvols))
		vols, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vols) == 1, "expected len(vols) == 1, got", len(vols))
		bricks, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bricks) == 3, "expected len(bricks) == 3, got", len(bricks))
		pendingOps, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pendingOps) == 1, "expected len(pendingOps) == 1, got", len(pendingOps))
		for _, id := range vols {
			volume, e = NewVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			// free size = block hosting volume size - 2% reserved size - block volume size
			expectedVolumeFreeSize := volume.Info.Size - (volume.Info.Size * 2 / 100) - 150
			tests.Assert(t, volume.Info.BlockInfo.FreeSize == expectedVolumeFreeSize,
				"expected free size == ", expectedVolumeFreeSize, " got:", volume.Info.BlockInfo.FreeSize)
		}
		for _, id := range bvols {
			blockvolume, e = NewBlockVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, blockvolume.Info.Size == 100, "expected block volume size == 100, got:", blockvolume.Info.Size)
			tests.Assert(t, blockvolume.Info.UsableSize == 100, "expected block volume usable size == 100, got:", blockvolume.Info.UsableSize)
		}
		return nil
	})

	// pretend Exec failed
	app.xo.MockBlockVolumeExpand = func(host string, blockHostingVolumeName string,
		blockVolumeName string, newSize int) error {
		return fmt.Errorf("Failed to expand block volume")
	}

	e = bve.Exec(app.executor)
	tests.Assert(t, e != nil, "expected e != nil, got", e)

	// pretend that we have called gluster-block cli and got some info
	// assumption, gluster-block failed after expanding the backend file
	app.xo.MockBlockVolumeInfo = func(host string, blockhostingvolume string,
		blockVolumeName string) (*executors.BlockVolumeInfo, error) {
		var blockVolumeInfo executors.BlockVolumeInfo

		blockVolumeInfo.BlockHosts = []string{"FakeHost1", "FakeHost2", "FakeHost3"}
		blockVolumeInfo.GlusterNode = "Fake GlusterNode"
		blockVolumeInfo.GlusterVolumeName = volume.Info.Name
		blockVolumeInfo.Hacount = blockvolume.Info.Hacount
		blockVolumeInfo.Iqn = "Fake Iqn"
		blockVolumeInfo.Name = blockvolume.Info.Name
		blockVolumeInfo.Size = 150       // new size
		blockVolumeInfo.UsableSize = 100 // old size, as there is a partial failure
		blockVolumeInfo.Username = "Fake Username"
		blockVolumeInfo.Password = "Fake Password"

		return &blockVolumeInfo, nil
	}

	e = bve.Rollback(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify,
	// there are no pending ops
	// block hosting volume free size is reduced
	// block volume size is equal to new size
	app.db.View(func(tx *bolt.Tx) error {
		bvols, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvols) == 1, "expected len(bvols) == 0, got", len(bvols))
		vols, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vols) == 1, "expected len(vols) == 1, got", len(vols))
		bricks, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bricks) == 3, "expected len(bricks) == 3, got", len(bricks))
		pendingOps, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pendingOps) == 0, "expected len(pendingOps) == 0, got", len(pendingOps))
		for _, id := range vols {
			v, e := NewVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			// free size = block hosting volume size - 2% reserved size - block volume size
			expectedVolumeFreeSize := v.Info.Size - (v.Info.Size * 2 / 100) - 150
			tests.Assert(t, v.Info.BlockInfo.FreeSize == expectedVolumeFreeSize,
				"expected free size == ", expectedVolumeFreeSize, " got:", v.Info.BlockInfo.FreeSize)
		}
		for _, id := range bvols {
			bv, e := NewBlockVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, bv.Info.Size == 150, "expected block volume size == 150, got:", bv.Info.Size)
			tests.Assert(t, bv.Info.UsableSize == 100, "expected block volume usable size == 100, got:", bv.Info.UsableSize)
		}
		return nil
	})
}

func TestBlockVolumeExpandOperationBuildParallel(t *testing.T) {
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

	// block volume request 1
	bvolReq1 := &api.BlockVolumeCreateRequest{}
	bvolReq1.Size = 100 // Initial size

	bvol1 := NewBlockVolumeEntryFromRequest(bvolReq1)
	bvc1 := NewBlockVolumeCreateOperation(bvol1, app.db)

	// block volume request 2
	bvolReq2 := &api.BlockVolumeCreateRequest{}
	bvolReq2.Size = 100 // Initial size

	bvol2 := NewBlockVolumeEntryFromRequest(bvolReq2)
	bvc2 := NewBlockVolumeCreateOperation(bvol2, app.db)

	// verify,
	// there are no volumes, bricks or pending operations
	app.db.View(func(tx *bolt.Tx) error {
		vols, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vols) == 0, "expected len(vols) == 0, got", len(vols))
		bricks, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bricks) == 0, "expected len(bricks) == 0, got", len(bricks))
		pendingOps, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pendingOps) == 0, "expected len(pendingOps) == 0, got", len(pendingOps))
		return nil
	})

	e := bvc1.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)
	e = bvc1.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)
	e = bvc1.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	e = bvc2.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)
	e = bvc2.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)
	e = bvc2.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify,
	// the two block volumes, block hosting volume and bricks exist
	// there are no pending ops
	// the size, usable size of two block volumes and FreeSize on block hosting volume
	app.db.View(func(tx *bolt.Tx) error {
		bvols, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvols) == 2, "expected len(bvols) == 2, got", len(bvols))
		vols, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vols) == 1, "expected len(vols) == 1, got", len(vols))
		bricks, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bricks) == 3, "expected len(bricks) == 3, got", len(bricks))
		pendingOps, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pendingOps) == 0, "expected len(pendingOps) == 0, got", len(pendingOps))
		for _, id := range vols {
			v, e := NewVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			// free size = block hosting volume size - 2% reserved size - block volume1 size - block volume2 size
			expectedVolumeFreeSize := v.Info.Size - (v.Info.Size * 2 / 100) - 100 - 100
			tests.Assert(t, v.Info.BlockInfo.FreeSize == expectedVolumeFreeSize,
				"expected free size == ", expectedVolumeFreeSize, " got:", v.Info.BlockInfo.FreeSize)
		}
		for _, id := range bvols {
			bv, e := NewBlockVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, bv.Info.Size == 100, "expected block volume size == 100, got:", bv.Info.Size)
			tests.Assert(t, bv.Info.UsableSize == 100, "expected block volume usable size == 100, got:", bv.Info.UsableSize)
		}
		return nil
	})

	bve1 := NewBlockVolumeExpandOperation(bvol1.Info.Id, app.db, 978) // newSize = oldSize + remaining FreeSize on BHV
	bve2 := NewBlockVolumeExpandOperation(bvol2.Info.Id, app.db, 978) // newSize = oldSize + remaining FreeSize on BHV

	e = bve1.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	e = bve2.Build()
	tests.Assert(t, e == ErrNoSpace, "expected e == ErrNoSpace', got", e)

	// verify,
	// we have a pending op for the block volume 1 expand
	// as the Build for block volume 1 is done, block hosting volume FreeSize should be 0
	// block volumes size, usable size is same as Initial
	var volume *VolumeEntry
	var blockvolume1 *BlockVolumeEntry
	var blockvolume2 *BlockVolumeEntry
	app.db.View(func(tx *bolt.Tx) error {
		bvols, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvols) == 2, "expected len(bvols) == 2, got", len(bvols))
		vols, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vols) == 1, "expected len(vols) == 1, got", len(vols))
		bricks, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bricks) == 3, "expected len(bricks) == 3, got", len(bricks))
		pendingOps, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pendingOps) == 1, "expected len(pendingOps) == 1, got", len(pendingOps))
		for _, id := range vols {
			volume, e = NewVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, volume.Info.BlockInfo.FreeSize == 0,
				"expected free size == 0", " got:", volume.Info.BlockInfo.FreeSize)
		}
		blockvolume1, e = NewBlockVolumeEntryFromId(tx, bve1.bvolId)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, blockvolume1.Info.Size == 100, "expected block volume size == 100, got:", blockvolume1.Info.Size)
		tests.Assert(t, blockvolume1.Info.UsableSize == 100, "expected block volume usable size == 100, got:", blockvolume1.Info.UsableSize)

		blockvolume2, e = NewBlockVolumeEntryFromId(tx, bve2.bvolId)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, blockvolume2.Info.Size == 100, "expected block volume size == 100, got:", blockvolume2.Info.Size)
		tests.Assert(t, blockvolume2.Info.UsableSize == 100, "expected block volume usable size == 100, got:", blockvolume2.Info.UsableSize)
		return nil
	})

	// pretend that we have called gluster-block cli and got some info
	// assumption, gluster-block failed way before expanding the backend file
	app.xo.MockBlockVolumeInfo = func(host string, blockhostingvolume string,
		blockVolumeName string) (*executors.BlockVolumeInfo, error) {
		var blockVolumeInfo executors.BlockVolumeInfo

		blockVolumeInfo.BlockHosts = []string{"FakeHost1", "FakeHost2", "FakeHost3"}
		blockVolumeInfo.GlusterNode = "Fake GlusterNode"
		blockVolumeInfo.GlusterVolumeName = volume.Info.Name
		blockVolumeInfo.Hacount = blockvolume1.Info.Hacount
		blockVolumeInfo.Iqn = "Fake Iqn"
		blockVolumeInfo.Name = blockvolume1.Info.Name
		blockVolumeInfo.Size = 100       // original/old size
		blockVolumeInfo.UsableSize = 100 // old size
		blockVolumeInfo.Username = "Fake Username"
		blockVolumeInfo.Password = "Fake Password"

		return &blockVolumeInfo, nil
	}

	e = bve1.Rollback(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify,
	// there are no pending ops
	// block hosting volume free size is reset
	// block volumes size, usable size is same as Initial
	app.db.View(func(tx *bolt.Tx) error {
		bvols, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvols) == 2, "expected len(bvols) == 0, got", len(bvols))
		vols, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vols) == 1, "expected len(vols) == 1, got", len(vols))
		bricks, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bricks) == 3, "expected len(bricks) == 3, got", len(bricks))
		pendingOps, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pendingOps) == 0, "expected len(pendingOps) == 0, got", len(pendingOps))
		for _, id := range vols {
			v, e := NewVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			// free size = block hosting volume size - 2% reserved size - block volume1 size - block volume2 size
			expectedVolumeFreeSize := v.Info.Size - (v.Info.Size * 2 / 100) - 100 - 100
			tests.Assert(t, v.Info.BlockInfo.FreeSize == expectedVolumeFreeSize,
				"expected free size == ", expectedVolumeFreeSize, " got:", v.Info.BlockInfo.FreeSize)
		}
		for _, id := range bvols {
			bv, e := NewBlockVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, bv.Info.Size == 100, "expected block volume size == 100, got:", bv.Info.Size)
			tests.Assert(t, bv.Info.UsableSize == 100, "expected block volume usable size == 100, got:", bv.Info.UsableSize)
		}
		return nil
	})

	// retry: now that block volume expand 1 request is Rollback'ed,
	//        the free size can be claimed by block volume expand 2 request
	e = bve2.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify,
	// we have a pending op for the block volume 2 expand
	// as the Build for block volume 2 is done, block hosting volume FreeSize should be 0
	// block volumes size, usable size is same as Initial
	app.db.View(func(tx *bolt.Tx) error {
		bvols, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvols) == 2, "expected len(bvols) == 2, got", len(bvols))
		vols, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vols) == 1, "expected len(vols) == 1, got", len(vols))
		bricks, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bricks) == 3, "expected len(bricks) == 3, got", len(bricks))
		pendingOps, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pendingOps) == 1, "expected len(pendingOps) == 1, got", len(pendingOps))
		for _, id := range vols {
			v, e := NewVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, v.Info.BlockInfo.FreeSize == 0,
				"expected free size == 0", " got:", v.Info.BlockInfo.FreeSize)
		}
		for _, id := range bvols {
			bv, e := NewBlockVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, bv.Info.Size == 100, "expected block volume size == 100, got:", bv.Info.Size)
			tests.Assert(t, bv.Info.UsableSize == 100, "expected block volume usable size == 100, got:", bv.Info.UsableSize)
		}
		return nil
	})

	e = bve2.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	e = bve2.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify,
	// the pending op is gone
	// block volume 2 new size, usable size is effective
	app.db.View(func(tx *bolt.Tx) error {
		bvols, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvols) == 2, "expected len(bvols) == 0, got", len(bvols))
		vols, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vols) == 1, "expected len(vols) == 1, got", len(vols))
		bricks, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bricks) == 3, "expected len(bricks) == 3, got", len(bricks))
		pendingOps, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pendingOps) == 0, "expected len(pendingOps) == 0, got", len(pendingOps))
		for _, id := range vols {
			v, e := NewVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, v.Info.BlockInfo.FreeSize == 0,
				"expected free size == 0", " got:", v.Info.BlockInfo.FreeSize)
		}
		bv1, e := NewBlockVolumeEntryFromId(tx, bve1.bvolId)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, bv1.Info.Size == 100, "expected block volume size == 100, got:", bv1.Info.Size)
		tests.Assert(t, bv1.Info.UsableSize == 100, "expected block volume usable size == 100, got:", bv1.Info.UsableSize)

		bv2, e := NewBlockVolumeEntryFromId(tx, bve2.bvolId)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, bv2.Info.Size == 978, "expected block volume size == 978, got:", bv2.Info.Size)
		tests.Assert(t, bv2.Info.UsableSize == 978, "expected block volume usable size == 978, got:", bv2.Info.UsableSize)
		return nil
	})
}
