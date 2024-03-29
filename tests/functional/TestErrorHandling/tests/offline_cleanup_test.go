//go:build functional
// +build functional

//
// Copyright (c) 2018 The heketi Authors
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
	"encoding/json"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/heketi/tests"

	inj "github.com/heketi/heketi/v10/executors/injectexec"
	"github.com/heketi/heketi/v10/pkg/glusterfs/api"
	"github.com/heketi/heketi/v10/pkg/testutils"
	"github.com/heketi/heketi/v10/server/config"
)

func TestOfflineCleanup(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")
	origConf := path.Join(heketiServer.ServerDir, heketiServer.ConfPath)
	heketiServer.ConfPath = tests.Tempfile()
	defer os.Remove(heketiServer.ConfPath)
	CopyFile(origConf, heketiServer.ConfPath)

	fullTeardown := func() {
		CopyFile(origConf, heketiServer.ConfPath)
		testutils.ServerRestarted(t, heketiServer)
		testCluster.Teardown(t)
		testutils.ServerStopped(t, heketiServer)
	}
	partialTeardown := func() {
		CopyFile(origConf, heketiServer.ConfPath)
		testutils.ServerRestarted(t, heketiServer)
		testCluster.VolumeTeardown(t)
	}

	defer fullTeardown()
	testutils.ServerStarted(t, heketiServer)
	heketiServer.KeepDB = true
	testCluster.Setup(t, 3, 3)

	t.Run("NoOp", func(t *testing.T) {
		defer partialTeardown()
		testOfflineCleanupNoOp(t, heketiServer, origConf)
		checkConsistent(t, heketiServer)
	})
	t.Run("ThreeVolumesFailed", func(t *testing.T) {
		defer partialTeardown()
		testOfflineCleanupThreeVolumesFailed(t, heketiServer, origConf)
		checkConsistent(t, heketiServer)
	})
	t.Run("RetryThreeVolumes", func(t *testing.T) {
		defer partialTeardown()
		testOfflineCleanupRetryThreeVolumes(t, heketiServer, origConf)
		checkConsistent(t, heketiServer)
	})
	t.Run("VolumeExpand", func(t *testing.T) {
		defer partialTeardown()
		testOfflineCleanupVolumeExpand(t, heketiServer, origConf)
		checkConsistent(t, heketiServer)
	})
	t.Run("VolumeDelete", func(t *testing.T) {
		defer partialTeardown()
		testOfflineCleanupVolumeDelete(t, heketiServer, origConf)
		checkConsistent(t, heketiServer)
	})
	t.Run("BlockVolumeCreates", func(t *testing.T) {
		defer partialTeardown()
		testOfflineCleanupBlockVolumeCreates(t, heketiServer, origConf)
		checkConsistent(t, heketiServer)
	})
	t.Run("BlockVolumeCreateOldBHV", func(t *testing.T) {
		defer partialTeardown()
		testOfflineCleanupBlockVolumeCreateOldBHV(t, heketiServer, origConf)
		checkConsistent(t, heketiServer)
	})
	t.Run("BlockVolumeDelete", func(t *testing.T) {
		defer partialTeardown()
		testOfflineCleanupBlockVolumeDelete(t, heketiServer, origConf)
		checkConsistent(t, heketiServer)
	})
}

func testOfflineCleanupNoOp(
	t *testing.T, heketiServer *testutils.ServerCtl, origConf string) {

	testutils.ServerStarted(t, heketiServer)

	// create three volumes
	for i := 0; i < 3; i++ {
		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 5
		volReq.Durability.Type = api.DurabilityReplicate
		_, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	info, err := heketi.OperationsInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, info.InFlight == 0,
		"expected info.InFlight == 0, got:", info.InFlight)

	testutils.ServerStopped(t, heketiServer)
	err = heketiServer.RunOfflineCmd(
		[]string{"offline", "cleanup-operations", heketiServer.ConfigArg()})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}

func testOfflineCleanupThreeVolumesFailed(
	t *testing.T, heketiServer *testutils.ServerCtl, origConf string) {

	testutils.ServerStopped(t, heketiServer)
	UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
		c.GlusterFS.Executor = "inject/ssh"
		c.GlusterFS.DisableBackgroundCleaner = true
		c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
			inj.CmdHook{
				Cmd: ".*blammo.*",
				Reaction: inj.Reaction{
					Err: "saw blammo. got blammod!",
				},
			},
		}
	})
	testutils.ServerRestarted(t, heketiServer)

	// create three good volumes
	for i := 0; i < 3; i++ {
		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 5
		volReq.Durability.Type = api.DurabilityReplicate
		_, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}
	// fail three volumes
	for i := 0; i < 3; i++ {
		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 5
		volReq.Name = fmt.Sprintf("vblammo%v", i)
		volReq.Durability.Type = api.DurabilityReplicate
		_, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
	}

	info, err := heketi.OperationsInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, info.InFlight == 0,
		"expected info.InFlight == 0, got:", info.InFlight)

	l, err := heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 3,
		"expected len(l.PendingOperations)t == 3, got:", len(l.PendingOperations))

	CopyFile(origConf, heketiServer.ConfPath)
	testutils.ServerStopped(t, heketiServer)
	err = heketiServer.RunOfflineCmd(
		[]string{"offline", "cleanup-operations", heketiServer.ConfigArg()})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	testutils.ServerStarted(t, heketiServer)
	l, err = heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 0,
		"expected len(l.PendingOperations)t == 0, got:", len(l.PendingOperations))
}

func testOfflineCleanupRetryThreeVolumes(
	t *testing.T, heketiServer *testutils.ServerCtl, origConf string) {

	testutils.ServerStopped(t, heketiServer)
	UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
		c.GlusterFS.Executor = "inject/ssh"
		c.GlusterFS.DisableBackgroundCleaner = true
		c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
			inj.CmdHook{
				Cmd: ".*blammo.*",
				Reaction: inj.Reaction{
					Err: "saw blammo. got blammod!",
				},
			},
		}
	})
	testutils.ServerRestarted(t, heketiServer)

	// create three good volumes
	for i := 0; i < 3; i++ {
		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 5
		volReq.Durability.Type = api.DurabilityReplicate
		_, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}
	// fail three volumes
	for i := 0; i < 3; i++ {
		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 5
		volReq.Name = fmt.Sprintf("vblammo%v", i)
		volReq.Durability.Type = api.DurabilityReplicate
		_, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
	}

	info, err := heketi.OperationsInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, info.InFlight == 0,
		"expected info.InFlight == 0, got:", info.InFlight)

	l, err := heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 3,
		"expected len(l.PendingOperations)t == 3, got:", len(l.PendingOperations))

	// stop the server but leave the overridden config for the offline cmd
	testutils.ServerStopped(t, heketiServer)
	err = heketiServer.RunOfflineCmd(
		[]string{"offline", "cleanup-operations", heketiServer.ConfigArg()})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	testutils.ServerStarted(t, heketiServer)
	l, err = heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 3,
		"expected len(l.PendingOperations)t == 3, got:", len(l.PendingOperations))

	// now retry the clean up with the "good" config
	CopyFile(origConf, heketiServer.ConfPath)
	testutils.ServerStopped(t, heketiServer)
	err = heketiServer.RunOfflineCmd(
		[]string{"offline", "cleanup-operations", heketiServer.ConfigArg()})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	testutils.ServerStarted(t, heketiServer)
	l, err = heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 0,
		"expected len(l.PendingOperations)t == 0, got:", len(l.PendingOperations))
}

func testOfflineCleanupVolumeExpand(
	t *testing.T, heketiServer *testutils.ServerCtl, origConf string) {

	var err error
	// create three good volumes
	testutils.ServerStarted(t, heketiServer)
	var v *api.VolumeInfoResponse
	for i := 0; i < 3; i++ {
		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 5
		volReq.Durability.Type = api.DurabilityReplicate
		v, err = heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}
	tests.Assert(t, v.Id != "")

	testutils.ServerStopped(t, heketiServer)
	UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
		c.GlusterFS.Executor = "inject/ssh"
		c.GlusterFS.DisableBackgroundCleaner = true
		c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
			inj.CmdHook{
				Cmd: ".*add-brick .*",
				Reaction: inj.Reaction{
					Panic: "injected panic on add-brick",
				},
			},
		}
	})
	testutils.ServerRestarted(t, heketiServer)

	// attempt to expand one volume
	volExReq := &api.VolumeExpandRequest{Size: 10}
	_, err = heketi.VolumeExpand(v.Id, volExReq)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
	tests.Assert(t, !heketiServer.IsAlive(),
		"server is alive; expected server dead due to panic")

	// clean up with the "good" config
	CopyFile(origConf, heketiServer.ConfPath)
	testutils.ServerStopped(t, heketiServer)
	err = heketiServer.RunOfflineCmd(
		[]string{"offline", "cleanup-operations", heketiServer.ConfigArg()})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	testutils.ServerStarted(t, heketiServer)
	l, err := heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 0,
		"expected len(l.PendingOperations)t == 0, got:", len(l.PendingOperations))
}

func testOfflineCleanupVolumeDelete(
	t *testing.T, heketiServer *testutils.ServerCtl, origConf string) {

	var err error
	// create three good volumes
	testutils.ServerStarted(t, heketiServer)
	var v *api.VolumeInfoResponse
	for i := 0; i < 3; i++ {
		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 5
		volReq.Durability.Type = api.DurabilityReplicate
		v, err = heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}
	tests.Assert(t, v.Id != "")

	testutils.ServerStopped(t, heketiServer)
	UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
		c.GlusterFS.Executor = "inject/ssh"
		c.GlusterFS.DisableBackgroundCleaner = true
		c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
			inj.CmdHook{
				Cmd: "^gluster .*volume .*delete .*",
				Reaction: inj.Reaction{
					Panic: "injected panic on delete!",
				},
			},
		}
	})
	testutils.ServerRestarted(t, heketiServer)

	// attempt to delete one volume
	err = heketi.VolumeDelete(v.Id)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
	tests.Assert(t, !heketiServer.IsAlive(),
		"server is alive; expected server dead due to panic")

	// clean up with the "good" config
	CopyFile(origConf, heketiServer.ConfPath)
	testutils.ServerStopped(t, heketiServer)
	err = heketiServer.RunOfflineCmd(
		[]string{"offline", "cleanup-operations", heketiServer.ConfigArg()})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	testutils.ServerStarted(t, heketiServer)
	l, err := heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 0,
		"expected len(l.PendingOperations)t == 0, got:", len(l.PendingOperations))
}

func testOfflineCleanupBlockVolumeCreates(
	t *testing.T, heketiServer *testutils.ServerCtl, origConf string) {

	testutils.ServerStopped(t, heketiServer)
	UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
		c.GlusterFS.Executor = "inject/ssh"
		c.GlusterFS.DisableBackgroundCleaner = true
		c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
			inj.CmdHook{
				Cmd: "^gluster-block .*",
				Reaction: inj.Reaction{
					Err: "failing g-b command",
				},
			},
		}
	})
	testutils.ServerRestarted(t, heketiServer)

	// fail to create a block volume
	volReq := &api.BlockVolumeCreateRequest{}
	volReq.Size = 8
	_, err := heketi.BlockVolumeCreate(volReq)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)

	info, err := heketi.OperationsInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, info.InFlight == 0,
		"expected info.InFlight == 0, got:", info.InFlight)

	l, err := heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 1,
		"expected len(l.PendingOperations)t == 1, got:", len(l.PendingOperations))

	CopyFile(origConf, heketiServer.ConfPath)
	testutils.ServerStopped(t, heketiServer)
	err = heketiServer.RunOfflineCmd(
		[]string{"offline", "cleanup-operations", heketiServer.ConfigArg()})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	testutils.ServerStarted(t, heketiServer)
	l, err = heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 0,
		"expected len(l.PendingOperations)t == 0, got:", len(l.PendingOperations))
}

func testOfflineCleanupBlockVolumeCreateOldBHV(
	t *testing.T, heketiServer *testutils.ServerCtl, origConf string) {

	// create a BHV and block volumes
	testutils.ServerStarted(t, heketiServer)
	volReq := &api.BlockVolumeCreateRequest{}
	volReq.Size = 8
	_, err := heketi.BlockVolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	_, err = heketi.BlockVolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	testutils.ServerStopped(t, heketiServer)
	UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
		c.GlusterFS.Executor = "inject/ssh"
		c.GlusterFS.DisableBackgroundCleaner = true
		c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
			inj.CmdHook{
				Cmd: "^gluster-block .*",
				Reaction: inj.Reaction{
					Err: "failing g-b command",
				},
			},
		}
	})
	testutils.ServerRestarted(t, heketiServer)

	// fail to create a block volume
	_, err = heketi.BlockVolumeCreate(volReq)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)

	info, err := heketi.OperationsInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, info.InFlight == 0,
		"expected info.InFlight == 0, got:", info.InFlight)

	l, err := heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 1,
		"expected len(l.PendingOperations)t == 1, got:", len(l.PendingOperations))

	CopyFile(origConf, heketiServer.ConfPath)
	testutils.ServerStopped(t, heketiServer)
	err = heketiServer.RunOfflineCmd(
		[]string{"offline", "cleanup-operations", heketiServer.ConfigArg()})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	testutils.ServerStarted(t, heketiServer)
	l, err = heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 0,
		"expected len(l.PendingOperations)t == 0, got:", len(l.PendingOperations))

	// assert that the BHV still exists
	vl, err := heketi.VolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(vl.Volumes) == 1)
}

func testOfflineCleanupBlockVolumeDelete(
	t *testing.T, heketiServer *testutils.ServerCtl, origConf string) {

	// create a BHV and block volumes
	testutils.ServerStarted(t, heketiServer)
	volReq := &api.BlockVolumeCreateRequest{}
	volReq.Size = 8
	_, err := heketi.BlockVolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	victim, err := heketi.BlockVolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	testutils.ServerStopped(t, heketiServer)
	UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
		c.GlusterFS.Executor = "inject/ssh"
		c.GlusterFS.DisableBackgroundCleaner = true
		c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
			inj.CmdHook{
				Cmd: "^gluster-block .*",
				Reaction: inj.Reaction{
					Panic: "panicking on g-b command",
				},
			},
		}
	})
	testutils.ServerRestarted(t, heketiServer)

	// check the number of volumes, get num. bvs
	var bvCount int
	ci, err := heketi.ClusterInfo(victim.Cluster)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(ci.Volumes) == 1,
		"expected len(ci.Volumes) == 1, got:", ci.Volumes)
	bvCount = len(ci.BlockVolumes)

	// fail to delete a bv
	err = heketi.BlockVolumeDelete(victim.Id)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
	tests.Assert(t, !heketiServer.IsAlive(),
		"server is alive; expected server dead due to panic")

	CopyFile(origConf, heketiServer.ConfPath)
	testutils.ServerStopped(t, heketiServer)
	err = heketiServer.RunOfflineCmd(
		[]string{"offline", "cleanup-operations", heketiServer.ConfigArg()})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	testutils.ServerStarted(t, heketiServer)
	l, err := heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 0,
		"expected len(l.PendingOperations)t == 0, got:", len(l.PendingOperations))

	// assert that the BHV and first BV still exists
	ci, err = heketi.ClusterInfo(victim.Cluster)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(ci.Volumes) == 1,
		"expected len(ci.Volumes) == 1, got:", ci.Volumes)
	tests.Assert(t, len(ci.BlockVolumes) == bvCount-1,
		"expected len(ci.BlockVolumes) == bvCount - 1, got:", ci.BlockVolumes, bvCount-1)
}

func checkConsistent(t *testing.T, heketiServer *testutils.ServerCtl) {
	testutils.ServerStarted(t, heketiServer)
	chk, err := heketi.DbCheck()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	k := map[string]interface{}{}
	err = json.Unmarshal([]byte(chk), &k)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	ti := int(k["totalinconsistencies"].(float64))
	tests.Assert(t, ti == 0, "expected ti == 0, got:", ti, ":", chk)
}
