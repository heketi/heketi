//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

type PlacerEntryCommon interface {
	// Id returns a unique identifier for this item
	Id() string
}

type PlacerNode interface {
	PlacerEntryCommon
	// Zone returns an int representing a logical zone for the
	// node. This is used as a hint about the failure domain the
	// node resides in.
	Zone() int
}

type PlacerDevice interface {
	PlacerEntryCommon
	// ParentNodeId returns the unique id for the node that
	// hosts this device.
	ParentNodeId() string
	// NewBrick produces a new brick stub hosted on the current
	// device.
	NewBrick(uint64, float64, int64, string) PlacerBrick
	// BrickAdd records the given brick ID as a component of
	// the current device.
	BrickAdd(string)
}

type PlacerBrick interface {
	PlacerEntryCommon
	// DeviceId returns the unique id of the device that hosts
	// the current brick.
	DeviceId() string
	// NodeId returns the unique id of the node that hosts the
	// current brick.
	NodeId() string
	// SetId updates the unique id of the brick with the given string.
	SetId(string)
	// Valid returns true if the brick object this interface represents
	// is usable.
	Valid() bool
}
