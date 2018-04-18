// +build functional

//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package functional

import (
	"testing"

	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/utils"
	"github.com/heketi/tests"
)

func TestReqTrottling(t *testing.T) {
	setupCluster(t, 4, 8)
	defer teardownCluster(t)
	t.Run("testReqTrottlingCreateVolume", testReqTrottlingCreateVolume)

}

func testReqTrottlingCreateVolume(t *testing.T) {
	vl, err := heketi.VolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(vl.Volumes) == 0,
		"expected len(vl.Volumes) == 0, got:", len(vl.Volumes))

	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 1
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3
	wg := utils.NewStatusGroup()
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(t *testing.T, wg *utils.StatusGroup) {
			defer wg.Done()
			_, err := heketi.VolumeCreate(volReq)
			wg.Err(err)

		}(t, wg)
	}

	err = wg.Result()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	vl, err = heketi.VolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(vl.Volumes) == 30,
		"expected len(vl.Volumes) == 30, got:", len(vl.Volumes))

	throttlingteardownVolumes(t)
}

func throttlingteardownVolumes(t *testing.T) {
	PauseBeforeTeardown()
	clusters, err := heketi.ClusterList()
	tests.Assert(t, err == nil, err)
	sg := utils.NewStatusGroup()
	for _, cluster := range clusters.Clusters {
		clusterInfo, err := heketi.ClusterInfo(cluster)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		for _, volume := range clusterInfo.Volumes {
			sg.Add(1)
			go func(t *testing.T, sg *utils.StatusGroup, volume string) {

				defer sg.Done()
				err := heketi.VolumeDelete(volume)
				sg.Err(err)
			}(t, sg, volume)
		}
	}
	err = sg.Result()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	vl, err := heketi.VolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(vl.Volumes) == 0,
		"expected len(vl.Volumes) == 0, got:", len(vl.Volumes))
}
