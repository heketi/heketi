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
	"bytes"
	"encoding/gob"
	"fmt"
	"time"

	"github.com/boltdb/bolt"
	wdb "github.com/heketi/heketi/v10/pkg/db"
	"github.com/heketi/heketi/v10/pkg/glusterfs/api"
	"github.com/heketi/heketi/v10/pkg/idgen"
	"github.com/lpabon/godbc"
)

type OperationStatus string

const (
	NEW_ID                    = ""
	BOLTDB_BUCKET_PENDING_OPS = "PENDING_OPERATIONS"
	DB_HAS_PENDING_OPS_BUCKET = "DB_HAS_PENDING_OPS_BUCKET"
)

// define constants for OperationStatus
const (
	NewOperation    OperationStatus = ""
	StaleOperation  OperationStatus = "stale"
	FailedOperation OperationStatus = "failed"
)

var (
	// support unit test dep. injection for custom timestamps
	operationTimestamp = func() int64 { return time.Now().Unix() }
)

// PendingOperationEntry tracks pending operations within the Heketi db.
type PendingOperationEntry struct {
	PendingOperation

	// tracking the status of operations
	Status OperationStatus
}

// PendingOperationList returns the IDs of all pending operation entries
// currently in the Heketi db.
func PendingOperationList(tx *bolt.Tx) ([]string, error) {
	list := EntryKeys(tx, BOLTDB_BUCKET_PENDING_OPS)
	if list == nil {
		return nil, ErrAccessList
	}
	return list, nil
}

// HasPendingOperations returns true if the db contains one or more pending
// operation entries. If the db cannot be read the function panics.
func HasPendingOperations(db wdb.RODB) bool {
	var pending bool
	if err := db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		if err != nil {
			return err
		}
		pending = (len(l) > 0)
		return nil
	}); err != nil {
		panic(err)
	}
	return pending
}

// BucketName returns the name of the db bucket for pending operation entries.
func (p *PendingOperationEntry) BucketName() string {
	return BOLTDB_BUCKET_PENDING_OPS
}

// NewPendingOperationEntry returns a newly constructed pending operation entry.
// If id is a non-zero-length string then that value will be used as the ID of
// the object. Otherwise pass NEW_ID to have a new uuid be automatically allocated
// for the new object.
func NewPendingOperationEntry(id string) *PendingOperationEntry {
	if id == NEW_ID {
		id = idgen.GenUUID()
	}
	entry := &PendingOperationEntry{
		PendingOperation: PendingOperation{
			PendingItem: PendingItem{id},
			Timestamp:   operationTimestamp(),
			Actions:     []PendingOperationAction{},
		},
	}
	return entry
}

// NewPendingOperationEntryFromId fetches an existing pending operation entry
// from the heketi db based on the provided id.
func NewPendingOperationEntryFromId(tx *bolt.Tx, id string) (
	*PendingOperationEntry, error) {
	godbc.Require(tx != nil)
	godbc.Require(id != "")

	entry := &PendingOperationEntry{}
	err := EntryLoad(tx, entry, id)
	if err != nil {
		return nil, err
	}

	if entry.Actions == nil {
		entry.Actions = []PendingOperationAction{}
	}

	return entry, nil
}

// Save records the pending operation entry object in the db, keyed by the
// value of its ID.
func (p *PendingOperationEntry) Save(tx *bolt.Tx) error {
	godbc.Require(tx != nil)
	godbc.Require(p.Id != "")
	godbc.Require(p.Type != OperationUnknown)

	return EntrySave(tx, p, p.Id)
}

// Delete removes a pending operation entry from the db.
func (p *PendingOperationEntry) Delete(tx *bolt.Tx) error {
	p.Reset()
	return EntryDelete(tx, p, p.Id)
}

// Reset clears the all of PendingOperationEntry's state except for
// the ID so that it may be reused.
func (p *PendingOperationEntry) Reset() {
	p.Type = OperationUnknown
	p.Actions = []PendingOperationAction{}
}

// Marshal serializes the object for storage in the db.
func (p *PendingOperationEntry) Marshal() ([]byte, error) {
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)
	err := enc.Encode(*p)

	return buffer.Bytes(), err
}

// Unmarshal de-serializes the object from the db.
func (p *PendingOperationEntry) Unmarshal(buffer []byte) error {
	dec := gob.NewDecoder(bytes.NewReader(buffer))
	err := dec.Decode(p)
	if err != nil {
		return err
	}

	return nil
}

// recordChange is a helper function to reduce some of the boilerplate around
// adding a new change action item to the pending operation entry.
func (p *PendingOperationEntry) recordChange(c PendingChangeType, id string) {
	godbc.Require(p.Id != "")
	godbc.Require(id != "")
	p.Actions = append(p.Actions, PendingOperationAction{Change: c, Id: id})
}

// recordSizeChange is a helper function to reduce some of the boilerplate around
// adding a new change action item that includes a size change to the entry.
func (p *PendingOperationEntry) recordSizeChange(c PendingChangeType,
	id string,
	sizeDelta int) {

	godbc.Require(p.Id != "")
	godbc.Require(id != "")
	p.Actions = append(p.Actions,
		PendingOperationAction{
			Change: c,
			Id:     id,
			Delta:  sizeDelta,
		})
}

// RecordAddVolume adds tracking metadata for a new volume to the
// PendingOperationEntry and VolumeEntry.
func (p *PendingOperationEntry) RecordAddVolume(v *VolumeEntry) {
	// track which volume this op is created
	p.recordChange(OpAddVolume, v.Info.Id)
	p.Type = OperationCreateVolume
	// link back from "temporary" object to op
	v.Pending.Id = p.Id
}

// FinalizeVolume removes tracking metadata from the volume entry.
// This means that the volume is no longer pending.
func (p *PendingOperationEntry) FinalizeVolume(v *VolumeEntry) {
	v.Pending.Id = ""
	return
}

// RecordAddVolume adds tracking metadata for a new brick to the
// PendingOperationEntry and BrickEntry.
func (p *PendingOperationEntry) RecordAddBrick(b *BrickEntry) {
	p.recordChange(OpAddBrick, b.Info.Id)
	// link back from the temporary object to the op
	b.Pending.Id = p.Id
}

// RecordDeleteBrick adds tracking metadata for a to-be-deleted brick
// to the PendingOperationEntry and BrickEntry.
func (p *PendingOperationEntry) RecordDeleteBrick(b *BrickEntry) {
	p.recordChange(OpDeleteBrick, b.Info.Id)
	b.Pending.Id = p.Id
}

// FinalizeVolume removes tracking metadata from the brick entry.
// This means that the brick is no longer pending.
func (p *PendingOperationEntry) FinalizeBrick(b *BrickEntry) {
	b.Pending.Id = ""
	return
}

// RecordExpandVolume adds tracking metadata for a volume that is being
// expanded to the PendingOperationEntry and VolumeEntry.
func (p *PendingOperationEntry) RecordExpandVolume(v *VolumeEntry, sizeGB int) {
	p.recordSizeChange(OpExpandVolume, v.Info.Id, sizeGB)
	p.Type = OperationExpandVolume
}

// RecordDeleteVolume adds tracking metadata for a to-be-deleted volume
// to the PendingOperationEntry and BrickEntry.
func (p *PendingOperationEntry) RecordDeleteVolume(v *VolumeEntry) {
	p.recordChange(OpDeleteVolume, v.Info.Id)
	p.Type = OperationDeleteVolume
	v.Pending.Id = p.Id
}

func (p *PendingOperationEntry) RecordCloneVolume(v *VolumeEntry) {
	p.recordChange(OpCloneVolume, v.Info.Id)
	p.Type = OperationCloneVolume
	v.Pending.Id = p.Id
}

func (p *PendingOperationEntry) RecordAddVolumeClone(v *VolumeEntry) {
	p.recordChange(OpAddVolumeClone, v.Info.Id)
	v.Pending.Id = p.Id
}

func (p *PendingOperationEntry) FinalizeVolumeClone(v *VolumeEntry) {
	v.Pending.Id = ""
	return
}

// RecordAddHostingVolume adds tracking metadata for a file volume that hosts
// a block volume
func (p *PendingOperationEntry) RecordAddHostingVolume(v *VolumeEntry) {
	p.recordChange(OpAddVolume, v.Info.Id)
	v.Pending.Id = p.Id
}

// RecordAddBlockVolume adds tracking metadata for a new block volume.
func (p *PendingOperationEntry) RecordAddBlockVolume(bv *BlockVolumeEntry) {
	p.recordChange(OpAddBlockVolume, bv.Info.Id)
	p.Type = OperationCreateBlockVolume
	bv.Pending.Id = p.Id
}

// FinalizeBlockVolume removes tracking metadata from a block volume entry.
func (p *PendingOperationEntry) FinalizeBlockVolume(bv *BlockVolumeEntry) {
	bv.Pending.Id = ""
}

// RecordDeleteBlockVolume adds tracking metadata for a to-be-deleted
// block volume.
func (p *PendingOperationEntry) RecordDeleteBlockVolume(bv *BlockVolumeEntry) {
	p.recordChange(OpDeleteBlockVolume, bv.Info.Id)
	p.Type = OperationDeleteBlockVolume
	bv.Pending.Id = p.Id
}

// RecordExpandBlockVolume adds tracking metadata for a block volume that is being
// expanded to the PendingOperationEntry and BlockVolumeEntry.
func (p *PendingOperationEntry) RecordExpandBlockVolume(bv *BlockVolumeEntry, newSizeGB int) {
	p.recordSizeChange(OpExpandBlockVolume, bv.Info.Id, newSizeGB)
	p.Type = OperationExpandBlockVolume
}

// RecordRemoveDevice adds tracking metadata for a long-running device
// removal operation.
func (p *PendingOperationEntry) RecordRemoveDevice(d *DeviceEntry) {
	p.recordChange(OpRemoveDevice, d.Info.Id)
	p.Type = OperationRemoveDevice
}

// RecordChild adds or replaces a child operation for the current
// pending operation entry. Both child and parent can only have
// one parent/child relationship. Both are updated.
func (p *PendingOperationEntry) RecordChild(child *PendingOperationEntry) {
	// track childs id on parent
	parentsChild := findChange(p.Actions, OpChildOperation)
	if parentsChild < 0 {
		a := PendingOperationAction{Change: OpChildOperation, Id: child.Id}
		p.Actions = append(p.Actions, a)
	} else {
		p.Actions[parentsChild].Id = child.Id
	}
	// track parent's id on child
	childsParent := findChange(child.Actions, OpParentOperation)
	if childsParent < 0 {
		a := PendingOperationAction{Change: OpParentOperation, Id: p.Id}
		child.Actions = append(child.Actions, a)
	} else {
		child.Actions[childsParent].Id = p.Id
	}
}

// ClearChild removes any child operations from the parent.
func (p *PendingOperationEntry) ClearChild() {
	newActions := []PendingOperationAction{}
	for _, action := range p.Actions {
		if action.Change != OpChildOperation {
			newActions = append(newActions, action)
		}
	}
	p.Actions = newActions
}

// IsParent returns true if this pending operation entry is the parent
// of another operation.
func (p *PendingOperationEntry) IsParent() bool {
	return findChange(p.Actions, OpChildOperation) > 0
}

// ChildId returns the id of a child operation connected to this parent
// if one exists. Returns empty string otherwise.
func (p *PendingOperationEntry) ChildId() string {
	i := findChange(p.Actions, OpChildOperation)
	if i < 0 {
		return ""
	}
	return p.Actions[i].Id
}

// IsChild returns true if this pending operation entry is the child
// of another operation.
func (p *PendingOperationEntry) IsChild() bool {
	return findChange(p.Actions, OpParentOperation) > 0
}

func (p *PendingOperationEntry) ToInfo() api.PendingOperationInfo {
	return api.PendingOperationInfo{
		Id:       p.Id,
		TypeName: p.Type.Name(),
		Status:   string(p.Status),
		// label and substatus must be filled in later
	}
}

// PendingOperationUpgrade updates the heketi db with metadata needed to
// support pending operation entries.
func PendingOperationUpgrade(tx *bolt.Tx) error {
	entry, err := NewDbAttributeEntryFromKey(tx, DB_HAS_PENDING_OPS_BUCKET)
	switch err {
	case ErrNotFound:
		entry = NewDbAttributeEntry()
		entry.Key = DB_HAS_PENDING_OPS_BUCKET
		entry.Value = "yes"
	case nil:
		entry.Value = "yes"
	default:
		return err
	}

	// there are no data changes related to enabling Pending Ops in the db
	// so simply save the entry to record that this db now has them
	return entry.Save(tx)
}

// MarkPendingOperationsStale iterates through all the pending operations
// in the DB and ensures they are marked as stale operations.
func MarkPendingOperationsStale(tx *bolt.Tx) error {
	pops, err := PendingOperationList(tx)
	if err != nil {
		return err
	}
	for _, id := range pops {
		pop, err := NewPendingOperationEntryFromId(tx, id)
		if err != nil {
			return err
		}
		// don't bother updating ops that are already stale
		if pop.Status != StaleOperation {
			pop.Status = StaleOperation
			pop.Save(tx)
		}
	}
	return nil
}

// PendingOperationStateCount returns a mapping of pending operation
// statuses to the count of the operations of that status in the db.
func PendingOperationStateCount(tx *bolt.Tx) (map[OperationStatus]int, error) {
	pops, err := PendingOperationList(tx)
	if err != nil {
		return nil, err
	}
	count := map[OperationStatus]int{}
	for _, id := range pops {
		pop, err := NewPendingOperationEntryFromId(tx, id)
		if err != nil {
			return nil, err
		}
		count[pop.Status] += 1
	}
	return count, nil
}

// PendingOperationEntrySelection returns all pending operation entries in
// the database that match the selection function `sel`.
func PendingOperationEntrySelection(
	tx *bolt.Tx,
	sel func(*PendingOperationEntry) bool) ([]*PendingOperationEntry, error) {

	selection := []*PendingOperationEntry{}
	pops, err := PendingOperationList(tx)
	if err != nil {
		return nil, err
	}
	for _, id := range pops {
		pop, err := NewPendingOperationEntryFromId(tx, id)
		if err != nil {
			return nil, err
		}
		if sel(pop) {
			selection = append(selection, pop)
		}
	}
	return selection, nil
}

func (p *PendingOperationEntry) consistencyCheck(db Db) (response DbEntryCheckResponse) {

	for _, action := range p.Actions {
		switch action.Change {
		case OpAddBrick, OpDeleteBrick:
			if p.Id != db.Bricks[action.Id].Pending.Id {
				response.Inconsistencies = append(response.Inconsistencies, fmt.Sprintf("pending op %v id in change missing %v not found in bricks", p.Id, action.Id))
			}
		case OpAddVolume, OpDeleteVolume, OpCloneVolume, OpSnapshotVolume, OpAddVolumeClone:
			if p.Id != db.Volumes[action.Id].Pending.Id {
				response.Inconsistencies = append(response.Inconsistencies, fmt.Sprintf("pending op %v id in change missing %v not found in volumes", p.Id, action.Id))
			}
		case OpAddBlockVolume, OpDeleteBlockVolume:
			if p.Id != db.BlockVolumes[action.Id].Pending.Id {
				response.Inconsistencies = append(response.Inconsistencies, fmt.Sprintf("pending op %v id in change missing %v not found in blockvolumes", p.Id, action.Id))
			}
		case OpExpandVolume:
			if _, found := db.Volumes[action.Id]; !found {
				response.Inconsistencies = append(response.Inconsistencies,
					fmt.Sprintf("pending op %v: change id missing %v not found in volumes", p.Id, action.Id))
			}
		case OpExpandBlockVolume:
			if _, found := db.BlockVolumes[action.Id]; !found {
				response.Inconsistencies = append(response.Inconsistencies,
					fmt.Sprintf("pending op %v: change id missing %v not found in blockvolumes", p.Id, action.Id))
			}
		case OpRemoveDevice:
			// This is a noop
		default:
			response.Inconsistencies = append(response.Inconsistencies, fmt.Sprintf("Pending Op %v unexpected change type %v", p.Id, action.Change))
		}
	}
	return
}

func findChange(actions []PendingOperationAction, c PendingChangeType) int {
	for i, action := range actions {
		if action.Change == c {
			return i
		}
	}
	return -1
}
