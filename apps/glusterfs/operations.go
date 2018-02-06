package glusterfs

import (
	"fmt"
	"net/http"

	"github.com/heketi/heketi/executors"
	wdb "github.com/heketi/heketi/pkg/db"

	"github.com/boltdb/bolt"
)

type Operation interface {
	Label() string
	ResourceUrl() string
	Build(allocator Allocator) error
	Exec(executor executors.Executor) error
	Rollback(executor executors.Executor) error
	Finalize() error
}

type OperationManager struct {
	db wdb.DB
	op *PendingOperationEntry
}

func (om *OperationManager) Id() string {
	return om.op.Id
}

type VolumeCreateOperation struct {
	OperationManager
	vol *VolumeEntry
}

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

func bricksFromOp(db wdb.RODB,
	op *PendingOperationEntry, gid int64) ([]*BrickEntry, error) {

	brick_entries := []*BrickEntry{}
	err := db.View(func(tx *bolt.Tx) error {
		for _, a := range op.Actions {
			if a.Change == OpAddBrick {
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

func expandSizeFromOp(db wdb.RODB,
	op *PendingOperationEntry) (sizeGB int, e error) {
	err := db.View(func(tx *bolt.Tx) error {
		for _, a := range op.Actions {
			if a.Change == OpExpandVolume {
				sizeGB, e = a.ExpandSize()
				return nil
			}
		}
		e = fmt.Errorf("no OpExpandVolume action in pending op: %v",
			op.Id)
		return nil
	})
	if err != nil && e == nil {
		e = err
	}
	return
}

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
