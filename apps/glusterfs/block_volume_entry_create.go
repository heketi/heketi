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
	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/executors"
	"github.com/lpabon/godbc"
)

func (v *BlockVolumeEntry) createBlockVolume(db *bolt.DB,
	executor executors.Executor, blockHostingVolumeId string) error {

	godbc.Require(db != nil)
	godbc.Require(blockHostingVolumeId != "")

	vr, host, err := v.createBlockVolumeRequest(db, executor,
		blockHostingVolumeId)
	if err != nil {
		return err
	}

	blockVolumeInfo, err := executor.BlockVolumeCreate(host, vr)
	if err != nil {
		return err
	}

	v.Info.BlockVolume.Iqn = blockVolumeInfo.Iqn
	v.Info.BlockVolume.Hosts = blockVolumeInfo.BlockHosts
	v.Info.BlockVolume.Lun = 0
	v.Info.BlockVolume.Username = blockVolumeInfo.Username
	v.Info.BlockVolume.Password = blockVolumeInfo.Password

	return nil
}

func (v *BlockVolumeEntry) createBlockVolumeRequest(db *bolt.DB,
	executor executors.Executor,
	blockHostingVolumeId string) (*executors.BlockVolumeRequest, string, error) {
	godbc.Require(db != nil)
	godbc.Require(blockHostingVolumeId != "")

	var blockHostingVolumeName string

	err := db.View(func(tx *bolt.Tx) error {
		logger.Debug("Looking for block hosting volume %v", blockHostingVolumeId)
		bhvol, err := NewVolumeEntryFromId(tx, blockHostingVolumeId)
		if err != nil {
			return err
		}

		v.Info.BlockVolume.Hosts = bhvol.Info.Mount.GlusterFS.Hosts
		v.Info.Hacount = len(v.Info.BlockVolume.Hosts)

		v.Info.Cluster = bhvol.Info.Cluster
		blockHostingVolumeName = bhvol.Info.Name

		return nil
	})
	if err != nil {
		logger.Err(err)
		return nil, "", err
	}

	executorhost, err := GetVerifiedManageHostname(db, executor, v.Info.Cluster)
	if err != nil {
		return nil, "", err
	}

	logger.Debug("Using executor host [%v]", executorhost)

	// Setup volume information in the request
	vr := &executors.BlockVolumeRequest{}
	vr.Name = v.Info.Name
	vr.BlockHosts = v.Info.BlockVolume.Hosts
	vr.GlusterVolumeName = blockHostingVolumeName
	vr.Hacount = v.Info.Hacount
	vr.Size = v.Info.Size
	vr.Auth = v.Info.Auth

	return vr, executorhost, nil
}
