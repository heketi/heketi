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
	"bytes"
	"encoding/gob"

	"github.com/boltdb/bolt"
	wdb "github.com/heketi/heketi/pkg/db"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/utils"
	"github.com/lpabon/godbc"
)

// SnapshotEntry struct represents a volume snapshot in heketi.
type SnapshotEntry struct {
	Info           api.SnapshotInfo
	OriginVolumeID string
	Pending        PendingItem
}

func SnapshotList(tx *bolt.Tx) ([]string, error) {
	list := EntryKeys(tx, BOLTDB_BUCKET_SNAPSHOT)
	if list == nil {
		return nil, ErrAccessList
	}
	return list, nil
}

func NewSnapshotEntry() *SnapshotEntry {
	return &SnapshotEntry{}

}
func NewSnapshotEntryFromId(tx *bolt.Tx, id string) (*SnapshotEntry, error) {
	godbc.Require(tx != nil)
	entry := NewSnapshotEntry()
	err := EntryLoad(tx, entry, id)
	if err != nil {
		return nil, err
	}
	return entry, nil
}

func NewSnapshotEntryFromRequest(req *api.VolumeSnapshotRequest) *SnapshotEntry {
	godbc.Require(req != nil)

	entry := NewSnapshotEntry()
	entry.Info.Id = utils.GenUUID()
	entry.Info.Description = req.Description
	if req.Name == "" {
		entry.Info.Name = "snapshot_" + entry.Info.Id
	} else {
		entry.Info.Name = req.Name
	}
	return entry
}

func (s *SnapshotEntry) BucketName() string {
	return BOLTDB_BUCKET_SNAPSHOT
}

func (s *SnapshotEntry) Marshal() ([]byte, error) {
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)
	err := enc.Encode(*s)

	return buffer.Bytes(), err
}

func (s *SnapshotEntry) Unmarshal(buffer []byte) error {
	dec := gob.NewDecoder(bytes.NewReader(buffer))
	err := dec.Decode(s)
	if err != nil {
		return err
	}

	return nil
}

func (s *SnapshotEntry) Save(tx *bolt.Tx) error {
	godbc.Require(tx != nil)
	godbc.Require(len(s.Info.Id) > 0)

	return EntrySave(tx, s, s.Info.Id)
}

func (s *SnapshotEntry) SaveDeleteEntry(db wdb.DB) error {
	return db.Update(func(tx *bolt.Tx) error {
		volumeEntry, err := NewVolumeEntryFromId(tx, s.OriginVolumeID)
		if err != nil {
			logger.Err(err)
		}
		// Remove volume from cluster
		cluster, err := NewClusterEntryFromId(tx, volumeEntry.Info.Cluster)
		if err != nil {
			logger.Err(err)
			// Do not return here.. keep going
		}
		cluster.SnapshotDelete(s.Info.Id)

		err = cluster.Save(tx)
		if err != nil {
			logger.Err(err)
			// Do not return here.. keep going
		}

		// Delete volume
		s.Delete(tx)

		return nil
	})
}

func (s *SnapshotEntry) Delete(tx *bolt.Tx) error {
	return EntryDelete(tx, s, s.Info.Id)
}

// Visible returns true if this snapshot is meant to be visible to
// API calls.
func (s *SnapshotEntry) Visible() bool {
	return s.Pending.Id == ""
}

func (s *SnapshotEntry) NewInfoResponse(tx *bolt.Tx) (*api.SnapshotInfoResponse, error) {
	godbc.Require(tx != nil)

	info := &api.SnapshotInfoResponse{}
	info.Id = s.Info.Id
	info.Name = s.Info.Name
	info.Description = s.Info.Description

	return info, nil
}
