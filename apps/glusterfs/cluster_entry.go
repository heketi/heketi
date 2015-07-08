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
	"bytes"
	"encoding/gob"
	"github.com/heketi/heketi/utils"
	"github.com/lpabon/godbc"
	"sort"
)

type ClusterEntry struct {
	Info ClusterInfoResponse
}

func NewClusterEntry() *ClusterEntry {
	entry := &ClusterEntry{}
	entry.Info.Nodes = make(sort.StringSlice, 0)
	entry.Info.Volumes = make(sort.StringSlice, 0)

	return entry
}

func (c *ClusterEntry) Marshal() ([]byte, error) {
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)
	err := enc.Encode(*c)

	return buffer.Bytes(), err
}

func (c *ClusterEntry) Unmarshal(buffer []byte) error {
	dec := gob.NewDecoder(bytes.NewReader(buffer))
	err := dec.Decode(c)
	if err != nil {
		return err
	}

	// Make sure to setup arrays if nil
	if c.Info.Nodes == nil {
		c.Info.Nodes = make(sort.StringSlice, 0)
	}
	if c.Info.Volumes == nil {
		c.Info.Volumes = make(sort.StringSlice, 0)
	}

	return nil
}

func (c *ClusterEntry) NodeAdd(id string) {
	c.Info.Nodes = append(c.Info.Nodes, id)
	c.Info.Nodes.Sort()
}

func (c *ClusterEntry) VolumeAdd(id string) {
	c.Info.Volumes = append(c.Info.Volumes, id)
	c.Info.Volumes.Sort()
}

func (c *ClusterEntry) VolumeDelete(id string) {
	c.Info.Volumes = utils.SortedStringsDelete(c.Info.Volumes, id)
}

func (c *ClusterEntry) NodeDelete(id string) {
	c.Info.Nodes = utils.SortedStringsDelete(c.Info.Nodes, id)
}

func (c *ClusterEntry) StorageAdd(amount uint64) {
	c.Info.Storage.Free += amount
	c.Info.Storage.Total += amount

	godbc.Ensure(c.Info.Storage.Free >= 0)
	godbc.Ensure(c.Info.Storage.Used >= 0)
	godbc.Ensure(c.Info.Storage.Total >= 0)
}

func (c *ClusterEntry) StorageAllocate(amount uint64) {
	c.Info.Storage.Free -= amount
	c.Info.Storage.Used += amount
	c.Info.Storage.Total -= amount

	godbc.Ensure(c.Info.Storage.Free >= 0)
	godbc.Ensure(c.Info.Storage.Used >= 0)
	godbc.Ensure(c.Info.Storage.Total >= 0)
}

func (c *ClusterEntry) StorageFree(amount uint64) {
	c.Info.Storage.Free += amount
	c.Info.Storage.Used -= amount
	c.Info.Storage.Total += amount

	godbc.Ensure(c.Info.Storage.Free >= 0)
	godbc.Ensure(c.Info.Storage.Used >= 0)
	godbc.Ensure(c.Info.Storage.Total >= 0)
}
