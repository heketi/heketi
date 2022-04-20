//go:build functional
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
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/heketi/tests"

	inj "github.com/heketi/heketi/v10/executors/injectexec"
	"github.com/heketi/heketi/v10/pkg/glusterfs/api"
	rex "github.com/heketi/heketi/v10/pkg/remoteexec"
	"github.com/heketi/heketi/v10/pkg/remoteexec/ssh"
	"github.com/heketi/heketi/v10/pkg/testutils"
	"github.com/heketi/heketi/v10/server/config"
)

func TestBrickEvict(t *testing.T) {
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
	testCluster.Setup(t, 3, 2)

	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 1
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3

	t.Run("basicEvict", func(t *testing.T) {
		resetConf()

		vinfo1, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		aggUsed := deviceAggregateUsedSize(t)

		toEvict := vinfo1.Bricks[0].Id
		err = heketi.BrickEvict(toEvict, nil)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		vinfo2, err := heketi.VolumeInfo(vinfo1.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		oldIds := map[string]bool{}
		for _, b := range vinfo1.Bricks {
			oldIds[b.Id] = true
		}
		tests.Assert(t, len(oldIds) == 3,
			"expected len(oldIds) == 3, got:", len(oldIds))
		found := 0
		for _, b := range vinfo2.Bricks {
			if oldIds[b.Id] {
				found++
			} else {
				tests.Assert(t, b.Id != toEvict,
					"expected b.Id != toEvict, got", b.Id, toEvict)
			}
		}
		tests.Assert(t, found == 2)
		newAggUsed := deviceAggregateUsedSize(t)
		tests.Assert(t, newAggUsed == aggUsed,
			"expected device used aggregate sizes to match", newAggUsed, aggUsed)
		checkConsistent(t, heketiServer)
	})
	t.Run("tripleEvict", func(t *testing.T) {
		resetConf()

		vinfo1, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		aggUsed := deviceAggregateUsedSize(t)

		evictBricks := map[string]bool{}
		for _, b := range vinfo1.Bricks {
			evictBricks[b.Id] = true
		}
		for toEvict := range evictBricks {
			err = heketi.BrickEvict(toEvict, nil)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
		}

		vinfo2, err := heketi.VolumeInfo(vinfo1.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		for _, b := range vinfo2.Bricks {
			tests.Assert(t, !evictBricks[b.Id],
				"expected brick id not in evictBricks map", b.Id, evictBricks)
		}
		newAggUsed := deviceAggregateUsedSize(t)
		tests.Assert(t, newAggUsed == aggUsed,
			"expected device used aggregate sizes to match", newAggUsed, aggUsed)
		checkConsistent(t, heketiServer)
	})
	t.Run("tripleEvictTryParallel", func(t *testing.T) {
		resetConf()

		vinfo1, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		aggUsed := deviceAggregateUsedSize(t)

		evictBricks := map[string]bool{}
		for _, b := range vinfo1.Bricks {
			evictBricks[b.Id] = true
		}

		evictResults := map[string]error{}
		wg := sync.WaitGroup{}
		l := sync.Mutex{}
		for toEvict := range evictBricks {
			wg.Add(1)
			go func(toEvict string) {
				defer wg.Done()
				err = heketi.BrickEvict(toEvict, nil)
				l.Lock()
				defer l.Unlock()
				evictResults[toEvict] = err
			}(toEvict)
		}
		wg.Wait()

		tests.Assert(t, len(evictResults) == 3,
			"expected len(evictResults) == 3, got:", len(evictResults))
		var errCount int
		for _, err := range evictResults {
			if err != nil {
				errCount += 1
			}
		}
		tests.Assert(t, errCount == 2, "expected errCount == 2, got:", errCount)

		vinfo2, err := heketi.VolumeInfo(vinfo1.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		for _, b := range vinfo2.Bricks {
			if evictResults[b.Id] == nil {
				tests.Assert(t, !evictBricks[b.Id],
					"expected brick id not in evictBricks map", b.Id, evictBricks)
			} else {
				tests.Assert(t, evictBricks[b.Id],
					"expected brick id in evictBricks map", b.Id, evictBricks)
			}
		}
		newAggUsed := deviceAggregateUsedSize(t)
		tests.Assert(t, newAggUsed == aggUsed,
			"expected device used aggregate sizes to match", newAggUsed, aggUsed)
		checkConsistent(t, heketiServer)
	})

	t.Run("simpleEarlyError", func(t *testing.T) {
		resetConf()

		vinfo1, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

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

		toEvict := vinfo1.Bricks[0].Id
		err = heketi.BrickEvict(toEvict, nil)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)

		vinfo2, err := heketi.VolumeInfo(vinfo1.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		for i, b := range vinfo2.Bricks {
			tests.Assert(t, vinfo1.Bricks[i].Id == b.Id,
				"expected brick id to match, got:",
				vinfo1.Bricks[i].Id, b.Id)
		}
		checkConsistent(t, heketiServer)
	})

	t.Run("errorAfterReplace", func(t *testing.T) {
		resetConf()

		vinfo1, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		aggUsed := deviceAggregateUsedSize(t)

		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.ResultHooks = inj.ResultHooks{
				inj.ResultHook{
					CmdHook: inj.CmdHook{
						Cmd: ".*volume replace-brick.*",
						Reaction: inj.Reaction{
							Err: "thwack!",
						},
					},
					Result: ".*",
				},
			}
		})
		testutils.ServerRestarted(t, heketiServer)

		toEvict := vinfo1.Bricks[0].Id
		err = heketi.BrickEvict(toEvict, nil)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)

		// in this case even tho the operation "failed" the state of
		// the gluster system is as if the replace succeeded. The state
		// of the system after the operation should be similar to
		// the state of a successful op.

		vinfo2, err := heketi.VolumeInfo(vinfo1.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		oldIds := map[string]bool{}
		for _, b := range vinfo1.Bricks {
			oldIds[b.Id] = true
		}
		tests.Assert(t, len(oldIds) == 3,
			"expected len(oldIds) == 3, got:", len(oldIds))
		found := 0
		for _, b := range vinfo2.Bricks {
			if oldIds[b.Id] {
				found++
			} else {
				tests.Assert(t, b.Id != toEvict,
					"expected b.Id != toEvict, got", b.Id, toEvict)
			}
		}
		tests.Assert(t, found == 2)
		newAggUsed := deviceAggregateUsedSize(t)
		tests.Assert(t, newAggUsed == aggUsed,
			"expected device used aggregate sizes to match", newAggUsed, aggUsed)
		checkConsistent(t, heketiServer)
	})

	t.Run("lvremoveErrorRecover", func(t *testing.T) {
		resetConf()

		vinfo1, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		aggUsed := deviceAggregateUsedSize(t)

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

		toEvict := vinfo1.Bricks[0].Id
		err = heketi.BrickEvict(toEvict, nil)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)

		// the pending op should remain because lvremove is in the
		// clean/rollback path.
		l, err := heketi.PendingOperationList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l.PendingOperations) == 1,
			"expected len(l.PendingOperations)t == 1, got:",
			len(l.PendingOperations))

		// reset server behavior
		resetConf()

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
			"expected len(l.PendingOperations)t == 0, got:",
			len(l.PendingOperations))

		// The state of the system after the clean up should be similar to the
		// state of a successful run
		vinfo2, err := heketi.VolumeInfo(vinfo1.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		oldIds := map[string]bool{}
		for _, b := range vinfo1.Bricks {
			oldIds[b.Id] = true
		}
		tests.Assert(t, len(oldIds) == 3,
			"expected len(oldIds) == 3, got:", len(oldIds))
		found := 0
		for _, b := range vinfo2.Bricks {
			if oldIds[b.Id] {
				found++
			} else {
				tests.Assert(t, b.Id != toEvict,
					"expected b.Id != toEvict, got", b.Id, toEvict)
			}
		}
		tests.Assert(t, found == 2)
		newAggUsed := deviceAggregateUsedSize(t)
		tests.Assert(t, newAggUsed == aggUsed,
			"expected device used aggregate sizes to match", newAggUsed, aggUsed)
		checkConsistent(t, heketiServer)
	})

	t.Run("handleVolumeMissingInGluster", func(t *testing.T) {
		na := testutils.RequireNodeAccess(t)
		exec := na.Use(logger)
		resetConf()

		vinfo1, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		err = rmVolume(vinfo1.Id, testCluster.SshHost(0), exec)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		toEvict := vinfo1.Bricks[0].Id
		err = heketi.BrickEvict(toEvict, nil)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
		// before fix, heketi would panic. Assert heketi is alive (didn't panic)
		tests.Assert(t, heketiServer.IsAlive(),
			"server is not alive; expected server not to panic")
	})
}

func deviceAggregateUsedSize(t *testing.T) uint64 {
	var agg uint64
	topo, err := heketi.TopologyInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	for _, c := range topo.ClusterList {
		for _, n := range c.Nodes {
			for _, d := range n.DevicesInfo {
				agg += d.Storage.Used
			}
		}
	}
	return agg
}

func rmVolume(v string, node string, exec *ssh.SshExec) error {
	vname := fmt.Sprintf("vol_%s", v)
	stopVol := fmt.Sprintf("gluster --mode=script volume stop %v", vname)
	delVol := fmt.Sprintf("gluster --mode=script volume delete %v", vname)
	cmds := rex.Cmds{
		rex.ToCmd(stopVol),
		rex.ToCmd(delVol),
	}
	err := rex.AnyError(exec.ExecCommands(node, cmds, 10, true))
	return err
}
