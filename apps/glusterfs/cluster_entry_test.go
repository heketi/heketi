//
// Copyright (c) 2015 The heketi Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package glusterfs

import (
	"github.com/heketi/heketi/tests"
	"github.com/heketi/heketi/utils"
	"testing"
)

func TestNewClusterEntry(t *testing.T) {
	c := NewClusterEntry()
	tests.Assert(t, c.Info.Id == "")
	tests.Assert(t, c.Info.Volumes != nil)
	tests.Assert(t, c.Info.Nodes != nil)
	tests.Assert(t, len(c.Info.Volumes) == 0)
	tests.Assert(t, len(c.Info.Nodes) == 0)
}

func TestClusterEntryMarshal(t *testing.T) {
	m := NewClusterEntry()
	m.Info.Id = "123"
	m.Info.Nodes = []string{"1", "2"}
	m.Info.Volumes = []string{"3", "4", "5"}

	m.Info.Storage.Free = 10
	m.Info.Storage.Used = 100
	m.Info.Storage.Total = 1000

	buffer, err := m.Marshal()
	tests.Assert(t, err == nil)
	tests.Assert(t, buffer != nil)
	tests.Assert(t, len(buffer) > 0)

	um := NewClusterEntry()
	err = um.Unmarshal(buffer)
	tests.Assert(t, err == nil)

	tests.Assert(t, m.Info.Id == um.Info.Id)
	tests.Assert(t, len(um.Info.Volumes) == 3)
	tests.Assert(t, len(um.Info.Nodes) == 2)
	tests.Assert(t, um.Info.Nodes[0] == "1")
	tests.Assert(t, um.Info.Nodes[1] == "2")
	tests.Assert(t, um.Info.Volumes[0] == "3")
	tests.Assert(t, um.Info.Volumes[1] == "4")
	tests.Assert(t, um.Info.Volumes[2] == "5")
	tests.Assert(t, um.Info.Storage.Free == 10)
	tests.Assert(t, um.Info.Storage.Used == 100)
	tests.Assert(t, um.Info.Storage.Total == 1000)
}

func TestClusterEntryAddDeleteElements(t *testing.T) {
	c := NewClusterEntry()

	c.NodeAdd("123")
	tests.Assert(t, len(c.Info.Nodes) == 1)
	tests.Assert(t, len(c.Info.Volumes) == 0)
	tests.Assert(t, utils.SortedStringHas(c.Info.Nodes, "123"))

	c.NodeAdd("456")
	tests.Assert(t, len(c.Info.Nodes) == 2)
	tests.Assert(t, len(c.Info.Volumes) == 0)
	tests.Assert(t, utils.SortedStringHas(c.Info.Nodes, "123"))
	tests.Assert(t, utils.SortedStringHas(c.Info.Nodes, "456"))

	c.VolumeAdd("aabb")
	tests.Assert(t, len(c.Info.Nodes) == 2)
	tests.Assert(t, len(c.Info.Volumes) == 1)
	tests.Assert(t, utils.SortedStringHas(c.Info.Nodes, "123"))
	tests.Assert(t, utils.SortedStringHas(c.Info.Nodes, "456"))
	tests.Assert(t, utils.SortedStringHas(c.Info.Volumes, "aabb"))

	c.NodeDelete("aabb")
	tests.Assert(t, len(c.Info.Nodes) == 2)
	tests.Assert(t, len(c.Info.Volumes) == 1)
	tests.Assert(t, utils.SortedStringHas(c.Info.Nodes, "123"))
	tests.Assert(t, utils.SortedStringHas(c.Info.Nodes, "456"))
	tests.Assert(t, utils.SortedStringHas(c.Info.Volumes, "aabb"))

	c.NodeDelete("456")
	tests.Assert(t, len(c.Info.Nodes) == 1)
	tests.Assert(t, len(c.Info.Volumes) == 1)
	tests.Assert(t, utils.SortedStringHas(c.Info.Nodes, "123"))
	tests.Assert(t, !utils.SortedStringHas(c.Info.Nodes, "456"))
	tests.Assert(t, utils.SortedStringHas(c.Info.Volumes, "aabb"))

	c.NodeDelete("123")
	tests.Assert(t, len(c.Info.Nodes) == 0)
	tests.Assert(t, len(c.Info.Volumes) == 1)
	tests.Assert(t, !utils.SortedStringHas(c.Info.Nodes, "123"))
	tests.Assert(t, !utils.SortedStringHas(c.Info.Nodes, "456"))
	tests.Assert(t, utils.SortedStringHas(c.Info.Volumes, "aabb"))

	c.VolumeDelete("123")
	tests.Assert(t, len(c.Info.Nodes) == 0)
	tests.Assert(t, len(c.Info.Volumes) == 1)
	tests.Assert(t, !utils.SortedStringHas(c.Info.Nodes, "123"))
	tests.Assert(t, !utils.SortedStringHas(c.Info.Nodes, "456"))
	tests.Assert(t, utils.SortedStringHas(c.Info.Volumes, "aabb"))

	c.VolumeDelete("aabb")
	tests.Assert(t, len(c.Info.Nodes) == 0)
	tests.Assert(t, len(c.Info.Volumes) == 0)
	tests.Assert(t, !utils.SortedStringHas(c.Info.Nodes, "123"))
	tests.Assert(t, !utils.SortedStringHas(c.Info.Nodes, "456"))
	tests.Assert(t, !utils.SortedStringHas(c.Info.Volumes, "aabb"))
}
