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
	"time"

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/pkg/utils"
)

const (
	DB_GENERATION_ID = "DB_GENERATION_ID"
)

type Db struct {
	Clusters          map[string]ClusterEntry          `json:"clusterentries"`
	Volumes           map[string]VolumeEntry           `json:"volumeentries"`
	Bricks            map[string]BrickEntry            `json:"brickentries"`
	Nodes             map[string]NodeEntry             `json:"nodeentries"`
	Devices           map[string]DeviceEntry           `json:"deviceentries"`
	BlockVolumes      map[string]BlockVolumeEntry      `json:"blockvolumeentries"`
	DbAttributes      map[string]DbAttributeEntry      `json:"dbattributeentries"`
	PendingOperations map[string]PendingOperationEntry `json:"pendingoperations"`
}

func initializeBuckets(tx *bolt.Tx) error {
	// Create Cluster Bucket
	_, err := tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_CLUSTER))
	if err != nil {
		logger.LogError("Unable to create cluster bucket in DB")
		return err
	}

	// Create Node Bucket
	_, err = tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_NODE))
	if err != nil {
		logger.LogError("Unable to create node bucket in DB")
		return err
	}

	// Create Volume Bucket
	_, err = tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_VOLUME))
	if err != nil {
		logger.LogError("Unable to create volume bucket in DB")
		return err
	}

	// Create Device Bucket
	_, err = tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_DEVICE))
	if err != nil {
		logger.LogError("Unable to create device bucket in DB")
		return err
	}

	// Create Brick Bucket
	_, err = tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_BRICK))
	if err != nil {
		logger.LogError("Unable to create brick bucket in DB")
		return err
	}

	_, err = tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_BLOCKVOLUME))
	if err != nil {
		logger.LogError("Unable to create blockvolume bucket in DB")
		return err
	}

	_, err = tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_DBATTRIBUTE))
	if err != nil {
		logger.LogError("Unable to create dbattribute bucket in DB")
		return err
	}

	_, err = tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_PENDING_OPS))
	if err != nil {
		logger.LogError("Unable to create pending ops bucket in DB")
		return err
	}

	return nil
}

// UpgradeDB runs all upgrade routines in order to to update the DB
// to the latest "schemas" and data.
func UpgradeDB(tx *bolt.Tx) error {

	err := ClusterEntryUpgrade(tx)
	if err != nil {
		logger.LogError("Failed to upgrade db for cluster entries")
		return err
	}

	err = NodeEntryUpgrade(tx)
	if err != nil {
		logger.LogError("Failed to upgrade db for node entries")
		return err
	}

	err = VolumeEntryUpgrade(tx)
	if err != nil {
		logger.LogError("Failed to upgrade db for volume entries")
		return err
	}

	err = DeviceEntryUpgrade(tx)
	if err != nil {
		logger.LogError("Failed to upgrade db for device entries")
		return err
	}

	err = BrickEntryUpgrade(tx)
	if err != nil {
		logger.LogError("Failed to upgrade db for brick entries: %v", err)
		return err
	}

	err = PendingOperationUpgrade(tx)
	if err != nil {
		logger.LogError("Failed to upgrade db for pending operations: %v", err)
		return err
	}

	err = upgradeDBGenerationID(tx)
	if err != nil {
		logger.LogError("Failed to record DB Generation ID: %v", err)
		return err
	}

	return nil
}

func upgradeDBGenerationID(tx *bolt.Tx) error {
	_, err := NewDbAttributeEntryFromKey(tx, DB_GENERATION_ID)
	switch err {
	case ErrNotFound:
		return recordNewDBGenerationID(tx)
	case nil:
		return nil
	default:
		return err
	}
}

func recordNewDBGenerationID(tx *bolt.Tx) error {
	entry := NewDbAttributeEntry()
	entry.Key = DB_GENERATION_ID
	entry.Value = utils.GenUUID()
	return entry.Save(tx)
}

// OpenDB is a wrapper over bolt.Open. It takes a bool to decide whether it should be a read-only open.
// Other bolt DB config options remain local to this function.
func OpenDB(dbfilename string, ReadOnly bool) (dbhandle *bolt.DB, err error) {

	if ReadOnly {
		dbhandle, err = bolt.Open(dbfilename, 0666, &bolt.Options{ReadOnly: true})
		if err != nil {
			logger.LogError("Unable to open database in read only mode: %v", err)
		}
		return dbhandle, err
	}

	dbhandle, err = bolt.Open(dbfilename, 0600, &bolt.Options{Timeout: 3 * time.Second})
	if err != nil {
		logger.LogError("Unable to open database: %v", err)
	}
	return dbhandle, err

}
