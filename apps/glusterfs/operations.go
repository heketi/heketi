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

const (
	VOLUME_MAX_RETRIES int = 4
)

type OperationRetryError struct {
	OriginalError error
}

func (ore OperationRetryError) Error() string {
	return fmt.Sprintf("Operation Should Be Retried; Error: %v",
		ore.OriginalError.Error())
}

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
	// Label returns a short descriptive string indicating the kind
	// of operation being performed. Examples include "Create Volume"
	// and "Delete Block Volume". This string is most frequently used
	// for logging.
	Label() string
	// ResourceUrl returns a string indicating the steady-state result
	// of the operation and will be passed up to the API on successful
	// operations. Not all operations have a concrete result (deletes
	// for example) and those should return an empty string.
	ResourceUrl() string
	// Build functions implement the build phase of an operation; the
	// build phase constructs the db entries needed to perform the
	// operation in all subsequent steps. The db changes in Build should
	// be performed in a single transaction. This phase is responsible
	// for creating the PendingOperationEntry items in the db and
	// associating them with other elements.
	Build() error
	// Exec functions implement the exec phase of an operation; the
	// exec phase is responsible for manipulating the storage nodes
	// to apply the expected changes to the gluster system. The
	// exec phase is expected to take a large amount of time relative
	// to the other operation phases. DB transactions within the
	// exec phase should be read-only.
	Exec(executor executors.Executor) error
	// Rollback functions are responsible for undoing any state left
	// in the DB and/or storage nodes in case of a Build phase error.
	// Calling rollback should make it like Build and Exec never ran,
	// this includes removing pending operation entries from the db.
	Rollback(executor executors.Executor) error
	// Finalize functions implement the finalize phase of the operation;
	// it takes any of the db changes that were marked pending
	// by the build phase and removes the pending markers and pending
	// operation entries. This function should be performed in a
	// single transaction.
	Finalize() error
	MaxRetries() int
}

type noRetriesOperation struct{}

func (n *noRetriesOperation) MaxRetries() int {
	return 0
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

// BlockVolumeCreateOperation  implements the operation functions used to
// create a new volume.
type BlockVolumeCreateOperation struct {
	OperationManager
	noRetriesOperation
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

// DeviceRemoveOperation is a phony-ish operation that exists
// primarily to a) know that set state was being performed
// and b) to serve as a starting point for a more proper
// operation in the future.
type DeviceRemoveOperation struct {
	OperationManager
	noRetriesOperation
	DeviceId string
}

func NewDeviceRemoveOperation(
	deviceId string, db wdb.DB) *DeviceRemoveOperation {

	return &DeviceRemoveOperation{
		OperationManager: OperationManager{
			db: db,
			op: NewPendingOperationEntry(NEW_ID),
		},
		DeviceId: deviceId,
	}
}

func (dro *DeviceRemoveOperation) Label() string {
	return "Remove Device"
}

func (dro *DeviceRemoveOperation) ResourceUrl() string {
	return ""
}

func (dro *DeviceRemoveOperation) Build() error {
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
	if e := dro.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, id)
		return err
	}); e != nil {
		return e
	}

	return d.removeBricksFromDevice(dro.db, executor)
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
					logger.LogError("failed to find brick with id: %v", a.Id)
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
