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
	"sort"
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
	BRICK_MIN_SIZE = uint64(1 * GB)
	BRICK_MAX_SIZE = uint64(4 * TB)
	BRICK_MAX_NUM  = 200
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
	info.Mount.GlusterFS.MountPoint = "/some/mount"
	info.Mount.GlusterFS.Options["some"] = "options"
	info.Snapshot = v.Info.Snapshot
	info.Size = v.Info.Size
	info.Replica = v.Info.Replica
	info.Name = v.Info.Name

	for _, brickid := range v.Bricks {
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

func (v *VolumeEntry) Create(db *bolt.DB) error {

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

	// Volume size in KB
	volSize := uint64(v.Info.Size) * GB

	// For each cluster look for storage space for this volume
	var err error
	for _, cluster := range clusters {
		// :TODO: Make this a function to handle err easier

		// Set size bricks will satisfy.  This value will keep
		// being halved until either space is found, or it is
		// determined that the cluster is full
		size := volSize

		// Continue adjust 'size' until space is found
		for done := false; !done; {
			// Determine brick size needed
			var brick_size uint64
			brick_size, err = v.determineBrickSize(size)
			if err != nil {
				done = true
				continue
			}
			logger.Debug("brick_size = %v", brick_size)

			// Calculate number of bricks needed to satisfy the volume request
			// according to the brick size
			num_bricks := int(volSize / brick_size)
			logger.Debug("num_bricks = %v", num_bricks)

			// Check that the volume does not have too many bricks
			if num_bricks > BRICK_MAX_NUM {
				logger.Debug("Maximum number of bricks reached")
				// Try other clusters if possible
				err = ErrMaxBricks
				done = true
				continue
			}

			// Allocate bricks in the cluster
			err = v.allocBricks(db, cluster, num_bricks, brick_size)
			if err == ErrNoSpace {
				logger.Debug("No space, need to reduce size and try again")
				// Out of space for the specified brick size, try again
				// with smaller bricks
				size /= 2
				continue
			}
			if err != nil {
				logger.Err(err)
				// Unknown error occurred, let's try another cluster
				done = true
				continue
			}

			logger.Debug("Volume to be created on cluster %v", cluster)

			// We were able to allocate bricks
			return nil
		}
	}
	if err != nil {
		return err
	}

	// Create bricks

	// Create GlusterFS volume

	return nil
}

func (v *VolumeEntry) Destroy(db *bolt.DB) error {
	logger.Info("Destroying volume %v", v.Info.Id)

	// Stop volume

	// Destory bricks
	sg := utils.NewStatusGroup()
	db.View(func(tx *bolt.Tx) error {
		for _, id := range v.Bricks {
			brick, err := NewBrickEntryFromId(tx, id)
			if err != nil {
				logger.LogError("Brick %v not found in db: %v", id, err)
				continue
			}

			sg.Add(1)
			go func(b *BrickEntry) {
				defer sg.Done()
				sg.Err(b.Destroy(db))
			}(brick)
		}

		return nil
	})
	err := sg.Result()
	if err != nil {
		logger.LogError("Unable to delete bricks: %v", err)
		return err
	}

	// Remove from db
	err = db.Update(func(tx *bolt.Tx) error {
		for _, brickid := range v.Bricks {
			brick, err := NewBrickEntryFromId(tx, brickid)
			if err != nil {
				logger.Err(err)
				return err
			}

			device, err := NewDeviceEntryFromId(tx, brick.Info.DeviceId)
			if err != nil {
				logger.Err(err)
				return err
			}

			device.BrickDelete(brickid)

			err = brick.Delete(tx)
			if err != nil {
				logger.Err(err)
				return err
			}

			err = device.Save(tx)
			if err != nil {
				logger.Err(err)
				return err
			}

		}
		v.Delete(tx)

		return nil
	})

	return err
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
	brick_size uint64) (err error) {

	// Setup garbage collector function in case of error
	defer func() {

		// Check the named return value 'err'
		if err != nil {
			logger.Debug("Error detected.  Cleaning up volume %v", v.Info.Id)
			db.Update(func(tx *bolt.Tx) error {
				for _, brickid := range v.Bricks {
					brick, err := NewBrickEntryFromId(tx, brickid)
					godbc.Check(err == nil)

					device, err := NewDeviceEntryFromId(tx, brick.Info.DeviceId)
					godbc.Check(err == nil)

					device.BrickDelete(brickid)

					err = brick.Delete(tx)
					godbc.Check(err == nil)

					err = device.Save(tx)
					godbc.Check(err == nil)

				}
				v.Delete(tx)

				return nil
			})
			v.Bricks = make(sort.StringSlice, 0)
		}
	}()

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
			return err
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
						brick := NewBrickEntry(brick_size, device.Id(), device.NodeId)
						if i == 0 {
							brick.SetId(brickId)
						}

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
						err = brick.Save(tx)
						if err != nil {
							return err
						}
						err = v.Save(tx)
						if err != nil {
							return err
						}

						break
					}
				}

				return nil
			})
			if err != nil {
				return err
			}
		}
	}

	// Save this cluster
	v.Info.Cluster = cluster

	// Add to cluster
	err = db.Update(func(tx *bolt.Tx) error {

		// Save volume information
		err = v.Save(tx)
		if err != nil {
			return err
		}

		// Save cluster
		entry, err := NewClusterEntryFromId(tx, cluster)
		if err != nil {
			return err
		}
		entry.VolumeAdd(v.Info.Id)
		return entry.Save(tx)
	})
	if err != nil {
		return err
	}

	return nil

}
