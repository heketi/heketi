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
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/pkg/glusterfs/api"
)

func dbDumpInternal(db *bolt.DB) (Db, error) {
	var dump Db
	clusterEntryList := make(map[string]ClusterEntry, 0)
	volEntryList := make(map[string]VolumeEntry, 0)
	brickEntryList := make(map[string]BrickEntry, 0)
	nodeEntryList := make(map[string]NodeEntry, 0)
	deviceEntryList := make(map[string]DeviceEntry, 0)
	blockvolEntryList := make(map[string]BlockVolumeEntry, 0)
	dbattributeEntryList := make(map[string]DbAttributeEntry, 0)
	pendingOpEntryList := make(map[string]PendingOperationEntry, 0)

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

		if b := tx.Bucket([]byte(BOLTDB_BUCKET_BLOCKVOLUME)); b == nil {
			logger.Warning("unable to find block volume bucket... skipping")
		} else {
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
		}

		has_pendingops := false

		if b := tx.Bucket([]byte(BOLTDB_BUCKET_DBATTRIBUTE)); b == nil {
			logger.Warning("unable to find dbattribute bucket... skipping")
		} else {
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
				has_pendingops = (has_pendingops ||
					(dbattributeEntry.Key == DB_HAS_PENDING_OPS_BUCKET &&
						dbattributeEntry.Value == "yes"))
			}
		}

		if has_pendingops {
			pendingops, err := PendingOperationList(tx)
			if err != nil {
				return err
			}

			for _, opid := range pendingops {
				entry, err := NewPendingOperationEntryFromId(tx, opid)
				if err != nil {
					return err
				}
				pendingOpEntryList[opid] = *entry
			}
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
	dump.PendingOperations = pendingOpEntryList

	return dump, nil
}

// DbDump ... Creates a JSON output representing the state of DB
// This is the variant to be called offline, i.e. when the server is not
// running.
func DbDump(jsonfile string, dbfile string) error {

	var fp *os.File
	if jsonfile == "-" {
		fp = os.Stdout
	} else {
		var err error
		fp, err = os.OpenFile(jsonfile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return fmt.Errorf("Could not create json file: %v", err.Error())
		}
		defer fp.Close()
	}

	db, err := OpenDB(dbfile, false)
	if err != nil {
		return fmt.Errorf("Unable to open database: %v", err)
	}

	dump, err := dbDumpInternal(db)
	if err != nil {
		return fmt.Errorf("Could not construct dump from DB: %v", err.Error())
	}
	enc := json.NewEncoder(fp)
	enc.SetIndent("", "    ")

	if err := enc.Encode(dump); err != nil {
		return fmt.Errorf("Could not encode dump as JSON: %v", err.Error())
	}

	return nil
}

// DbCreate ... Creates a bolt db file based on JSON input
func DbCreate(jsonfile string, dbfile string) error {

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

	dbhandle, err := OpenDB(dbfile, false)
	if err != nil {
		return fmt.Errorf("Could not open db file: %v", err.Error())
	}
	defer dbhandle.Close()

	err = dbhandle.Update(func(tx *bolt.Tx) error {
		return initializeBuckets(tx)
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
				return fmt.Errorf("Not a known volume durability type: %v", durability)
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
		for _, pendingop := range dump.PendingOperations {
			logger.Debug("adding pending operation entry %v", pendingop.Id)
			err := pendingop.Save(tx)
			if err != nil {
				return fmt.Errorf("Could not save pending operation bucket: %v", err.Error())
			}
		}
		// always record a new generation id on db import as the db contents
		// were no longer fully under heketi's control
		logger.Debug("recording new DB generation ID")
		if err := recordNewDBGenerationID(tx); err != nil {
			return fmt.Errorf("Could not record DB generation ID: %v", err.Error())
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func DeleteBricksWithEmptyPath(db *bolt.DB, all bool, clusterIDs []string, nodeIDs []string, deviceIDs []string) error {

	for _, id := range clusterIDs {
		if err := api.ValidateUUID(id); err != nil {
			return err
		}
	}
	for _, id := range nodeIDs {
		if err := api.ValidateUUID(id); err != nil {
			return err
		}
	}
	for _, id := range deviceIDs {
		if err := api.ValidateUUID(id); err != nil {
			return err
		}
	}

	err := db.Update(func(tx *bolt.Tx) error {
		if true == all {
			logger.Debug("deleting all bricks with empty path")
			clusters, err := ClusterList(tx)
			if err != nil {
				return err
			}
			for _, cluster := range clusters {
				clusterEntry, err := NewClusterEntryFromId(tx, cluster)
				if err != nil {
					return err
				}
				err = clusterEntry.DeleteBricksWithEmptyPath(tx)
				if err != nil {
					return err
				}
			}
			// no need to look at other IDs as we cleaned all bricks
			return nil
		}
		for _, cluster := range clusterIDs {
			clusterEntry, err := NewClusterEntryFromId(tx, cluster)
			if err != nil {
				return err
			}
			logger.Debug("deleting bricks with empty path in cluster %v from given list of clusters", clusterEntry.Info.Id)
			err = clusterEntry.DeleteBricksWithEmptyPath(tx)
			if err != nil {
				return err
			}
		}
		for _, node := range nodeIDs {
			nodeEntry, err := NewNodeEntryFromId(tx, node)
			if err != nil {
				return err
			}
			logger.Debug("deleting bricks with empty path in node %v from given list of nodes", nodeEntry.Info.Id)
			err = nodeEntry.DeleteBricksWithEmptyPath(tx)
			if err != nil {
				return err
			}
		}
		for _, device := range deviceIDs {
			deviceEntry, err := NewDeviceEntryFromId(tx, device)
			if err != nil {
				return err
			}
			logger.Debug("deleting bricks with empty path in device %v from given list of devices", deviceEntry.Info.Id)
			err = deviceEntry.DeleteBricksWithEmptyPath(tx)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}
