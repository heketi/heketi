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
	"errors"
	"github.com/boltdb/bolt"
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

func NewClusterEntryFromRequest() *ClusterEntry {
	entry := NewClusterEntry()
	entry.Info.Id = utils.GenUUID()

	return entry
}

func NewClusterEntryFromId(tx *bolt.Tx, id string) (*ClusterEntry, error) {
	b := tx.Bucket([]byte(BOLTDB_BUCKET_CLUSTER))
	if b == nil {
		logger.LogError("Unable to access cluster bucket")
		err := errors.New("Unable to access database")
		return nil, err
	}

	val := b.Get([]byte(id))
	if val == nil {
		return nil, ErrNotFound
	}

	entry := &ClusterEntry{}
	err := entry.Unmarshal(val)
	if err != nil {
		logger.LogError(
			"Unable to unmarshal cluster [%v] information from db: %v",
			id, err)
		return nil, err
	}

	return entry, nil

}

func (c *ClusterEntry) Save(tx *bolt.Tx) error {
	godbc.Require(tx != nil)
	godbc.Require(len(c.Info.Id) > 0)

	b := tx.Bucket([]byte(BOLTDB_BUCKET_CLUSTER))
	if b == nil {
		logger.LogError("Unable to save new cluster information in db")
		return errors.New("Unable to open bucket")
	}

	buffer, err := c.Marshal()
	if err != nil {
		logger.LogError("Unable to marshal cluster code")
		return err
	}

	err = b.Put([]byte(c.Info.Id), buffer)
	if err != nil {
		logger.LogError("Unable to save new cluster information in db")
		return err
	}

	return nil
}

func (c *ClusterEntry) Delete(tx *bolt.Tx) error {
	godbc.Require(tx != nil)

	// Check if the nodes still has drives
	if len(c.Info.Nodes) > 0 || len(c.Info.Volumes) > 0 {
		logger.Warning("Unable to delete cluster [%v] because it contains volumes and/or nodes", c.Info.Id)
		return ErrConflict
	}

	b := tx.Bucket([]byte(BOLTDB_BUCKET_CLUSTER))
	if b == nil {
		err := errors.New("Unable to access database")
		logger.Err(err)
		return err
	}

	// Delete key
	err := b.Delete([]byte(c.Info.Id))
	if err != nil {
		logger.LogError("Unable to delete container key [%v] in db: %v", c.Info.Id, err.Error())
		return err
	}

	return nil
}

func (c *ClusterEntry) NewClusterInfoResponse(tx *bolt.Tx) (*ClusterInfoResponse, error) {

	info := &ClusterInfoResponse{}
	*info = c.Info

	return info, nil
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
