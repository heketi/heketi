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
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/utils/ssh"
	"github.com/heketi/tests"
)

func teardownVolumes(t *testing.T) {
	PauseBeforeTeardown()
	clusters, err := heketi.ClusterList()
	tests.Assert(t, err == nil, err)

	for _, cluster := range clusters.Clusters {
		clusterInfo, err := heketi.ClusterInfo(cluster)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		// Delete volumes in this cluster
		for _, volume := range clusterInfo.Volumes {
			err := heketi.VolumeDelete(volume)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
		}
	}

	vl, err := heketi.VolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(vl.Volumes) == 0,
		"expected len(vl.Volumes) == 0, got:", len(vl.Volumes))
}

func PauseBeforeTeardown() {
	s := os.Getenv("HEKETI_TEST_PAUSE_BEFORE_TEARDOWN")
	if len(s) == 0 {
		return
	}
	count, err := strconv.Atoi(s)
	if err != nil {
		return
	}
	fmt.Println("Continuing in ...")
	for i := 0; i < count; i++ {
		fmt.Println("   ", count-i)
		time.Sleep(time.Second)
	}
}

func TestArbiterFlatCluster(t *testing.T) {
	setupCluster(t, 4, 8)
	defer teardownCluster(t)
	t.Run("testArbiterCreateSimple", testArbiterCreateSimple)
	teardownVolumes(t)
	t.Run("testArbiterCreateAndVerify", testArbiterCreateAndVerify)
	teardownVolumes(t)
	t.Run("testNonArbiterIsNotArbiter", testNonArbiterIsNotArbiter)
	teardownVolumes(t)
	t.Run("testArbiterReplaceDataBrick", testArbiterReplaceDataBrick)
	teardownVolumes(t)
	t.Run("testArbiterReplaceArbiterBrick", testArbiterReplaceArbiterBrick)
}

func testArbiterCreateSimple(t *testing.T) {
	vl, err := heketi.VolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(vl.Volumes) == 0,
		"expected len(vl.Volumes) == 0, got:", len(vl.Volumes))

	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 10
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3
	volReq.GlusterVolumeOptions = []string{"user.heketi.arbiter true"}

	_, err = heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vl, err = heketi.VolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(vl.Volumes) == 1,
		"expected len(vl.Volumes) == 1, got:", len(vl.Volumes))
}

func testArbiterCreateAndVerify(t *testing.T) {
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 10
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3
	volReq.GlusterVolumeOptions = []string{"user.heketi.arbiter true"}

	vcr, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// SSH into system and check that arbiter is really in use
	s := ssh.NewSshExecWithKeyFile(
		logger, "vagrant", "../config/insecure_private_key")
	cmd := []string{
		fmt.Sprintf("gluster volume info %v | grep -q \"^Brick.* .arbiter.\"", vcr.Name),
	}
	_, err = s.ConnectAndExec(storage0ssh, cmd, 10, true)
	tests.Assert(t, err == nil, "No bricks marked as arbiter")
}

// Test that a volume not flagged for arbiter support does
// not have arbiter tagging on gluster side.
func testNonArbiterIsNotArbiter(t *testing.T) {
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 10
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3

	vcr, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// SSH into system and check that arbiter is really in use
	s := ssh.NewSshExecWithKeyFile(
		logger, "vagrant", "../config/insecure_private_key")
	cmd := []string{
		fmt.Sprintf("gluster volume info %v | grep -q \"^Brick.* .arbiter.\"", vcr.Name),
	}
	_, err = s.ConnectAndExec(storage0ssh, cmd, 10, true)
	tests.Assert(t, err != nil, "Bricks marked as arbiter")
}

func testArbiterReplaceDataBrick(t *testing.T) {
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 10
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3
	volReq.GlusterVolumeOptions = []string{"user.heketi.arbiter true"}

	vcr, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// determine a device that a data brick landed on
	size := uint64(0)
	var deviceId string
	for _, b := range vcr.Bricks {
		if b.Size > size {
			deviceId = b.DeviceId
			size = b.Size
		}
	}

	err = heketi.DeviceState(
		deviceId, &api.StateRequest{api.EntryStateOffline})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	defer func() {
		if err = heketi.DeviceState(
			deviceId, &api.StateRequest{api.EntryStateOnline}); err != nil {
			logger.Warning("Failed to return device %v to online state",
				deviceId)
		}
	}()

	err = heketi.DeviceState(
		deviceId, &api.StateRequest{api.EntryStateFailed})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	defer func() {
		if err = heketi.DeviceState(
			deviceId, &api.StateRequest{api.EntryStateOffline}); err != nil {
			logger.Warning("Failed to return device %v to online state",
				deviceId)
		}
	}()

	vi, err := heketi.VolumeInfo(vcr.Id)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	for _, b := range vi.Bricks {
		tests.Assert(t, deviceId != b.DeviceId,
			"device still in use by volume", deviceId)
	}
}

func testArbiterReplaceArbiterBrick(t *testing.T) {
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 10
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3
	volReq.GlusterVolumeOptions = []string{"user.heketi.arbiter true"}

	vcr, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// determine a device that a data brick landed on
	size := uint64(0)
	var deviceId, path string
	for _, b := range vcr.Bricks {
		if size == 0 || b.Size < size {
			deviceId = b.DeviceId
			size = b.Size
			path = b.Path
		}
	}

	// extra confirmation this is the arbiter brick
	s := ssh.NewSshExecWithKeyFile(
		logger, "vagrant", "../config/insecure_private_key")
	cmd := []string{
		fmt.Sprintf("gluster volume info %v | grep \"^Brick.* .arbiter.\"", vcr.Name),
	}
	o, err := s.ConnectAndExec(storage0ssh, cmd, 10, true)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, strings.Contains(o[0], path),
		"expected output to contain brick path",
		"output:", o, "path:", path)

	err = heketi.DeviceState(
		deviceId, &api.StateRequest{api.EntryStateOffline})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	defer func() {
		if err = heketi.DeviceState(
			deviceId, &api.StateRequest{api.EntryStateOnline}); err != nil {
			logger.Warning("Failed to return device %v to online state",
				deviceId)
		}
	}()

	err = heketi.DeviceState(
		deviceId, &api.StateRequest{api.EntryStateFailed})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	defer func() {
		if err = heketi.DeviceState(
			deviceId, &api.StateRequest{api.EntryStateOffline}); err != nil {
			logger.Warning("Failed to return device %v to online state",
				deviceId)
		}
	}()

	vi, err := heketi.VolumeInfo(vcr.Id)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	for _, b := range vi.Bricks {
		tests.Assert(t, deviceId != b.DeviceId,
			"device still in use by volume", deviceId)
	}

	s = ssh.NewSshExecWithKeyFile(
		logger, "vagrant", "../config/insecure_private_key")
	cmd = []string{
		fmt.Sprintf("gluster volume info %v | grep \"^Brick.* .arbiter.\"", vcr.Name),
	}
	o, err = s.ConnectAndExec(storage0ssh, cmd, 10, true)
	tests.Assert(t, !strings.Contains(o[0], path),
		"expected output not to contain old brick path",
		"output:", o, "path:", path)
}
