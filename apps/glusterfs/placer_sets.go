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

	"github.com/lpabon/godbc"
)

type BrickSet struct {
	SetSize int
	Bricks  []PlacerBrick
}

func NewBrickSet(s int) *BrickSet {
	return &BrickSet{SetSize: s, Bricks: []PlacerBrick{}}
}

func (bs *BrickSet) Add(b PlacerBrick) {
	godbc.Require(!bs.Full())
	bs.Bricks = append(bs.Bricks, b)
}

func (bs *BrickSet) Insert(index int, b PlacerBrick) {
	switch {
	case index >= bs.SetSize:
		panic(fmt.Errorf("Insert index (%v) out of bounds", index))
	case index == len(bs.Bricks):
		// we grow the bricks slice by one item
		bs.Bricks = append(bs.Bricks, b)
	case index < len(bs.Bricks):
		// we replace an existing item
		bs.Bricks[index] = b
	default:
		panic(fmt.Errorf(
			"Brick set may only be extended one (index=%v, len=%v)",
			index, len(bs.Bricks)))
	}
}

func (bs *BrickSet) Full() bool {
	return len(bs.Bricks) == bs.SetSize
}

// Drop returns a new brick set with the brick at the given
// index removed. Does not preserve brick positioning and
// is not suitable for position dependent allocations.
func (bs *BrickSet) Drop(index int) *BrickSet {
	bs2 := NewBrickSet(bs.SetSize)
	bs2.Bricks = append(bs.Bricks[:index], bs.Bricks[index+1:]...)
	return bs2
}

func (bs *BrickSet) String() string {
	ids := []string{}
	for _, b := range bs.Bricks {
		ids = append(ids, b.Id())
	}
	return fmt.Sprintf("BrickSet(%v)%v", bs.SetSize, ids)
}

type DeviceSet struct {
	SetSize int
	Devices []PlacerDevice
}

func NewDeviceSet(s int) *DeviceSet {
	return &DeviceSet{SetSize: s, Devices: []PlacerDevice{}}
}

func (ds *DeviceSet) Add(d PlacerDevice) {
	godbc.Require(!ds.Full())
	ds.Devices = append(ds.Devices, d)
}

func (ds *DeviceSet) Insert(index int, d PlacerDevice) {
	switch {
	case index >= ds.SetSize:
		panic(fmt.Errorf("Insert index (%v) out of bounds", index))
	case index == len(ds.Devices):
		// we grow the bricks slice by one item
		ds.Devices = append(ds.Devices, d)
	case index < len(ds.Devices):
		// we replace an existing item
		ds.Devices[index] = d
	default:
		panic(fmt.Errorf(
			"Brick set may only be extended one (index=%v, len=%v)",
			index, len(ds.Devices)))
	}
}

func (ds *DeviceSet) Full() bool {
	return len(ds.Devices) == ds.SetSize
}

type BrickAllocation struct {
	BrickSets  []*BrickSet
	DeviceSets []*DeviceSet
}
