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
	"testing"

	"github.com/heketi/tests"

	"github.com/heketi/heketi/pkg/idgen"
)

func TestOpTrackerCounts(t *testing.T) {
	ot := newOpTracker(50)
	var (
		i1 = idgen.GenUUID()
		i2 = idgen.GenUUID()
		i3 = idgen.GenUUID()
		i4 = idgen.GenUUID()
		i5 = idgen.GenUUID()
	)

	ot.Add(i1, TrackNormal)
	ot.Add(i2, TrackNormal)
	ot.Add(i3, TrackNormal)
	tests.Assert(t, ot.Get() == 3, "expected ot.Get() == 3, got", ot.Get())

	ot.Add(i4, TrackNormal)
	ot.Add(i5, TrackNormal)
	ot.Remove(i1)
	ot.Add(i1, TrackNormal)
	tests.Assert(t, ot.Get() == 5, "expected ot.Get() == 5, got", ot.Get())
}

func TestOpTrackerLimits(t *testing.T) {
	ot := newOpTracker(5)
	var (
		i1 = idgen.GenUUID()
		i2 = idgen.GenUUID()
		i3 = idgen.GenUUID()
		i4 = idgen.GenUUID()
	)

	ot.Add(i1, TrackNormal)
	ot.Add(i2, TrackNormal)
	ot.Add(i3, TrackNormal)
	ot.Add(i4, TrackNormal)

	var (
		r      bool
		token  string
		token2 string
	)
	r, token = ot.ThrottleOrToken()
	tests.Assert(t, r == false, "expected r == false, got", r)
	tests.Assert(t, ot.Get() == 5, "expected ot.Get() == 5, got", ot.Get())
	tests.Assert(t, token != "", "expected token != \"\"")

	r, token2 = ot.ThrottleOrToken()
	tests.Assert(t, r == true, "expected r == true, got", r)
	tests.Assert(t, ot.Get() == 5, "expected ot.Get() == 5, got", ot.Get())
	tests.Assert(t, token2 == "", "expected token2 == \"\"")

	ot.Remove(token)

	r, token = ot.ThrottleOrToken()
	tests.Assert(t, r == false, "expected r == false, got", r)
	tests.Assert(t, ot.Get() == 5, "expected ot.Get() == 5, got", ot.Get())
	tests.Assert(t, token != "", "expected token != \"\"")

	r, token2 = ot.ThrottleOrToken()
	tests.Assert(t, r == true, "expected r == true, got", r)
	tests.Assert(t, ot.Get() == 5, "expected ot.Get() == 5, got", ot.Get())
	tests.Assert(t, token2 == "", "expected token2 == \"\"")
}
