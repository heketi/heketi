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

type replaceStatus int

const (
	replaceStatusUnknown replaceStatus = iota
	replaceIncomplete
	replaceComplete
)

// BrickEvictOperation removes exactly one brick from a volume
// automatically replacing it to maintain any other volume restrictions.
type BrickEvictOperation struct {
	OperationManager
	noRetriesOperation
	BrickId string

	// internal caching params
	replaceBrickSet *BrickSet
	replaceIndex    int
	newBrickEntryId string
	reclaimed       ReclaimMap
	replaceResult   replaceStatus
}

type brickContext struct {
	brick  *BrickEntry
	volume *VolumeEntry
	device *DeviceEntry
	node   *NodeEntry
}

func (bc brickContext) bhmap() brickHostMap {
	return brickHostMap{
		bc.brick: bc.node.ManageHostName(),
	}
}

func NewBrickEvictOperation(
	brickId string, db wdb.DB) *BrickEvictOperation {

	return &BrickEvictOperation{
		OperationManager: OperationManager{
			db: db,
			op: NewPendingOperationEntry(NEW_ID),
		},
		BrickId: brickId,
	}
}

// loadBrickEvictOperation returns a BrickEvictOperation populated
// from an existing pending operation entry in the db.
func loadBrickEvictOperation(
	db wdb.DB, p *PendingOperationEntry) (*BrickEvictOperation, error) {

	var brickId string
	for _, action := range p.Actions {
		if action.Change == OpDeleteBrick {
			brickId = action.Id
		}
	}
	if brickId == "" {
		return nil, fmt.Errorf(
			"Missing brick to evict in operation")
	}

	return &BrickEvictOperation{
		OperationManager: OperationManager{
			db: db,
			op: p,
		},
		BrickId: brickId,
	}, nil
}

func (beo *BrickEvictOperation) Label() string {
	return "Evict Brick"
}

func (beo *BrickEvictOperation) ResourceUrl() string {
	return ""
}

func (beo *BrickEvictOperation) Build() error {
	// this build is pretty minimal as it only can record the brick
	// in need of eviction. The replacement brick can not be determined
	// without running (gluster) commands.
	return beo.db.Update(func(tx *bolt.Tx) error {
		old, err := beo.current(wdb.WrapTx(tx))
		if err != nil {
			return err
		}
		if old.brick.Pending.Id != "" {
			return fmt.Errorf(
				"Can not evict brick %v: brick is pending", old.brick.Id())
		}
		for _, bid := range old.volume.Bricks {
			b, err := NewBrickEntryFromId(tx, bid)
			if err != nil {
				return err
			}
			if b.Pending.Id != "" {
				logger.Warning("Found pending brick %v on volume %v",
					bid, old.volume.Info.Id)
				return fmt.Errorf(
					"Can not evict brick %v from volume %v: volume has pending bricks",
					old.brick.Id(), old.volume.Info.Id)
			}
		}

		brickEntry, err := NewBrickEntryFromId(tx, beo.BrickId)
		if err != nil {
			return err
		}
		beo.op.Type = OperationBrickEvict
		beo.op.RecordDeleteBrick(brickEntry)
		if e := brickEntry.Save(tx); e != nil {
			return e
		}
		if e := beo.op.Save(tx); e != nil {
			return e
		}
		return nil
	})
}

func (beo *BrickEvictOperation) buildNewBrick(bs *BrickSet, index int) error {
	return beo.db.Update(func(tx *bolt.Tx) error {
		old, err := beo.current(wdb.WrapTx(tx))
		if err != nil {
			return err
		}
		// determine the placement for the new brick
		newBrickEntry, newDeviceEntry, err := old.volume.allocBrickReplacement(
			wdb.WrapTx(tx), old.brick, old.device, bs, index)
		if err != nil {
			return err
		}
		// update pending op with new brick info
		for _, action := range beo.op.Actions {
			if action.Change == OpAddBrick {
				return fmt.Errorf("operation already has a new brick")
			}
		}
		beo.op.RecordAddBrick(newBrickEntry)
		if e := beo.op.Save(tx); e != nil {
			return e
		}
		// save new brick entry
		if e := newBrickEntry.Save(tx); e != nil {
			return e
		}
		// update operation cached entries for new brick
		beo.newBrickEntryId = newBrickEntry.Id()
		return nil
	})
}

// current returns the brickContext containing a brick entry, volume entry,
// device entry and node entry for the brick to be removed.
func (beo *BrickEvictOperation) current(db wdb.RODB) (brickContext, error) {

	var bc brickContext
	err := beo.db.View(func(tx *bolt.Tx) error {
		var err error
		bc.brick, err = NewBrickEntryFromId(tx, beo.BrickId)
		if err != nil {
			return err
		}
		bc.device, err = NewDeviceEntryFromId(tx, bc.brick.Info.DeviceId)
		if err != nil {
			return err
		}
		bc.node, err = NewNodeEntryFromId(tx, bc.brick.Info.NodeId)
		if err != nil {
			return err
		}
		bc.volume, err = NewVolumeEntryFromId(tx, bc.brick.Info.VolumeId)
		if err != nil {
			return err
		}
		return nil
	})
	return bc, err
}

func (beo *BrickEvictOperation) execGetReplacmentInfo(
	executor executors.Executor) error {

	old, err := beo.current(beo.db)
	if err != nil {
		return err
	}
	node := old.node.ManageHostName()

	bs, index, err := old.volume.getBrickSetForBrickId(
		beo.db, executor, old.brick.Info.Id, node)
	if err != nil {
		return err
	}

	err = old.volume.canReplaceBrickInBrickSet(beo.db, executor, node, bs, index)
	if err != nil {
		return err
	}
	beo.replaceBrickSet = bs
	beo.replaceIndex = index
	return nil
}

func (beo *BrickEvictOperation) execReplaceBrick(
	executor executors.Executor) error {

	var (
		err               error
		old               brickContext
		newBrickEntry     *BrickEntry
		newBrickNodeEntry *NodeEntry
	)
	err = beo.db.View(func(tx *bolt.Tx) error {
		var err error
		old, err = beo.current(beo.db)
		if err != nil {
			return err
		}
		newBrickEntry, err = NewBrickEntryFromId(tx, beo.newBrickEntryId)
		if err != nil {
			return err
		}
		newBrickNodeEntry, err = NewNodeEntryFromId(
			tx, newBrickEntry.Info.NodeId)
		return err
	})
	if err != nil {
		return err
	}

	brickEntries := []*BrickEntry{newBrickEntry}
	err = CreateBricks(beo.db, executor, brickEntries)
	if err != nil {
		return err
	}

	var (
		oldBrick executors.BrickInfo
		newBrick executors.BrickInfo
	)
	oldBrick.Path = old.brick.Info.Path
	oldBrick.Host = old.node.StorageHostName()
	newBrick.Path = newBrickEntry.Info.Path
	newBrick.Host = newBrickNodeEntry.StorageHostName()

	node := newBrickNodeEntry.ManageHostName()
	err = executor.VolumeReplaceBrick(
		node, old.volume.Info.Name, &oldBrick, &newBrick)
	if err != nil {
		return err
	}

	beo.reclaimed, err = old.bhmap().destroy(executor)
	if err != nil {
		return logger.LogError("Error destroying old brick: %v", err)
	}
	return nil
}

func (beo *BrickEvictOperation) Exec(executor executors.Executor) error {
	// PHASE I
	// first we need to determine a) if we can remove a brick at this
	// time. and b) what brick set our current brick belongs to.
	// This updates the operation state.
	err := beo.execGetReplacmentInfo(executor)
	if err != nil {
		return err
	}
	// PHASE II
	// update the operation metadata with the new brick
	if e := beo.buildNewBrick(beo.replaceBrickSet, beo.replaceIndex); e != nil {
		return e
	}
	// PHASE III
	// actually perform the replacement of the brick
	return beo.execReplaceBrick(executor)
}

func (beo *BrickEvictOperation) Rollback(executor executors.Executor) error {
	return rollbackViaClean(beo, executor)
}

func (beo *BrickEvictOperation) Finalize() error {
	return beo.db.Update(func(tx *bolt.Tx) error {
		return beo.finishAccept(tx)
	})
}

func (beo *BrickEvictOperation) refreshOp(db wdb.RODB) error {
	return db.View(func(tx *bolt.Tx) error {
		op, err := NewPendingOperationEntryFromId(tx, beo.op.Id)
		if err != nil {
			return err
		}
		beo.op = op
		return nil
	})
}

func (beo *BrickEvictOperation) Clean(executor executors.Executor) error {
	logger.Info("Starting Clean for %v op:%v", beo.Label(), beo.op.Id)
	// ensure our cached op is synced with the db state
	if e := beo.refreshOp(beo.db); e != nil {
		return e
	}
	bes, err := evictStatus(beo.op)
	if err != nil {
		return err
	}

	if bes.newBrickId == "" {
		// no new brick has been allocated by heketi yet. thus no changes have
		// been made on the backend. We can just clean up our db w/o making
		// any backend clean ups.
		return nil
	}
	beo.newBrickEntryId = bes.newBrickId

	// The operation reached some point after having decided to create a
	// new brick. At any point before requesting gluster do the brick replace
	// we can just "unroll" the new brick components but if gluster accepted
	// our brick replace call we're past a point of no return as that brick
	// is probably in use and we should just "push forward" with the
	// replacement. Therefore we must start off by querying gluster's state.
	old, err := beo.current(beo.db)
	if err != nil {
		return err
	}
	node := old.node.ManageHostName() // TODO: try on multiple nodes?
	vinfo, err := executor.VolumeInfo(node, old.volume.Info.Name)
	if err != nil {
		logger.LogError("Unable to get volume info from gluster node %v for volume %v: %v", node, old.volume.Info.Name, err)
		return err
	}

	var newBHMap brickHostMap
	err = beo.db.View(func(tx *bolt.Tx) error {
		txdb := wdb.WrapTx(tx)
		bmap, err := old.volume.brickNameMap(txdb)
		if err != nil {
			return err
		}
		newBrick, err := NewBrickEntryFromId(tx, beo.newBrickEntryId)
		if err != nil {
			return err
		}
		newBrickNodeEntry, err := NewNodeEntryFromId(
			tx, newBrick.Info.NodeId)
		if err != nil {
			return err
		}
		newBHMap = brickHostMap{
			newBrick: newBrickNodeEntry.ManageHostName(),
		}
		newBrickHostPath, err := brickHostPath(txdb, newBrick)
		if err != nil {
			return err
		}
		// default to assuming old brick is still in use by gluster
		beo.replaceResult = replaceIncomplete
		for _, gbrick := range vinfo.Bricks.BrickList {
			if _, found := bmap[gbrick.Name]; found {
				logger.Info(
					"Found existing gluster brick in volume: %v", gbrick.Name)
			} else if newBrickHostPath == gbrick.Name {
				beo.replaceResult = replaceComplete
				logger.Info(
					"Found new brick in gluster volume: %v", newBrickHostPath)
			} else {
				return logger.LogError(
					"Found gluster brick not matching in heketi db: %v",
					gbrick.Name)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	switch beo.replaceResult {
	case replaceComplete:
		logger.Info("Destroying old brick contents")
		beo.reclaimed, err = old.bhmap().destroy(executor)
		if err != nil {
			return logger.LogError("Error destroying old brick: %v", err)
		}
	case replaceIncomplete:
		logger.Info("Destroying unused new brick contents")
		beo.reclaimed, err = newBHMap.destroy(executor)
		if err != nil {
			return logger.LogError("Error destroying new brick: %v", err)
		}
	default:
		return logger.LogError("Replace state unknown")
	}
	return nil
}

func (beo *BrickEvictOperation) CleanDone() error {
	logger.Info("Clean is done for %v op:%v", beo.Label(), beo.op.Id)
	return beo.db.Update(func(tx *bolt.Tx) error {
		if e := beo.refreshOp(beo.db); e != nil {
			return e
		}
		bes, err := evictStatus(beo.op)
		if err != nil {
			return err
		}

		if bes.newBrickId == "" {
			logger.Debug("no new brick: operation never started")
			return beo.finishNeverStarted(wdb.WrapTx(tx), bes)
		}
		switch beo.replaceResult {
		case replaceComplete:
			return beo.finishAccept(tx)
		case replaceIncomplete:
			return beo.finishRevert(tx)
		default:
			return logger.LogError("Replace state unknown (was clean not executed)")
		}
	})
}

func (beo *BrickEvictOperation) finishNeverStarted(db wdb.DB, bes brickEvictStatus) error {
	return db.Update(func(tx *bolt.Tx) error {
		b, err := NewBrickEntryFromId(tx, bes.oldBrickId)
		if err != nil {
			return err
		}
		beo.op.FinalizeBrick(b)
		if e := b.Save(tx); e != nil {
			return e
		}
		return beo.op.Delete(tx)
	})
}

func (beo *BrickEvictOperation) finishAccept(tx *bolt.Tx) error {
	old, err := beo.current(beo.db)
	if err != nil {
		return err
	}
	newBrick, err := NewBrickEntryFromId(tx, beo.newBrickEntryId)
	if err != nil {
		return err
	}
	// remove old brick (return space + update links)
	err = old.brick.removeAndFree(
		tx, old.volume, beo.reclaimed[old.brick.Info.DeviceId])
	if err != nil {
		return err
	}
	// update (new) device with new brick
	// reminder: New brick function takes space from device (!!!)
	newDevice, err := NewDeviceEntryFromId(tx, newBrick.Info.DeviceId)
	if err != nil {
		return err
	}
	newDevice.BrickAdd(newBrick.Id())
	// add new brick to volume
	old.volume.BrickAdd(newBrick.Id())
	// save volume
	if e := old.volume.Save(tx); e != nil {
		return e
	}
	// save new device
	if e := newDevice.Save(tx); e != nil {
		return e
	}
	// save new brick
	beo.op.FinalizeBrick(newBrick)
	if e := newBrick.Save(tx); e != nil {
		return e
	}
	// remove pending markers
	return beo.op.Delete(tx)
}

func (beo *BrickEvictOperation) finishRevert(tx *bolt.Tx) error {
	old, err := beo.current(beo.db)
	if err != nil {
		return err
	}
	newBrick, err := NewBrickEntryFromId(tx, beo.newBrickEntryId)
	if err != nil {
		return err
	}
	// remove new brick (return space + update links)
	err = newBrick.removeAndFree(
		tx, old.volume, beo.reclaimed[newBrick.Info.DeviceId])
	if err != nil {
		return err
	}
	// the old brick has not been replaced. remove pending state
	beo.op.FinalizeBrick(old.brick)
	if e := old.brick.Save(tx); e != nil {
		return e
	}
	// save volume
	if e := old.volume.Save(tx); e != nil {
		return e
	}
	// remove pending markers
	return beo.op.Delete(tx)
}

type brickEvictStatus struct {
	oldBrickId string
	newBrickId string
}

func evictStatus(op *PendingOperationEntry) (brickEvictStatus, error) {
	bes := brickEvictStatus{}
	for _, action := range op.Actions {
		switch action.Change {
		case OpDeleteBrick:
			logger.Info("found brick to be replaced: %v", action.Id)
			bes.oldBrickId = action.Id
		case OpAddBrick:
			logger.Info("found replacement brick: %v", action.Id)
			bes.newBrickId = action.Id
		default:
			logger.Info("found invalid action: %v, %v",
				action.Change, action.Id)
			return bes, fmt.Errorf(
				"Malformed pending operation entry for brick evict operation")
		}
	}
	return bes, nil
}
