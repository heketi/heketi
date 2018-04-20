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

	"github.com/boltdb/bolt"
	"github.com/lpabon/godbc"

	wdb "github.com/heketi/heketi/pkg/db"
	"github.com/heketi/heketi/pkg/utils"
)

type deviceFetcher func(string) (*DeviceEntry, error)

func tryAllocateBrickOnDevice(
	opts PlacementOpts,
	pred DeviceFilter,
	device *DeviceEntry,
	bs *BrickSet) *BrickEntry {

	// Do not allow a device from the same node to be in the set
	deviceOk := true
	for _, brickInSet := range bs.Bricks {
		if brickInSet.Info.NodeId == device.NodeId {
			deviceOk = false
		}
	}

	if !deviceOk {
		return nil
	}
	if pred != nil && !pred(bs, device) {
		return nil
	}

	// Try to allocate a brick on this device
	brickSize, snapFactor := opts.BrickSizes()
	brick := device.NewBrickEntry(brickSize, snapFactor,
		opts.BrickGid(), opts.BrickOwner())
	if brick == nil {
		logger.Debug(
			"Unable to place a brick of size %v & factor %v on device %v",
			brickSize, snapFactor, device.Info.Id)
	}
	return brick
}

func findDeviceAndBrickForSet(
	opts PlacementOpts,
	fetchDevice deviceFetcher,
	pred DeviceFilter,
	deviceCh <-chan string,
	bs *BrickSet) (*BrickEntry, *DeviceEntry, error) {

	// Check the ring for devices to place the brick
	for deviceId := range deviceCh {

		device, err := fetchDevice(deviceId)
		if err != nil {
			return nil, nil, err
		}

		brick := tryAllocateBrickOnDevice(opts, pred, device, bs)
		if brick == nil {
			continue
		}

		return brick, device, nil
	}

	// No devices found
	return nil, nil, ErrNoSpace
}

func populateBrickSet(
	opts PlacementOpts,
	fetchDevice deviceFetcher,
	pred DeviceFilter,
	deviceCh <-chan string,
	initId string) (*BrickSet, *DeviceSet, error) {

	ssize := opts.SetSize()
	bs := NewBrickSet(ssize)
	ds := NewDeviceSet(ssize)
	for i := 0; i < ssize; i++ {
		logger.Debug("%v / %v", i, ssize)

		brick, device, err := findDeviceAndBrickForSet(
			opts, fetchDevice, pred, deviceCh, bs)
		if err != nil {
			return bs, ds, err
		}

		// If the first in the set, then reset the id
		if i == 0 {
			brick.SetId(initId)
		}

		// Save the brick entry to create later
		bs.Add(brick)
		ds.Add(device)

		device.BrickAdd(brick.Id())
	}
	return bs, ds, nil
}

func allocateBricks(
	db wdb.RODB,
	cluster string,
	v *VolumeEntry,
	numBrickSets int,
	brick_size uint64) (*BrickAllocation, error) {

	var r *BrickAllocation
	opts := NewVolumePlacementOpts(v, brick_size, numBrickSets)
	err := db.View(func(tx *bolt.Tx) error {
		var err error
		dsrc := NewClusterDeviceSource(tx, cluster)
		placer := PlacerForVolume(v)
		r, err = placer.PlaceAll(dsrc, opts, nil)
		return err
	})
	return r, err
}

type VolumePlacementOpts struct {
	v            *VolumeEntry
	brickSize    uint64
	numBrickSets int
}

func NewVolumePlacementOpts(v *VolumeEntry,
	brickSize uint64, numBrickSets int) *VolumePlacementOpts {
	return &VolumePlacementOpts{v, brickSize, numBrickSets}
}

func (vp *VolumePlacementOpts) BrickSizes() (uint64, float64) {
	return vp.brickSize, float64(vp.v.Info.Snapshot.Factor)
}

func (vp *VolumePlacementOpts) BrickOwner() string {
	return vp.v.Info.Id
}

func (vp *VolumePlacementOpts) BrickGid() int64 {
	return vp.v.Info.Gid
}

func (vp *VolumePlacementOpts) SetSize() int {
	return vp.v.Durability.BricksInSet()
}

func (vp *VolumePlacementOpts) SetCount() int {
	return vp.numBrickSets
}

func (vp *VolumePlacementOpts) AverageFileSize() uint64 {
	return vp.v.GetAverageFileSize()
}

type StandardBrickPlacer struct{}

func NewStandardBrickPlacer() *StandardBrickPlacer {
	return &StandardBrickPlacer{}
}

func (bp *StandardBrickPlacer) PlaceAll(
	dsrc DeviceSource,
	opts PlacementOpts,
	pred DeviceFilter) (
	*BrickAllocation, error) {

	r := &BrickAllocation{
		BrickSets:  []*BrickSet{},
		DeviceSets: []*DeviceSet{},
	}

	numBrickSets := opts.SetCount()
	for sn := 0; sn < numBrickSets; sn++ {
		logger.Info("Allocating brick set #%v", sn)

		// Generate an id for the brick, this is used as a
		// random index into the ring(s)
		brickId := utils.GenUUID()

		a := NewSimpleAllocator()
		deviceCh, done, err := a.GetNodesFromDeviceSource(dsrc, brickId)
		defer close(done)
		if err != nil {
			return r, err
		}

		bs, ds, err := populateBrickSet(
			opts,
			dsrc.Device,
			pred,
			deviceCh,
			brickId)
		if err != nil {
			return r, err
		}
		r.BrickSets = append(r.BrickSets, bs)
		r.DeviceSets = append(r.DeviceSets, ds)
	}

	return r, nil
}

func (bp *StandardBrickPlacer) Replace(
	dsrc DeviceSource,
	opts PlacementOpts,
	pred DeviceFilter,
	bs *BrickSet,
	index int) (
	*BrickAllocation, error) {

	if index < 0 || index >= bs.SetSize {
		return nil, fmt.Errorf(
			"brick replace index out of bounds (got %v, set size %v)",
			index, bs.SetSize)
	}
	logger.Info("Replace brick in brick set %v with index %v",
		bs, index)

	// we return a brick allocation for symmetry with PlaceAll
	// but it only contains one pair of sets
	r := &BrickAllocation{
		BrickSets:  []*BrickSet{NewBrickSet(bs.SetSize)},
		DeviceSets: []*DeviceSet{NewDeviceSet(bs.SetSize)},
	}

	brickId := utils.GenUUID()
	a := NewSimpleAllocator()
	deviceCh, done, err := a.GetNodesFromDeviceSource(dsrc, brickId)
	defer close(done)
	if err != nil {
		return r, err
	}

	newBrickEntry, newDeviceEntry, err := findDeviceAndBrickForSet(
		opts, dsrc.Device, pred, deviceCh, bs.Drop(index))
	if err != nil {
		return r, err
	}
	newBrickEntry.SetId(brickId)

	// if this all seems like an awful lot of boilerplate
	// and busy work, consider that in real gluster the positions
	// of the bricks w/in the brickset are meaningful and
	// this will make more sense in future position-aware placers
	// (e.g. arbiter)
	newBricks := make([]*BrickEntry, bs.SetSize)
	newDevices := make([]*DeviceEntry, bs.SetSize)
	for i := 0; i < bs.SetSize; i++ {
		if i == index {
			newBricks[i] = newBrickEntry
			newDevices[i] = newDeviceEntry
		} else {
			newBricks[i] = bs.Bricks[i]
			d, err := dsrc.Device(bs.Bricks[i].Info.DeviceId)
			if err != nil {
				return r, err
			}
			newDevices[i] = d
		}
	}
	r.BrickSets[0].Bricks = newBricks
	r.DeviceSets[0].Devices = newDevices

	godbc.Require(r.BrickSets[0].Full())
	godbc.Require(r.DeviceSets[0].Full())
	return r, nil
}
