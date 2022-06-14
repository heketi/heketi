//go:build functional
// +build functional

//
// Copyright (c) 2020 The heketi Authors
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
	"os"
	"path"
	"strings"
	"testing"

	"github.com/heketi/tests"

	inj "github.com/heketi/heketi/v10/executors/injectexec"
	"github.com/heketi/heketi/v10/pkg/glusterfs/api"
	"github.com/heketi/heketi/v10/pkg/testutils"
	"github.com/heketi/heketi/v10/server/config"
)

func TestBlockExpandInject(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")
	origConf := path.Join(heketiServer.ServerDir, heketiServer.ConfPath)

	baseConf := tests.Tempfile()
	defer os.Remove(baseConf)
	UpdateConfig(origConf, baseConf, func(c *config.Config) {
		// we want the background cleaner disabled for all
		// of the sub-tests we'll be running as we are testing
		// on demand cleaning and want predictable behavior.
		c.GlusterFS.DisableBackgroundCleaner = true
	})

	heketiServer.ConfPath = tests.Tempfile()
	defer os.Remove(heketiServer.ConfPath)
	CopyFile(baseConf, heketiServer.ConfPath)

	defer func() {
		CopyFile(baseConf, heketiServer.ConfPath)
		testutils.ServerRestarted(t, heketiServer)
		testCluster.Teardown(t)
		testutils.ServerStopped(t, heketiServer)
	}()

	resetConf := func() {
		CopyFile(baseConf, heketiServer.ConfPath)
		testutils.ServerRestarted(t, heketiServer)
	}

	testutils.ServerStarted(t, heketiServer)
	heketiServer.KeepDB = true
	testCluster.Setup(t, 3, 4)

	t.Run("TryExpandSuccess", func(t *testing.T) {
		resetConf()

		blockReq := &api.BlockVolumeCreateRequest{}
		blockReq.Size = 1
		blockReq.Hacount = 3

		// Create the block volume of 1GiB
		bvol, err := heketi.BlockVolumeCreate(blockReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		testutils.ServerRestarted(t, heketiServer)

		beReq := &api.BlockVolumeExpandRequest{}
		beReq.Size = 2

		// Expand the block volume to 2GiB, expect a success
		_, err = heketi.BlockVolumeExpand(bvol.Id, beReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		// Check size and usable size match
		bvolInfo, err := heketi.BlockVolumeInfo(bvol.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, bvolInfo.Size == 2, "expected bvolInfo.Size == 2 got:", bvolInfo.Size)
		tests.Assert(t, bvolInfo.UsableSize == 2, "expected bvolInfo.UsableSize == 2 got:", bvolInfo.UsableSize)

		// Check PendingOperations
		l, err := heketi.PendingOperationList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l.PendingOperations) == 0,
			"expected len(l.PendingOperations) == 0, got:", len(l.PendingOperations))

		// Check if the FreeSize on block hosting volume is changed
		volInfo, err := heketi.VolumeInfo(bvol.BlockHostingVolume)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, volInfo.BlockInfo.FreeSize == 194, "expected volInfo.BlockInfo.FreeSize == 194 got:", volInfo.BlockInfo.FreeSize)

		// Done with it, delete now
		err = heketi.BlockVolumeDelete(bvol.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		err = heketi.VolumeDelete(volInfo.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})

	t.Run("TryExpandFailAtVeryBeginning", func(t *testing.T) {
		resetConf()

		blockReq := &api.BlockVolumeCreateRequest{}
		blockReq.Size = 1
		blockReq.Hacount = 3

		// Create the block volume of 1GiB
		bvol, err := heketi.BlockVolumeCreate(blockReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		// Inject a failure,
		// pretend the fail happen at very beginning, and nothing changed
		// i.e. all the nodes are at same old size
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
				inj.CmdHook{
					Cmd: "^gluster-block modify .*",
					Reaction: inj.Reaction{
						Err: `{ "RESULT": "FAIL", "errCode": 255, "errMsg": "Version check failed [host HOST2 returned -107] (Hint: See if all servers are up and running gluster-blockd daemon)" }`,
					},
				},
				inj.CmdHook{
					Cmd: "^gluster-block info .*",
					Reaction: inj.Reaction{
						Result: `{ "NAME": "fakeIt", "VOLUME": "fakeIt", "GBID": "fakeIt", "SIZE": "1.0 GiB", "HA": 3, "PASSWORD": "", "EXPORTED ON": [ "HOST1", "HOST2", "HOST3" ] }`,
					},
				},
			}
		})
		testutils.ServerRestarted(t, heketiServer)

		beReq := &api.BlockVolumeExpandRequest{}
		beReq.Size = 2

		// Expand the block volume to 2GiB, expect a failure
		_, err = heketi.BlockVolumeExpand(bvol.Id, beReq)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
		tests.Assert(t,
			strings.Contains(err.Error(), "Failed to expand block volume: Version check failed [host HOST2 returned -107] (Hint: See if all servers are up and running gluster-blockd daemon) (see logs for details, and retry the operation)"),
			`Failed to expand block volume: Version check failed [host HOST2 returned -107] (Hint: See if all servers are up and running gluster-blockd daemon) (see logs for details, and retry the operation)" in err, got:`,
			err.Error())

		// assert that no pending ops remain
		l, err := heketi.PendingOperationList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l.PendingOperations) == 0,
			"expected len(l.PendingOperations) == 0, got:", len(l.PendingOperations))

		// Check for block volume size and usable size
		bvolInfo, err := heketi.BlockVolumeInfo(bvol.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, bvolInfo.Size == 1, "expected bvolInfo.Size == 1 got:", bvolInfo.Size)
		tests.Assert(t, bvolInfo.UsableSize == 1, "expected bvolInfo.UsableSize == 1 got:", bvolInfo.UsableSize)

		// Check if the FreeSize on block hosting volume is changed
		volInfo, err := heketi.VolumeInfo(bvol.BlockHostingVolume)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, volInfo.BlockInfo.FreeSize == 195, "expected volInfo.BlockInfo.FreeSize == 195 got:", volInfo.BlockInfo.FreeSize)

		// Done with it, delete now
		err = heketi.BlockVolumeDelete(bvol.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		err = heketi.VolumeDelete(volInfo.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})

	t.Run("TryExpandFail", func(t *testing.T) {
		resetConf()

		blockReq := &api.BlockVolumeCreateRequest{}
		blockReq.Size = 1
		blockReq.Hacount = 3

		// Create the block volume of 1GiB
		bvol, err := heketi.BlockVolumeCreate(blockReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		// Inject a failure,
		// pretend the fail happen on one node, and let it stick with size 1GiB
		// and rest of the nodes will expand to 2GiB
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
				inj.CmdHook{
					Cmd: "^gluster-block modify .*",
					Reaction: inj.Reaction{
						Err: `{ "IQN": "fakeIQN", "FAILED ON": [ "HOST3" ], "SUCCESSFUL ON": [ "HOST1", "HOST2" ], "RESULT": "FAIL", "errCode": 255, "errMsg": "block volume resize failed: HOST3:[1.0 GiB]" }`,
					},
				},
				inj.CmdHook{
					Cmd: "^gluster-block info .*",
					Reaction: inj.Reaction{
						Result: `{ "NAME": "fakeIt", "VOLUME": "fakeIt", "GBID": "fakeIt", "SIZE": "2.0 GiB", "HA": 3, "PASSWORD": "", "EXPORTED ON": [ "HOST1", "HOST2", "HOST3" ], "RESIZE FAILED ON": { "HOST3": "1.0 GiB" } }`,
					},
				},
			}
		})
		testutils.ServerRestarted(t, heketiServer)

		beReq := &api.BlockVolumeExpandRequest{}
		beReq.Size = 2

		// Expand the block volume to 2GiB, expect a failure
		_, err = heketi.BlockVolumeExpand(bvol.Id, beReq)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
		tests.Assert(t,
			strings.Contains(err.Error(), "Failed to expand block volume: block volume resize failed: HOST3:[1.0 GiB] (see logs for details, and retry the operation)"),
			`expected string "Failed to expand block volume: block volume resize failed: HOST3:[1.0 GiB] (see logs for details, and retry the operation)" in err, got:`,
			err.Error())

		// assert that no pending ops remain
		l, err := heketi.PendingOperationList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l.PendingOperations) == 0,
			"expected len(l.PendingOperations) == 0, got:", len(l.PendingOperations))

		// Check for block volume size and usable size
		bvolInfo, err := heketi.BlockVolumeInfo(bvol.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, bvolInfo.Size == 2, "expected bvolInfo.Size == 2 got:", bvolInfo.Size)
		tests.Assert(t, bvolInfo.UsableSize == 1, "expected bvolInfo.UsableSize == 1 got:", bvolInfo.UsableSize)

		// Check if the FreeSize on block hosting volume is changed
		volInfo, err := heketi.VolumeInfo(bvol.BlockHostingVolume)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, volInfo.BlockInfo.FreeSize == 194, "expected volInfo.BlockInfo.FreeSize == 194 got:", volInfo.BlockInfo.FreeSize)

		// Done with it, delete now
		err = heketi.BlockVolumeDelete(bvol.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		err = heketi.VolumeDelete(volInfo.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})

	t.Run("RetryExpandFail", func(t *testing.T) {
		resetConf()

		blockReq := &api.BlockVolumeCreateRequest{}
		blockReq.Size = 2
		blockReq.Hacount = 3

		// Create the block volume of 2GiB
		bvol, err := heketi.BlockVolumeCreate(blockReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		// Try:
		// Inject a failure,
		// pretend the fail happen on one node, and let it stick with size 2GiB
		// and rest of the nodes will expand to 3GiB
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
				inj.CmdHook{
					Cmd: "^gluster-block modify .*",
					Reaction: inj.Reaction{
						Err: `{ "IQN": "fakeIQN", "FAILED ON": [ "HOST3" ], "SUCCESSFUL ON": [ "HOST1", "HOST2" ], "RESULT": "FAIL", "errCode": 255, "errMsg": "block volume resize failed: HOST3:[2.0 GiB]" }`,
					},
				},
				inj.CmdHook{
					Cmd: "^gluster-block info .*",
					Reaction: inj.Reaction{
						Result: `{ "NAME": "fakeIt", "VOLUME": "fakeIt", "GBID": "fakeIt", "SIZE": "3.0 GiB", "HA": 3, "PASSWORD": "", "EXPORTED ON": [ "HOST1", "HOST2", "HOST3" ], "RESIZE FAILED ON": { "HOST3": "2.0 GiB" } }`,
					},
				},
			}
		})
		testutils.ServerRestarted(t, heketiServer)

		beReq := &api.BlockVolumeExpandRequest{}
		beReq.Size = 3

		// Expand the block volume to 3GiB, expect a failure
		_, err = heketi.BlockVolumeExpand(bvol.Id, beReq)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
		tests.Assert(t,
			strings.Contains(err.Error(), "Failed to expand block volume: block volume resize failed: HOST3:[2.0 GiB] (see logs for details, and retry the operation)"),
			`expected string "Failed to expand block volume: block volume resize failed: HOST3:[2.0 GiB] (see logs for details, and retry the operation)" in err, got:`,
			err.Error())

		// assert that no pending ops remain
		l, err := heketi.PendingOperationList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l.PendingOperations) == 0,
			"expected len(l.PendingOperations) == 0, got:", len(l.PendingOperations))

		// Check for block volume size and usable size
		bvolInfo, err := heketi.BlockVolumeInfo(bvol.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, bvolInfo.Size == 3, "expected bvolInfo.Size == 3 got:", bvolInfo.Size)
		tests.Assert(t, bvolInfo.UsableSize == 2, "expected bvolInfo.UsableSize == 2 got:", bvolInfo.UsableSize)

		// Check if the FreeSize on block hosting volume is changed
		volInfo, err := heketi.VolumeInfo(bvol.BlockHostingVolume)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, volInfo.BlockInfo.FreeSize == 193, "expected volInfo.BlockInfo.FreeSize == 193 got:", volInfo.BlockInfo.FreeSize)

		// Retry:
		// Inject a failure,
		// pretend the expand for size 3GiB failed already on HOST3 (as part of Retry1)
		// and this request is expand for size 4GiB (say Retry2) and this time it happen to fail on two nodes,
		// so, HOST3 size is 2GiB and now it failed on HOST2 too, which will stick with size 3GiB
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
				inj.CmdHook{
					Cmd: "^gluster-block modify .*",
					Reaction: inj.Reaction{
						Err: `{ "IQN": "fakeIQN", "FAILED ON": [ "HOST2", "HOST3" ], "SUCCESSFUL ON": [ "HOST1" ], "RESULT": "FAIL", "errCode": 255, "errMsg": "block volume resize failed: HOST3:[2.0 GiB] HOST2:[3.0 GiB]"}`,
					},
				},
				inj.CmdHook{
					Cmd: "^gluster-block info .*",
					Reaction: inj.Reaction{
						Result: `{ "NAME": "fakeIt", "VOLUME": "fakeIt", "GBID": "fakeIt", "SIZE": "4.0 GiB", "HA": 3, "PASSWORD": "", "EXPORTED ON": [ "HOST1", "HOST2", "HOST3" ], "RESIZE FAILED ON": { "HOST3": "2.0 GiB", "HOST2": "3.0 GiB"} }`,
					},
				},
			}
		})
		testutils.ServerRestarted(t, heketiServer)

		beReq.Size = 4
		// Expand the block volume to 4GiB, expect a failure on two nodes
		_, err = heketi.BlockVolumeExpand(bvol.Id, beReq)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
		tests.Assert(t,
			strings.Contains(err.Error(), "Failed to expand block volume: block volume resize failed: HOST3:[2.0 GiB] HOST2:[3.0 GiB] (see logs for details, and retry the operation)"),
			`expected string "Failed to expand block volume: block volume resize failed: HOST3:[2.0 GiB] HOST2:[3.0 GiB] (see logs for details, and retry the operation)" in err, got:`,
			err.Error())

		// assert that no pending ops remain
		l, err = heketi.PendingOperationList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l.PendingOperations) == 0,
			"expected len(l.PendingOperations) == 0, got:", len(l.PendingOperations))

		// Check for block volume size and usable size
		bvolInfo, err = heketi.BlockVolumeInfo(bvol.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, bvolInfo.Size == 4, "expected bvolInfo.Size == 4 got:", bvolInfo.Size)
		tests.Assert(t, bvolInfo.UsableSize == 2, "expected bvolInfo.UsableSize == 2 got:", bvolInfo.UsableSize)

		// Check if the FreeSize on block hosting volume is changed
		volInfo, err = heketi.VolumeInfo(bvol.BlockHostingVolume)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, volInfo.BlockInfo.FreeSize == 192, "expected volInfo.BlockInfo.FreeSize == 192 got:", volInfo.BlockInfo.FreeSize)

		// Done with it, delete now
		err = heketi.BlockVolumeDelete(bvol.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		err = heketi.VolumeDelete(volInfo.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})

	t.Run("RetryExpandSuccessWithSameSize", func(t *testing.T) {
		resetConf()

		blockReq := &api.BlockVolumeCreateRequest{}
		blockReq.Size = 1
		blockReq.Hacount = 3

		// Create the block volume of 1GiB
		bvol, err := heketi.BlockVolumeCreate(blockReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		// Try:
		// Inject a failure,
		// pretend the fail happen on one node, and let it stick with size 1GiB
		// and rest of the nodes will expand to 2GiB
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
				inj.CmdHook{
					Cmd: "^gluster-block modify .*",
					Reaction: inj.Reaction{
						Err: `{ "IQN": "fakeIQN", "FAILED ON": [ "HOST3" ], "SUCCESSFUL ON": [ "HOST1", "HOST2" ], "RESULT": "FAIL", "errCode": 255, "errMsg": "block volume resize failed: HOST3:[1.0 GiB]" }`,
					},
				},
				inj.CmdHook{
					Cmd: "^gluster-block info .*",
					Reaction: inj.Reaction{
						Result: `{ "NAME": "fakeIt", "VOLUME": "fakeIt", "GBID": "fakeIt", "SIZE": "2.0 GiB", "HA": 3, "PASSWORD": "", "EXPORTED ON": [ "HOST1", "HOST2", "HOST3" ], "RESIZE FAILED ON": { "HOST3": "1.0 GiB" } }`,
					},
				},
			}
		})
		testutils.ServerRestarted(t, heketiServer)

		beReq := &api.BlockVolumeExpandRequest{}
		beReq.Size = 2

		// Expand the block volume to 2GiB, expect a failure
		_, err = heketi.BlockVolumeExpand(bvol.Id, beReq)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
		tests.Assert(t,
			strings.Contains(err.Error(), "Failed to expand block volume: block volume resize failed: HOST3:[1.0 GiB] (see logs for details, and retry the operation)"),
			`expected string "Failed to expand block volume: block volume resize failed: HOST3:[1.0 GiB] (see logs for details, and retry the operation)" in err, got:`,
			err.Error())

		// assert that no pending ops remain
		l, err := heketi.PendingOperationList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l.PendingOperations) == 0,
			"expected len(l.PendingOperations) == 0, got:", len(l.PendingOperations))

		// Check for block volume size and usable size
		bvolInfo, err := heketi.BlockVolumeInfo(bvol.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, bvolInfo.Size == 2, "expected bvolInfo.Size == 2 got:", bvolInfo.Size)
		tests.Assert(t, bvolInfo.UsableSize == 1, "expected bvolInfo.UsableSize == 1 got:", bvolInfo.UsableSize)

		// Check if the FreeSize on block hosting volume is changed
		volInfo, err := heketi.VolumeInfo(bvol.BlockHostingVolume)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, volInfo.BlockInfo.FreeSize == 194, "expected volInfo.BlockInfo.FreeSize == 194 got:", volInfo.BlockInfo.FreeSize)

		// Retry:
		// Inject a success,
		// pretend the expand for size 2GiB failed already on HOST3 (as part of Retry1)
		// and this request is expand for same size (say Retry2) and this time it succeeded
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
				inj.CmdHook{
					Cmd: "^gluster-block modify .*",
					Reaction: inj.Reaction{
						Result: `{ "IQN": "fakeIQN", "RESULT": "SUCCESS" }`,
					},
				},
				inj.CmdHook{
					Cmd: "^gluster-block info .*",
					Reaction: inj.Reaction{
						Result: `{ "NAME": "fakeIt", "VOLUME": "fakeIt", "GBID": "fakeIt", "SIZE": "2.0 GiB", "HA": 3, "PASSWORD": "", "EXPORTED ON": [ "HOST1", "HOST2", "HOST3" ] }`,
					},
				},
			}
		})
		testutils.ServerRestarted(t, heketiServer)

		// Retry expanding the block volume to same size again, expect a success
		_, err = heketi.BlockVolumeExpand(bvol.Id, beReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		// assert that no pending ops remain
		l, err = heketi.PendingOperationList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l.PendingOperations) == 0,
			"expected len(l.PendingOperations) == 0, got:", len(l.PendingOperations))

		// Check for block volume size and usable size
		bvolInfo, err = heketi.BlockVolumeInfo(bvol.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, bvolInfo.Size == 2, "expected bvolInfo.Size == 2 got:", bvolInfo.Size)
		tests.Assert(t, bvolInfo.UsableSize == 2, "expected bvolInfo.UsableSize == 2 got:", bvolInfo.UsableSize)

		// Check if the FreeSize on block hosting volume is changed
		volInfo, err = heketi.VolumeInfo(bvol.BlockHostingVolume)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, volInfo.BlockInfo.FreeSize == 194, "expected volInfo.BlockInfo.FreeSize == 194 got:", volInfo.BlockInfo.FreeSize)

		// Done with it, delete now
		err = heketi.BlockVolumeDelete(bvol.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		err = heketi.VolumeDelete(volInfo.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})

	t.Run("RetryExpandSuccessWithDifferentSize", func(t *testing.T) {
		resetConf()

		blockReq := &api.BlockVolumeCreateRequest{}
		blockReq.Size = 1
		blockReq.Hacount = 3

		// Create the block volume of 1GiB
		bvol, err := heketi.BlockVolumeCreate(blockReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		// Try:
		// Inject a failure,
		// pretend the fail happen on one node, and let it stick with size 1GiB
		// and rest of the nodes will expand to 2GiB
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
				inj.CmdHook{
					Cmd: "^gluster-block modify .*",
					Reaction: inj.Reaction{
						Err: `{ "IQN": "fakeIQN", "FAILED ON": [ "HOST3" ], "SUCCESSFUL ON": [ "HOST1", "HOST2" ], "RESULT": "FAIL", "errCode": 255, "errMsg": "block volume resize failed: HOST3:[1.0 GiB]" }`,
					},
				},
				inj.CmdHook{
					Cmd: "^gluster-block info .*",
					Reaction: inj.Reaction{
						Result: `{ "NAME": "fakeIt", "VOLUME": "fakeIt", "GBID": "fakeIt", "SIZE": "2.0 GiB", "HA": 3, "PASSWORD": "", "EXPORTED ON": [ "HOST1", "HOST2", "HOST3" ], "RESIZE FAILED ON": { "HOST3": "1.0 GiB" } }`,
					},
				},
			}
		})
		testutils.ServerRestarted(t, heketiServer)

		beReq := &api.BlockVolumeExpandRequest{}
		beReq.Size = 2

		// Expand the block volume to 2GiB, expect a failure
		_, err = heketi.BlockVolumeExpand(bvol.Id, beReq)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
		tests.Assert(t,
			strings.Contains(err.Error(), "Failed to expand block volume: block volume resize failed: HOST3:[1.0 GiB] (see logs for details, and retry the operation)"),
			`expected string "Failed to expand block volume: block volume resize failed: HOST3:[1.0 GiB] (see logs for details, and retry the operation)" in err, got:`,
			err.Error())

		// assert that no pending ops remain
		l, err := heketi.PendingOperationList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l.PendingOperations) == 0,
			"expected len(l.PendingOperations) == 0, got:", len(l.PendingOperations))

		// Check for block volume size and usable size
		bvolInfo, err := heketi.BlockVolumeInfo(bvol.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, bvolInfo.Size == 2, "expected bvolInfo.Size == 2 got:", bvolInfo.Size)
		tests.Assert(t, bvolInfo.UsableSize == 1, "expected bvolInfo.UsableSize == 1 got:", bvolInfo.UsableSize)

		// Check if the FreeSize on block hosting volume is changed
		volInfo, err := heketi.VolumeInfo(bvol.BlockHostingVolume)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, volInfo.BlockInfo.FreeSize == 194, "expected volInfo.BlockInfo.FreeSize == 194 got:", volInfo.BlockInfo.FreeSize)

		// Retry:
		// Inject a success,
		// pretend the expand for size 2GiB failed already on HOST3 (as part of Retry1)
		// and this request is expand for size 3GiB (say Retry2) and this time it succeeded
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
				inj.CmdHook{
					Cmd: "^gluster-block modify .*",
					Reaction: inj.Reaction{
						Result: `{ "IQN": "fakeIQN", "RESULT": "SUCCESS" }`,
					},
				},
				inj.CmdHook{
					Cmd: "^gluster-block info .*",
					Reaction: inj.Reaction{
						Result: `{ "NAME": "fakeIt", "VOLUME": "fakeIt", "GBID": "fakeIt", "SIZE": "3.0 GiB", "HA": 3, "PASSWORD": "", "EXPORTED ON": [ "HOST1", "HOST2", "HOST3" ] }`,
					},
				},
			}
		})
		testutils.ServerRestarted(t, heketiServer)

		// try a different size now
		beReq.Size = 3

		// Expand the block volume to 3GiB, expect a success
		_, err = heketi.BlockVolumeExpand(bvol.Id, beReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		// assert that no pending ops remain
		l, err = heketi.PendingOperationList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l.PendingOperations) == 0,
			"expected len(l.PendingOperations) == 0, got:", len(l.PendingOperations))

		// Check for block volume size and usable size
		bvolInfo, err = heketi.BlockVolumeInfo(bvol.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, bvolInfo.Size == 3, "expected bvolInfo.Size == 3 got:", bvolInfo.Size)
		tests.Assert(t, bvolInfo.UsableSize == 3, "expected bvolInfo.UsableSize == 3 got:", bvolInfo.UsableSize)

		// Check if the FreeSize on block hosting volume is changed
		volInfo, err = heketi.VolumeInfo(bvol.BlockHostingVolume)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, volInfo.BlockInfo.FreeSize == 193, "expected volInfo.BlockInfo.FreeSize == 193 got:", volInfo.BlockInfo.FreeSize)

		// Done with it, delete now
		err = heketi.BlockVolumeDelete(bvol.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		err = heketi.VolumeDelete(volInfo.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})

	t.Run("TryExpandFailWithOldGlusterBlock", func(t *testing.T) {
		resetConf()

		blockReq := &api.BlockVolumeCreateRequest{}
		blockReq.Size = 1
		blockReq.Hacount = 3

		// Create the block volume of 1GiB
		bvol, err := heketi.BlockVolumeCreate(blockReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		// Note: we are using a old gluster-block version for this case
		// Inject a failure,
		// pretend the fail happen on one node
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
				inj.CmdHook{
					Cmd: "^gluster-block modify .*",
					Reaction: inj.Reaction{
						Err: `{ "IQN": "fakeIQN", "FAILED ON": [ "HOST3" ], "SUCCESSFUL ON": [ "HOST1", "HOST2" ], "RESULT": "FAIL" }`,
					},
				},
				inj.CmdHook{
					Cmd: "^gluster-block info .*",
					Reaction: inj.Reaction{
						Result: `{ "NAME": "fakeIt", "VOLUME": "fakeIt", "GBID": "fakeIt", "SIZE": "2.0 GiB", "HA": 3, "PASSWORD": "", "EXPORTED ON": [ "HOST1", "HOST2", "HOST3" ] }`,
					},
				},
			}
		})
		testutils.ServerRestarted(t, heketiServer)

		beReq := &api.BlockVolumeExpandRequest{}
		beReq.Size = 2

		// Expand the block volume to 2GiB, expect a failure
		_, err = heketi.BlockVolumeExpand(bvol.Id, beReq)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
		tests.Assert(t,
			strings.Contains(err.Error(), "Failed to expand block volume (see logs for details, and retry the operation)"),
			`expected string "Failed to expand block volume (see logs for details, and retry the operation)" in err, got:`,
			err.Error())

		// assert that no pending ops remain
		l, err := heketi.PendingOperationList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l.PendingOperations) == 0,
			"expected len(l.PendingOperations) == 0, got:", len(l.PendingOperations))

		// Check for block volume size and usable size
		bvolInfo, err := heketi.BlockVolumeInfo(bvol.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, bvolInfo.Size == 2, "expected bvolInfo.Size == 2 got:", bvolInfo.Size)
		tests.Assert(t, bvolInfo.UsableSize == 1, "expected bvolInfo.UsableSize == 1 got:", bvolInfo.UsableSize)

		// Check if the FreeSize on block hosting volume is changed
		volInfo, err := heketi.VolumeInfo(bvol.BlockHostingVolume)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, volInfo.BlockInfo.FreeSize == 194, "expected volInfo.BlockInfo.FreeSize == 194 got:", volInfo.BlockInfo.FreeSize)

		// Done with it, delete now
		err = heketi.BlockVolumeDelete(bvol.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		err = heketi.VolumeDelete(volInfo.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})
}
