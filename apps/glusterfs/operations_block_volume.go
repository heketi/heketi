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
	"fmt"

	"github.com/heketi/heketi/executors"
	wdb "github.com/heketi/heketi/pkg/db"

	"github.com/boltdb/bolt"
)

// BlockVolumeCreateOperation  implements the operation functions used to
// create a new volume.
type BlockVolumeCreateOperation struct {
	OperationManager
	noRetriesOperation
	bvol *BlockVolumeEntry
}

// NewBlockVolumeCreateOperation  returns a new BlockVolumeCreateOperation  populated
// with the given volume entry and db connection and allocates a new
// pending operation entry.
func NewBlockVolumeCreateOperation(
	bv *BlockVolumeEntry, db wdb.DB) *BlockVolumeCreateOperation {

	return &BlockVolumeCreateOperation{
		OperationManager: OperationManager{
			db: db,
			op: NewPendingOperationEntry(NEW_ID),
		},
		bvol: bv,
	}
}

func (bvc *BlockVolumeCreateOperation) Label() string {
	return "Create Block Volume"
}

func (bvc *BlockVolumeCreateOperation) ResourceUrl() string {
	return fmt.Sprintf("/blockvolumes/%v", bvc.bvol.Info.Id)
}

// Build allocates and saves new volume and brick entries (tagged as pending)
// in the db.
func (bvc *BlockVolumeCreateOperation) Build() error {
	return bvc.db.Update(func(tx *bolt.Tx) error {
		txdb := wdb.WrapTx(tx)
		clusters, volumes, err := bvc.bvol.eligibleClustersAndVolumes(txdb)
		if err != nil {
			return err
		}
		reducedSize := ReduceRawSize(BlockHostingVolumeSize)
		if len(volumes) > 0 {
			bvc.bvol.Info.BlockHostingVolume = volumes[0].Info.Id
			bvc.bvol.Info.Cluster = volumes[0].Info.Cluster
		} else if bvc.bvol.Info.Size > reducedSize {
			return fmt.Errorf("The size configured for "+
				"automatic creation of block hosting volumes "+
				"(%v) is too small to host the requested "+
				"block volume of size %v. The available "+
				"size on this block hosting volume, minus overhead, is %v. "+
				"Please create a "+
				"sufficiently large block hosting volume "+
				"manually.",
				BlockHostingVolumeSize, bvc.bvol.Info.Size, reducedSize)
		} else {
			if found, err := hasPendingBlockHostingVolume(tx); found {
				logger.Warning(
					"temporarily rejecting block volume request:" +
						" pending block-hosting-volume found")
				return ErrTooManyOperations
			} else if err != nil {
				return err
			}
			vol, err := NewVolumeEntryForBlockHosting(clusters)
			if err != nil {
				return err
			}
			brick_entries, err := vol.createVolumeComponents(txdb)
			if err != nil {
				return err
			}
			// we just allocated a new volume and bricks, we need to record
			// these in the op
			for _, brick := range brick_entries {
				bvc.op.RecordAddBrick(brick)
				if e := brick.Save(tx); e != nil {
					return e
				}
			}
			bvc.op.RecordAddHostingVolume(vol)
			if e := vol.Save(tx); e != nil {
				return e
			}
			bvc.bvol.Info.BlockHostingVolume = vol.Info.Id
			bvc.bvol.Info.Cluster = vol.Info.Cluster
		}

		if e := bvc.bvol.saveNewEntry(txdb); e != nil {
			return e
		}

		// we've figured out what block-volume, hosting volume, and bricks we
		// will be using for the next phase of the operation, save our pending sate
		bvc.op.RecordAddBlockVolume(bvc.bvol)
		if e := bvc.bvol.Save(tx); e != nil {
			return e
		}

		if e := bvc.op.Save(tx); e != nil {
			return e
		}
		return nil
	})
}

func (bvc *BlockVolumeCreateOperation) volAndBricks(db wdb.RODB) (
	vol *VolumeEntry, brick_entries []*BrickEntry, err error) {

	// NOTE: It is perfectly fine and normal for there to be no bricks or volumes
	// on the op. However if there are bricks there must be volumes (and vice versa).
	vol = nil
	volume_entries, err := volumesFromOp(db, bvc.op)
	if err != nil {
		logger.LogError("Failed to get volumes from op: %v", err)
		return
	}
	// try to get gid now even though we haven't done any sanity checks
	// yet. Otherwise we have to go to the db for bricks twice
	brickGid := int64(0)
	if len(volume_entries) == 1 {
		brickGid = volume_entries[0].Info.Gid
	}
	brick_entries, err = bricksFromOp(db, bvc.op, brickGid)
	if err != nil {
		logger.LogError("Failed to get bricks from op: %v", err)
		return
	}

	if len(volume_entries) > 1 {
		err = logger.LogError("Unexpected number of new volume entries (%v)",
			len(volume_entries))
		return
	}
	if len(volume_entries) > 0 && len(brick_entries) == 0 {
		err = logger.LogError("Cannot create a new block hosting volume without bricks")
		return
	}
	if len(volume_entries) == 0 && len(brick_entries) > 0 {
		err = logger.LogError("Cannot create bricks without a hosting volume")
		return
	}

	if len(volume_entries) == 1 {
		vol = volume_entries[0]
	}
	return
}

// Exec creates new bricks and volume on the underlying glusterfs storage system.
func (bvc *BlockVolumeCreateOperation) Exec(executor executors.Executor) error {
	vol, brick_entries, err := bvc.volAndBricks(bvc.db)
	if err != nil {
		return err
	}

	if vol != nil {
		err = vol.createVolumeExec(bvc.db, executor, brick_entries)
		if err != nil {
			logger.LogError("Error executing create volume: %v", err)
			return err
		}
	}
	// NOTE: unlike regular volume create this function does update attributes
	// of the block volume entry with values that come back from the exec commands.
	// this doesn't break the Operation model but does mean this is non trivially
	// resumeable if we ever add resume support to normal volume create.
	err = bvc.bvol.createBlockVolume(bvc.db, executor, bvc.bvol.Info.BlockHostingVolume)
	if err != nil {
		logger.LogError("Error executing create block volume: %v", err)
	}
	return err
}

// Finalize marks our new volume and brick db entries as no longer pending.
func (bvc *BlockVolumeCreateOperation) Finalize() error {
	return bvc.db.Update(func(tx *bolt.Tx) error {
		txdb := wdb.WrapTx(tx)
		vol, brick_entries, err := bvc.volAndBricks(txdb)
		if err != nil {
			return err
		}
		if vol != nil {
			for _, brick := range brick_entries {
				bvc.op.FinalizeBrick(brick)
				if e := brick.Save(tx); e != nil {
					return e
				}
			}
			bvc.op.FinalizeVolume(vol)
			if e := vol.Save(tx); e != nil {
				return e
			}
		}

		// block volume properties are mutated by the results coming
		// back during exec. These properties need to be saved back
		// to the db.
		// This is only noteworthy because it is different from regular
		// volumes which determines everything up front. Here certain
		// values are determined by gluster-block commands.
		if e := bvc.bvol.Save(tx); e != nil {
			return e
		}

		bvc.op.FinalizeBlockVolume(bvc.bvol)
		if e := bvc.bvol.Save(tx); e != nil {
			return e
		}

		bvc.op.Delete(tx)
		return nil
	})
}

// Rollback removes any dangling volume and bricks from the underlying storage
// systems and removes the corresponding pending volume and brick entries from
// the db.
func (bvc *BlockVolumeCreateOperation) Rollback(executor executors.Executor) error {
	// TODO make this into one transaction too
	vol, brick_entries, err := bvc.volAndBricks(bvc.db)
	if err != nil {
		return err
	}
	hvname, err := bvc.bvol.blockHostingVolumeName(bvc.db)
	if err != nil {
		return err
	}
	err = bvc.bvol.deleteBlockVolumeExec(bvc.db, hvname, executor)
	if err != nil {
		return err
	}
	if e := bvc.bvol.removeComponents(bvc.db, false); e != nil {
		return e
	}
	if vol != nil {
		err = vol.cleanupCreateVolume(bvc.db, executor, brick_entries)
		if err != nil {
			logger.LogError("Error on create volume rollback: %v", err)
			return err
		}
	}
	err = bvc.db.Update(func(tx *bolt.Tx) error {
		return bvc.op.Delete(tx)
	})
	return err
}

// BlockVolumeDeleteOperation implements the operation functions used to
// delete an existing volume.
type BlockVolumeDeleteOperation struct {
	OperationManager
	noRetriesOperation
	bvol *BlockVolumeEntry
}

func NewBlockVolumeDeleteOperation(
	bvol *BlockVolumeEntry, db wdb.DB) *BlockVolumeDeleteOperation {

	return &BlockVolumeDeleteOperation{
		OperationManager: OperationManager{
			db: db,
			op: NewPendingOperationEntry(NEW_ID),
		},
		bvol: bvol,
	}
}

func (vdel *BlockVolumeDeleteOperation) Label() string {
	return "Delete Block Volume"
}

func (vdel *BlockVolumeDeleteOperation) ResourceUrl() string {
	return ""
}

// Build determines what volumes and bricks need to be deleted and
// marks the db entries as such.
func (vdel *BlockVolumeDeleteOperation) Build() error {
	return vdel.db.Update(func(tx *bolt.Tx) error {
		v, err := NewBlockVolumeEntryFromId(tx, vdel.bvol.Info.Id)
		if err != nil {
			return err
		}
		vdel.bvol = v
		if vdel.bvol.Pending.Id != "" {
			logger.LogError("Pending block volume %v can not be deleted",
				vdel.bvol.Info.Id)
			return ErrConflict
		}
		vdel.op.RecordDeleteBlockVolume(vdel.bvol)
		if e := vdel.op.Save(tx); e != nil {
			return e
		}
		if e := vdel.bvol.Save(tx); e != nil {
			return e
		}
		return nil
	})
}

// Exec performs the volume and brick deletions on the storage systems.
func (vdel *BlockVolumeDeleteOperation) Exec(executor executors.Executor) error {
	hvname, err := vdel.bvol.blockHostingVolumeName(vdel.db)
	if err != nil {
		return err
	}
	return vdel.bvol.deleteBlockVolumeExec(vdel.db, hvname, executor)
}

func (vdel *BlockVolumeDeleteOperation) Rollback(executor executors.Executor) error {
	// currently rollback only removes the pending operation for delete block volume,
	// leaving the db in the same state as it was before an exec failure.
	// In the future we should make this operation resume-able
	return vdel.db.Update(func(tx *bolt.Tx) error {
		// REMINDER: Block volume delete and create are not symmetric in regards to
		// removing vs. creating the block hosting volume
		vdel.op.FinalizeBlockVolume(vdel.bvol)
		if e := vdel.bvol.Save(tx); e != nil {
			return e
		}

		vdel.op.Delete(tx)
		return nil
	})
}

// Finalize marks all brick and volume entries for this operation as
// fully deleted.
func (vdel *BlockVolumeDeleteOperation) Finalize() error {
	return vdel.db.Update(func(tx *bolt.Tx) error {
		txdb := wdb.WrapTx(tx)
		if e := vdel.bvol.removeComponents(txdb, false); e != nil {
			logger.LogError("Failed to remove block volume from db")
			return e
		}

		vdel.op.Delete(tx)
		return nil
	})
}
