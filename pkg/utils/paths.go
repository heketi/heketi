//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package utils

import (
	"path"
)

const (
	brickMountPointRoot = "/var/lib/heketi/mounts"
	deviceMapperRoot    = "/dev/mapper"
)

// VgIdToName return the string to be used for the name of
// an LVM VG given the id of the vg.
func VgIdToName(vgId string) string {
	return "vg_" + vgId
}

// BrickIdToName returns the string to be used for the
// name of the brick when used in paths or lvm device names.
func BrickIdToName(brickId string) string {
	return "brick_" + brickId
}

// BrickIdToThinPoolName returns the string to be used for
// a LVM thin-pool name for a given brick id.
func BrickIdToThinPoolName(brickId string) string {
	return "tp_" + brickId
}

// BrickMountPoint returns the path of a directory
// where a brick is to be mounted.
func BrickMountPoint(vgId, brickId string) string {
	return path.Join(
		brickMountPointRoot,
		VgIdToName(vgId),
		BrickIdToName(brickId))
}

// BrickMountPointParent returns the path of the parent
// directory where a brick is to be mounted.
func BrickMountPointParent(vgId string) string {
	return path.Join(
		brickMountPointRoot,
		VgIdToName(vgId))
}

// BrickThinLvName returns the name of the thin-pool LV
// for a brick.
func BrickThinLvName(vgId, brickId string) string {
	return path.Join(
		VgIdToName(vgId),
		BrickIdToThinPoolName(brickId))
}

// BrickDevNode returns the path to the device node
// managed by LVM/device-mapper for a brick.
func BrickDevNode(vgId, brickId string) string {
	return path.Join(
		deviceMapperRoot,
		VgIdToName(vgId)+"-"+BrickIdToName(brickId))
}
