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
	"time"

	"github.com/heketi/tests"

	inj "github.com/heketi/heketi/v10/executors/injectexec"
	"github.com/heketi/heketi/v10/pkg/glusterfs/api"
	"github.com/heketi/heketi/v10/pkg/testutils"
	"github.com/heketi/heketi/v10/server/config"
)

func TestDeviceRemoveOperation(t *testing.T) {
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

	resetConfFile := func() {
		CopyFile(baseConf, heketiServer.ConfPath)
	}

	testutils.ServerStarted(t, heketiServer)
	heketiServer.KeepDB = true
	testCluster.Setup(t, 3, 2)

	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 1
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3

	// create a few volumes
	for i := 0; i < 6; i++ {
		_, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	t.Run("verifyHealCheckEnable", func(t *testing.T) {
		var err error
		deviceToRemove := getDeviceToRemove(t)
		defer enableAllDevices(t)

		defer testutils.ServerRestarted(t, heketiServer)
		defer resetConfFile()

		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
				inj.CmdHook{
					Cmd: ".* volume heal .* info --xml",
					Reaction: inj.Reaction{
						Err: `i like to show fake error`,
					},
				},
			}
		})

		testutils.ServerRestarted(t, heketiServer)

		err = heketi.DeviceState(deviceToRemove, &api.StateRequest{
			State:     api.EntryStateOffline,
			HealCheck: api.HealCheckDisable,
		})
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		err = heketi.DeviceState(deviceToRemove, &api.StateRequest{
			State:     api.EntryStateFailed,
			HealCheck: api.HealCheckEnable,
		})
		tests.Assert(t,
			strings.Contains(err.Error(), "i like to show fake error"),
			"expected error triggered by 'gluster volume heal info' command, but no matching error was found")

		// there should not be any pending ops in the db
		l, err := heketi.PendingOperationList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l.PendingOperations) == 0,
			"expected len(l.PendingOperations) == 0, got:",
			len(l.PendingOperations))
	})

	t.Run("verifyHealCheckDisable", func(t *testing.T) {
		var err error
		deviceToRemove := getDeviceToRemove(t)
		defer enableAllDevices(t)

		defer testutils.ServerRestarted(t, heketiServer)
		defer resetConfFile()

		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
				inj.CmdHook{
					Cmd: ".* volume heal .* info --xml",
					Reaction: inj.Reaction{
						Err: `i like to show fake error`,
					},
				},
			}
		})

		testutils.ServerRestarted(t, heketiServer)

		err = heketi.DeviceState(deviceToRemove, &api.StateRequest{
			State:     api.EntryStateOffline,
			HealCheck: api.HealCheckEnable,
		})
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		err = heketi.DeviceState(deviceToRemove, &api.StateRequest{
			State:     api.EntryStateFailed,
			HealCheck: api.HealCheckDisable,
		})
		tests.Assert(t, err == nil, "expected no error from gluster commands, heal check disabled")

		// there should not be any pending ops in the db
		l, err := heketi.PendingOperationList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l.PendingOperations) == 0,
			"expected len(l.PendingOperations) == 0, got:",
			len(l.PendingOperations))

		dused := perDeviceUsedSize(t)
		tests.Assert(t, dused[deviceToRemove] == 0,
			"expected dused[deviceToRemove] == 0, got:",
			dused[deviceToRemove])
	})

	t.Run("happyPathHealCheckDisabled", func(t *testing.T) {
		var err error
		deviceToRemove := getDeviceToRemove(t)
		defer enableAllDevices(t)

		err = heketi.DeviceState(deviceToRemove, &api.StateRequest{
			State:     api.EntryStateOffline,
			HealCheck: api.HealCheckEnable,
		})
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		err = heketi.DeviceState(deviceToRemove, &api.StateRequest{
			State:     api.EntryStateFailed,
			HealCheck: api.HealCheckDisable,
		})
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		// there should not be any pending ops in the db
		l, err := heketi.PendingOperationList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l.PendingOperations) == 0,
			"expected len(l.PendingOperations) == 0, got:",
			len(l.PendingOperations))

		dused := perDeviceUsedSize(t)
		tests.Assert(t, dused[deviceToRemove] == 0,
			"expected dused[deviceToRemove] == 0, got:",
			dused[deviceToRemove])
	})

	t.Run("happyPath", func(t *testing.T) {
		var err error
		deviceToRemove := getDeviceToRemove(t)
		defer enableAllDevices(t)

		err = heketi.DeviceState(deviceToRemove, &api.StateRequest{
			api.EntryStateOffline,
			api.HealCheckEnable,
		})
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		err = heketi.DeviceState(deviceToRemove, &api.StateRequest{
			api.EntryStateFailed,
			api.HealCheckEnable,
		})
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		// there should not be any pending ops in the db
		l, err := heketi.PendingOperationList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l.PendingOperations) == 0,
			"expected len(l.PendingOperations) == 0, got:",
			len(l.PendingOperations))

		dused := perDeviceUsedSize(t)
		tests.Assert(t, dused[deviceToRemove] == 0,
			"expected dused[deviceToRemove] == 0, got:",
			dused[deviceToRemove])
	})

	t.Run("lvcreateError", func(t *testing.T) {
		var err error
		resetConfFile()
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
				inj.CmdHook{
					Cmd: ".*lvcreate.*",
					Reaction: inj.Reaction{
						Err: "thwack!",
					},
				},
			}
		})
		testutils.ServerRestarted(t, heketiServer)
		deviceToRemove := getDeviceToRemove(t)
		defer enableAllDevices(t)

		err = heketi.DeviceState(deviceToRemove, &api.StateRequest{
			api.EntryStateOffline,
			api.HealCheckEnable,
		})
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		err = heketi.DeviceState(deviceToRemove, &api.StateRequest{
			api.EntryStateFailed,
			api.HealCheckEnable,
		})
		tests.Assert(t, err != nil, "expected err != nil, got:", err)

		// there should not be any pending ops in the db
		l, err := heketi.PendingOperationList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l.PendingOperations) == 0,
			"expected len(l.PendingOperations) == 0, got:",
			len(l.PendingOperations))
	})

	t.Run("lvremoveError", func(t *testing.T) {
		var err error
		resetConfFile()
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
				inj.CmdHook{
					Cmd: ".*lvremove.*",
					Reaction: inj.Reaction{
						Err: "thwack!",
					},
				},
			}
		})
		testutils.ServerRestarted(t, heketiServer)
		deviceToRemove := getDeviceToRemove(t)
		defer enableAllDevices(t)

		err = heketi.DeviceState(deviceToRemove, &api.StateRequest{
			api.EntryStateOffline,
			api.HealCheckEnable,
		})
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		err = heketi.DeviceState(deviceToRemove, &api.StateRequest{
			api.EntryStateFailed,
			api.HealCheckEnable,
		})
		tests.Assert(t, err != nil, "expected err != nil, got:", err)

		// there should be two pending ops in the db:
		// * the parent op: device remove
		// * the child op: brick evict
		l, err := heketi.PendingOperationList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l.PendingOperations) == 2,
			"expected len(l.PendingOperations) == 2, got:",
			len(l.PendingOperations))

		// remove our lvremove injected failure - let clean do its thing
		logger.Info("restoring heketi default behavior")
		resetConfFile()
		testutils.ServerRestarted(t, heketiServer)

		// request a clean
		err = heketi.PendingOperationCleanUp(
			&api.PendingOperationsCleanRequest{})
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		for i := 0; i < 15; i++ {
			l, err = heketi.PendingOperationList()
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			if len(l.PendingOperations) == 0 {
				break
			}
			time.Sleep(time.Second)
		}
		tests.Assert(t, len(l.PendingOperations) == 0,
			"expected len(l.PendingOperations) == 0, got:",
			len(l.PendingOperations))
	})

	t.Run("lvcreatePanicRetry", func(t *testing.T) {
		var err error
		resetConfFile()
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
				inj.CmdHook{
					Cmd: ".*lvcreate.*",
					Reaction: inj.Reaction{
						Panic: "panicking on lvcreate command",
					},
				},
			}
		})
		testutils.ServerRestarted(t, heketiServer)
		deviceToRemove := getDeviceToRemove(t)
		defer enableAllDevices(t)

		err = heketi.DeviceState(deviceToRemove, &api.StateRequest{
			api.EntryStateOffline,
			api.HealCheckEnable,
		})
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		err = heketi.DeviceState(deviceToRemove, &api.StateRequest{
			api.EntryStateFailed,
			api.HealCheckEnable,
		})
		tests.Assert(t, err != nil, "expected err != nil, got:", err)

		// remove our injected panic & restart the server
		logger.Info("restoring heketi default behavior")
		resetConfFile()
		testutils.ServerRestarted(t, heketiServer)

		// there should be two pending ops in the db:
		// * the parent op: device remove
		// * the child op: brick evict
		l, err := heketi.PendingOperationList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l.PendingOperations) == 2,
			"expected len(l.PendingOperations) == 2, got:",
			len(l.PendingOperations))

		// request a clean
		err = heketi.PendingOperationCleanUp(
			&api.PendingOperationsCleanRequest{})
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		for i := 0; i < 15; i++ {
			l, err = heketi.PendingOperationList()
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			if len(l.PendingOperations) == 0 {
				break
			}
			time.Sleep(time.Second)
		}
		tests.Assert(t, len(l.PendingOperations) == 0,
			"expected len(l.PendingOperations) == 0, got:",
			len(l.PendingOperations))

		// now that the db is free of pending ops it's safe
		// to try removing this device once again
		err = heketi.DeviceState(deviceToRemove, &api.StateRequest{
			api.EntryStateFailed,
			api.HealCheckEnable,
		})
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		// the device should now be empty after the 2nd remove
		dused := perDeviceUsedSize(t)
		tests.Assert(t, dused[deviceToRemove] == 0,
			"expected dused[deviceToRemove] == 0, got:",
			dused[deviceToRemove])
	})

	// replaceBrickErrorBefore - inject an error before gluster has replaced
	// the brick to test the recovery path where we keep the old brick
	// and remove the new brick
	t.Run("replaceBrickErrorBefore", func(t *testing.T) {
		var err error
		resetConfFile()
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
				inj.CmdHook{
					Cmd: ".*volume replace-brick.*",
					Reaction: inj.Reaction{
						Err: "ouch!",
					},
				},
			}
		})
		testutils.ServerRestarted(t, heketiServer)
		deviceToRemove := getDeviceToRemove(t)
		defer enableAllDevices(t)

		bCountBefore := getBrickCounts(t)

		err = heketi.DeviceState(deviceToRemove, &api.StateRequest{
			api.EntryStateOffline,
			api.HealCheckEnable,
		})
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		err = heketi.DeviceState(deviceToRemove, &api.StateRequest{
			api.EntryStateFailed,
			api.HealCheckEnable,
		})
		tests.Assert(t, err != nil, "expected err != nil, got:", err)

		// there should not be any pending ops in the db
		l, err := heketi.PendingOperationList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l.PendingOperations) == 0,
			"expected len(l.PendingOperations) == 0, got:",
			len(l.PendingOperations))

		// we failed "before" gluster took the replacement brick
		// the number of bricks on the device shouldn't have changed
		bCountAfter := getBrickCounts(t)
		tests.Assert(t,
			bCountBefore[deviceToRemove] == bCountAfter[deviceToRemove],
			"expected before count == after count, got:",
			bCountBefore[deviceToRemove], bCountAfter[deviceToRemove])
	})

	// replaceBrickErrorAfter - inject an error after gluster has replaced
	// the brick to test the recovery path where we keep the new brick
	// and remove the old
	t.Run("replaceBrickErrorAfter", func(t *testing.T) {
		var err error
		resetConfFile()
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.ResultHooks = inj.ResultHooks{
				inj.ResultHook{
					CmdHook: inj.CmdHook{
						Cmd: ".*volume replace-brick.*",
						Reaction: inj.Reaction{
							Err: "owiee!",
						},
					},
					Result: ".*",
				},
			}
		})
		testutils.ServerRestarted(t, heketiServer)
		deviceToRemove := getDeviceToRemove(t)
		defer enableAllDevices(t)

		bCountBefore := getBrickCounts(t)

		err = heketi.DeviceState(deviceToRemove, &api.StateRequest{
			api.EntryStateOffline,
			api.HealCheckEnable,
		})
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		err = heketi.DeviceState(deviceToRemove, &api.StateRequest{
			api.EntryStateFailed,
			api.HealCheckEnable,
		})
		tests.Assert(t, err != nil, "expected err != nil, got:", err)

		// there should not be any pending ops in the db
		l, err := heketi.PendingOperationList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l.PendingOperations) == 0,
			"expected len(l.PendingOperations) == 0, got:",
			len(l.PendingOperations))

		// we failed "after" gluster took the replacement brick
		// one brick should have been removed from the device in heketi
		// even if the device has > 1 bricks; we only recover from the
		// single "brick evict" in progress
		bCountAfter := getBrickCounts(t)
		tests.Assert(t,
			(bCountBefore[deviceToRemove]-1) == bCountAfter[deviceToRemove],
			"expected before count - 1 == after count, got:",
			(bCountBefore[deviceToRemove] - 1), bCountAfter[deviceToRemove])
	})
	checkConsistent(t, heketiServer)
}

func enableAllDevices(t *testing.T) {
	topo, err := heketi.TopologyInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	for _, c := range topo.ClusterList {
		for _, n := range c.Nodes {
			for _, d := range n.DevicesInfo {
				switch d.State {
				case api.EntryStateFailed:
					err = heketi.DeviceState(d.Id, &api.StateRequest{
						api.EntryStateOffline,
						api.HealCheckEnable,
					})
					tests.Assert(t, err == nil, "expected err == nil, got:", err)
					fallthrough
				case api.EntryStateOffline:
					err = heketi.DeviceState(d.Id, &api.StateRequest{
						api.EntryStateOnline,
						api.HealCheckEnable,
					})
					tests.Assert(t, err == nil, "expected err == nil, got:", err)
				}
			}
		}
	}
}

func getBrickCounts(t *testing.T) map[string]int {
	topo, err := heketi.TopologyInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	brickCount := map[string]int{}
	for _, c := range topo.ClusterList {
		for _, n := range c.Nodes {
			for _, d := range n.DevicesInfo {
				brickCount[d.Id] += len(d.Bricks)
			}
		}
	}
	return brickCount
}

func getDeviceToRemove(t *testing.T) string {
	brickCount := getBrickCounts(t)
	bestDevice := ""
	for id, count := range brickCount {
		if count > brickCount[bestDevice] {
			bestDevice = id
			logger.Info("device-to-remove candidate: %v", bestDevice)
		}
	}
	tests.Assert(t, bestDevice != "", "failed to find a device with bricks")
	return bestDevice
}

func perDeviceUsedSize(t *testing.T) map[string]uint64 {
	data := map[string]uint64{}
	topo, err := heketi.TopologyInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	for _, c := range topo.ClusterList {
		for _, n := range c.Nodes {
			for _, d := range n.DevicesInfo {
				data[d.Id] += d.Storage.Used
			}
		}
	}
	return data
}
