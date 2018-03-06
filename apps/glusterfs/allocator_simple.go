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
	"github.com/boltdb/bolt"
	wdb "github.com/heketi/heketi/pkg/db"
)

// Simple allocator contains a map to rings of clusters
type SimpleAllocator struct {
}

// Create a new simple allocator
func NewSimpleAllocator() *SimpleAllocator {
	s := &SimpleAllocator{}
	return s
}

func loadRingFromDB(tx *bolt.Tx, clusterId string) (*SimpleAllocatorRing, error) {
	cluster, err := NewClusterEntryFromId(tx, clusterId)
	if err != nil {
		return nil, err
	}

	ring := NewSimpleAllocatorRing()

	for _, nodeId := range cluster.Info.Nodes {
		node, err := NewNodeEntryFromId(tx, nodeId)
		if err != nil {
			return nil, err
		}

		// Check node is online
		if !node.isOnline() {
			continue
		}

		for _, deviceId := range node.Devices {
			device, err := NewDeviceEntryFromId(tx, deviceId)
			if err != nil {
				return nil, err
			}

			// Check device is online
			if !device.isOnline() {
				continue
			}

			// Add device to ring
			ring.Add(&SimpleDevice{
				zone:     node.Info.Zone,
				nodeId:   node.Info.Id,
				deviceId: device.Info.Id,
			})
		}
	}

	return ring, nil
}

func getDeviceListFromDB(db wdb.RODB, clusterId,
	brickId string) (SimpleDevices, error) {

	var ring *SimpleAllocatorRing
	err := db.View(func(tx *bolt.Tx) (e error) {
		ring, e = loadRingFromDB(tx, clusterId)
		return e
	})
	if err != nil {
		return nil, err
	}

	devicelist := ring.GetDeviceList(brickId)

	return devicelist, nil

}

func (s *SimpleAllocator) GetNodes(db wdb.RODB, clusterId,
	brickId string) (<-chan string, chan<- struct{}, error) {

	// Initialize channels
	device, done := make(chan string), make(chan struct{})

	// Get the list of devices for this brick id
	devicelist, err := getDeviceListFromDB(db, clusterId, brickId)

	if err != nil {
		close(device)
		return device, done, err
	}

	// Start generator in a new goroutine
	go func() {
		defer func() {
			close(device)
		}()

		for _, d := range devicelist {
			select {
			case device <- d.deviceId:
			case <-done:
				return
			}
		}

	}()

	return device, done, nil
}
