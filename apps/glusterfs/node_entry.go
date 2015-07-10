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

type NodeEntry struct {
	Info    NodeInfoResponse
	Devices sort.StringSlice
}

func NewNodeEntry() *NodeEntry {
	entry := &NodeEntry{}
	entry.Devices = make(sort.StringSlice, 0)

	// Unused
	// entry.Info.DevicesInfo

	return entry
}

func (n *NodeEntry) Cluster() string {
	return n.Info.ClusterId
}

func (n *NodeEntry) InfoReponse() *NodeInfoResponse {
	info := &NodeInfoResponse{}
	*info = n.Info

	return info
}

func (n *NodeEntry) Marshal() ([]byte, error) {
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)
	err := enc.Encode(*n)

	return buffer.Bytes(), err
}

func (n *NodeEntry) Unmarshal(buffer []byte) error {
	dec := gob.NewDecoder(bytes.NewReader(buffer))
	err := dec.Decode(n)
	if err != nil {
		return err
	}

	// Make sure to setup arrays if nil
	if n.Devices == nil {
		n.Devices = make(sort.StringSlice, 0)
	}

	return nil
}

func (n *NodeEntry) DeviceAdd(id string) {
	n.Devices = append(n.Devices, id)
	n.Devices.Sort()
}

func (n *NodeEntry) DeviceDelete(id string) {
	n.Devices = utils.SortedStringsDelete(n.Devices, id)
}

func (n *NodeEntry) StorageAdd(amount uint64) {
	n.Info.Storage.Free += amount
	n.Info.Storage.Total += amount

	godbc.Ensure(n.Info.Storage.Free >= 0)
	godbc.Ensure(n.Info.Storage.Used >= 0)
	godbc.Ensure(n.Info.Storage.Total >= 0)
}

func (n *NodeEntry) StorageAllocate(amount uint64) {
	n.Info.Storage.Free -= amount
	n.Info.Storage.Used += amount
	n.Info.Storage.Total -= amount

	godbc.Ensure(n.Info.Storage.Free >= 0)
	godbc.Ensure(n.Info.Storage.Used >= 0)
	godbc.Ensure(n.Info.Storage.Total >= 0)
}

func (n *NodeEntry) StorageFree(amount uint64) {
	n.Info.Storage.Free += amount
	n.Info.Storage.Used -= amount
	n.Info.Storage.Total += amount

	godbc.Ensure(n.Info.Storage.Free >= 0)
	godbc.Ensure(n.Info.Storage.Used >= 0)
	godbc.Ensure(n.Info.Storage.Total >= 0)
}
