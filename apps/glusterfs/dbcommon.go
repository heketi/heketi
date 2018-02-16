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
	"github.com/boltdb/bolt"
)

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
