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

	"github.com/heketi/heketi/pkg/glusterfs/api"
)

// ListCompleteVolumes returns a list of volume ID strings for volumes
// that are not pending.
func ListCompleteVolumes(tx *bolt.Tx) ([]string, error) {
	return VolumeList(tx)
}

// ListCompleteBlockVolumes returns a list of block volume ID strings for bricks
// that are not pending.
func ListCompleteBlockVolumes(tx *bolt.Tx) ([]string, error) {
	return BlockVolumeList(tx)
}

// UpdateClusterInfoComplete updates the given ClusterInfoResponse object so
// that it only contains references to complete volumes, etc.
func UpdateClusterInfoComplete(tx *bolt.Tx, ci *api.ClusterInfoResponse) error {
	// currently this is a no-op because we have nothing to filter out
	return nil
}
