//
// Copyright (c) 2017 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/executors"
	wdb "github.com/heketi/heketi/pkg/db"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/utils"
	"github.com/lpabon/godbc"
)

type BlockVolumeEntry struct {
	Info    api.BlockVolumeInfo
	Pending PendingItem
}

func BlockVolumeList(tx *bolt.Tx) ([]string, error) {
	list := EntryKeys(tx, BOLTDB_BUCKET_BLOCKVOLUME)
	if list == nil {
		return nil, ErrAccessList
	}
	return list, nil
}

func NewVolumeEntryForBlockHosting(clusters []string) (*VolumeEntry, error) {
	var msg api.VolumeCreateRequest
	msg.Clusters = clusters
	msg.Durability.Type = api.DurabilityReplicate
	msg.Size = BlockHostingVolumeSize
	msg.Durability.Replicate.Replica = 3
	msg.Block = true
	msg.GlusterVolumeOptions = []string{"group gluster-block"}

	vol := NewVolumeEntryFromRequest(&msg)

	if uint64(msg.Size)*GB < vol.Durability.MinVolumeSize() {
		return nil, fmt.Errorf("Requested volume size (%v GB) is "+
			"smaller than the minimum supported volume size (%v)",
			msg.Size, vol.Durability.MinVolumeSize())
	}
	return vol, nil
}

func NewBlockVolumeEntry() *BlockVolumeEntry {
	entry := &BlockVolumeEntry{}

	return entry
}

func NewBlockVolumeEntryFromRequest(req *api.BlockVolumeCreateRequest) *BlockVolumeEntry {
	godbc.Require(req != nil)

	vol := NewBlockVolumeEntry()
	vol.Info.Id = utils.GenUUID()
	vol.Info.Size = req.Size
	vol.Info.Auth = req.Auth

	if req.Name == "" {
		vol.Info.Name = "blockvol_" + vol.Info.Id
	} else {
		vol.Info.Name = req.Name
	}

	// If Clusters is zero, then it will be assigned during volume creation
	vol.Info.Clusters = req.Clusters
	vol.Info.Hacount = req.Hacount

	return vol
}

func NewBlockVolumeEntryFromId(tx *bolt.Tx, id string) (*BlockVolumeEntry, error) {
	godbc.Require(tx != nil)

	entry := NewBlockVolumeEntry()
	err := EntryLoad(tx, entry, id)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func (v *BlockVolumeEntry) BucketName() string {
	return BOLTDB_BUCKET_BLOCKVOLUME
}

func (v *BlockVolumeEntry) Visible() bool {
	return v.Pending.Id == ""
}

func (v *BlockVolumeEntry) Save(tx *bolt.Tx) error {
	godbc.Require(tx != nil)
	godbc.Require(len(v.Info.Id) > 0)

	return EntrySave(tx, v, v.Info.Id)
}

func (v *BlockVolumeEntry) Delete(tx *bolt.Tx) error {
	return EntryDelete(tx, v, v.Info.Id)
}

func (v *BlockVolumeEntry) NewInfoResponse(tx *bolt.Tx) (*api.BlockVolumeInfoResponse, error) {
	godbc.Require(tx != nil)

	info := api.NewBlockVolumeInfoResponse()
	info.Id = v.Info.Id
	info.Cluster = v.Info.Cluster
	info.BlockVolume = v.Info.BlockVolume
	info.Size = v.Info.Size
	info.Name = v.Info.Name
	info.Hacount = v.Info.Hacount
	info.BlockHostingVolume = v.Info.BlockHostingVolume

	return info, nil
}

func (v *BlockVolumeEntry) Marshal() ([]byte, error) {
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)
	err := enc.Encode(*v)

	return buffer.Bytes(), err
}

func (v *BlockVolumeEntry) Unmarshal(buffer []byte) error {
	dec := gob.NewDecoder(bytes.NewReader(buffer))
	err := dec.Decode(v)
	if err != nil {
		return err
	}

	return nil
}

func (v *BlockVolumeEntry) eligibleClustersAndVolumes(db wdb.RODB) (
	possibleClusters []string, volumes []string, e error) {

	if len(v.Info.Clusters) == 0 {
		err := db.View(func(tx *bolt.Tx) error {
			var err error
			possibleClusters, err = ClusterList(tx)
			return err
		})
		if err != nil {
			e = err
			return
		}
	} else {
		possibleClusters = v.Info.Clusters
	}

	// find clusters that support block volumes
	cr := ClusterReq{Block: true}
	possibleClusters, e = eligibleClusters(db, cr, possibleClusters)
	if e != nil {
		return
	}
	if len(possibleClusters) == 0 {
		logger.LogError("No clusters eligible to satisfy create block volume request")
		e = ErrNoSpace
		return
	}
	logger.Debug("Using the following clusters: %+v", possibleClusters)

	var possibleVolumes []string
	for _, clusterId := range possibleClusters {
		err := db.View(func(tx *bolt.Tx) error {
			var err error
			c, err := NewClusterEntryFromId(tx, clusterId)
			for _, vol := range c.Info.Volumes {
				volEntry, err := NewVolumeEntryFromId(tx, vol)
				if err != nil {
					return err
				}
				if volEntry.Info.Block {
					possibleVolumes = append(possibleVolumes, vol)
				}
			}
			return err
		})
		if err != nil {
			e = err
			return
		}
	}

	logger.Debug("Using the following possible block hosting volumes: %+v", possibleVolumes)

	for _, vol := range possibleVolumes {
		err := db.View(func(tx *bolt.Tx) error {
			volEntry, err := NewVolumeEntryFromId(tx, vol)
			if err != nil {
				return err
			}
			if ok, err := canHostBlockVolume(tx, v, volEntry); ok {
				volumes = append(volumes, vol)
			} else if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			e = err
			return
		}
	}
	return
}

func (v *BlockVolumeEntry) cleanupBlockVolumeCreate(db wdb.DB,
	executor executors.Executor) error {

	hvname, err := v.blockHostingVolumeName(db)
	if err != nil {
		return err
	}

	// best effort removal of anything on system
	v.deleteBlockVolumeExec(db, hvname, executor)

	return v.removeComponents(db)
}

func (v *BlockVolumeEntry) Create(db wdb.DB,
	executor executors.Executor,
	allocator Allocator) (e error) {

	return RunOperation(
		NewBlockVolumeCreateOperation(v, db),
		allocator,
		executor)
}

func (v *BlockVolumeEntry) saveCreateBlockVolume(db wdb.DB) error {
	return db.Update(func(tx *bolt.Tx) error {

		err := v.Save(tx)
		if err != nil {
			return err
		}

		cluster, err := NewClusterEntryFromId(tx, v.Info.Cluster)
		if err != nil {
			return err
		}

		cluster.BlockVolumeAdd(v.Info.Id)

		err = cluster.Save(tx)
		if err != nil {
			return err
		}

		volume, err := NewVolumeEntryFromId(tx, v.Info.BlockHostingVolume)
		if err != nil {
			return err
		}

		volume.Info.BlockInfo.FreeSize = volume.Info.BlockInfo.FreeSize - v.Info.Size

		volume.BlockVolumeAdd(v.Info.Id)
		err = volume.Save(tx)
		if err != nil {
			return err
		}

		return err
	})
}

func (v *BlockVolumeEntry) blockHostingVolumeName(db wdb.RODB) (name string, e error) {
	e = db.View(func(tx *bolt.Tx) error {
		volume, err := NewVolumeEntryFromId(tx, v.Info.BlockHostingVolume)
		if err != nil {
			logger.LogError("Unable to load block hosting volume: %v", err)
			return err
		}
		name = volume.Info.Name
		return nil
	})
	return
}

func (v *BlockVolumeEntry) deleteBlockVolumeExec(db wdb.RODB,
	hvname string,
	executor executors.Executor) error {

	executorhost, err := GetVerifiedManageHostname(db, executor, v.Info.Cluster)
	if err != nil {
		return err
	}

	logger.Debug("Using executor host [%v]", executorhost)

	err = executor.BlockVolumeDestroy(executorhost, hvname, v.Info.Name)
	if err != nil {
		logger.LogError("Unable to delete volume: %v", err)
		return err
	}
	return nil
}

func (v *BlockVolumeEntry) removeComponents(db wdb.DB) error {
	return db.Update(func(tx *bolt.Tx) error {
		// Remove volume from cluster
		cluster, err := NewClusterEntryFromId(tx, v.Info.Cluster)
		if err != nil {
			logger.Err(err)
			// Do not return here.. keep going
		}
		cluster.BlockVolumeDelete(v.Info.Id)
		err = cluster.Save(tx)
		if err != nil {
			logger.Err(err)
			// Do not return here.. keep going
		}

		blockHostingVolume, err := NewVolumeEntryFromId(tx, v.Info.BlockHostingVolume)
		if err != nil {
			logger.Err(err)
			// Do not return here.. keep going
		}

		blockHostingVolume.BlockVolumeDelete(v.Info.Id)
		if err != nil {
			logger.Err(err)
			// Do not return here.. keep going
		}
		blockHostingVolume.Info.BlockInfo.FreeSize = blockHostingVolume.Info.BlockInfo.FreeSize + v.Info.Size
		blockHostingVolume.Save(tx)

		if err != nil {
			logger.Err(err)
			// Do not return here.. keep going
		}

		v.Delete(tx)

		return nil
	})
}

func (v *BlockVolumeEntry) Destroy(db wdb.DB, executor executors.Executor) error {
	logger.Info("Destroying volume %v", v.Info.Id)

	return RunOperation(
		NewBlockVolumeDeleteOperation(v, db),
		nil,
		executor)
}

// canHostBlockVolume returns true if the existing volume entry object
// can host the incoming block volume. It returns false (and nil error) if
// the volume is incompatible. It returns false, and an error if the
// database operation fails.
func canHostBlockVolume(tx *bolt.Tx, bv *BlockVolumeEntry, vol *VolumeEntry) (bool, error) {
	if vol.Info.BlockInfo.FreeSize < bv.Info.Size {
		logger.Warning("Free size is less than the block volume requested")
		return false, nil
	}

	for _, blockvol := range vol.Info.BlockInfo.BlockVolumes {
		existingbv, err := NewBlockVolumeEntryFromId(tx, blockvol)
		if err != nil {
			return false, err
		}
		if bv.Info.Name == existingbv.Info.Name {
			logger.Warning("Name %v already in use in file volume %v",
				bv.Info.Name, vol.Info.Name)
			return false, nil
		}
	}

	return true, nil
}
