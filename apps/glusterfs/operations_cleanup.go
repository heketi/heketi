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
	"github.com/heketi/heketi/executors"
	wdb "github.com/heketi/heketi/pkg/db"

	"github.com/boltdb/bolt"
)

type OperationCleaner struct {
	db       wdb.DB
	sel      func(*PendingOperationEntry) bool
	executor executors.Executor
}

func (oc OperationCleaner) Clean() error {
	logger.Debug("Going to clean up operations")
	var pops []*PendingOperationEntry
	err := oc.db.View(func(tx *bolt.Tx) error {
		var err error
		pops, err = PendingOperationEntrySelection(tx, oc.sel)
		return err
	})
	if err != nil {
		return err
	}

	for _, pop := range pops {
		logger.Info("Found operation %v in need of clean up", pop.Id)
		op, err := LoadOperation(oc.db, pop)
		if _, ok := err.(ErrNotLoadable); ok {
			logger.Err(err)
			continue
		} else if err != nil {
			return err
		}
		cop, ok := op.(CleanableOperation)
		if !ok {
			logger.Warning("%v operation %v not cleanable", op.Label(), pop.Id)
			continue
		}
		// TODO gather errors
		err = oc.cleanOp(cop)
		if err != nil {
			logger.Err(err)
		}
	}
	return nil
}

func (oc OperationCleaner) cleanOp(cop CleanableOperation) error {
	err := cop.Clean(oc.executor)
	if err != nil {
		return err
	}
	return cop.CleanDone()
}

func CleanAll(p *PendingOperationEntry) bool {
	return p.Status == StaleOperation || p.Status == FailedOperation
}
