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

	"github.com/heketi/heketi/v10/executors"
	wdb "github.com/heketi/heketi/v10/pkg/db"
	"github.com/heketi/heketi/v10/pkg/glusterfs/api"

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

	healCheck api.HealInfoCheck

	// we need state gathered by clean in clean-done in the child operation
	// we can't always pull the op from the db or we lose this state.
	// This field is used to retain that state between calls.
	currentChild *BrickEvictOperation
}

func NewDeviceRemoveOperation(
	deviceId string, db wdb.DB, h api.HealInfoCheck) *DeviceRemoveOperation {

	return &DeviceRemoveOperation{
		OperationManager: OperationManager{
			db: db,
			op: NewPendingOperationEntry(NEW_ID),
		},
		DeviceId:  deviceId,
		healCheck: h,
	}
}

// loadDeviceRemoveOperation returns a DeviceRemoveOperation populated
// from an existing pending operation entry in the db.
func loadDeviceRemoveOperation(
	db wdb.DB, p *PendingOperationEntry) (*DeviceRemoveOperation, error) {

	var deviceId string
	for _, action := range p.Actions {
		if action.Change == OpRemoveDevice {
			deviceId = action.Id
		}
	}
	if deviceId == "" {
		return nil, fmt.Errorf(
			"Missing device to remove in device-remove operation")
	}

	return &DeviceRemoveOperation{
		OperationManager: OperationManager{
			db: db,
			op: p,
		},
		DeviceId: deviceId,
	}, nil
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

	return dro.migrateBricks(executor, d)
}

func (dro *DeviceRemoveOperation) migrateBricks(
	executor executors.Executor, d *DeviceEntry) error {

	toEvict, err := d.removeableBricks(dro.db)
	if err != nil {
		return err
	}
	for _, brickId := range toEvict {
		nestedOp := newRemoveBrickComboOperation(
			dro,
			NewBrickEvictOperation(brickId, dro.db, dro.healCheck))
		err = RunOperation(nestedOp, executor)
		if err != nil {
			return err
		}
	}
	return nil
}

func (dro *DeviceRemoveOperation) updateChildOperation(
	db wdb.DB, childOp *PendingOperationEntry) error {

	return db.Update(func(tx *bolt.Tx) error {
		var err error
		dro.op, err = NewPendingOperationEntryFromId(tx, dro.op.Id)
		if err != nil {
			return err
		}
		dro.op.RecordChild(childOp)
		// RecordChild alters both parent and child so save them both
		if err := childOp.Save(tx); err != nil {
			return err
		}
		if err := dro.op.Save(tx); err != nil {
			return err
		}
		return nil
	})
}

func (dro *DeviceRemoveOperation) clearChildOperation(db wdb.DB) error {
	return db.Update(func(tx *bolt.Tx) error {
		var err error
		dro.op, err = NewPendingOperationEntryFromId(tx, dro.op.Id)
		if err != nil {
			return err
		}
		dro.op.ClearChild()
		return dro.op.Save(tx)
	})
}

func (dro *DeviceRemoveOperation) Rollback(executor executors.Executor) error {
	return rollbackViaClean(dro, executor)
}

func (dro *DeviceRemoveOperation) loadOpAndChild(
	tx *bolt.Tx) (*BrickEvictOperation, error) {

	var err error
	dro.op, err = NewPendingOperationEntryFromId(tx, dro.op.Id)
	if err != nil {
		return nil, err
	}
	childId := dro.op.ChildId()
	if childId == "" {
		// no child op present in db. either the system was stopped
		// between child-op updates or this is an old device-remove
		// from a previous version. Either way there's nothing to do.
		return nil, nil
	}
	childOp, err := NewPendingOperationEntryFromId(tx, childId)
	if err != nil {
		return nil, err
	}
	op, err := LoadOperation(wdb.WrapTx(tx), childOp)
	if err != nil {
		return nil, err
	}
	if bop, ok := op.(*BrickEvictOperation); ok {
		return bop, nil
	}
	return nil, fmt.Errorf("unexpected child operation %v", childOp.Id)
}

func (dro *DeviceRemoveOperation) Clean(executor executors.Executor) error {
	var brickEvictOp *BrickEvictOperation
	err := dro.db.View(func(tx *bolt.Tx) error {
		bop, err := dro.loadOpAndChild(tx)
		brickEvictOp = bop
		return err
	})
	if err != nil {
		return err
	}
	if brickEvictOp != nil {
		logger.Info("need to clean child [%s] of %s [%s]",
			brickEvictOp.Id(), dro.Label(), dro.Id())
		// annoyingly, we need to change the brick evict operations db
		// to our db because it was created in the View txn above.
		// only to need to swap it around later :-(
		brickEvictOp.db = dro.db
		dro.currentChild = brickEvictOp
		nestedOp := newRemoveBrickComboOperation(dro, dro.currentChild)
		return nestedOp.Clean(executor)
	}
	return nil
}

func (dro *DeviceRemoveOperation) CleanDone() error {
	err := dro.db.View(func(tx *bolt.Tx) error {
		bop, err := dro.loadOpAndChild(tx)
		if bop == nil && dro.currentChild != nil {
			return fmt.Errorf("db has no child op. operation has child!")
		} else if bop != nil && dro.currentChild == nil {
			return fmt.Errorf("db has child op. operation has no child!")
		}
		return err
	})
	if err != nil {
		return err
	}
	if dro.currentChild != nil {
		logger.Info("need to finish clean child [%s] of %s [%s]",
			dro.currentChild.Id(), dro.Label(), dro.Id())
		nestedOp := newRemoveBrickComboOperation(dro, dro.currentChild)
		if err := nestedOp.CleanDone(); err != nil {
			return err
		}
	}
	return dro.db.Update(func(tx *bolt.Tx) error {
		var err error
		dro.op, err = NewPendingOperationEntryFromId(tx, dro.op.Id)
		if err != nil {
			return err
		}
		if dro.op.IsParent() {
			// child should have already been cleaned
			return fmt.Errorf("device remove is still parent in clean done")
		}
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

	healCheck api.HealInfoCheck

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
	brickId string, db wdb.DB, h api.HealInfoCheck) *BrickEvictOperation {

	return &BrickEvictOperation{
		OperationManager: OperationManager{
			db: db,
			op: NewPendingOperationEntry(NEW_ID),
		},
		BrickId:   brickId,
		healCheck: h,
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
		logger.Debug(
			"brick evict wants to replace [%s] on [%s] with [%s] on [%s]",
			old.brick.Id(), old.device.Id(),
			newBrickEntry.Id(), newDeviceEntry.Id())
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

	node, err := getWorkingNode(old.node, beo.db, executor)
	if err != nil {
		return err
	}

	bs, index, err := old.volume.getBrickSetForBrickId(
		beo.db, executor, old.brick.Info.Id, node)
	if err != nil {
		return err
	}

	if beo.healCheck != api.HealCheckDisable {
		err = old.volume.canReplaceBrickInBrickSet(beo.db, executor, node, bs, index)
		if err != nil {
			return err
		}
	} else {
		logger.Info("Skipping heal info check for volume %+v", old.volume)
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

	node, err := getWorkingNode(newBrickNodeEntry, beo.db, executor)
	if err != nil {
		return err
	}

	err = executor.VolumeReplaceBrick(
		node, old.volume.Info.Name, &oldBrick, &newBrick)
	if err != nil {
		return err
	}

	beo.reclaimed, err = tryDestroyBrickMap(old.bhmap(), executor)
	return err
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
	node, err := getWorkingNode(old.node, beo.db, executor)
	if err != nil {
		return nil
	}

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
		beo.reclaimed, err = tryDestroyBrickMap(old.bhmap(), executor)
		if err != nil {
			return logger.LogError("Error destroying old brick: %v", err)
		}
	case replaceIncomplete:
		logger.Info("Destroying unused new brick contents")
		beo.reclaimed, err = tryDestroyBrickMap(newBHMap, executor)
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
		if e := beo.refreshOp(wdb.WrapTx(tx)); e != nil {
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
	logger.Debug("finishing operation: no changes")
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
	logger.Debug(
		"finishing operation: accepting new brick [%v]",
		beo.newBrickEntryId)
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
	logger.Debug(
		"finishing operation: reverting new brick [%s]",
		beo.newBrickEntryId)
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
		case OpParentOperation:
			logger.Info("this is a child op of: %v", action.Id)
		default:
			logger.Info("found invalid action: %v, %v",
				action.Change, action.Id)
			return bes, fmt.Errorf(
				"Malformed pending operation entry for brick evict operation")
		}
	}
	return bes, nil
}

// removeBrickComboOperation are ephemeral operations that combine
// db changes for the parent operation (device remove) and child
// (brick evict) such that certain changes to both are made within
// a single db transaction but we can still re-use as much of the
// existing functions from the child.
type removeBrickComboOperation struct {
	noRetriesOperation

	deviceRemoveOp *DeviceRemoveOperation
	brickEvictOp   *BrickEvictOperation
}

func newRemoveBrickComboOperation(dro *DeviceRemoveOperation, beo *BrickEvictOperation) *removeBrickComboOperation {

	return &removeBrickComboOperation{
		deviceRemoveOp: dro,
		brickEvictOp:   beo,
	}
}

func (bco *removeBrickComboOperation) Id() string {
	return bco.deviceRemoveOp.Id()
}

func (bco *removeBrickComboOperation) Label() string {
	return "Remove Brick from Device"
}

func (bco *removeBrickComboOperation) ResourceUrl() string {
	return ""
}

func (bco *removeBrickComboOperation) childPushDB(db wdb.DB) {
	// tad bit hacky
	bco.brickEvictOp.db = db
}

func (bco *removeBrickComboOperation) childPopDB() {
	bco.brickEvictOp.db = bco.deviceRemoveOp.db
}

func (bco *removeBrickComboOperation) Build() error {
	beo := bco.brickEvictOp
	dro := bco.deviceRemoveOp
	return dro.db.Update(func(tx *bolt.Tx) error {
		txdb := wdb.WrapTx(tx)
		bco.childPushDB(txdb)
		defer bco.childPopDB()
		if err := beo.Build(); err != nil {
			return fmt.Errorf(
				"failed to construct brick-evict for device remove (%v): %v",
				dro.op.Id,
				err)
		}
		if err := dro.updateChildOperation(txdb, beo.op); err != nil {
			return fmt.Errorf(
				"failed to add brick-evict as child op for device remove (%v): %v",
				dro.op.Id,
				err)
		}
		return nil
	})
}

func (bco *removeBrickComboOperation) Exec(executor executors.Executor) error {
	return bco.brickEvictOp.Exec(executor)
}

func (bco *removeBrickComboOperation) Clean(executor executors.Executor) error {
	return bco.brickEvictOp.Clean(executor)
}

func (bco *removeBrickComboOperation) Rollback(executor executors.Executor) error {
	return rollbackViaClean(bco, executor)
}

func (bco *removeBrickComboOperation) CleanDone() error {
	beo := bco.brickEvictOp
	dro := bco.deviceRemoveOp
	return dro.db.Update(func(tx *bolt.Tx) error {
		txdb := wdb.WrapTx(tx)
		bco.childPushDB(txdb)
		defer bco.childPopDB()
		if err := beo.CleanDone(); err != nil {
			return fmt.Errorf(
				"failed to complete clean of child op (%v): %v",
				beo.op.Id,
				err)
		}
		if err := dro.clearChildOperation(txdb); err != nil {
			return fmt.Errorf(
				"failed to clear child op [%v] from pending op [%v]: %v",
				beo.op.Id,
				dro.op.Id,
				err)
		}
		return nil
	})
}

func (bco *removeBrickComboOperation) Finalize() error {
	beo := bco.brickEvictOp
	dro := bco.deviceRemoveOp
	return dro.db.Update(func(tx *bolt.Tx) error {
		txdb := wdb.WrapTx(tx)
		bco.childPushDB(txdb)
		defer bco.childPopDB()
		if err := beo.Finalize(); err != nil {
			return fmt.Errorf(
				"failed to finalize child op (%v): %v",
				beo.op.Id,
				err)
		}
		if err := dro.clearChildOperation(txdb); err != nil {
			return fmt.Errorf(
				"failed to clear child op [%v] from pending op [%v]: %v",
				beo.op.Id,
				dro.op.Id,
				err)
		}
		return nil
	})
}

func getWorkingNode(n *NodeEntry, db wdb.RODB, executor executors.Executor) (string, error) {
	node := n.ManageHostName()
	err := executor.GlusterdCheck(node)
	if err != nil {
		node, err = GetVerifiedManageHostname(db, executor, n.Info.NodeAddRequest.ClusterId)
		if err != nil {
			return "", err
		}
	}
	return node, err
}

func tryDestroyBrickMap(bhmap brickHostMap, executor executors.Executor) (ReclaimMap, error) {
	reclaimed, err := bhmap.destroy(executor)
	if err == nil {
		return reclaimed, nil
	}

	// If the destroy failed because the node is not running (ideally,
	// permanently down) we want to continue with the operation . But if the
	// node works we failed for a "real" reason we don't want to blindly ignore
	// errors. So we do a probe to see if the host is responsive. If the
	// destroy has failed we will use the node's responsiveness to decide if
	// this should be treated as a failure or not.  It is bit hacky but should
	// work around the lack of proper error handling in most common cases.
	for b, node := range bhmap {
		destroyErr := logger.LogError("Error destroying brick [%v]: %v", b, err)
		err := executor.GlusterdCheck(node)
		if err == nil {
			logger.Warning("node [%v] responds to probe: failing operation", node)
			return reclaimed, destroyErr
		}
		logger.Warning("node [%v] does not respond to probe: ignoring error destroying bricks", node)
	}
	return reclaimed, nil
}
