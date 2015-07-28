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
	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/utils"
	"github.com/lpabon/godbc"
)

type BrickEntry struct {
	Info BrickInfo
}

func BrickList(tx *bolt.Tx) ([]string, error) {

	list := EntryKeys(tx, BOLTDB_BUCKET_BRICK)
	if list == nil {
		return nil, ErrAccessList
	}
	return list, nil
}

func NewBrickEntry(size uint64, deviceid, nodeid string) *BrickEntry {
	entry := &BrickEntry{}
	entry.Info.Id = utils.GenUUID()
	entry.Info.Size = size
	entry.Info.NodeId = nodeid
	entry.Info.DeviceId = deviceid

	return entry
}

func NewBrickEntryFromId(tx *bolt.Tx, id string) (*BrickEntry, error) {
	godbc.Require(tx != nil)

	entry := &BrickEntry{}
	err := EntryLoad(tx, entry, id)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func (b *BrickEntry) BucketName() string {
	return BOLTDB_BUCKET_BRICK
}

func (b *BrickEntry) SetId(id string) {
	b.Info.Id = id
}

func (b *BrickEntry) Id() string {
	return b.Info.Id
}

func (b *BrickEntry) Save(tx *bolt.Tx) error {
	godbc.Require(tx != nil)
	godbc.Require(len(b.Info.Id) > 0)

	return EntrySave(tx, b, b.Info.Id)
}

func (b *BrickEntry) Delete(tx *bolt.Tx) error {
	return EntryDelete(tx, b, b.Info.Id)
}

func (b *BrickEntry) NewInfoResponse(tx *bolt.Tx) (*BrickInfo, error) {
	info := &BrickInfo{}
	*info = b.Info

	return info, nil
}

func (b *BrickEntry) Marshal() ([]byte, error) {
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)
	err := enc.Encode(*b)

	return buffer.Bytes(), err
}

func (b *BrickEntry) Unmarshal(buffer []byte) error {
	dec := gob.NewDecoder(bytes.NewReader(buffer))
	err := dec.Decode(b)
	if err != nil {
		return err
	}

	return nil
}

func (b *BrickEntry) Create(db *bolt.DB) error {
	logger.Info("Creating brick %v", b.Info.Id)
	return nil
}

func (b *BrickEntry) Destroy(db *bolt.DB) error {
	logger.Info("Destroying brick %v", b.Info.Id)
	return nil
}
