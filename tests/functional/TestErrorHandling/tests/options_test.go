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
	"os"
	"path"
	"strings"
	"testing"

	"github.com/heketi/tests"

	client "github.com/heketi/heketi/v10/client/api/go-client"
	"github.com/heketi/heketi/v10/middleware"
	"github.com/heketi/heketi/v10/pkg/glusterfs/api"
	"github.com/heketi/heketi/v10/pkg/testutils"
	"github.com/heketi/heketi/v10/server/config"
)

func TestBlockVolumeAllocDefaults(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")
	origConf := path.Join(heketiServer.ServerDir, heketiServer.ConfPath)

	heketiServer.ConfPath = tests.Tempfile()
	defer os.Remove(heketiServer.ConfPath)
	CopyFile(origConf, heketiServer.ConfPath)

	defer func() {
		CopyFile(origConf, heketiServer.ConfPath)
		testutils.ServerRestarted(t, heketiServer)
		testCluster.Teardown(t)
		testutils.ServerStopped(t, heketiServer)
	}()

	testutils.ServerStarted(t, heketiServer)
	heketiServer.KeepDB = true
	testCluster.Setup(t, 3, 3)

	blockReq := &api.BlockVolumeCreateRequest{}
	blockReq.Size = 3
	blockReq.Hacount = 3

	// create a volume (and BHV) with default unset
	_, err := heketi.BlockVolumeCreate(blockReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	t.Run("AllocFull", func(t *testing.T) {
		// explicitly set the default to "full"
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.SshConfig.BlockVolumePrealloc = "full"
		})
		testutils.ServerRestarted(t, heketiServer)

		_, err := heketi.BlockVolumeCreate(blockReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})
	t.Run("AllocNo", func(t *testing.T) {
		// explicitly set the default to "full"
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.SshConfig.BlockVolumePrealloc = "no"
		})
		testutils.ServerRestarted(t, heketiServer)

		_, err := heketi.BlockVolumeCreate(blockReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})
	t.Run("AllocInvalid", func(t *testing.T) {
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.SshConfig.BlockVolumePrealloc = "XXXfoobarXXX"
		})
		testutils.ServerRestarted(t, heketiServer)

		_, err := heketi.BlockVolumeCreate(blockReq)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)

		// assert that no pending ops remain
		l, err := heketi.PendingOperationList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l.PendingOperations) == 0,
			"expected len(l.PendingOperations) == 0, got:", len(l.PendingOperations))
	})
}

func TestServerStateDefaults(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")
	origConf := path.Join(heketiServer.ServerDir, heketiServer.ConfPath)

	heketiServer.ConfPath = tests.Tempfile()
	defer os.Remove(heketiServer.ConfPath)
	CopyFile(origConf, heketiServer.ConfPath)

	defer func() {
		CopyFile(origConf, heketiServer.ConfPath)
		testutils.ServerRestarted(t, heketiServer)
		testCluster.Teardown(t)
		testutils.ServerStopped(t, heketiServer)
	}()

	testutils.ServerStarted(t, heketiServer)
	heketiServer.KeepDB = true
	testCluster.Setup(t, 3, 3)

	t.Run("ReadOnly", func(t *testing.T) {
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.DefaultState = "read-only"
		})
		testutils.ServerRestarted(t, heketiServer)

		req := &api.VolumeCreateRequest{}
		req.Size = 1
		_, err := heketi.VolumeCreate(req)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
		tests.Assert(t, strings.Contains(err.Error(), "maintenance"),
			"expect err contains 'maintenance', got:", err)
	})
	t.Run("LocalOnly", func(t *testing.T) {
		// unfortunately we don't have a way to really verify that local-client
		// is local only here in these tests... :-\
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.DefaultState = "local-client"
		})
		testutils.ServerRestarted(t, heketiServer)

		req := &api.VolumeCreateRequest{}
		req.Size = 1
		_, err := heketi.VolumeCreate(req)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})
	t.Run("Normal", func(t *testing.T) {
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.DefaultState = "normal"
		})
		testutils.ServerRestarted(t, heketiServer)

		req := &api.VolumeCreateRequest{}
		req.Size = 1
		_, err := heketi.VolumeCreate(req)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})
	t.Run("Invalid", func(t *testing.T) {
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.DefaultState = "blatt"
		})
		testutils.ServerStopped(t, heketiServer)
		err := heketiServer.Start()
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
		isAlive := heketiServer.IsAlive()
		tests.Assert(t, !isAlive, "expected isAlive == false")
	})
}

func TestServerAuthConfig(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")
	origConf := path.Join(heketiServer.ServerDir, heketiServer.ConfPath)

	heketiServer.ConfPath = tests.Tempfile()
	defer os.Remove(heketiServer.ConfPath)
	CopyFile(origConf, heketiServer.ConfPath)

	defer func() {
		heketiServer.DisableAuth = true
		CopyFile(origConf, heketiServer.ConfPath)
		testutils.ServerRestarted(t, heketiServer)
		testCluster.Teardown(t)
		testutils.ServerStopped(t, heketiServer)
	}()

	testutils.ServerStarted(t, heketiServer)
	heketiServer.KeepDB = true
	testCluster.Setup(t, 3, 1)

	t.Run("StartServerAuthNoConfig", func(t *testing.T) {
		testutils.ServerStopped(t, heketiServer)
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.JwtConfig = middleware.JwtAuthConfig{}
		})
		heketiServer.DisableAuth = false

		err := heketiServer.Start()
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
		isAlive := heketiServer.IsAlive()
		tests.Assert(t, !isAlive, "expected isAlive == false")
	})

	t.Run("StartServerAuth", func(t *testing.T) {
		// start server with auth (default)
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.JwtConfig = middleware.JwtAuthConfig{
				Admin: middleware.Issuer{"snivlem"},
				User:  middleware.Issuer{"neew"},
			}
		})
		heketiServer.DisableAuth = false
		testutils.ServerRestarted(t, heketiServer)

		// verify a client without auth can't connect
		copts := client.DefaultClientOptions()
		hc := client.NewClientWithOptions(
			testCluster.HeketiUrl, "", "", copts)

		_, err := hc.TopologyInfo()
		tests.Assert(t, err != nil, "expected err != nil, got:", err)

		// verify a client with proper auth can connect
		hc2 := client.NewClientWithOptions(
			testCluster.HeketiUrl, "admin", "snivlem", copts)

		_, err = hc2.TopologyInfo()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		// verify a client with incorrect auth can't connect
		hc3 := client.NewClientWithOptions(
			testCluster.HeketiUrl, "admin", "whoopsie", copts)

		_, err = hc3.TopologyInfo()
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
	})

	t.Run("StartServerDisableAuth", func(t *testing.T) {
		// start server without auth
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.JwtConfig = middleware.JwtAuthConfig{}
		})
		heketiServer.DisableAuth = true
		testutils.ServerRestarted(t, heketiServer)

		// verify a client without auth connects
		copts := client.DefaultClientOptions()
		hc := client.NewClientWithOptions(
			testCluster.HeketiUrl, "", "", copts)

		_, err := hc.TopologyInfo()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		// verify a client with auth params is ok (ignored)
		hc2 := client.NewClientWithOptions(
			testCluster.HeketiUrl, "admin", "snivlem", copts)

		_, err = hc2.TopologyInfo()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		// verify a client with auth params is ok (ignored)
		hc3 := client.NewClientWithOptions(
			testCluster.HeketiUrl, "admin", "whoopsie", copts)

		_, err = hc3.TopologyInfo()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})
}

func TestLVMWrapper(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")
	origConf := path.Join(heketiServer.ServerDir, heketiServer.ConfPath)

	heketiServer.ConfPath = tests.Tempfile()
	defer os.Remove(heketiServer.ConfPath)
	CopyFile(origConf, heketiServer.ConfPath)

	defer func() {
		CopyFile(origConf, heketiServer.ConfPath)
		testutils.ServerRestarted(t, heketiServer)
		testCluster.Teardown(t)
		testutils.ServerStopped(t, heketiServer)
	}()

	testutils.ServerStarted(t, heketiServer)
	heketiServer.KeepDB = true
	testCluster.Setup(t, 3, 3)

	req := &api.VolumeCreateRequest{}
	req.Size = 1

	t.Run("NoLVMWrapper", func(t *testing.T) {
		// explicitly set the wrapper to ""
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.SshConfig.LVMWrapper = ""
		})
		testutils.ServerRestarted(t, heketiServer)

		_, err := heketi.VolumeCreate(req)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})
	t.Run("NSEnterLVMWrapper", func(t *testing.T) {
		// explicitly set the wrapper to "nsenter" withou arguments,
		// stays in the same namespace, should work like no wrapper is set
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.SshConfig.LVMWrapper = "nsenter"
		})
		testutils.ServerRestarted(t, heketiServer)

		_, err := heketi.VolumeCreate(req)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})
	t.Run("BrokenLVMWrapper", func(t *testing.T) {
		// explicitly set the wrapper to some non-existing executable, should fail
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.SshConfig.LVMWrapper = "/no/such/script"
		})
		testutils.ServerRestarted(t, heketiServer)

		_, err := heketi.VolumeCreate(req)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)

		// assert that no pending ops remain
		l, err := heketi.PendingOperationList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l.PendingOperations) == 0,
			"expected len(l.PendingOperations) == 0, got:", len(l.PendingOperations))
	})
}
