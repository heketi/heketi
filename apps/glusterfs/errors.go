//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

type constErr string

func (e constErr) Error() string {
	return string(e)
}

const (
	ErrNoSpace          = constErr("No space")
	ErrFound            = constErr("Id already exists")
	ErrNotFound         = constErr("Id not found")
	ErrConflict         = constErr("The target exists, contains other items, or is in use.")
	ErrMaxBricks        = constErr("Maximum number of bricks reached.")
	ErrMinimumBrickSize = constErr("Minimum brick size limit reached.  Out of space.")
	ErrDbAccess         = constErr("Unable to access db")
	ErrAccessList       = constErr("Unable to access list")
	ErrKeyExists        = constErr("Key already exists in the database")
	ErrNoReplacement    = constErr("No Replacement was found for resource requested to be removed")
	ErrCloneBlockVol    = constErr("Cloning of block hosting volumes is not supported")

	// well known errors for cluster device source
	ErrEmptyCluster = constErr("No nodes in cluster")
	ErrNoStorage    = constErr("No online storage devices in cluster")

	// returned by code related to operations load
	ErrTooManyOperations = constErr("Server handling too many operations")
)
