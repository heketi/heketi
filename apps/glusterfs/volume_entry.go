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
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/executors"
	"github.com/heketi/heketi/utils"
	"github.com/lpabon/godbc"
	"sort"
	"strings"
)

const (

	// Byte values in KB
	KB = 1
	MB = KB * 1024
	GB = MB * 1024
	TB = GB * 1024

	// Default values
	DEFAULT_REPLICA               = 2
	DEFAULT_THINP_SNAPSHOT_FACTOR = 1.5

	// Default limits
	BRICK_MIN_SIZE = uint64(4 * GB)
	BRICK_MAX_SIZE = uint64(4 * TB)
	BRICK_MAX_NUM  = 500
)

type VolumeEntry struct {
	Info   VolumeInfo
	Bricks sort.StringSlice
}

func VolumeList(tx *bolt.Tx) ([]string, error) {

	list := EntryKeys(tx, BOLTDB_BUCKET_VOLUME)
	if list == nil {
		return nil, ErrAccessList
	}
	return list, nil
}

func NewVolumeEntry() *VolumeEntry {
	entry := &VolumeEntry{}
	entry.Bricks = make(sort.StringSlice, 0)

	return entry
}

func NewVolumeEntryFromRequest(req *VolumeCreateRequest) *VolumeEntry {
	godbc.Require(req != nil)

	vol := NewVolumeEntry()
	vol.Info.Id = utils.GenUUID()
	vol.Info.Replica = req.Replica
	vol.Info.Snapshot = req.Snapshot
	vol.Info.Size = req.Size

	// Set default replica
	if vol.Info.Replica == 0 {
		vol.Info.Replica = DEFAULT_REPLICA
	}

	// Set default name
	if req.Name == "" {
		vol.Info.Name = "vol_" + vol.Info.Id
	} else {
		vol.Info.Name = req.Name
	}

	// Set default thinp factor
	// :TODO: Are forgetting to add the requested snapshot?
	if vol.Info.Snapshot.Enable && vol.Info.Snapshot.Factor == 0 {
		vol.Info.Snapshot.Factor = DEFAULT_THINP_SNAPSHOT_FACTOR
	} else if !vol.Info.Snapshot.Enable {
		vol.Info.Snapshot.Factor = 1
	}

	// If it is zero, then it will be assigned
	vol.Info.Clusters = req.Clusters

	return vol
}

func NewVolumeEntryFromId(tx *bolt.Tx, id string) (*VolumeEntry, error) {
	godbc.Require(tx != nil)

	entry := NewVolumeEntry()
	err := EntryLoad(tx, entry, id)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func (v *VolumeEntry) BucketName() string {
	return BOLTDB_BUCKET_VOLUME
}

func (v *VolumeEntry) Save(tx *bolt.Tx) error {
	godbc.Require(tx != nil)
	godbc.Require(len(v.Info.Id) > 0)

	return EntrySave(tx, v, v.Info.Id)
}

func (v *VolumeEntry) Delete(tx *bolt.Tx) error {
	return EntryDelete(tx, v, v.Info.Id)
}

func (v *VolumeEntry) NewInfoResponse(tx *bolt.Tx) (*VolumeInfoResponse, error) {
	godbc.Require(tx != nil)

	info := NewVolumeInfoResponse()
	info.Id = v.Info.Id
	info.Cluster = v.Info.Cluster
	info.Mount = v.Info.Mount
	info.Snapshot = v.Info.Snapshot
	info.Size = v.Info.Size
	info.Replica = v.Info.Replica
	info.Name = v.Info.Name

	for _, brickid := range v.BricksIds() {
		brick, err := NewBrickEntryFromId(tx, brickid)
		if err != nil {
			return nil, err
		}
		brickinfo, err := brick.NewInfoResponse(tx)
		if err != nil {
			return nil, err
		}

		info.Bricks = append(info.Bricks, *brickinfo)
	}

	return info, nil
}

func (v *VolumeEntry) Marshal() ([]byte, error) {
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)
	err := enc.Encode(*v)

	return buffer.Bytes(), err
}

func (v *VolumeEntry) Unmarshal(buffer []byte) error {
	dec := gob.NewDecoder(bytes.NewReader(buffer))
	err := dec.Decode(v)
	if err != nil {
		return err
	}

	// Make sure to setup arrays if nil
	if v.Bricks == nil {
		v.Bricks = make(sort.StringSlice, 0)
	}

	return nil
}

func (v *VolumeEntry) BrickAdd(id string) {
	godbc.Require(!utils.SortedStringHas(v.Bricks, id))

	v.Bricks = append(v.Bricks, id)
	v.Bricks.Sort()
}

func (v *VolumeEntry) BrickDelete(id string) {
	v.Bricks = utils.SortedStringsDelete(v.Bricks, id)
}

func (v *VolumeEntry) Create(db *bolt.DB, executor executors.Executor) (e error) {

	// On any error, remove the volume
	defer func() {
		if e != nil {
			db.Update(func(tx *bolt.Tx) error {
				v.Delete(tx)

				return nil
			})
		}
	}()

	// Get list of clusters
	var clusters []string
	if len(v.Info.Clusters) == 0 {
		err := db.View(func(tx *bolt.Tx) error {
			var err error
			clusters, err = ClusterList(tx)
			return err

		})
		if err != nil {
			return err
		}
	} else {
		clusters = v.Info.Clusters
	}

	// Check we have clusters
	if len(clusters) == 0 {
		logger.LogError("Volume being ask to be created, but there are no clusters configured")
		return ErrNoSpace
	}
	logger.Debug("Using the following clusters: %+v", clusters)

	// For each cluster look for storage space for this volume
	var brick_entries []*BrickEntry
	for _, cluster := range clusters {
		var err error

		// Check this cluster for space
		brick_entries, err = v.allocBricksInCluster(db, cluster, v.Info.Size)

		// Check if allocation was successfull
		if err == nil {
			v.Info.Cluster = cluster
			logger.Debug("Volume to be created on cluster %v", cluster)
			break
		}
	}
	if brick_entries == nil {
		return ErrNoSpace
	}

	// Make sure to clean up bricks on error
	defer func() {
		if e != nil {
			db.Update(func(tx *bolt.Tx) error {
				for _, brick := range brick_entries {
					v.removeBrickFromDb(tx, brick)
				}
				return nil
			})
		}
	}()

	// Create the bricks on the nodes
	err := CreateBricks(db, executor, brick_entries)
	if err != nil {
		return err
	}

	// Clean up created bricks on failure
	defer func() {
		if e != nil {
			DestroyBricks(db, executor, brick_entries)
		}
	}()

	// Create GlusterFS volume
	err = v.createVolume(db, executor, brick_entries)
	if err != nil {
		return err
	}

	// Destroy volume on failure
	defer func() {
		if e != nil {
			v.Destroy(db, executor)
		}
	}()

	// Save information on db
	err = db.Update(func(tx *bolt.Tx) error {

		// Save brick entries
		for _, brick := range brick_entries {
			err := brick.Save(tx)
			if err != nil {
				return err
			}
		}

		// Save volume information
		err = v.Save(tx)
		if err != nil {
			return err
		}

		// Save cluster
		cluster, err := NewClusterEntryFromId(tx, v.Info.Cluster)
		if err != nil {
			return err
		}
		cluster.VolumeAdd(v.Info.Id)
		return cluster.Save(tx)
	})
	if err != nil {
		return err
	}

	return nil

}

func (v *VolumeEntry) Destroy(db *bolt.DB, executor executors.Executor) error {
	logger.Info("Destroying volume %v", v.Info.Id)

	// Get the entries from the database
	brick_entries := make([]*BrickEntry, len(v.Bricks))
	var sshhost string
	db.View(func(tx *bolt.Tx) error {
		for index, id := range v.BricksIds() {
			brick, err := NewBrickEntryFromId(tx, id)
			if err != nil {
				logger.LogError("Brick %v not found in db: %v", id, err)
				continue
			}
			brick_entries[index] = brick

			// Set ssh host to send volume commands
			if sshhost == "" {
				node, err := NewNodeEntryFromId(tx, brick.Info.NodeId)
				if err != nil {
					logger.LogError("Unable to determine brick node: %v", err)
					return err
				}
				sshhost = node.ManageHostName()
			}
		}
		return nil
	})

	// :TODO: What if the host is no longer available, we may need to try others
	// Stop volume
	err := executor.VolumeDestroy(sshhost, v.Info.Name)
	if err != nil {
		logger.LogError("Unable to delete volume: %v", err)
		return err
	}

	// Destroy bricks
	err = DestroyBricks(db, executor, brick_entries)
	if err != nil {
		logger.LogError("Unable to delete bricks: %v", err)
		return err
	}

	// Remove from entries from the db
	err = db.Update(func(tx *bolt.Tx) error {
		for _, brick := range brick_entries {
			err = v.removeBrickFromDb(tx, brick)
			if err != nil {
				logger.Err(err)
				// Everything is destroyed anyways, just keep deleting the others
				// Do not return here
			}
		}

		// Remove volume from cluster
		cluster, err := NewClusterEntryFromId(tx, v.Info.Cluster)
		if err != nil {
			logger.Err(err)
			// Do not return here.. keep going
		}
		cluster.VolumeDelete(v.Info.Id)

		err = cluster.Save(tx)
		if err != nil {
			logger.Err(err)
			// Do not return here.. keep going
		}

		// Delete volume
		v.Delete(tx)

		return nil
	})

	return err
}

func (v *VolumeEntry) Expand(db *bolt.DB,
	executor executors.Executor,
	sizeGB int) (e error) {

	// Allocate new bricks in the cluster
	brick_entries, err := v.allocBricksInCluster(db, v.Info.Cluster, sizeGB)
	if err != nil {
		return err
	}

	// Setup cleanup function
	defer func() {
		if e != nil {
			logger.Debug("Error detected, cleaning up")

			// Remove from db
			db.Update(func(tx *bolt.Tx) error {
				for _, brick := range brick_entries {
					v.removeBrickFromDb(tx, brick)
				}
				err := v.Save(tx)
				godbc.Check(err == nil)

				return nil
			})
		}
	}()

	// Create bricks
	err = CreateBricks(db, executor, brick_entries)
	if err != nil {
		logger.Err(err)
		return err
	}

	// Setup cleanup function
	defer func() {
		if e != nil {
			logger.Debug("Error detected, cleaning up")
			DestroyBricks(db, executor, brick_entries)
		}
	}()

	// Create a volume request to send to executor
	// so that it can add the new bricks
	vr, host, err := v.createVolumeRequest(db, brick_entries)
	if err != nil {
		return err
	}

	// Expand the volume
	_, err = executor.VolumeExpand(host, vr)
	if err != nil {
		return err
	}

	// Increase the recorded volume size
	v.Info.Size += sizeGB

	// Save volume entry
	err = db.Update(func(tx *bolt.Tx) error {

		// Save brick entries
		for _, brick := range brick_entries {
			err := brick.Save(tx)
			if err != nil {
				return err
			}
		}

		return v.Save(tx)
	})

	return err

}

func (v *VolumeEntry) allocBricksInCluster(db *bolt.DB, cluster string, gbsize int) ([]*BrickEntry, error) {

	// This value will keep being halved until either
	// space is found, or it is determined that the cluster is full
	size := uint64(gbsize) * GB
	volSize := size

	// Continue adjust 'size' until space is found
	for {
		// Determine brick size needed
		brick_size, err := v.determineBrickSize(size)
		if err != nil {
			return nil, err
		}
		logger.Debug("brick_size = %v", brick_size)

		// Calculate number of bricks needed to satisfy the volume request
		// according to the brick size
		num_bricks := int(volSize / brick_size)
		logger.Debug("num_bricks = %v", num_bricks)

		// Check that the volume does not have too many bricks
		if (num_bricks + len(v.Bricks)) > BRICK_MAX_NUM {
			logger.Debug("Maximum number of bricks reached")
			// Try other clusters if possible
			return nil, ErrMaxBricks
		}

		// Allocate bricks in the cluster
		brick_entries, err := v.allocBricks(db, cluster, num_bricks, brick_size)
		if err == ErrNoSpace {
			logger.Debug("No space, need to reduce size and try again")
			// Out of space for the specified brick size, try again
			// with smaller bricks
			size /= 2
			continue
		}
		if err != nil {
			logger.Err(err)
			return nil, err
		}

		// We were able to allocate bricks
		return brick_entries, nil
	}
}

// Return size of each brick in KB, error
func (v *VolumeEntry) determineBrickSize(size uint64) (uint64, error) {
	brick_size := size / 2

	if brick_size < BRICK_MIN_SIZE {
		return 0, ErrMininumBrickSize
	} else if brick_size > BRICK_MAX_SIZE {
		return v.determineBrickSize(brick_size)
	}

	return brick_size, nil
}

func (v *VolumeEntry) allocBricks(
	db *bolt.DB,
	cluster string,
	num_bricks int,
	brick_size uint64) (brick_entries []*BrickEntry, e error) {

	// Setup garbage collector function in case of error
	defer func() {

		// Check the named return value 'err'
		if e != nil {
			logger.Debug("Error detected.  Cleaning up volume %v: Len(%v) ", v.Info.Id, len(brick_entries))
			db.Update(func(tx *bolt.Tx) error {
				for _, brick := range brick_entries {
					v.removeBrickFromDb(tx, brick)
				}
				return nil
			})
		}
	}()

	// Initialize brick_entries
	brick_entries = make([]*BrickEntry, 0)

	// Allocate size for the brick plus the snapshot
	tpsize := uint64(float32(brick_size) * v.Info.Snapshot.Factor)

	// Determine allocation for each brick required for this volume
	for brick_num := 0; brick_num < num_bricks; brick_num++ {

		// Generate an id for the brick
		brickId := utils.GenUUID()

		// Get list of brick locations
		// :TODO: Change this to ring XXXXXXXXXXXXXXXX
		devicelist := NewAllocationList()
		err := db.View(func(tx *bolt.Tx) error {
			devices, err := DeviceList(tx)
			if err != nil {
				return err
			}

			for _, id := range devices {

				device, err := NewDeviceEntryFromId(tx, id)
				if err != nil {
					return err
				}

				node, err := NewNodeEntryFromId(tx, device.NodeId)
				if err != nil {
					return err
				}

				if cluster == node.Info.ClusterId {
					devicelist.Append(id)
				}

			}
			return err
		})
		if err != nil {
			return nil, err
		}

		// Check location has space for each brick and its replicas
		for i := 0; i < v.Info.Replica; i++ {

			// Do the work in the database context so that the cluster
			// data does not change while determining brick location
			err := db.Update(func(tx *bolt.Tx) error {
				for {

					// Check if we have no more nodes
					if devicelist.IsEmpty() {
						return ErrNoSpace
					}

					// Get device entry
					device, err := NewDeviceEntryFromId(tx, devicelist.Pop())
					if err != nil {
						return err
					}

					logger.Debug("device %v[%v] > tpsize [%v] ?",
						device.Id(),
						device.Info.Storage.Free, tpsize)
					// Determine if we have space
					if device.StorageCheck(tpsize) {

						// Create a new brick element
						brick := NewBrickEntry(brick_size,
							tpsize,
							device.Id(),
							device.NodeId)
						if i == 0 {
							brick.SetId(brickId)
						}
						brick_entries = append(brick_entries, brick)

						// Allocate space on device
						device.StorageAllocate(tpsize)

						// Add brick to device
						device.BrickAdd(brick.Id())

						// Add brick to volume
						v.BrickAdd(brick.Id())

						// Save values
						err := device.Save(tx)
						if err != nil {
							return err
						}

						break
					}
				}

				return nil
			})
			if err != nil {
				return brick_entries, err
			}
		}
	}

	return brick_entries, nil

}

func (v *VolumeEntry) removeBrickFromDb(tx *bolt.Tx, brick *BrickEntry) error {

	// Access device
	device, err := NewDeviceEntryFromId(tx, brick.Info.DeviceId)
	if err != nil {
		logger.Err(err)
		return err
	}

	// Deallocate space on device
	device.StorageFree(brick.TpSize)

	// Delete brick from device
	device.BrickDelete(brick.Info.Id)

	// Save device
	err = device.Save(tx)
	if err != nil {
		logger.Err(err)
		return err
	}

	// Delete brick entryfrom db
	err = brick.Delete(tx)
	if err != nil {
		logger.Err(err)
		return err
	}

	// Delete brick from volume db
	v.BrickDelete(brick.Info.Id)
	if err != nil {
		logger.Err(err)
		return err
	}

	return nil
}

func (v *VolumeEntry) BricksIds() sort.StringSlice {
	ids := make(sort.StringSlice, len(v.Bricks))
	copy(ids, v.Bricks)
	return ids
}

func (v *VolumeEntry) createVolume(db *bolt.DB,
	executor executors.Executor,
	brick_entries []*BrickEntry) error {

	godbc.Require(db != nil)
	godbc.Require(brick_entries != nil)

	// Create a volume request for executor with
	// the bricks allocated
	vr, host, err := v.createVolumeRequest(db, brick_entries)
	if err != nil {
		return err
	}

	// Create the volume
	_, err = executor.VolumeCreate(host, vr)
	if err != nil {
		return err
	}

	// Save volume information
	v.Info.Mount.GlusterFS.MountPoint = fmt.Sprintf("%v:%v",
		vr.Bricks[0].Host, vr.Name)

	// Set glusterfs mount volfile-servers options
	v.Info.Mount.GlusterFS.Options = make(map[string]string)
	stringset := utils.NewStringSet()
	for _, brick := range vr.Bricks[1:] {
		if vr.Bricks[0].Host != brick.Host {
			stringset.Add(brick.Host)
		}
	}
	v.Info.Mount.GlusterFS.Options["backupvolfile-servers"] =
		strings.Join(stringset.Strings(), ",")

	godbc.Ensure(v.Info.Mount.GlusterFS.MountPoint != "")
	return nil
}

func (v *VolumeEntry) createVolumeRequest(db *bolt.DB,
	brick_entries []*BrickEntry) (*executors.VolumeRequest, string, error) {
	godbc.Require(db != nil)
	godbc.Require(brick_entries != nil)

	// Setup list of bricks
	vr := &executors.VolumeRequest{}
	vr.Bricks = make([]executors.BrickInfo, len(brick_entries))
	var sshhost string
	for i, b := range brick_entries {

		// Setup path
		vr.Bricks[i].Path = b.Info.Path

		// Get storage host name from Node entry
		err := db.View(func(tx *bolt.Tx) error {
			node, err := NewNodeEntryFromId(tx, b.Info.NodeId)
			if err != nil {
				return err
			}

			if sshhost == "" {
				sshhost = node.ManageHostName()
			}
			vr.Bricks[i].Host = node.StorageHostName()
			godbc.Check(vr.Bricks[i].Host != "")

			return nil
		})
		if err != nil {
			logger.Err(err)
			return nil, "", err
		}
	}

	// Setup volume information in the request
	vr.Name = v.Info.Name
	vr.Replica = v.Info.Replica

	return vr, sshhost, nil
}
