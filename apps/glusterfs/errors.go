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
	"github.com/heketi/heketi/pkg/errtag"
)

var (
	ErrNoSpace          = errtag.NewTag("No space")
	ErrFound            = errtag.NewTag("Id already exists")
	ErrNotFound         = errtag.NewTag("Id not found")
	ErrConflict         = errtag.NewTag("The target exists, contains other items, or is in use.")
	ErrMaxBricks        = errtag.NewTag("Maximum number of bricks reached.")
	ErrMinimumBrickSize = errtag.NewTag("Minimum brick size limit reached.  Out of space.")
	ErrDbAccess         = errtag.NewTag("Unable to access db")
	ErrAccessList       = errtag.NewTag("Unable to access list")
	ErrKeyExists        = errtag.NewTag("Key already exists in the database")
	ErrNoReplacement    = errtag.NewTag("No Replacement was found for resource requested to be removed")
	ErrCloneBlockVol    = errtag.NewTag("Cloning of block hosting volumes is not supported")

	// well known errors for cluster device source
	ErrEmptyCluster = errtag.NewTag("No nodes in cluster")
	ErrNoStorage    = errtag.NewTag("No online storage devices in cluster")

	// returned by code related to operations load
	ErrTooManyOperations = errtag.NewTag("Server handling too many operations")
)
