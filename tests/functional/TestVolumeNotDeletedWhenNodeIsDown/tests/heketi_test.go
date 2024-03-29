//go:build functional
// +build functional

//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//
package functional

import (
	"fmt"
	"testing"
	"time"

	client "github.com/heketi/heketi/v10/client/api/go-client"
	"github.com/heketi/heketi/v10/pkg/glusterfs/api"
	"github.com/heketi/heketi/v10/pkg/logging"
	"github.com/heketi/heketi/v10/pkg/testutils"
	"github.com/heketi/heketi/v10/pkg/utils"
	"github.com/heketi/tests"
)

// These are the settings for the vagrant file
const (

	// The heketi server must be running on the host
	heketiUrl = "http://127.0.0.1:8080"

	// VM Information
	DISKS    = 3
	NODES    = 3
	ZONES    = 2
	CLUSTERS = 1
)

var (
	// Heketi client
	heketi = client.NewClient(heketiUrl, "admin", "adminkey")
	logger = logging.NewLogger("[test]", logging.LEVEL_DEBUG)
)

func getdisks() []string {

	diskletters := make([]string, DISKS)
	for index, i := 0, []byte("b")[0]; index < DISKS; index, i = index+1, i+1 {
		diskletters[index] = "/dev/vd" + string(i)
	}

	return diskletters
}

func getnodes() []string {
	nodelist := make([]string, NODES)

	for index, ip := 0, 100; index < NODES; index, ip = index+1, ip+1 {
		nodelist[index] = "192.168.10." + fmt.Sprintf("%v", ip)
	}

	return nodelist
}

func setupCluster(t *testing.T) {
	tests.Assert(t, heketi != nil)

	nodespercluster := NODES / CLUSTERS
	nodes := getnodes()
	sg := utils.NewStatusGroup()
	for cluster := 0; cluster < CLUSTERS; cluster++ {
		sg.Add(1)
		go func(nodes_in_cluster []string) {
			defer sg.Done()
			// Create a cluster
			cluster_req := &api.ClusterCreateRequest{
				ClusterFlags: api.ClusterFlags{
					Block: true,
					File:  true,
				},
			}
			cluster, err := heketi.ClusterCreate(cluster_req)
			if err != nil {
				logger.Err(err)
				sg.Err(err)
				return
			}

			// Add nodes sequentially due to probes
			for index, hostname := range nodes_in_cluster {
				nodeReq := &api.NodeAddRequest{}
				nodeReq.ClusterId = cluster.Id
				nodeReq.Hostnames.Manage = []string{hostname}
				nodeReq.Hostnames.Storage = []string{hostname}
				nodeReq.Zone = index%ZONES + 1

				node, err := heketi.NodeAdd(nodeReq)
				if err != nil {
					logger.Err(err)
					sg.Err(err)
					return
				}

				// Add devices all concurrently
				for _, disk := range getdisks() {
					sg.Add(1)
					go func(d string) {
						defer sg.Done()

						driveReq := &api.DeviceAddRequest{}
						driveReq.Name = d
						driveReq.NodeId = node.Id

						err := heketi.DeviceAdd(driveReq)
						if err != nil {
							logger.Err(err)
							sg.Err(err)
						}
					}(disk)
				}
			}
		}(nodes[cluster*nodespercluster : (cluster+1)*nodespercluster])
	}

	// Wait here for results
	err := sg.Result()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

}

func teardownCluster(t *testing.T) {
	clusters, err := heketi.ClusterList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	sg := utils.NewStatusGroup()
	for _, cluster := range clusters.Clusters {

		sg.Add(1)
		go func(clusterId string) {
			defer sg.Done()

			clusterInfo, err := heketi.ClusterInfo(clusterId)
			if err != nil {
				logger.Err(err)
				sg.Err(err)
				return
			}

			// Delete volumes in this cluster
			for _, volume := range clusterInfo.Volumes {
				err := heketi.VolumeDelete(volume)
				if err != nil {
					logger.Err(err)
					sg.Err(err)
					return
				}
			}

			// Delete all devices in the cluster concurrently
			deviceSg := utils.NewStatusGroup()
			for _, node := range clusterInfo.Nodes {

				// Get node information
				nodeInfo, err := heketi.NodeInfo(node)
				if err != nil {
					logger.Err(err)
					deviceSg.Err(err)
					return
				}

				// Delete each device
				for _, device := range nodeInfo.DevicesInfo {
					deviceSg.Add(1)
					go func(id string) {
						defer deviceSg.Done()

						stateReq := &api.StateRequest{}
						stateReq.State = api.EntryStateOffline
						err := heketi.DeviceState(id, stateReq)
						if err != nil {
							logger.Err(err)
							deviceSg.Err(err)
							return
						}

						stateReq.State = api.EntryStateFailed
						err = heketi.DeviceState(id, stateReq)
						if err != nil {
							logger.Err(err)
							deviceSg.Err(err)
							return
						}

						err = heketi.DeviceDelete(id)
						if err != nil {
							logger.Err(err)
							deviceSg.Err(err)
							return
						}

					}(device.Id)
				}
			}
			err = deviceSg.Result()
			if err != nil {
				logger.Err(err)
				sg.Err(err)
				return
			}

			// Delete nodes
			for _, node := range clusterInfo.Nodes {
				err = heketi.NodeDelete(node)
				if err != nil {
					logger.Err(err)
					sg.Err(err)
					return
				}
			}

			// Delete cluster
			err = heketi.ClusterDelete(clusterId)
			if err != nil {
				logger.Err(err)
				sg.Err(err)
				return
			}

		}(cluster)

	}

	err = sg.Result()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}

func TestConnection(t *testing.T) {
	err := heketi.Hello()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}

// NOTE: this test is "unclean" as it turns off a node and then
// never resets the node state nor tears down the cluster.
// This means it cannot be easily combined with other test suites
// without changing the behavior of the test or supporting
// more control of nodes from the test code.
// Fixes TDB.
func TestVolumeNotDeletedWhenNodeIsDown(t *testing.T) {
	na := testutils.RequireNodeAccess(t)

	// Setup the VM storage topology
	teardownCluster(t)
	setupCluster(t)

	// Create a volume
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 100
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3

	volInfo, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// SSH into one system and power it off
	exec := na.Use(logger)

	// Turn off glusterd
	cmd := []string{"service glusterd stop"}
	exec.ConnectAndExec("192.168.10.100:22", cmd, 10, true)
	time.Sleep(2 * time.Second)

	// Try to delete the volume
	err = heketi.VolumeDelete(volInfo.Id)
	tests.Assert(t, err != nil, err)
	logger.Info("Error Returned:\n%v", err)

	// Check that the volume is still there
	info, err := heketi.VolumeInfo(volInfo.Id)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, info.Id == volInfo.Id)

	// Now poweroff node
	// Don't check for error, since it the connection will be disconnected
	// by powering off, it will return an error
	cmd = []string{"poweroff"}
	exec.ConnectAndExec("192.168.10.100:22", cmd, 10, true)

	// Wait some time for the system to come down
	time.Sleep(5 * time.Second)

	// Try to delete the volume
	err = heketi.VolumeDelete(volInfo.Id)
	tests.Assert(t, err != nil, err)
	logger.Info("Error Returned:\n%v", err)

	// Check that the volume is still there
	info, err = heketi.VolumeInfo(volInfo.Id)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, info.Id == volInfo.Id)
}
