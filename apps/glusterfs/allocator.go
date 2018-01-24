//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

type Allocator interface {

	// Inform the brick allocator to include device
	AddDevice(c *ClusterEntry, n *NodeEntry, d *DeviceEntry) error

	// Inform the brick allocator to not use the specified device
	RemoveDevice(c *ClusterEntry, n *NodeEntry, d *DeviceEntry) error

	// Add cluster information from allocator.
	AddCluster(clusterId string) error

	// Remove cluster information from allocator
	RemoveCluster(clusterId string) error

	// Returns a generator, done, and error channel.
	// The generator returns the location for the brick, then the possible locations
	// of its replicas. The caller must close() the done channel when it no longer
	// needs to read from the generator.
	GetNodes(clusterId, brickId string) (<-chan string,
		chan<- struct{}, <-chan error)

	// Check whether a node is currently considered by the allocator
	HasNode(clusterId string, zone int, nodeId string) bool

	// Check whether a device is currently considered by the allocator
	HasDevice(clusterId string, zone int, nodeId, deviceId string) bool
}
