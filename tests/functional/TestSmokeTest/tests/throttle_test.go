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
	"sync"
	"testing"

	client "github.com/heketi/heketi/client/api/go-client"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/utils"

	"github.com/heketi/tests"
)

func TestThrottledOps(t *testing.T) {

	teardownCluster(t)
	setupCluster(t, 3, 8)
	defer teardownCluster(t)

	t.Run("VolumeCreate", testThrottledVolumeCreate)
	teardownVolumes(t)
	t.Run("VolumeCreateFails", testThrottledVolumeCreateFails)
}

func testThrottledVolumeCreate(t *testing.T) {
	// create a client with internal retries disabled
	// we will be able to use this to test that the server returned
	// 429 error responses
	hc := client.NewClientWithOptions(heketiUrl, "", "", client.ClientOptions{
		RetryEnabled: false,
	})

	oi, err := hc.OperationsInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, oi.Total == 0, "expected oi.Total == 0, got", oi.Total)
	tests.Assert(t, oi.InFlight == 0, "expected oi.InFlight == 0, got", oi.Total)

	l := sync.Mutex{}
	errCount := 0
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 2
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3

	// create a bunch of volume requests at once
	sg := utils.NewStatusGroup()
	for i := 0; i < 12; i++ {
		sg.Add(1)
		go func() {
			defer sg.Done()
			_, err := hc.VolumeCreate(volReq)
			if err != nil {
				l.Lock()
				defer l.Unlock()
				errCount++
			}
			sg.Err(err)
		}()
	}

	sg.Result()
	tests.Assert(t, errCount > 1, "expected errCount > 1, got:", errCount)

	oi, err = hc.OperationsInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, oi.Total == 0, "expected oi.Total == 0, got", oi.Total)
	tests.Assert(t, oi.InFlight == 0, "expected oi.InFlight == 0, got", oi.Total)

	volumes, err := heketi.VolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(volumes.Volumes) == 5,
		"expected len(volumes.Volumes) == 5, got:", len(volumes.Volumes))
}

func testThrottledVolumeCreateFails(t *testing.T) {
	oi, err := heketi.OperationsInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, oi.Total == 0, "expected oi.Total == 0, got", oi.Total)
	tests.Assert(t, oi.InFlight == 0, "expected oi.InFlight == 0, got", oi.Total)

	l := sync.Mutex{}
	errCount := 0
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 300
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3

	// create a bunch of volume requests at once
	sg := utils.NewStatusGroup()
	for i := 0; i < 25; i++ {
		sg.Add(1)
		go func() {
			defer sg.Done()
			_, err := heketi.VolumeCreate(volReq)
			if err != nil {
				l.Lock()
				defer l.Unlock()
				errCount++
			}
			sg.Err(err)
		}()
	}

	sg.Result()
	tests.Assert(t, errCount > 1, "expected errCount > 1, got:", errCount)

	// there should not be any ops on the server now
	oi, err = heketi.OperationsInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, oi.Total == 0, "expected oi.Total == 0, got", oi.Total)
	tests.Assert(t, oi.InFlight == 0, "expected oi.InFlight == 0, got", oi.Total)

	// we use a count of the volumes as a proxy for determining how
	// many volume requests failed. We made 25 requests but should
	// only have been able to allocate a few. This tests two things:
	// - when the Operation's build step fails it decrements the op count
	// - that the scenario where large amount of requests come into
	//   the server and only a portion of them can ultimately be done
	volumes, err := heketi.VolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(volumes.Volumes) >= 10,
		"expected len(volumes.Volumes) == 5, got:", len(volumes.Volumes))
	tests.Assert(t, len(volumes.Volumes) < 20,
		"expected len(volumes.Volumes) == 5, got:", len(volumes.Volumes))
}
