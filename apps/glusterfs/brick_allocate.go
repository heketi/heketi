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
	"github.com/boltdb/bolt"

	"github.com/heketi/heketi/apps/glusterfs/placer"
	wdb "github.com/heketi/heketi/pkg/db"
)

func allocateBricks(
	db wdb.RODB,
	cluster string,
	v *VolumeEntry,
	numBrickSets int,
	brick_size uint64) (*placer.BrickAllocation, error) {

	var r *placer.BrickAllocation
	opts := NewVolumePlacementOpts(v, brick_size, numBrickSets)
	err := db.View(func(tx *bolt.Tx) error {
		var err error
		dsrc := NewClusterDeviceSource(tx, cluster)
		placer := PlacerForVolume(v)
		r, err = placer.PlaceAll(dsrc, opts, nil)
		return err
	})
	return r, err
}
