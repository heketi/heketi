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

func TestTagMatchingRuleFilter(t *testing.T) {
	dsrc := NewTestDeviceSource()
	var (
		n         *NodeEntry
		d         *DeviceEntry
		deviceAdd func(string, string) *DeviceEntry
	)
	n, deviceAdd = tmrNodeAdd(dsrc, "foo")
	n.Info.Tags["charles"] = "brown"
	d = deviceAdd("foo000d1", "/dev/d1")
	d.Info.Tags["tier"] = "one"

	t.Run("on dev", func(t *testing.T) {
		tmr := &TagMatchingRule{"tier", "=", "one"}
		f := tmr.GetFilter(dsrc)
		res := f(nil, d)
		tests.Assert(t, res)
	})
	t.Run("on node", func(t *testing.T) {
		tmr := &TagMatchingRule{"charles", "=", "brown"}
		f := tmr.GetFilter(dsrc)
		res := f(nil, d)
		tests.Assert(t, res)
	})
	t.Run("no match on node or dev", func(t *testing.T) {
		tmr := &TagMatchingRule{"charles", "!=", "xavier"}
		f := tmr.GetFilter(dsrc)
		res := f(nil, d)
		tests.Assert(t, res)
	})
	t.Run("not on node or dev", func(t *testing.T) {
		tmr := &TagMatchingRule{"charles", "=", "darwin"}
		f := tmr.GetFilter(dsrc)
		res := f(nil, d)
		tests.Assert(t, !res)
	})

	// test invalid lookup. for completeness & coverage
	dx := NewDeviceEntry()
	dx.Info.Id = "bob"
	dx.Info.Name = "bad"
	dx.NodeId = "xxx"
	dsrc.AddDevice(dx)

	t.Run("with bad lookup", func(t *testing.T) {
		tmr := &TagMatchingRule{"yeah", "=", "fangoriously"}
		f := tmr.GetFilter(dsrc)
		res := f(nil, dx)
		tests.Assert(t, !res)
	})
}

func tmrNodeAdd(tds *TestDeviceSource, nodeId string) (*NodeEntry, func(string, string) *DeviceEntry) {

	n := NewNodeEntry()
	n.Info.Id = nodeId
	n.Info.Zone = 1
	n.Info.Hostnames.Manage = []string{"mng-" + nodeId}
	n.Info.Hostnames.Storage = []string{"stor-" + nodeId}
	n.Info.ClusterId = "0000000000c"
	n.Info.Tags = map[string]string{}
	tds.AddNode(n)

	f := func(deviceId, dname string) *DeviceEntry {
		d := NewDeviceEntry()
		d.Info.Id = deviceId
		d.Info.Name = dname
		d.Info.Storage.Total = 100
		d.Info.Storage.Free = 100
		d.NodeId = nodeId
		d.Info.Tags = map[string]string{}

		n.Devices = append(n.Devices, d.Info.Id)
		tds.AddDevice(d)
		return d
	}
	return n, f
}
