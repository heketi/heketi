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

type singleClusterRing struct {
	clusterId string
	ring      *SimpleAllocatorRing
}

// Create a new simple allocator
func NewSimpleAllocator() *SimpleAllocator {
	s := &SimpleAllocator{}
	return s
}

func (s *singleClusterRing) loadRingFromDB(tx *bolt.Tx) error {
	s.ring = &SimpleAllocatorRing{}

	cluster, err := NewClusterEntryFromId(tx, s.clusterId)
	if err != nil {
		return err
	}

	// Add Cluster to ring
	if err = s.addCluster(cluster.Info.Id); err != nil {
		return err
	}

	for _, nodeId := range cluster.Info.Nodes {
		node, err := NewNodeEntryFromId(tx, nodeId)
		if err != nil {
			return err
		}

		// Check node is online
		if !node.isOnline() {
			continue
		}

		for _, deviceId := range node.Devices {
			device, err := NewDeviceEntryFromId(tx, deviceId)
			if err != nil {
				return err
			}

			// Check device is online
			if !device.isOnline() {
				continue
			}

			// Add device to ring
			err = s.addDevice(cluster, node, device)
			if err != nil {
				return err
			}

		}
	}
	return nil
}

func (s *singleClusterRing) addDevice(cluster *ClusterEntry,
	node *NodeEntry,
	device *DeviceEntry) error {

	s.ring.Add(&SimpleDevice{
		zone:     node.Info.Zone,
		nodeId:   node.Info.Id,
		deviceId: device.Info.Id,
	})

	return nil

}

// addCluster adds an entry to the rings map. Must be called before addDevice so
// that the entry exists.
func (s *singleClusterRing) addCluster(clusterId string) error {

	// Add cluster to map
	s.ring = NewSimpleAllocatorRing()

	return nil
}

func (s *singleClusterRing) getDeviceList(brickId string) (SimpleDevices, error) {

	s.ring.Rebalance()
	devicelist := s.ring.GetDeviceList(brickId)

	return devicelist, nil

}

func (s *SimpleAllocator) GetNodes(db wdb.RODB, clusterId,
	brickId string) (<-chan string, chan<- struct{}, error) {

	// Initialize channels
	device, done := make(chan string), make(chan struct{})

	scr := &singleClusterRing{clusterId: clusterId}
	if err := db.View(scr.loadRingFromDB); err != nil {
		close(device)
		return device, done, err
	}

	// Get the list of devices for this brick id
	devicelist, err := scr.getDeviceList(brickId)

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
