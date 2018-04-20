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
)

type ClusterDeviceSource struct {
	tx          *bolt.Tx
	deviceCache map[string]*DeviceEntry
	nodeCache   map[string]*NodeEntry
	clusterId   string
}

func NewClusterDeviceSource(tx *bolt.Tx,
	clusterId string) *ClusterDeviceSource {

	return &ClusterDeviceSource{
		tx:          tx,
		deviceCache: map[string](*DeviceEntry){},
		nodeCache:   map[string](*NodeEntry){},
		clusterId:   clusterId,
	}
}

func (cds *ClusterDeviceSource) Devices() ([]DeviceAndNode, error) {
	cluster, err := NewClusterEntryFromId(cds.tx, cds.clusterId)
	if err != nil {
		return nil, err
	}

	nodeUp := currentNodeHealthStatus()

	valid := [](DeviceAndNode){}
	for _, nodeId := range cluster.Info.Nodes {
		node, err := NewNodeEntryFromId(cds.tx, nodeId)
		if err != nil {
			return nil, err
		}
		if !node.isOnline() {
			continue
		}
		if up, found := nodeUp[nodeId]; found && !up {
			// if the node is in the cache and we know it was not
			// recently healthy, skip it
			continue
		}

		for _, deviceId := range node.Devices {
			device, err := NewDeviceEntryFromId(cds.tx, deviceId)
			if err != nil {
				return nil, err
			}
			if !device.isOnline() {
				continue
			}

			valid = append(valid, DeviceAndNode{
				Device: device,
				Node:   node,
			})
			// NOTE: it is extremely important not to overwrite
			// existing cache items because the allocation algorithms
			// mutate the device entries during the process.
			if _, found := cds.deviceCache[deviceId]; !found {
				cds.deviceCache[deviceId] = device
			}
			if _, found := cds.nodeCache[nodeId]; !found {
				cds.nodeCache[nodeId] = node
			}
		}
	}

	return valid, nil
}

func (cds *ClusterDeviceSource) Device(id string) (*DeviceEntry, error) {
	device, ok := cds.deviceCache[id]
	if !ok {
		// Get device entry from db otherwise
		var err error
		device, err = NewDeviceEntryFromId(cds.tx, id)
		if err != nil {
			return nil, err
		}
		cds.deviceCache[id] = device
	}
	return device, nil
}

func (cds *ClusterDeviceSource) Node(id string) (*NodeEntry, error) {
	node, ok := cds.nodeCache[id]
	if !ok {
		// Get node entry from db otherwise
		var err error
		node, err = NewNodeEntryFromId(cds.tx, id)
		if err != nil {
			return nil, err
		}
		cds.nodeCache[id] = node
	}
	return node, nil
}
