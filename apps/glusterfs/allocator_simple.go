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
	"sync"

	"github.com/boltdb/bolt"
	wdb "github.com/heketi/heketi/pkg/db"
)

// Simple allocator contains a map to rings of clusters
type SimpleAllocator struct {
	rings map[string]*SimpleAllocatorRing
	lock  sync.Mutex
}

// Create a new simple allocator
func NewSimpleAllocator() *SimpleAllocator {
	s := &SimpleAllocator{}
	s.rings = make(map[string]*SimpleAllocatorRing)
	return s
}

func (s *SimpleAllocator) loadRingFromDB(tx *bolt.Tx) error {
	s.rings = map[string]*SimpleAllocatorRing{}

	clusters, err := ClusterList(tx)
	if err != nil {
		return err
	}

	for _, clusterId := range clusters {
		cluster, err := NewClusterEntryFromId(tx, clusterId)
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
	}
	return nil
}

func (s *SimpleAllocator) addDevice(cluster *ClusterEntry,
	node *NodeEntry,
	device *DeviceEntry) error {

	clusterId := cluster.Info.Id

	// Check the cluster id is in the map
	if _, ok := s.rings[clusterId]; !ok {
		logger.LogError("Unknown cluster id requested: %v", clusterId)
		return ErrNotFound
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	s.rings[clusterId].Add(&SimpleDevice{
		zone:     node.Info.Zone,
		nodeId:   node.Info.Id,
		deviceId: device.Info.Id,
	})

	return nil

}

// addCluster adds an entry to the rings map. Must be called before addDevice so
// that the entry exists.
func (s *SimpleAllocator) addCluster(clusterId string) error {

	s.lock.Lock()
	defer s.lock.Unlock()

	if _, ok := s.rings[clusterId]; ok {
		logger.LogError("cluster id %s already exists", clusterId)
		return ErrFound
	}

	// Add cluster to map
	s.rings[clusterId] = NewSimpleAllocatorRing()

	return nil
}

func (s *SimpleAllocator) getDeviceList(clusterId, brickId string) (SimpleDevices, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if _, ok := s.rings[clusterId]; !ok {
		logger.LogError("Unknown cluster id requested: %v", clusterId)
		return nil, ErrNotFound
	}

	ring := s.rings[clusterId]
	ring.Rebalance()
	devicelist := ring.GetDeviceList(brickId)

	return devicelist, nil

}

func (s *SimpleAllocator) GetNodes(db wdb.RODB, clusterId,
	brickId string) (<-chan string, chan<- struct{}, <-chan error) {

	// Initialize channels
	device, done := make(chan string), make(chan struct{})

	// Make sure to make a buffered channel for the error, so we can
	// set it and return
	errc := make(chan error, 1)

	if err := db.View(s.loadRingFromDB); err != nil {
		errc <- err
		close(device)
		return device, done, errc
	}

	// Get the list of devices for this brick id
	devicelist, err := s.getDeviceList(clusterId, brickId)

	if err != nil {
		errc <- err
		close(device)
		return device, done, errc
	}

	// Start generator in a new goroutine
	go func() {
		defer func() {
			errc <- nil
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

	return device, done, errc
}
