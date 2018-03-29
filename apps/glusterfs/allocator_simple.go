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

	nodeUp := currentNodeHealthStatus()

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
		if up, found := nodeUp[nodeId]; found && !up {
			// if the node is in the cache and we know it was not
			// recently healthy, skip it
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

func loadRingFromDeviceSource(dsrc DeviceSource) (
	*SimpleAllocatorRing, error) {

	ring := NewSimpleAllocatorRing()
	dnl, err := dsrc.Devices()
	if err != nil {
		return nil, err
	}
	for _, dan := range dnl {
		ring.Add(&SimpleDevice{
			zone:     dan.Node.Info.Zone,
			nodeId:   dan.Node.Info.Id,
			deviceId: dan.Device.Info.Id,
		})
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

	device, done := make(chan string), make(chan struct{})

	devicelist, err := getDeviceListFromDB(db, clusterId, brickId)
	if err != nil {
		close(device)
		return device, done, err
	}

	generateDevices(devicelist, device, done)
	return device, done, nil
}

// GetNodesFromDeviceSource is a shim function that should only
// exist as long as we keep the intermediate simple allocator.
func (s *SimpleAllocator) GetNodesFromDeviceSource(dsrc DeviceSource,
	brickId string) (
	<-chan string, chan<- struct{}, error) {

	device, done := make(chan string), make(chan struct{})

	ring, err := loadRingFromDeviceSource(dsrc)
	if err != nil {
		close(device)
		return device, done, err
	}
	devicelist := ring.GetDeviceList(brickId)

	generateDevices(devicelist, device, done)
	return device, done, nil
}

func generateDevices(devicelist SimpleDevices,
	device chan<- string, done <-chan struct{}) {

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
}
