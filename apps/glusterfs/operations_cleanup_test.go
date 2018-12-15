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

	// currently no operations are actually cleanable.
	// we must assert that the operations count is the same as before
	app.db.View(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})
}
