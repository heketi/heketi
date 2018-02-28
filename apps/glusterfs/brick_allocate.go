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

	wdb "github.com/heketi/heketi/pkg/db"
	"github.com/heketi/heketi/pkg/utils"
)

type BrickAllocation struct {
	Bricks  []*BrickEntry
	Devices []*DeviceEntry
}

func tryAllocateBrickOnDevice(v *VolumeEntry, device *DeviceEntry,
	setlist []*BrickEntry, brick_size uint64) *BrickEntry {

	// Do not allow a device from the same node to be in the set
	deviceOk := true
	for _, brickInSet := range setlist {
		if brickInSet.Info.NodeId == device.NodeId {
			deviceOk = false
		}
	}

	if !deviceOk {
		return nil
	}

	// Try to allocate a brick on this device
	brick := device.NewBrickEntry(brick_size,
		float64(v.Info.Snapshot.Factor),
		v.Info.Gid, v.Info.Id)

	return brick
}

func findDeviceAndBrickForSet(tx *bolt.Tx, v *VolumeEntry,
	devcache map[string](*DeviceEntry),
	deviceCh <-chan string,
	setlist []*BrickEntry,
	brick_size uint64) (*BrickEntry, *DeviceEntry, error) {

	// Check the ring for devices to place the brick
	for deviceId := range deviceCh {

		// Get device entry from cache if possible
		device, ok := devcache[deviceId]
		if !ok {
			// Get device entry from db otherwise
			var err error
			device, err = NewDeviceEntryFromId(tx, deviceId)
			if err != nil {
				return nil, nil, err
			}
			devcache[deviceId] = device
		}

		brick := tryAllocateBrickOnDevice(v, device, setlist, brick_size)
		if brick == nil {
			continue
		}

		return brick, device, nil
	}

	// No devices found
	return nil, nil, ErrNoSpace
}

func allocateBricks(
	db wdb.RODB,
	allocator Allocator,
	cluster string,
	v *VolumeEntry,
	bricksets int,
	brick_size uint64) (*BrickAllocation, error) {

	r := &BrickAllocation{
		Bricks:  []*BrickEntry{},
		Devices: []*DeviceEntry{},
	}

	devcache := map[string](*DeviceEntry){}

	err := db.View(func(tx *bolt.Tx) error {
		txdb := wdb.WrapTx(tx)

		// Determine allocation for each brick required for this volume
		for brick_num := 0; brick_num < bricksets; brick_num++ {
			logger.Info("brick_num: %v", brick_num)

			// Create a brick set list to later make sure that the
			// proposed bricks and devices are acceptable
			setlist := make([]*BrickEntry, 0)

			// Generate an id for the brick
			brickId := utils.GenUUID()

			// Get allocator generator
			// The same generator should be used for the brick and its replicas
			deviceCh, done, err := allocator.GetNodes(txdb, cluster, brickId)
			defer func() {
				close(done)
			}()
			if err != nil {
				return err
			}

			// Check location has space for each brick and its replicas
			for i := 0; i < v.Durability.BricksInSet(); i++ {
				logger.Debug("%v / %v", i, v.Durability.BricksInSet())

				brick, device, err := findDeviceAndBrickForSet(tx,
					v, devcache, deviceCh, setlist,
					brick_size)
				if err != nil {
					return err
				}

				// If the first in the set, then reset the id
				if i == 0 {
					brick.SetId(brickId)
				}

				// Save the brick entry to create later
				r.Bricks = append(r.Bricks, brick)
				r.Devices = append(r.Devices, device)

				setlist = append(setlist, brick)

				device.BrickAdd(brick.Id())
			}
		}

		return nil
	})
	if err != nil {
		return r, err
	}

	// Only assign bricks to the volume object on success
	for _, brick := range r.Bricks {
		logger.Debug("Adding brick %v to volume %v", brick.Id(), v.Info.Id)
		v.BrickAdd(brick.Id())
	}

	return r, nil
}
