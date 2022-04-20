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
	"testing"

	"github.com/heketi/heketi/v10/pkg/glusterfs/api"
	"github.com/heketi/heketi/v10/pkg/testutils"
	"github.com/heketi/tests"
)

func TestVolumeCreateTagMatchingRules(t *testing.T) {

	heketiServer := testutils.NewServerCtlFromEnv("..")
	origConf := path.Join(heketiServer.ServerDir, heketiServer.ConfPath)

	heketiServer.ConfPath = tests.Tempfile()
	defer os.Remove(heketiServer.ConfPath)
	CopyFile(origConf, heketiServer.ConfPath)

	tce := testCluster.Copy()
	tce.Update()
	defer func() {
		CopyFile(origConf, heketiServer.ConfPath)
		testutils.ServerRestarted(t, heketiServer)
		tce.Teardown(t)
		testutils.ServerStopped(t, heketiServer)
	}()

	tce.CustomizeNodeRequest = func(i int, req *api.NodeAddRequest) {
		req.Zone = 1
	}
	testutils.ServerStarted(t, heketiServer)
	heketiServer.KeepDB = true
	tce.Setup(t, 3, 4)

	cl, err := heketi.ClusterList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(cl.Clusters) > 0,
		"expected len(cl.Clusters) > 0, got:", len(cl.Clusters))
	clusterId := cl.Clusters[0]
	clusterInfo, err := heketi.ClusterInfo(clusterId)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	t.Run("testWithTagOnNodes", func(t *testing.T) {
		var err error
		err = heketi.NodeSetTags(clusterInfo.Nodes[0], &api.TagsChangeRequest{
			Tags:   map[string]string{"flavor": "banana"},
			Change: api.SetTags,
		})
		tests.Assert(t, err == nil, "failed to set tags", err)
		err = heketi.NodeSetTags(clusterInfo.Nodes[1], &api.TagsChangeRequest{
			Tags:   map[string]string{"flavor": "banana"},
			Change: api.SetTags,
		})
		tests.Assert(t, err == nil, "failed to set tags", err)
		err = heketi.NodeSetTags(clusterInfo.Nodes[2], &api.TagsChangeRequest{
			Tags:   map[string]string{"flavor": "banana"},
			Change: api.SetTags,
		})
		tests.Assert(t, err == nil, "failed to set tags", err)

		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 1
		volReq.Durability.Type = api.DurabilityReplicate
		volReq.Durability.Replicate.Replica = 3
		volReq.GlusterVolumeOptions = []string{
			"user.heketi.device-tag-match flavor=banana"}
		_, err = heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})
	t.Run("inverseMatch", func(t *testing.T) {
		var err error
		err = heketi.NodeSetTags(clusterInfo.Nodes[0], &api.TagsChangeRequest{
			Tags:   map[string]string{"flavor": "banana"},
			Change: api.SetTags,
		})
		tests.Assert(t, err == nil, "failed to set tags", err)
		err = heketi.NodeSetTags(clusterInfo.Nodes[1], &api.TagsChangeRequest{
			Tags:   map[string]string{"flavor": "banana"},
			Change: api.SetTags,
		})
		tests.Assert(t, err == nil, "failed to set tags", err)
		err = heketi.NodeSetTags(clusterInfo.Nodes[2], &api.TagsChangeRequest{
			Tags:   map[string]string{"flavor": "banana"},
			Change: api.SetTags,
		})
		tests.Assert(t, err == nil, "failed to set tags", err)

		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 1
		volReq.Durability.Type = api.DurabilityReplicate
		volReq.Durability.Replicate.Replica = 3
		volReq.GlusterVolumeOptions = []string{
			"user.heketi.device-tag-match flavor!=vanilla"}
		_, err = heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})
	t.Run("invalidMatch", func(t *testing.T) {
		var err error
		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 1
		volReq.Durability.Type = api.DurabilityReplicate
		volReq.Durability.Replicate.Replica = 3
		volReq.GlusterVolumeOptions = []string{
			"user.heketi.device-tag-match way=no"}
		_, err = heketi.VolumeCreate(volReq)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
	})
	t.Run("incompleteMatch", func(t *testing.T) {
		var err error
		err = heketi.NodeSetTags(clusterInfo.Nodes[0], &api.TagsChangeRequest{
			Tags:   map[string]string{"flavor": "banana"},
			Change: api.SetTags,
		})
		tests.Assert(t, err == nil, "failed to set tags", err)
		err = heketi.NodeSetTags(clusterInfo.Nodes[1], &api.TagsChangeRequest{
			Tags:   map[string]string{"flavor": "banana"},
			Change: api.SetTags,
		})
		tests.Assert(t, err == nil, "failed to set tags", err)
		err = heketi.NodeSetTags(clusterInfo.Nodes[2], &api.TagsChangeRequest{
			Tags:   map[string]string{"flavor": "cherry"},
			Change: api.SetTags,
		})
		tests.Assert(t, err == nil, "failed to set tags", err)

		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 1
		volReq.Durability.Type = api.DurabilityReplicate
		volReq.Durability.Replicate.Replica = 3
		volReq.GlusterVolumeOptions = []string{
			"user.heketi.device-tag-match flavor=banana"}
		_, err = heketi.VolumeCreate(volReq)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
	})
	t.Run("incompleteInverseMatch", func(t *testing.T) {
		var err error
		err = heketi.NodeSetTags(clusterInfo.Nodes[0], &api.TagsChangeRequest{
			Tags:   map[string]string{"flavor": "banana"},
			Change: api.SetTags,
		})
		tests.Assert(t, err == nil, "failed to set tags", err)
		err = heketi.NodeSetTags(clusterInfo.Nodes[1], &api.TagsChangeRequest{
			Tags:   map[string]string{"flavor": "banana"},
			Change: api.SetTags,
		})
		tests.Assert(t, err == nil, "failed to set tags", err)
		err = heketi.NodeSetTags(clusterInfo.Nodes[2], &api.TagsChangeRequest{
			Tags:   map[string]string{"flavor": "cherry"},
			Change: api.SetTags,
		})
		tests.Assert(t, err == nil, "failed to set tags", err)

		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 1
		volReq.Durability.Type = api.DurabilityReplicate
		volReq.Durability.Replicate.Replica = 3
		volReq.GlusterVolumeOptions = []string{
			"user.heketi.device-tag-match flavor!=banana"}
		_, err = heketi.VolumeCreate(volReq)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
	})

	t.Run("testWithTagsOnDevices", func(t *testing.T) {
		var err error
		for _, nid := range clusterInfo.Nodes {
			ni, err := heketi.NodeInfo(nid)
			tests.Assert(t, err == nil, "failed to set tags", err)

			for i, di := range ni.DevicesInfo {
				var tval string
				switch i {
				case 0:
					tval = "red"
				case 1:
					tval = "green"
				case 2:
					tval = "ecru"
				}
				err = heketi.DeviceSetTags(di.Id, &api.TagsChangeRequest{
					Tags:   map[string]string{"color": tval},
					Change: api.SetTags,
				})
				tests.Assert(t, err == nil, "failed to set tags", err)
			}
		}

		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 1
		volReq.Durability.Type = api.DurabilityReplicate
		volReq.Durability.Replicate.Replica = 3
		// there are sufficient red devices
		volReq.GlusterVolumeOptions = []string{
			"user.heketi.device-tag-match color=red"}
		_, err = heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		// there are sufficient non-ecru devices
		volReq.GlusterVolumeOptions = []string{
			"user.heketi.device-tag-match color!=ecru"}
		_, err = heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		// there are no mauve devices
		volReq.GlusterVolumeOptions = []string{
			"user.heketi.device-tag-match color=mauve"}
		_, err = heketi.VolumeCreate(volReq)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
	})

	t.Run("tagsAndZoneChecking", func(t *testing.T) {
		var err error
		err = heketi.NodeSetTags(clusterInfo.Nodes[0], &api.TagsChangeRequest{
			Tags:   map[string]string{"flavor": "grape"},
			Change: api.SetTags,
		})
		tests.Assert(t, err == nil, "failed to set tags", err)
		err = heketi.NodeSetTags(clusterInfo.Nodes[1], &api.TagsChangeRequest{
			Tags:   map[string]string{"flavor": "grape"},
			Change: api.SetTags,
		})
		tests.Assert(t, err == nil, "failed to set tags", err)
		err = heketi.NodeSetTags(clusterInfo.Nodes[2], &api.TagsChangeRequest{
			Tags:   map[string]string{"flavor": "grape"},
			Change: api.SetTags,
		})
		tests.Assert(t, err == nil, "failed to set tags", err)

		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 1
		volReq.Durability.Type = api.DurabilityReplicate
		volReq.Durability.Replicate.Replica = 3
		// tag match is OK
		volReq.GlusterVolumeOptions = []string{
			"user.heketi.device-tag-match flavor=grape"}
		_, err = heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		// we set this test cluster up with one zone. zone check will fail
		volReq.GlusterVolumeOptions = []string{
			"user.heketi.zone-checking strict",
			"user.heketi.device-tag-match flavor=grape"}
		_, err = heketi.VolumeCreate(volReq)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
	})
}
