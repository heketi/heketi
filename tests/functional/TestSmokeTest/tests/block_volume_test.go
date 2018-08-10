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
	"github.com/heketi/tests"
)

func TestBlockVolumeOperation(t *testing.T) {

	// Setup the VM storage topology
	setupCluster(t, 3, 4)
	defer teardownCluster(t)

	defer teardownBlock(t)

	req := &api.BlockVolumeCreateRequest{}
	//check it is not possible to create block volume if  size is greated then block hosting volume
	req.Size = 101

	_, err := heketi.BlockVolumeCreate(req)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)

	//check it is not possible to create block volume as same size of block hosting volume
	req.Size = 100
	_, err = heketi.BlockVolumeCreate(req)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)

	// 1% is reserved in blockhosting volume, we should be able to create
	//block volumes of total size 98 GB
	req.Size = 2
	for i := 1; i <= 49; i++ {
		vol, err := heketi.BlockVolumeCreate(req)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, vol.Size == 2, "expected vol.Size == 2 got:", vol.Size)
	}

	volList, err := heketi.BlockVolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(volList.BlockVolumes) == 49, "expected len(volList.BlockVolumes) == 49 got:", len(volList.BlockVolumes))

	for _, ID := range volList.BlockVolumes {
		volInfo, err := heketi.BlockVolumeInfo(ID)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, volInfo.Size == 2, "expected volInfo.Size == 2 got:", volInfo.Size)

		err = heketi.BlockVolumeDelete(ID)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

}

func teardownBlock(t *testing.T) {

	volList, err := heketi.BlockVolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	for _, ID := range volList.BlockVolumes {
		err = heketi.BlockVolumeDelete(ID)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}
}
