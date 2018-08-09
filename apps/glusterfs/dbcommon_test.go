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

	"github.com/heketi/heketi/pkg/glusterfs/api"

	"github.com/boltdb/bolt"
	"github.com/heketi/tests"
)

func TestFixIncorrectBlockHostingFreeSize(t *testing.T) {
	setup := func(t *testing.T) (*App, string) {
		tmpfile := tests.Tempfile()
		defer os.Remove(tmpfile)

		// Create the app
		app := NewTestApp(tmpfile)

		err := setupSampleDbWithTopology(app,
			1,    // clusters
			3,    // nodes_per_cluster
			2,    // devices_per_node,
			2*TB, // disksize)
		)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		req := &api.BlockVolumeCreateRequest{}
		req.Size = 1024

		vol := NewBlockVolumeEntryFromRequest(req)
		vc := NewBlockVolumeCreateOperation(vol, app.db)

		err = RunOperation(vc, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got", err)

		// we should now have one block volume with one bhv
		var volId string
		app.db.View(func(tx *bolt.Tx) error {
			vl, e := VolumeList(tx)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got", len(vl))
			volId = vl[0]
			bvl, e := BlockVolumeList(tx)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, len(bvl) == 1, "expected len(bvl) == 1, got", len(vl))
			pol, e := PendingOperationList(tx)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))
			return nil
		})

		return app, volId
	}

	t.Run("CorrectBadFreeSize", func(t *testing.T) {
		app, volId := setup(t)
		defer app.Close()

		app.db.Update(func(tx *bolt.Tx) error {
			// first, we intentionally mess up the FreeSize
			vol, e := NewVolumeEntryFromId(tx, volId)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			vol.Info.BlockInfo.FreeSize = 2048
			e = vol.Save(tx)
			tests.Assert(t, e == nil, "expected e == nil, got", e)

			// now run the autocorrection function
			e = fixIncorrectBlockHostingFreeSize(tx)
			tests.Assert(t, e == nil, "expected e == nil, got", e)

			// verify it was corrected
			vol, e = NewVolumeEntryFromId(tx, volId)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, vol.Info.BlockInfo.FreeSize == 54,
				"expected vol.Info.BlockInfo.FreeSize == 54, got:",
				vol.Info.BlockInfo.FreeSize)
			return nil
		})
	})
	t.Run("AlreadyOk", func(t *testing.T) {
		app, volId := setup(t)
		defer app.Close()

		app.db.Update(func(tx *bolt.Tx) error {
			// we run the autocorrect func on entries that are already ok
			e := fixIncorrectBlockHostingFreeSize(tx)
			tests.Assert(t, e == nil, "expected e == nil, got", e)

			// verify it is ok
			vol, e := NewVolumeEntryFromId(tx, volId)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, vol.Info.BlockInfo.FreeSize == 54,
				"expected vol.Info.BlockInfo.FreeSize == 54, got:",
				vol.Info.BlockInfo.FreeSize)
			return nil
		})
	})
	t.Run("SkipTooLow", func(t *testing.T) {
		app, volId := setup(t)
		defer app.Close()

		app.db.Update(func(tx *bolt.Tx) error {
			// first, we intentionally mess up the FreeSize
			vol, e := NewVolumeEntryFromId(tx, volId)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			vol.Info.BlockInfo.FreeSize = 2048
			e = vol.Save(tx)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			// also change the block volume size to something silly
			bvid := vol.Info.BlockInfo.BlockVolumes[0]
			bv, e := NewBlockVolumeEntryFromId(tx, bvid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			bv.Info.Size = 10001
			e = bv.Save(tx)
			tests.Assert(t, e == nil, "expected e == nil, got", e)

			// now run the autocorrection function
			e = fixIncorrectBlockHostingFreeSize(tx)
			tests.Assert(t, e == nil, "expected e == nil, got", e)

			// verify it was not changed
			vol, e = NewVolumeEntryFromId(tx, volId)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, vol.Info.BlockInfo.FreeSize == 2048,
				"expected vol.Info.BlockInfo.FreeSize == 2048, got:",
				vol.Info.BlockInfo.FreeSize)
			return nil
		})
	})
	t.Run("SkipTooHigh", func(t *testing.T) {
		app, volId := setup(t)
		defer app.Close()

		app.db.Update(func(tx *bolt.Tx) error {
			// first, we intentionally mess up the FreeSize
			vol, e := NewVolumeEntryFromId(tx, volId)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			vol.Info.BlockInfo.FreeSize = 2048
			// also change the reserved size to some nonsense
			vol.Info.BlockInfo.ReservedSize = -5000
			e = vol.Save(tx)
			tests.Assert(t, e == nil, "expected e == nil, got", e)

			// now run the autocorrection function
			e = fixIncorrectBlockHostingFreeSize(tx)
			tests.Assert(t, e == nil, "expected e == nil, got", e)

			// verify it was not changed
			vol, e = NewVolumeEntryFromId(tx, volId)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, vol.Info.BlockInfo.FreeSize == 2048,
				"expected vol.Info.BlockInfo.FreeSize == 2048, got:",
				vol.Info.BlockInfo.FreeSize)
			return nil
		})
	})
}
