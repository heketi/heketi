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
	"net/http"

	"github.com/heketi/heketi/executors"
	wdb "github.com/heketi/heketi/pkg/db"

	"github.com/boltdb/bolt"
)

// The operations.go file is meant to provide a common approach to planning,
// executing, and completing changes to the storage clusters under heketi
// management as well as accurately reflecting these changes in the heketi db.
//
// We define the Operation interface and helper functions that use the
// interface to create a uniform style for making high-level changes to the
// system. We also provide various concrete operation structs such as for
// volume create or volume delete to actually perform the actions.

// Operation is an interface meant to encapsulate any high-level action
// where we need to build and store data structures that reflect our
// pending state, execute actions to apply our configuration to the
// managed cluster(s), and then either record the data structures as final
// or roll back to the previous state on error.
type Operation interface {
	Label() string
	ResourceUrl() string
	Build(allocator Allocator) error
	Exec(executor executors.Executor) error
	Rollback(executor executors.Executor) error
	Finalize() error
}

// OperationManager is an embeddable struct meant to be used within any
// operation that tracks changes with a pending operation entry.
type OperationManager struct {
	db wdb.DB
	op *PendingOperationEntry
}

// Id returns the id of this operation's pending operation entry.
func (om *OperationManager) Id() string {
	return om.op.Id
}

// VolumeCreateOperation implements the operation functions used to
// create a new volume.
type VolumeCreateOperation struct {
	OperationManager
	vol *VolumeEntry
}

// NewVolumeCreateOperation returns a new VolumeCreateOperation populated
// with the given volume entry and db connection and allocates a new
// pending operation entry.
func NewVolumeCreateOperation(
	vol *VolumeEntry, db wdb.DB) *VolumeCreateOperation {

	return &VolumeCreateOperation{
		OperationManager: OperationManager{
			db: db,
			op: NewPendingOperationEntry(NEW_ID),
		},
		vol: vol,
	}
}

func (vc *VolumeCreateOperation) Label() string {
	return "Create Volume"
}

func (vc *VolumeCreateOperation) ResourceUrl() string {
	return fmt.Sprintf("/volumes/%v", vc.vol.Info.Id)
}

// Build allocates and saves new volume and brick entries (tagged as pending)
// in the db.
func (vc *VolumeCreateOperation) Build(allocator Allocator) error {
	return vc.db.Update(func(tx *bolt.Tx) error {
		txdb := wdb.WrapTx(tx)
		brick_entries, err := vc.vol.createVolumeComponents(txdb, allocator)
		if err != nil {
			return err
		}
		for _, brick := range brick_entries {
			vc.op.RecordAddBrick(brick)
			if e := brick.Save(tx); e != nil {
				return e
			}
		}
		vc.op.RecordAddVolume(vc.vol)
		if e := vc.vol.Save(tx); e != nil {
			return e
		}
		if e := vc.op.Save(tx); e != nil {
			return e
		}
		return nil
	})
}

// Exec creates new bricks and volume on the underlying glusterfs storage system.
func (vc *VolumeCreateOperation) Exec(executor executors.Executor) error {
	brick_entries, err := bricksFromOp(vc.db, vc.op, vc.vol.Info.Gid)
	if err != nil {
		logger.LogError("Failed to get bricks from op: %v", err)
		return err
	}
	err = vc.vol.createVolumeExec(vc.db, executor, brick_entries)
	if err != nil {
		logger.LogError("Error executing create volume: %v", err)
	}
	return err
}

// Finalize marks our new volume and brick db entries as no longer pending.
func (vc *VolumeCreateOperation) Finalize() error {
	return vc.db.Update(func(tx *bolt.Tx) error {
		brick_entries, err := bricksFromOp(wdb.WrapTx(tx), vc.op, vc.vol.Info.Gid)
		if err != nil {
			logger.LogError("Failed to get bricks from op: %v", err)
			return err
		}
		for _, brick := range brick_entries {
			vc.op.FinalizeBrick(brick)
			if e := brick.Save(tx); e != nil {
				return e
			}
		}
		vc.op.FinalizeVolume(vc.vol)
		if e := vc.vol.Save(tx); e != nil {
			return e
		}

		vc.op.Delete(tx)
		return nil
	})
}

// Rollback removes any dangling volume and bricks from the underlying storage
// systems and removes the corresponding pending volume and brick entries from
// the db.
func (vc *VolumeCreateOperation) Rollback(executor executors.Executor) error {
	// TODO make this into one transaction too
	brick_entries, err := bricksFromOp(vc.db, vc.op, vc.vol.Info.Gid)
	if err != nil {
		logger.LogError("Failed to get bricks from op: %v", err)
		return err
	}
	err = vc.vol.cleanupCreateVolume(vc.db, executor, brick_entries)
	if err != nil {
		logger.LogError("Error on create volume rollback: %v", err)
		return err
	}
	err = vc.db.Update(func(tx *bolt.Tx) error {
		return vc.op.Delete(tx)
	})
	return err
}

// VolumeExpandOperation implements the operation functions used to
// expand an existing volume.
type VolumeExpandOperation struct {
	OperationManager
	vol *VolumeEntry

	// modification values
	ExpandSize int
}

// NewVolumeCreateOperation creates a new VolumeExpandOperation populated
// with the given volume entry, db connection and size (in GB) that the
// volume is to be expanded by.
func NewVolumeExpandOperation(
	vol *VolumeEntry, db wdb.DB, sizeGB int) *VolumeExpandOperation {

	return &VolumeExpandOperation{
		OperationManager: OperationManager{
			db: db,
			op: NewPendingOperationEntry(NEW_ID),
		},
		vol:        vol,
		ExpandSize: sizeGB,
	}
}

func (ve *VolumeExpandOperation) Label() string {
	return "Expand Volume"
}

func (ve *VolumeExpandOperation) ResourceUrl() string {
	return fmt.Sprintf("/volumes/%v", ve.vol.Info.Id)
}

// Build determines what new bricks needs to be created to satisfy the
// new volume size. It marks new bricks as pending in the db.
func (ve *VolumeExpandOperation) Build(allocator Allocator) error {
	return ve.db.Update(func(tx *bolt.Tx) error {
		txdb := wdb.WrapTx(tx)
		brick_entries, err := ve.vol.expandVolumeComponents(
			txdb, allocator, ve.ExpandSize, false)
		if err != nil {
			return err
		}
		for _, brick := range brick_entries {
			ve.op.RecordAddBrick(brick)
			if e := brick.Save(tx); e != nil {
				return e
			}
		}
		ve.op.RecordExpandVolume(ve.vol, ve.ExpandSize)
		if e := ve.op.Save(tx); e != nil {
			return e
		}
		return nil
	})
}

// Exec creates new bricks on the underlying storage systems.
func (ve *VolumeExpandOperation) Exec(executor executors.Executor) error {
	brick_entries, err := bricksFromOp(ve.db, ve.op, ve.vol.Info.Gid)
	if err != nil {
		logger.LogError("Failed to get bricks from op: %v", err)
		return err
	}
	err = ve.vol.expandVolumeExec(ve.db, executor, brick_entries)
	if err != nil {
		logger.LogError("Error executing expand volume: %v", err)
	}
	return err
}

// Rollback cancels the volume expansion and remove pending brick entries
// from the db.
func (ve *VolumeExpandOperation) Rollback(executor executors.Executor) error {
	// TODO make this into one transaction too
	brick_entries, err := bricksFromOp(ve.db, ve.op, ve.vol.Info.Gid)
	if err != nil {
		logger.LogError("Failed to get bricks from op: %v", err)
		return err
	}
	err = ve.vol.cleanupExpandVolume(
		ve.db, executor, brick_entries, ve.vol.Info.Size)
	if err != nil {
		logger.LogError("Error on create volume rollback: %v", err)
		return err
	}
	err = ve.db.Update(func(tx *bolt.Tx) error {
		return ve.op.Delete(tx)
	})
	return err
}

// Finalize marks new bricks as no longer pending and updates the size
// of the existing volume entry.
func (ve *VolumeExpandOperation) Finalize() error {
	return ve.db.Update(func(tx *bolt.Tx) error {
		brick_entries, err := bricksFromOp(wdb.WrapTx(tx), ve.op, ve.vol.Info.Gid)
		if err != nil {
			logger.LogError("Failed to get bricks from op: %v", err)
			return err
		}
		sizeDelta, err := expandSizeFromOp(ve.op)
		if err != nil {
			logger.LogError("Failed to get expansion size from op: %v", err)
			return err
		}

		for _, brick := range brick_entries {
			ve.op.FinalizeBrick(brick)
			if e := brick.Save(tx); e != nil {
				return e
			}
		}
		ve.vol.Info.Size += sizeDelta
		ve.op.FinalizeVolume(ve.vol)
		if e := ve.vol.Save(tx); e != nil {
			return e
		}

		ve.op.Delete(tx)
		return nil
	})
}

// VolumeDeleteOperation implements the operation functions used to
// delete an existing volume.
type VolumeDeleteOperation struct {
	OperationManager
	vol *VolumeEntry
}

func NewVolumeDeleteOperation(
	vol *VolumeEntry, db wdb.DB) *VolumeDeleteOperation {

	return &VolumeDeleteOperation{
		OperationManager: OperationManager{
			db: db,
			op: NewPendingOperationEntry(NEW_ID),
		},
		vol: vol,
	}
}

func (vdel *VolumeDeleteOperation) Label() string {
	return "Delete Volume"
}

func (vdel *VolumeDeleteOperation) ResourceUrl() string {
	return ""
}

// Build determines what volumes and bricks need to be deleted and
// marks the db entries as such.
func (vdel *VolumeDeleteOperation) Build(allocator Allocator) error {
	return vdel.db.Update(func(tx *bolt.Tx) error {
		txdb := wdb.WrapTx(tx)
		brick_entries, err := vdel.vol.deleteVolumeComponents(txdb)
		if err != nil {
			return err
		}
		for _, brick := range brick_entries {
			vdel.op.RecordDeleteBrick(brick)
			if e := brick.Save(tx); e != nil {
				return e
			}
		}
		vdel.op.RecordDeleteVolume(vdel.vol)
		if e := vdel.op.Save(tx); e != nil {
			return e
		}
		return nil
	})
}

// Exec performs the volume and brick deletions on the storage systems.
func (vdel *VolumeDeleteOperation) Exec(executor executors.Executor) error {
	brick_entries, err := bricksFromOp(vdel.db, vdel.op, vdel.vol.Info.Gid)
	if err != nil {
		logger.LogError("Failed to get bricks from op: %v", err)
		return err
	}
	sshhost, err := vdel.vol.manageHostFromBricks(vdel.db, brick_entries)
	if err != nil {
		return err
	}
	err = vdel.vol.deleteVolumeExec(vdel.db, executor, brick_entries, sshhost)
	if err != nil {
		logger.LogError("Error executing expand volume: %v", err)
	}
	return err
}

func (vdel *VolumeDeleteOperation) Rollback(executor executors.Executor) error {
	// currently rollback only removes the pending operation for delete volume,
	// leaving the db in the same state as it was before an exec failure.
	// In the future we should make this operation resume-able
	return vdel.db.Update(func(tx *bolt.Tx) error {
		txdb := wdb.WrapTx(tx)
		brick_entries, err := bricksFromOp(txdb, vdel.op, vdel.vol.Info.Gid)
		if err != nil {
			logger.LogError("Failed to get bricks from op: %v", err)
			return err
		}

		for _, brick := range brick_entries {
			vdel.op.FinalizeBrick(brick)
			if e := brick.Save(tx); e != nil {
				return e
			}
		}
		vdel.op.FinalizeVolume(vdel.vol)
		if e := vdel.vol.Save(tx); e != nil {
			return err
		}

		vdel.op.Delete(tx)
		return nil
	})
}

// Finalize marks all brick and volume entries for this operation as
// fully deleted.
func (vdel *VolumeDeleteOperation) Finalize() error {
	return vdel.db.Update(func(tx *bolt.Tx) error {
		txdb := wdb.WrapTx(tx)
		brick_entries, err := bricksFromOp(txdb, vdel.op, vdel.vol.Info.Gid)
		if err != nil {
			logger.LogError("Failed to get bricks from op: %v", err)
			return err
		}
		if err := vdel.vol.saveDeleteVolume(txdb, brick_entries); err != nil {
			return err
		}

		vdel.op.Delete(tx)
		return nil
	})
}

// BlockVolumeCreateOperation  implements the operation functions used to
// create a new volume.
type BlockVolumeCreateOperation struct {
	OperationManager
	bvol *BlockVolumeEntry
	//vol *VolumeEntry
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
func (bvc *BlockVolumeCreateOperation) Build(allocator Allocator) error {
	return bvc.db.Update(func(tx *bolt.Tx) error {
		txdb := wdb.WrapTx(tx)
		clusters, volumes, err := bvc.bvol.eligibleClustersAndVolumes(txdb)
		if err != nil {
			return err
		}

		if len(volumes) > 0 {
			bvc.bvol.Info.BlockHostingVolume = volumes[0]
		} else {
			vol, err := NewVolumeEntryForBlockHosting(clusters)
			if err != nil {
				return err
			}
			brick_entries, err := vol.createVolumeComponents(txdb, allocator)
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
			if e := bvc.bvol.Save(tx); e != nil {
				return e
			}
			bvc.bvol.Info.BlockHostingVolume = vol.Info.Id
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

		// traditionally (that is before operations existed) the block volumes
		// were updating most of their metadata after exec. This is only
		// noteworthy because it is different from regular volumes which
		// do most of those updates upfront.
		if e := bvc.bvol.saveCreateBlockVolume(txdb); e != nil {
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
	if e := bvc.bvol.cleanupBlockVolumeCreate(bvc.db, executor); e != nil {
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
func (vdel *BlockVolumeDeleteOperation) Build(allocator Allocator) error {
	return vdel.db.Update(func(tx *bolt.Tx) error {
		vdel.op.RecordDeleteBlockVolume(vdel.bvol)
		if e := vdel.op.Save(tx); e != nil {
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
	// currently rollback does nothing for delete volume, leaving the
	// db in the same state as it was before an exec failure
	// TODO: revisit
	return nil
}

// Finalize marks all brick and volume entries for this operation as
// fully deleted.
func (vdel *BlockVolumeDeleteOperation) Finalize() error {
	return vdel.db.Update(func(tx *bolt.Tx) error {
		txdb := wdb.WrapTx(tx)
		if e := vdel.bvol.removeComponents(txdb); e != nil {
			logger.LogError("Failed to remove block volume from db")
			return e
		}

		vdel.op.Delete(tx)
		return nil
	})
}

// DeviceRemoveOperation is a phony-ish operation that exists
// primarily to a) know that set state was being performed
// and b) to serve as a starting point for a more proper
// operation in the future.
type DeviceRemoveOperation struct {
	OperationManager
	DeviceId  string
	allocator Allocator
}

// Note: passing this allocator here a big hack, but its a temporary
// workaround for how brick relocation currently works
func NewDeviceRemoveOperation(
	deviceId string, allocator Allocator, db wdb.DB) *DeviceRemoveOperation {

	return &DeviceRemoveOperation{
		OperationManager: OperationManager{
			db: db,
			op: NewPendingOperationEntry(NEW_ID),
		},
		DeviceId:  deviceId,
		allocator: allocator,
	}
}

func (dro *DeviceRemoveOperation) Label() string {
	return "Remove Device"
}

func (dro *DeviceRemoveOperation) ResourceUrl() string {
	return ""
}

func (dro *DeviceRemoveOperation) Build(allocator Allocator) error {
	return dro.db.Update(func(tx *bolt.Tx) error {
		d, err := NewDeviceEntryFromId(tx, dro.DeviceId)
		if err != nil {
			return err
		}
		txdb := wdb.WrapTx(tx)

		// If the device has no bricks, just change the state and we are done
		if err := d.markFailed(txdb); err == nil {
			// device was empty and is now marked failed
			return nil
		} else if err != ErrConflict {
			// we hit some sort of unexpected error
			return err
		}
		// if we're here markFailed couldn't apply due to conflicts
		// we don't need to actually record anything in the db
		// because this is not a long running operation

		if p, err := PendingOperationsOnDevice(txdb, d.Info.Id); err != nil {
			return err
		} else if p {
			logger.LogError("Found operations still pending on device."+
				" Can not remove device %v at this time.",
				d.Info.Id)
			return ErrConflict
		}

		dro.op.RecordRemoveDevice(d)
		if e := dro.op.Save(tx); e != nil {
			return e
		}
		return nil
	})
}

func (dro *DeviceRemoveOperation) deviceId() (string, error) {
	if len(dro.op.Actions) == 0 {
		// we intentionally avoid recording any actions when all needed bits
		// were taken care of in Build. There's nothing more to do here.
		return "", nil
	}
	if dro.op.Actions[0].Change != OpRemoveDevice {
		return "", fmt.Errorf("Unexpected action (%v) on DeviceRemoveOperation pending op",
			dro.op.Actions[0].Change)
	}
	return dro.op.Actions[0].Id, nil
}

func (dro *DeviceRemoveOperation) Exec(executor executors.Executor) error {
	id, err := dro.deviceId()
	if err != nil {
		return err
	}
	if id == "" {
		return nil
	}

	var d *DeviceEntry
	if e := dro.db.Update(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, id)
		return err
	}); e != nil {
		return e
	}

	// pulling the allocator from the operation is a huge hack.
	// its basically an intentional violation of the Operation model that
	// we need to do if for now because the remove bricks code is an
	// extra big tangle
	return d.removeBricksFromDevice(dro.db, executor, dro.allocator)
}

func (dro *DeviceRemoveOperation) Rollback(executor executors.Executor) error {
	return dro.db.Update(func(tx *bolt.Tx) error {
		dro.op.Delete(tx)
		return nil
	})
}

func (dro *DeviceRemoveOperation) Finalize() error {
	id, err := dro.deviceId()
	if err != nil {
		return err
	}
	if id == "" {
		return nil
	}
	return dro.db.Update(func(tx *bolt.Tx) error {
		txdb := wdb.WrapTx(tx)
		if e := markDeviceFailed(txdb, id, true); e != nil {
			return e
		}
		return dro.op.Delete(tx)
	})
}

// bricksFromOp returns pending brick entry objects from the db corresponding
// to the given pending operation entry. The gid of the volume must also be
// provided as the db does not store this metadata on the brick entries.
func bricksFromOp(db wdb.RODB,
	op *PendingOperationEntry, gid int64) ([]*BrickEntry, error) {

	brick_entries := []*BrickEntry{}
	err := db.View(func(tx *bolt.Tx) error {
		for _, a := range op.Actions {
			if a.Change == OpAddBrick || a.Change == OpDeleteBrick {
				brick, err := NewBrickEntryFromId(tx, a.Id)
				if err != nil {
					return err
				}
				// this next line is a bit of an unfortunate hack because
				// the db does not preserver the requested gid that is
				// needed for the request
				brick.gidRequested = gid
				brick_entries = append(brick_entries, brick)
			}
		}
		return nil
	})
	return brick_entries, err
}

func volumesFromOp(db wdb.RODB,
	op *PendingOperationEntry) ([]*VolumeEntry, error) {

	volume_entries := []*VolumeEntry{}
	err := db.View(func(tx *bolt.Tx) error {
		for _, a := range op.Actions {
			if a.Change == OpAddVolume {
				brick, err := NewVolumeEntryFromId(tx, a.Id)
				if err != nil {
					return err
				}
				volume_entries = append(volume_entries, brick)
			}
		}
		return nil
	})
	return volume_entries, err
}

// expandSizeFromOp returns the size of a volume expand operation assuming
// the given pending operation entry includes a volume expand change item.
// If the operation is of the wrong type error will be non-nil.
func expandSizeFromOp(op *PendingOperationEntry) (sizeGB int, e error) {
	for _, a := range op.Actions {
		if a.Change == OpExpandVolume {
			sizeGB, e = a.ExpandSize()
			return
		}
	}
	e = fmt.Errorf("no OpExpandVolume action in pending op: %v",
		op.Id)
	return
}

// AsyncHttpOperation runs all the steps of an operation with the long-running
// parts wrapped in an async http function. If AsyncHttpOperation returns nil
// then it has started the async function and the caller should respond to the
// client with success - otherwise an error object is returned. In the async
// function the Exec and Finalize or Rollback steps of the operation will be
// performed.
func AsyncHttpOperation(app *App,
	w http.ResponseWriter,
	r *http.Request,
	op Operation) error {

	label := op.Label()
	if err := op.Build(app.Allocator()); err != nil {
		logger.LogError("%v Build Failed: %v", label, err)
		return err
	}

	app.asyncManager.AsyncHttpRedirectFunc(w, r, func() (string, error) {
		logger.Info("Started async operation: %v", label)
		if err := op.Exec(app.executor); err != nil {
			if rerr := op.Rollback(app.executor); rerr != nil {
				logger.LogError("%v Rollback error: %v", label, rerr)
			}
			logger.LogError("%v Failed: %v", label, err)
			return "", err
		}
		if err := op.Finalize(); err != nil {
			logger.LogError("%v Finalize failed: %v", label, err)
			return "", err
		}
		logger.Info("%v succeeded", label)
		return op.ResourceUrl(), nil
	})
	return nil
}

// RunOperation performs all steps of an Operation and returns
// an error if any of those steps fail. This function is meant to
// make it easy to run an operation outside of the rest endpoints
// and should only be used in test code.
func RunOperation(o Operation,
	allocator Allocator,
	executor executors.Executor) (err error) {

	label := o.Label()
	defer func() {
		if err != nil {
			logger.LogError("Error in %v: %v", label, err)
		}
	}()

	logger.Info("Running %v", o.Label())
	if err := o.Build(allocator); err != nil {
		logger.LogError("%v Build Failed: %v", label, err)
		return err
	}
	if err := o.Exec(executor); err != nil {
		if rerr := o.Rollback(executor); rerr != nil {
			logger.LogError("%v Rollback error: %v", label, rerr)
		}
		logger.LogError("%v Failed: %v", label, err)
		return err
	}
	if err := o.Finalize(); err != nil {
		return err
	}
	return nil
}
