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
	"github.com/boltdb/bolt"
	"github.com/lpabon/godbc"

	wdb "github.com/heketi/heketi/pkg/db"
	"github.com/heketi/heketi/pkg/utils"
)

type BrickSet struct {
	SetSize int
	Bricks  []*BrickEntry
}

func NewBrickSet(s int) *BrickSet {
	return &BrickSet{SetSize: s, Bricks: []*BrickEntry{}}
}

func (bs *BrickSet) Add(b *BrickEntry) {
	godbc.Require(!bs.Full())
	bs.Bricks = append(bs.Bricks, b)
}

func (bs *BrickSet) Full() bool {
	return len(bs.Bricks) == bs.SetSize
}

type DeviceSet struct {
	SetSize int
	Devices []*DeviceEntry
}

func NewDeviceSet(s int) *DeviceSet {
	return &DeviceSet{SetSize: s, Devices: []*DeviceEntry{}}
}

func (ds *DeviceSet) Add(d *DeviceEntry) {
	godbc.Require(!ds.Full())
	ds.Devices = append(ds.Devices, d)
}

func (ds *DeviceSet) Full() bool {
	return len(ds.Devices) == ds.SetSize
}

type BrickAllocation struct {
	BrickSets  []*BrickSet
	DeviceSets []*DeviceSet
}

type deviceFilter func(*BrickSet, *DeviceEntry) bool

type deviceFetcher func(string) (*DeviceEntry, error)

func tryAllocateBrickOnDevice(v *VolumeEntry,
	pred deviceFilter,
	device *DeviceEntry,
	bs *BrickSet, brick_size uint64) *BrickEntry {

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
	brick := device.NewBrickEntry(brick_size,
		float64(v.Info.Snapshot.Factor),
		v.Info.Gid, v.Info.Id)

	return brick
}

func findDeviceAndBrickForSet(v *VolumeEntry,
	fetchDevice deviceFetcher,
	pred deviceFilter,
	deviceCh <-chan string,
	bs *BrickSet,
	brick_size uint64) (*BrickEntry, *DeviceEntry, error) {

	// Check the ring for devices to place the brick
	for deviceId := range deviceCh {

		device, err := fetchDevice(deviceId)
		if err != nil {
			return nil, nil, err
		}

		brick := tryAllocateBrickOnDevice(v, pred, device, bs, brick_size)
		if brick == nil {
			continue
		}

		return brick, device, nil
	}

	// No devices found
	return nil, nil, ErrNoSpace
}

func getCachedDevice(devcache map[string](*DeviceEntry),
	tx *bolt.Tx,
	deviceId string) (*DeviceEntry, error) {

	// Get device entry from cache if possible
	device, ok := devcache[deviceId]
	if !ok {
		// Get device entry from db otherwise
		var err error
		device, err = NewDeviceEntryFromId(tx, deviceId)
		if err != nil {
			return nil, err
		}
		devcache[deviceId] = device
	}
	return device, nil
}

func populateBrickSet(v *VolumeEntry,
	fetchDevice deviceFetcher,
	pred deviceFilter,
	deviceCh <-chan string,
	initId string,
	brick_size uint64,
	ssize int) (*BrickSet, *DeviceSet, error) {

	bs := NewBrickSet(ssize)
	ds := NewDeviceSet(ssize)
	for i := 0; i < ssize; i++ {
		logger.Debug("%v / %v", i, ssize)

		brick, device, err := findDeviceAndBrickForSet(
			v, fetchDevice, pred, deviceCh, bs,
			brick_size)
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

	r := &BrickAllocation{
		BrickSets:  []*BrickSet{},
		DeviceSets: []*DeviceSet{},
	}

	devcache := map[string](*DeviceEntry){}

	err := db.View(func(tx *bolt.Tx) error {
		txdb := wdb.WrapTx(tx)
		fetchDevice := func(id string) (*DeviceEntry, error) {
			return getCachedDevice(devcache, tx, id)
		}

		// Determine allocation for each brick required for this volume
		for sn := 0; sn < numBrickSets; sn++ {
			logger.Info("Allocating brick set #%v", sn)

			// Generate an id for the brick
			brickId := utils.GenUUID()

			a := NewSimpleAllocator()

			deviceCh, done, err := a.GetNodes(txdb, cluster, brickId)
			defer close(done)
			if err != nil {
				return err
			}

			// Fill in a complete set of bricks/devices. If not possible
			// err will be non-nil
			bs, ds, err := populateBrickSet(
				v, fetchDevice, nil, deviceCh, brickId,
				brick_size, v.Durability.BricksInSet())
			if err != nil {
				return err
			}
			r.BrickSets = append(r.BrickSets, bs)
			r.DeviceSets = append(r.DeviceSets, ds)
		}

		return nil
	})
	return r, err
}
