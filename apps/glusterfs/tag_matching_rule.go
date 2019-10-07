//
// Copyright (c) 2019 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"fmt"
	"regexp"
)

var tag_match_regex = regexp.MustCompile(
	`^([a-zA-Z0-9.-_]+)(\!?=)([a-zA-Z0-9.-_]+)$`)

type TagMatchingRule struct {
	Key   string
	Match string
	Value string
}

func ParseTagMatchingRule(s string) (*TagMatchingRule, error) {
	m := tag_match_regex.FindAllStringSubmatch(s, -1)
	if !(len(m) == 1 && len(m[0]) == 4) {
		return nil, fmt.Errorf("Invalid tag match rule: %v", s)
	}
	return &TagMatchingRule{m[0][1], m[0][2], m[0][3]}, nil
}

func (tmr *TagMatchingRule) Test(v string) bool {
	logger.Debug("Testing tag value %#v with rule %+v", v, tmr)
	if tmr.Match == "=" {
		return tmr.Value == v
	} else {
		return tmr.Value != v
	}
}

func (tmr *TagMatchingRule) GetFilter(dsrc DeviceSource) DeviceFilter {
	return func(bs *BrickSet, d *DeviceEntry) bool {
		n, err := dsrc.Node(d.NodeId)
		if err != nil {
			logger.LogError("failed to fetch node (%v) in tag matching filter: %v",
				d.NodeId, err)
			return false
		}
		tags := MergeTags(n, d)
		return tmr.Test(tags[tmr.Key])
	}
}
