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
	"testing"

	"github.com/heketi/tests"
)

func TestParseTagMatchingRule(t *testing.T) {
	t.Run("empty_string", func(t *testing.T) {
		r, e := ParseTagMatchingRule("")
		tests.Assert(t, r == nil)
		tests.Assert(t, e != nil)
	})
	t.Run("dummy string", func(t *testing.T) {
		r, e := ParseTagMatchingRule("joe")
		tests.Assert(t, r == nil)
		tests.Assert(t, e != nil)
	})
	t.Run("bad match 1", func(t *testing.T) {
		r, e := ParseTagMatchingRule("joe=")
		tests.Assert(t, r == nil)
		tests.Assert(t, e != nil)
	})
	t.Run("bad match 2", func(t *testing.T) {
		r, e := ParseTagMatchingRule("=crumb")
		tests.Assert(t, r == nil)
		tests.Assert(t, e != nil)
	})
	t.Run("match 1", func(t *testing.T) {
		r, e := ParseTagMatchingRule("joe=crumb")
		tests.Assert(t, r != nil)
		tests.Assert(t, e == nil)
		tests.Assert(t, r.Key == "joe")
		tests.Assert(t, r.Match == "=")
		tests.Assert(t, r.Value == "crumb")
	})
	t.Run("match 2", func(t *testing.T) {
		r, e := ParseTagMatchingRule("joe!=crumb")
		tests.Assert(t, r != nil)
		tests.Assert(t, e == nil)
		tests.Assert(t, r.Key == "joe")
		tests.Assert(t, r.Match == "!=")
		tests.Assert(t, r.Value == "crumb")
	})
}

func TestTagMatchingRuleTest(t *testing.T) {
	t.Run("match", func(t *testing.T) {
		tmr := &TagMatchingRule{"foo", "=", "bar"}
		tests.Assert(t, tmr.Test("bar"))
	})
	t.Run("no match", func(t *testing.T) {
		tmr := &TagMatchingRule{"foo", "=", "bar"}
		tests.Assert(t, !tmr.Test("baz"))
	})
	t.Run("invert match", func(t *testing.T) {
		tmr := &TagMatchingRule{"foo", "!=", "bar"}
		tests.Assert(t, !tmr.Test("bar"))
	})
	t.Run("invert no match", func(t *testing.T) {
		tmr := &TagMatchingRule{"foo", "!=", "bar"}
		tests.Assert(t, tmr.Test("blob"))
	})
}
