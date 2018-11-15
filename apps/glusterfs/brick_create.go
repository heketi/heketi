//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"sync"

	"github.com/heketi/heketi/executors"
	wdb "github.com/heketi/heketi/pkg/db"
	"github.com/heketi/heketi/pkg/utils"
)

// ReclaimMap tracks what bricks freed underlying storage when deleted.
// Deleting a brick does not always free space on the LV if snapshots are
// in use. The ReclaimMap values are set to true if the given brick id
// in the key freed underlying storage and false if not.
type ReclaimMap map[string]bool

func CreateBricks(db wdb.RODB, executor executors.Executor, brick_entries []*BrickEntry) error {
	sg := utils.NewStatusGroup()

	// Create a goroutine for each brick
	for _, brick := range brick_entries {
		sg.Add(1)
		go func(b *BrickEntry) {
			defer sg.Done()
			sg.Err(b.Create(db, executor))
		}(brick)
	}

	// Wait here until all goroutines have returned.  If
	// any of errored, it would be cought here
	err := sg.Result()
	if err != nil {
		logger.Err(err)

		// Destroy all bricks and cleanup
		DestroyBricks(db, executor, brick_entries)
	}

	return err
}

func DestroyBricks(db wdb.RODB, executor executors.Executor, brick_entries []*BrickEntry) (ReclaimMap, error) {
	sg := utils.NewStatusGroup()

	// return a map with the deviceId as key, and a bool if the space has been free'd
	reclaimed := map[string]bool{}
	// the mutex is used to prevent "fatal error: concurrent map writes"
	mutex := sync.Mutex{}

	// Create a goroutine for each brick
	for _, brick := range brick_entries {
		sg.Add(1)
		go func(b *BrickEntry, r map[string]bool, m *sync.Mutex) {
			defer sg.Done()
			spaceReclaimed, err := b.Destroy(db, executor)
			if err != nil {
				logger.LogError("error destroying brick %v: %v",
					b.Info.Id, err)
			} else {
				// mark space from device as freed
				m.Lock()
				r[b.Info.DeviceId] = spaceReclaimed
				m.Unlock()
			}

			sg.Err(err)
		}(brick, reclaimed, &mutex)
	}

	// Wait here until all goroutines have returned.  If
	// any of errored, it would be cought here
	err := sg.Result()
	if err != nil {
		logger.Err(err)
	}

	return reclaimed, err
}
