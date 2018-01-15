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
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/utils"
)

type Db struct {
	Clusters     map[string]ClusterEntry     `json:"clusterentries"`
	Volumes      map[string]VolumeEntry      `json:"volumeentries"`
	Bricks       map[string]BrickEntry       `json:"brickentries"`
	Nodes        map[string]NodeEntry        `json:"nodeentries"`
	Devices      map[string]DeviceEntry      `json:"deviceentries"`
	BlockVolumes map[string]BlockVolumeEntry `json:"blockvolumeentries"`
	DbAttributes map[string]DbAttributeEntry `json:"dbattributeentries"`
}

func dbDumpInternal(db *bolt.DB) (Db, error) {
	var dump Db
	clusterEntryList := make(map[string]ClusterEntry, 0)
	volEntryList := make(map[string]VolumeEntry, 0)
	brickEntryList := make(map[string]BrickEntry, 0)
	nodeEntryList := make(map[string]NodeEntry, 0)
	deviceEntryList := make(map[string]DeviceEntry, 0)
	blockvolEntryList := make(map[string]BlockVolumeEntry, 0)
	dbattributeEntryList := make(map[string]DbAttributeEntry, 0)

	err := db.View(func(tx *bolt.Tx) error {

		logger.Debug("volume bucket")

		// Volume Bucket
		volumes, err := VolumeList(tx)
		if err != nil {
			return err
		}

		for _, volume := range volumes {
			logger.Debug("adding volume entry %v", volume)
			volEntry, err := NewVolumeEntryFromId(tx, volume)
			if err != nil {
				return err
			}
			volEntryList[volEntry.Info.Id] = *volEntry
		}

		// Brick Bucket
		logger.Debug("brick bucket")
		bricks, err := BrickList(tx)
		if err != nil {
			return err
		}

		for _, brick := range bricks {
			logger.Debug("adding brick entry %v", brick)
			brickEntry, err := NewBrickEntryFromId(tx, brick)
			if err != nil {
				return err
			}
			brickEntryList[brickEntry.Info.Id] = *brickEntry
		}

		// Cluster Bucket
		logger.Debug("cluster bucket")
		clusters, err := ClusterList(tx)
		if err != nil {
			return err
		}

		for _, cluster := range clusters {
			logger.Debug("adding cluster entry %v", cluster)
			clusterEntry, err := NewClusterEntryFromId(tx, cluster)
			if err != nil {
				return err
			}
			clusterEntryList[clusterEntry.Info.Id] = *clusterEntry
		}

		// Node Bucket
		logger.Debug("node bucket")
		nodes, err := NodeList(tx)
		if err != nil {
			return err
		}

		for _, node := range nodes {
			logger.Debug("adding node entry %v", node)
			// Some entries are added for easy lookup of existing entries
			// Refer to http://lists.gluster.org/pipermail/heketi-devel/2017-May/000107.html
			// Don't output them to JSON. However, these entries must be created when
			// importing nodes into db from JSON.
			if strings.HasPrefix(node, "MANAGE") || strings.HasPrefix(node, "STORAGE") {
				logger.Debug("ignoring registry key %v", node)
			} else {
				nodeEntry, err := NewNodeEntryFromId(tx, node)
				if err != nil {
					return err
				}
				nodeEntryList[nodeEntry.Info.Id] = *nodeEntry
			}
		}

		// Device Bucket
		logger.Debug("device bucket")
		devices, err := DeviceList(tx)
		if err != nil {
			return err
		}

		for _, device := range devices {
			logger.Debug("adding device entry %v", device)
			// Some entries are added for easy lookup of existing entries
			// Refer to http://lists.gluster.org/pipermail/heketi-devel/2017-May/000107.html
			// Don't output them to JSON. However, these entries must be created when
			// importing devices into db from JSON.
			if strings.HasPrefix(device, "DEVICE") {
				logger.Debug("ignoring registry key %v", device)
			} else {
				deviceEntry, err := NewDeviceEntryFromId(tx, device)
				if err != nil {
					return err
				}
				deviceEntryList[deviceEntry.Info.Id] = *deviceEntry
			}
		}

		// BlockVolume Bucket
		logger.Debug("blockvolume bucket")
		blockvolumes, err := BlockVolumeList(tx)
		if err != nil {
			return err
		}

		for _, blockvolume := range blockvolumes {
			logger.Debug("adding blockvolume entry %v", blockvolume)
			blockvolEntry, err := NewBlockVolumeEntryFromId(tx, blockvolume)
			if err != nil {
				return err
			}
			blockvolEntryList[blockvolEntry.Info.Id] = *blockvolEntry
		}

		// DbAttributes Bucket
		dbattributes, err := DbAttributeList(tx)
		if err != nil {
			return err
		}

		for _, dbattribute := range dbattributes {
			logger.Debug("adding dbattribute entry %v", dbattribute)
			dbattributeEntry, err := NewDbAttributeEntryFromKey(tx, dbattribute)
			if err != nil {
				return err
			}
			dbattributeEntryList[dbattributeEntry.Key] = *dbattributeEntry
		}

		return nil
	})
	if err != nil {
		return Db{}, fmt.Errorf("Could not construct dump from DB: %v", err.Error())
	}

	dump.Clusters = clusterEntryList
	dump.Volumes = volEntryList
	dump.Bricks = brickEntryList
	dump.Nodes = nodeEntryList
	dump.Devices = deviceEntryList
	dump.BlockVolumes = blockvolEntryList
	dump.DbAttributes = dbattributeEntryList

	return dump, nil
}

// DbDump ... Creates a JSON output representing the state of DB
// This is the variant to be called offline, i.e. when the server is not
// running.
func DbDump(jsonfile string, dbfile string, debug bool) error {
	if debug {
		logger.SetLevel(utils.LEVEL_DEBUG)
	}
	fp, err := os.OpenFile(jsonfile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("Could not create json file: %v", err.Error())
	}
	defer fp.Close()

	db, err := bolt.Open(dbfile, 0600, &bolt.Options{Timeout: 3 * time.Second})
	if err != nil {
		return fmt.Errorf("Unable to open database: %v", err)
	}

	dump, err := dbDumpInternal(db)
	if err != nil {
		return fmt.Errorf("Could not construct dump from DB: %v", err.Error())
	}

	if err := json.NewEncoder(fp).Encode(dump); err != nil {
		return fmt.Errorf("Could not encode dump as JSON: %v", err.Error())
	}

	return nil
}

// DbDump ... Creates a JSON output representing the state of DB
// This is the variant to be called via the API and running in the App
func (a *App) DbDump(w http.ResponseWriter, r *http.Request) {
	dump, err := dbDumpInternal(a.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write msg
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(dump); err != nil {
		panic(err)
	}
}

// DbCreate ... Creates a bolt db file based on JSON input
func DbCreate(jsonfile string, dbfile string, debug bool) error {
	if debug {
		logger.SetLevel(utils.LEVEL_DEBUG)
	}

	// we use gob package to serialize entries before writing to db
	// to serialize an interface type, it needs to be registered with gob.
	// this should ideally be done during volume bucket creation
	// but it is done during first VolumeEntry creation in current code
	// TODO: create wrapper for volume bucket creation and type registration
	gob.Register(&NoneDurability{})
	gob.Register(&VolumeReplicaDurability{})
	gob.Register(&VolumeDisperseDurability{})
	logger.Debug("interface type registration complete")

	var dump Db

	fp, err := os.Open(jsonfile)
	if err != nil {
		return fmt.Errorf("Could not open input file: %v", err.Error())
	}
	defer fp.Close()

	dbParser := json.NewDecoder(fp)
	if err = dbParser.Decode(&dump); err != nil {
		return fmt.Errorf("Could not decode input file as JSON: %v", err.Error())
	}

	// We don't want to overwrite existing db file
	_, err = os.Stat(dbfile)
	if err == nil {
		return fmt.Errorf("%v file already exists", dbfile)
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("unable to stat path given for dbfile: %v", dbfile)
	}

	// Setup BoltDB database
	dbhandle, err := bolt.Open(dbfile, 0600, &bolt.Options{Timeout: 3 * time.Second})
	if err != nil {
		logger.Debug("Unable to open database: %v", err)
		return fmt.Errorf("Could not open db file: %v", err.Error())
	}

	err = dbhandle.Update(func(tx *bolt.Tx) error {
		// Create Cluster Bucket
		_, err := tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_CLUSTER))
		if err != nil {
			logger.Debug("Unable to create cluster bucket in DB")
			return err
		}

		// Create Node Bucket
		_, err = tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_NODE))
		if err != nil {
			logger.Debug("Unable to create node bucket in DB")
			return err
		}

		// Create Volume Bucket
		_, err = tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_VOLUME))
		if err != nil {
			logger.Debug("Unable to create volume bucket in DB")
			return err
		}

		// Create Device Bucket
		_, err = tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_DEVICE))
		if err != nil {
			logger.Debug("Unable to create device bucket in DB")
			return err
		}

		// Create Brick Bucket
		_, err = tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_BRICK))
		if err != nil {
			logger.Debug("Unable to create brick bucket in DB")
			return err
		}

		_, err = tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_BLOCKVOLUME))
		if err != nil {
			logger.Debug("Unable to create blockvolume bucket in DB")
			return err
		}

		_, err = tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_DBATTRIBUTE))
		if err != nil {
			logger.Debug("Unable to create dbattribute bucket in DB")
			return err
		}

		return nil

	})
	if err != nil {
		logger.Err(err)
		return nil
	}

	err = dbhandle.Update(func(tx *bolt.Tx) error {
		for _, cluster := range dump.Clusters {
			logger.Debug("adding cluster entry %v", cluster.Info.Id)
			err := cluster.Save(tx)
			if err != nil {
				return fmt.Errorf("Could not save cluster bucket: %v", err.Error())
			}
		}
		for _, volume := range dump.Volumes {
			logger.Debug("adding volume entry %v", volume.Info.Id)
			// When serializing to JSON we skipped volume.Durability
			// Hence, while creating volume entry, we populate it
			durability := volume.Info.Durability.Type
			switch {

			case durability == api.DurabilityReplicate:
				volume.Durability = NewVolumeReplicaDurability(&volume.Info.Durability.Replicate)

			case durability == api.DurabilityEC:
				volume.Durability = NewVolumeDisperseDurability(&volume.Info.Durability.Disperse)

			case durability == api.DurabilityDistributeOnly || durability == "":
				volume.Durability = NewNoneDurability()

			default:
				return fmt.Errorf("Not a known volume type: %v", err.Error())
			}

			// Set the default values accordingly
			volume.Durability.SetDurability()
			err := volume.Save(tx)
			if err != nil {
				return fmt.Errorf("Could not save volume bucket: %v", err.Error())
			}
		}
		for _, brick := range dump.Bricks {
			logger.Debug("adding brick entry %v", brick.Info.Id)
			err := brick.Save(tx)
			if err != nil {
				return fmt.Errorf("Could not save brick bucket: %v", err.Error())
			}
		}
		for _, node := range dump.Nodes {
			logger.Debug("adding node entry %v", node.Info.Id)
			err := node.Save(tx)
			if err != nil {
				return fmt.Errorf("Could not save node bucket: %v", err.Error())
			}
			logger.Debug("registering node entry %v", node.Info.Id)
			err = node.Register(tx)
			if err != nil {
				return fmt.Errorf("Could not register node: %v", err.Error())
			}
		}
		for _, device := range dump.Devices {
			logger.Debug("adding device entry %v", device.Info.Id)
			err := device.Save(tx)
			if err != nil {
				return fmt.Errorf("Could not save device bucket: %v", err.Error())
			}
			logger.Debug("registering device entry %v", device.Info.Id)
			err = device.Register(tx)
			if err != nil {
				return fmt.Errorf("Could not register device: %v", err.Error())
			}
		}
		for _, blockvolume := range dump.BlockVolumes {
			logger.Debug("adding blockvolume entry %v", blockvolume.Info.Id)
			err := blockvolume.Save(tx)
			if err != nil {
				return fmt.Errorf("Could not save blockvolume bucket: %v", err.Error())
			}
		}
		for _, dbattribute := range dump.DbAttributes {
			logger.Debug("adding dbattribute entry %v", dbattribute.Key)
			err := dbattribute.Save(tx)
			if err != nil {
				return fmt.Errorf("Could not save dbattribute bucket: %v", err.Error())
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}
