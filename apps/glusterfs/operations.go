package glusterfs

import (
	wdb "github.com/heketi/heketi/pkg/db"
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

func (vc *VolumeCreateOperation) Build(allocator Allocator) error {
	// vc.vol.createVolumeComponents(vc.db, allocator)
	return nil
}

func (vc *VolumeCreateOperation) Exec(executor executors.Executor) error {
	// vc.vol.createVolumeExec(vc.db, executor)
	return nil
}

func (vc *VolumeCreateOperation) Finalize() error {
	return nil
}

func (vc *VolumeCreateOperation) Rollback(executor executors.Executor) error {
	// vc.vol.cleanupCreateVolume(vc.db, executors, brick_entries)
	return nil
}
