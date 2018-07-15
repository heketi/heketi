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
	"errors"

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/executors"
	wdb "github.com/heketi/heketi/pkg/db"
	"github.com/lpabon/godbc"
)

func (v *VolumeEntry) snapshotVolumeRequest(db wdb.RODB,
	snapshotName string, description string) (request *executors.VolumeSnapshotRequest, host string, err error) {
	godbc.Require(db != nil)
	godbc.Require(snapshotName != "")

	vr := &executors.VolumeSnapshotRequest{}
	// Setup volume information in the request
	vr.Volume = v.Info.Name
	vr.Snapshot = snapshotName
	vr.Description = description
	var sshhost string
	err = db.View(func(tx *bolt.Tx) error {
		vol, err := NewVolumeEntryFromId(tx, v.Info.Id)
		if err != nil {
			return err
		}
		cluster, err := NewClusterEntryFromId(tx, vol.Info.Cluster)
		if err != nil {
			return err
		}
		node, err := NewNodeEntryFromId(tx, cluster.Info.Nodes[0])
		if err != nil {
			return err
		}
		sshhost = node.ManageHostName()
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	if sshhost == "" {
		return nil, "", errors.New("failed to find host for snaping shot volume " + v.Info.Name)
	}
	return vr, sshhost, nil
}
