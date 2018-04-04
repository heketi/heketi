//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"github.com/heketi/heketi/pkg/glusterfs/api"
)

// Generic tagging related functions

type Taggable interface {
	AllTags() map[string]string
	SetTags(map[string]string) error
}

// ApplyTags takes the request from the user and updates the
// tags on the object accordingly. This api has been designed
// such that in the future we could support add/exclusive-add/
// delete for individual keys.
func ApplyTags(t Taggable, req api.TagsChangeRequest) {
	t.SetTags(req.SetTags)
}

// MergeTags combines all the tags from the taggable items in the
// function arguments, with the rightmost items having priority.
func MergeTags(t ...Taggable) map[string]string {
	out := map[string]string{}
	for _, src := range t {
		for k, v := range src.AllTags() {
			out[k] = v
		}
	}
	return out
}

// copyTags makes a new tags map with the same contents
// as the input map t. If the input map is nil an empty
// map is returned rather than nil.
func copyTags(t map[string]string) map[string]string {
	out := map[string]string{}
	if t == nil || len(t) == 0 {
		return out
	}
	for k, v := range t {
		out[k] = v
	}
	return out
}
