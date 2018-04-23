//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

func PlacerForVolume(v *VolumeEntry) BrickPlacer {
	if v.HasArbiterOption() {
		return NewArbiterBrickPlacer(canHostArbiter, canHostData)
	}
	return NewStandardBrickPlacer()
}

func canHostArbiter(d PlacerDevice, dsrc DeviceSource) bool {
	return deviceHasArbiterTag(d, dsrc,
		TAG_VAL_ARBITER_REQUIRED, TAG_VAL_ARBITER_SUPPORTED)
}

func canHostData(d PlacerDevice, dsrc DeviceSource) bool {
	return deviceHasArbiterTag(d, dsrc,
		TAG_VAL_ARBITER_SUPPORTED, TAG_VAL_ARBITER_DISABLED)
}

func deviceHasArbiterTag(d PlacerDevice, dsrc DeviceSource, v ...string) bool {
	n, err := dsrc.Node(d.ParentNodeId())
	if err != nil {
		logger.LogError("failed to fetch node (%v) for arbiter tag: %v",
			d.ParentNodeId(), err)
		return false
	}
	a := ArbiterTag(MergeTags(n.(*NodeEntry), d.(*DeviceEntry)))
	for _, value := range v {
		if value == a {
			return true
		}
	}
	return false
}
