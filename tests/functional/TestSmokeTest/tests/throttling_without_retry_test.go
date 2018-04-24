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

	client "github.com/heketi/heketi/client/api/go-client"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/utils"
	"github.com/heketi/tests"
)

//Mimic old client with retry  0
func TestTrottling(t *testing.T) {
	//old client with retry  0
	heketi = client.NewClientWithRetry(heketiUrl, "", "", 0)
	setupCluster(t, 4, 8)
	defer teardownCluster(t)
	t.Run("throttlingcreatevolume", throttlingcreatevolume)

}

func throttlingcreatevolume(t *testing.T) {
	vl, err := heketi.VolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(vl.Volumes) == 0,
		"expected len(vl.Volumes) == 0, got:", len(vl.Volumes))

	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 1
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3
	wg := utils.NewStatusGroup()
	for i := 0; i < 25; i++ {
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
	tests.Assert(t, len(vl.Volumes) == 25,
		"expected len(vl.Volumes) == 25, got:", len(vl.Volumes))

	throttlingteardownVolumes(t)
}

func throttlingteardownVolumes(t *testing.T) {
	clusters, err := heketi.ClusterList()
	tests.Assert(t, err == nil, err)
	sg := utils.NewStatusGroup()
	for _, cluster := range clusters.Clusters {
		clusterInfo, err := heketi.ClusterInfo(cluster)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		for _, volume := range clusterInfo.Volumes {
			sg.Add(1)
			go func(sg *utils.StatusGroup, volID string) {

				defer sg.Done()
				err := heketi.VolumeDelete(volID)
				sg.Err(err)
			}(sg, volume)
		}
	}
	err = sg.Result()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	vl, err := heketi.VolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(vl.Volumes) == 0,
		"expected len(vl.Volumes) == 0, got:", len(vl.Volumes))
}
