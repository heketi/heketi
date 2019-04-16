// +build functional

//
// Copyright (c) 2019 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), as published by the Free Software Foundation,
// or under the Apache License, Version 2.0 <LICENSE-APACHE2 or
// http://www.apache.org/licenses/LICENSE-2.0>.
//
// You may not use this file except in compliance with those terms.
//

package tests

import (
	"fmt"
	"testing"

	"github.com/heketi/tests"

	"github.com/heketi/heketi/pkg/glusterfs/api"
	rex "github.com/heketi/heketi/pkg/remoteexec"
	"github.com/heketi/heketi/pkg/remoteexec/ssh"
	"github.com/heketi/heketi/pkg/testutils"
)

func TestDeviceAddRemoveSymlink(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")

	defer func() {
		testutils.ServerRestarted(t, heketiServer)
		testCluster.Teardown(t)
		testutils.ServerStopped(t, heketiServer)
	}()

	testutils.ServerStarted(t, heketiServer)
	heketiServer.KeepDB = true
	testCluster.Setup(t, 3, 2)

	na := testutils.RequireNodeAccess(t)
	exec := na.Use(logger)

	for i := 0; i < len(testCluster.Nodes); i++ {
		err := linkDevice(testCluster, exec, i, 3, "/dev/bender")
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	topo, err := heketi.TopologyInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	var firstNode string
	preDevices := map[string]bool{}
	for i, n := range topo.ClusterList[0].Nodes {
		if i == 0 {
			firstNode = n.Id
		}
		for _, d := range n.DevicesInfo {
			preDevices[d.Id] = true
		}
	}

	req := &api.DeviceAddRequest{}
	req.Name = "/dev/bender"
	req.NodeId = firstNode
	err = heketi.DeviceAdd(req)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	for i := 0; i < len(testCluster.Nodes); i++ {
		err := rmLink(testCluster, exec, i, "/dev/bender")
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	topo, err = heketi.TopologyInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	var newDeviceId string
	for _, n := range topo.ClusterList[0].Nodes {
		for _, d := range n.DevicesInfo {
			if !preDevices[d.Id] {
				newDeviceId = d.Id
			}
		}
	}

	stateReq := &api.StateRequest{}
	stateReq.State = api.EntryStateOffline
	err = heketi.DeviceState(newDeviceId, stateReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	stateReq = &api.StateRequest{}
	stateReq.State = api.EntryStateFailed
	err = heketi.DeviceState(newDeviceId, stateReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = heketi.DeviceDelete(newDeviceId)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}

func linkDevice(
	tc *testutils.ClusterEnv, exec *ssh.SshExec,
	hostIdx, diskIdx int, newPath string) error {

	sshHost := tc.SshHost(hostIdx)
	diskPath := tc.Disks[diskIdx]
	cmds := []string{
		fmt.Sprintf("ln -sf %s %s", diskPath, newPath),
	}
	return rex.AnyError(exec.ExecCommands(sshHost, cmds, 10, true))
}

func rmLink(
	tc *testutils.ClusterEnv, exec *ssh.SshExec,
	hostIdx int, path string) error {

	sshHost := tc.SshHost(hostIdx)
	cmds := []string{
		fmt.Sprintf("rm -f %s", path),
	}
	return rex.AnyError(exec.ExecCommands(sshHost, cmds, 10, true))
}
