//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"errors"
	"fmt"

	ctxErrors "github.com/pkg/errors"
)

var (
	ErrNoSpace          = errors.New("No space")
	ErrFound            = errors.New("Id already exists")
	ErrNotFound         = errors.New("Id not found")
	ErrConflict         = errors.New("The target exists, contains other items, or is in use.")
	ErrMaxBricks        = errors.New("Maximum number of bricks reached.")
	ErrMinimumBrickSize = errors.New("Minimum brick size limit reached.  Out of space.")
	ErrDbAccess         = errors.New("Unable to access db")
	ErrAccessList       = errors.New("Unable to access list")
	ErrKeyExists        = errors.New("Key already exists in the database")
	ErrNoReplacement    = errors.New("No Replacement was found for resource requested to be removed")
	ErrCloneBlockVol    = errors.New("Cloning of block hosting volumes is not supported")

	// well known errors for cluster device source
	ErrEmptyCluster = errors.New("No nodes in cluster")
	ErrNoStorage    = errors.New("No online storage devices in cluster")

	// returned by code related to operations load
	ErrTooManyOperations = errors.New("Server handling too many operations")
)

// IsRetry returns true if the error-generating operation should be retried.
func IsRetry(err error) bool {
	err = ctxErrors.Cause(err)
	te, ok := err.(interface {
		Retry() bool
	})
	return ok && te.Retry()
}

// Original returns a nested error if present or nil.
func Original(err error) error {
	err = ctxErrors.Cause(err)
	if ne, ok := err.(interface {
		Original() error
	}); ok {
		return ne.Original()
	}
	return nil
}

type retryError struct {
	originalError error
}

// NewRetryError wraps err in a retryError
func NewRetryError(err error) error {
	return retryError{originalError: err}
}

func (ore retryError) Error() string {
	return fmt.Sprintf("Operation Should Be Retried; Error: %v",
		ore.originalError.Error())
}

func (ore retryError) Original() error {
	return ore.originalError
}

func (ore retryError) Retry() bool {
	return true
}
