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
	"errors"
	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/executors"
	"github.com/heketi/heketi/tests"
	"github.com/heketi/heketi/utils"
	"os"
	"reflect"
	"sort"
	"testing"
)

func createSampleVolumeEntry(size int) *VolumeEntry {
	req := &VolumeCreateRequest{}
	req.Size = size

	v := NewVolumeEntryFromRequest(req)

	return v
}

func setupSampleDbWithTopology(db *bolt.DB,
	clusters, nodes_per_cluster, devices_per_node int,
	disksize uint64) error {

	return db.Update(func(tx *bolt.Tx) error {
		for c := 0; c < clusters; c++ {
			cluster := createSampleClusterEntry()

			for n := 0; n < nodes_per_cluster; n++ {
				node := createSampleNodeEntry()
				node.Info.ClusterId = cluster.Info.Id
				node.Info.Zone = n % 2

				cluster.NodeAdd(node.Info.Id)

				for d := 0; d < devices_per_node; d++ {
					device := createSampleDeviceEntry(node.Info.Id, disksize)
					node.DeviceAdd(device.Id())

					err := device.Save(tx)
					if err != nil {
						return err
					}
				}
				err := node.Save(tx)
				if err != nil {
					return err
				}
			}
			err := cluster.Save(tx)
			if err != nil {
				return err
			}
		}

		return nil
	})

}

func TestNewVolumeEntry(t *testing.T) {
	v := NewVolumeEntry()

	tests.Assert(t, v.Bricks != nil)
	tests.Assert(t, len(v.Info.Id) == 0)
	tests.Assert(t, len(v.Info.Cluster) == 0)
	tests.Assert(t, len(v.Info.Clusters) == 0)
}

func TestNewVolumeEntryFromRequestOnlySize(t *testing.T) {

	req := &VolumeCreateRequest{}
	req.Size = 1024

	v := NewVolumeEntryFromRequest(req)
	tests.Assert(t, v.Info.Name == "vol_"+v.Info.Id)
	tests.Assert(t, len(v.Info.Clusters) == 0)
	tests.Assert(t, v.Info.Replica == 2)
	tests.Assert(t, v.Info.Snapshot.Enable == false)
	tests.Assert(t, v.Info.Snapshot.Factor == 1)
	tests.Assert(t, v.Info.Size == 1024)
	tests.Assert(t, v.Info.Cluster == "")
	tests.Assert(t, len(v.Info.Id) != 0)
	tests.Assert(t, len(v.Bricks) == 0)

}

func TestNewVolumeEntryFromRequestReplica(t *testing.T) {

	req := &VolumeCreateRequest{}
	req.Size = 1024
	req.Replica = 3

	v := NewVolumeEntryFromRequest(req)
	tests.Assert(t, v.Info.Name == "vol_"+v.Info.Id)
	tests.Assert(t, len(v.Info.Clusters) == 0)
	tests.Assert(t, v.Info.Replica == 3)
	tests.Assert(t, v.Info.Snapshot.Enable == false)
	tests.Assert(t, v.Info.Snapshot.Factor == 1)
	tests.Assert(t, v.Info.Size == 1024)
	tests.Assert(t, v.Info.Cluster == "")
	tests.Assert(t, len(v.Info.Id) != 0)
	tests.Assert(t, len(v.Bricks) == 0)

}

func TestNewVolumeEntryFromRequestClusters(t *testing.T) {

	req := &VolumeCreateRequest{}
	req.Size = 1024
	req.Replica = 3
	req.Clusters = []string{"abc", "def"}

	v := NewVolumeEntryFromRequest(req)
	tests.Assert(t, v.Info.Name == "vol_"+v.Info.Id)
	tests.Assert(t, v.Info.Replica == 3)
	tests.Assert(t, v.Info.Snapshot.Enable == false)
	tests.Assert(t, v.Info.Snapshot.Factor == 1)
	tests.Assert(t, v.Info.Size == 1024)
	tests.Assert(t, reflect.DeepEqual(req.Clusters, v.Info.Clusters))
	tests.Assert(t, len(v.Info.Id) != 0)
	tests.Assert(t, len(v.Bricks) == 0)

}

func TestNewVolumeEntryFromRequestSnapshotEnabledDefaultFactor(t *testing.T) {

	req := &VolumeCreateRequest{}
	req.Size = 1024
	req.Replica = 3
	req.Clusters = []string{"abc", "def"}
	req.Snapshot.Enable = true

	v := NewVolumeEntryFromRequest(req)
	tests.Assert(t, v.Info.Name == "vol_"+v.Info.Id)
	tests.Assert(t, v.Info.Replica == 3)
	tests.Assert(t, v.Info.Snapshot.Enable == true)
	tests.Assert(t, v.Info.Snapshot.Factor == DEFAULT_THINP_SNAPSHOT_FACTOR)
	tests.Assert(t, v.Info.Size == 1024)
	tests.Assert(t, reflect.DeepEqual(req.Clusters, v.Info.Clusters))
	tests.Assert(t, len(v.Info.Id) != 0)
	tests.Assert(t, len(v.Bricks) == 0)

}

func TestNewVolumeEntryFromRequestSnapshotFactor(t *testing.T) {

	req := &VolumeCreateRequest{}
	req.Size = 1024
	req.Replica = 3
	req.Clusters = []string{"abc", "def"}
	req.Snapshot.Enable = true
	req.Snapshot.Factor = 1.3

	v := NewVolumeEntryFromRequest(req)
	tests.Assert(t, v.Info.Name == "vol_"+v.Info.Id)
	tests.Assert(t, v.Info.Replica == 3)
	tests.Assert(t, v.Info.Snapshot.Enable == true)
	tests.Assert(t, v.Info.Snapshot.Factor == 1.3)
	tests.Assert(t, v.Info.Size == 1024)
	tests.Assert(t, reflect.DeepEqual(req.Clusters, v.Info.Clusters))
	tests.Assert(t, len(v.Info.Id) != 0)
	tests.Assert(t, len(v.Bricks) == 0)

}

func TestNewVolumeEntryFromRequestName(t *testing.T) {

	req := &VolumeCreateRequest{}
	req.Size = 1024
	req.Replica = 3
	req.Clusters = []string{"abc", "def"}
	req.Snapshot.Enable = true
	req.Snapshot.Factor = 1.3
	req.Name = "myvol"

	v := NewVolumeEntryFromRequest(req)
	tests.Assert(t, v.Info.Name == "myvol")
	tests.Assert(t, v.Info.Replica == 3)
	tests.Assert(t, v.Info.Snapshot.Enable == true)
	tests.Assert(t, v.Info.Snapshot.Factor == 1.3)
	tests.Assert(t, v.Info.Size == 1024)
	tests.Assert(t, reflect.DeepEqual(req.Clusters, v.Info.Clusters))
	tests.Assert(t, len(v.Info.Id) != 0)
	tests.Assert(t, len(v.Bricks) == 0)

}

func TestNewVolumeEntryMarshal(t *testing.T) {

	req := &VolumeCreateRequest{}
	req.Size = 1024
	req.Replica = 3
	req.Clusters = []string{"abc", "def"}
	req.Snapshot.Enable = true
	req.Snapshot.Factor = 1.3
	req.Name = "myvol"

	v := NewVolumeEntryFromRequest(req)
	v.BrickAdd("abc")
	v.BrickAdd("def")

	buffer, err := v.Marshal()
	tests.Assert(t, err == nil)
	tests.Assert(t, buffer != nil)
	tests.Assert(t, len(buffer) > 0)

	um := &VolumeEntry{}
	err = um.Unmarshal(buffer)
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(v, um))

}

func TestVolumeEntryAddDeleteDevices(t *testing.T) {

	v := NewVolumeEntry()
	tests.Assert(t, len(v.Bricks) == 0)

	v.BrickAdd("123")
	tests.Assert(t, utils.SortedStringHas(v.Bricks, "123"))
	tests.Assert(t, len(v.Bricks) == 1)
	v.BrickAdd("abc")
	tests.Assert(t, utils.SortedStringHas(v.Bricks, "123"))
	tests.Assert(t, utils.SortedStringHas(v.Bricks, "abc"))
	tests.Assert(t, len(v.Bricks) == 2)

	v.BrickDelete("123")
	tests.Assert(t, !utils.SortedStringHas(v.Bricks, "123"))
	tests.Assert(t, utils.SortedStringHas(v.Bricks, "abc"))
	tests.Assert(t, len(v.Bricks) == 1)

	v.BrickDelete("ccc")
	tests.Assert(t, !utils.SortedStringHas(v.Bricks, "123"))
	tests.Assert(t, utils.SortedStringHas(v.Bricks, "abc"))
	tests.Assert(t, len(v.Bricks) == 1)
}

func TestVolumeEntryFromIdNotFound(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Test for ID not found
	err := app.db.View(func(tx *bolt.Tx) error {
		_, err := NewVolumeEntryFromId(tx, "123")
		return err
	})
	tests.Assert(t, err == ErrNotFound)

}

func TestVolumeEntryFromId(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a volume entry
	v := createSampleVolumeEntry(1024)

	// Save in database
	err := app.db.Update(func(tx *bolt.Tx) error {
		return v.Save(tx)
	})
	tests.Assert(t, err == nil)

	// Load from database
	var entry *VolumeEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		var err error
		entry, err = NewVolumeEntryFromId(tx, v.Info.Id)
		return err
	})
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(entry, v))

}

func TestVolumeEntrySaveDelete(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a volume entry
	v := createSampleVolumeEntry(1024)

	// Save in database
	err := app.db.Update(func(tx *bolt.Tx) error {
		return v.Save(tx)
	})
	tests.Assert(t, err == nil)

	// Delete entry which has devices
	var entry *VolumeEntry
	err = app.db.Update(func(tx *bolt.Tx) error {
		var err error
		entry, err = NewVolumeEntryFromId(tx, v.Info.Id)
		if err != nil {
			return err
		}

		err = entry.Delete(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == nil)

	// Check volume has been deleted and is not in db
	err = app.db.View(func(tx *bolt.Tx) error {
		var err error
		entry, err = NewVolumeEntryFromId(tx, v.Info.Id)
		if err != nil {
			return err
		}
		return nil

	})
	tests.Assert(t, err == ErrNotFound)
}

func TestNewVolumeEntryNewInfoResponse(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a volume entry
	v := createSampleVolumeEntry(1024)

	// Save in database
	err := app.db.Update(func(tx *bolt.Tx) error {
		return v.Save(tx)
	})
	tests.Assert(t, err == nil)

	// Retreive info response
	var info *VolumeInfoResponse
	err = app.db.View(func(tx *bolt.Tx) error {
		volume, err := NewVolumeEntryFromId(tx, v.Info.Id)
		if err != nil {
			return err
		}

		info, err = volume.NewInfoResponse(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == nil, err)

	tests.Assert(t, info.Cluster == v.Info.Cluster)
	tests.Assert(t, reflect.DeepEqual(info.Snapshot, v.Info.Snapshot))
	tests.Assert(t, info.Name == v.Info.Name)
	tests.Assert(t, info.Id == v.Info.Id)
	tests.Assert(t, info.Size == v.Info.Size)
	tests.Assert(t, info.Replica == v.Info.Replica)
	tests.Assert(t, len(info.Bricks) == 0)

	// :TODO: Add Mount check
}

func TestVolumeEntryCreateMissingCluster(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a volume entry
	v := createSampleVolumeEntry(1024)
	v.Info.Clusters = []string{}

	// Save in database
	err := app.db.Update(func(tx *bolt.Tx) error {
		return v.Save(tx)
	})
	tests.Assert(t, err == nil)

	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == ErrNoSpace)

}

func TestVolumeEntryCreateRunOutOfSpaceMinBrickSizeLimit(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Total 80GB
	err := setupSampleDbWithTopology(app.db,
		1,     // clusters
		2,     // nodes_per_cluster
		4,     // devices_per_node,
		10*GB, // disksize, 10G)
	)
	tests.Assert(t, err == nil)

	// Create a 100 GB volume
	// Shouldn't be able to break it down enough to allocate volume
	v := createSampleVolumeEntry(100)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == ErrNoSpace)
	tests.Assert(t, v.Info.Cluster == "")

	// Check database volume does not exist
	err = app.db.View(func(tx *bolt.Tx) error {
		_, err := NewVolumeEntryFromId(tx, v.Info.Id)
		return err
	})
	tests.Assert(t, err == ErrNotFound)

	// Check no bricks or volumes exist
	var bricks []string
	var volumes []string
	err = app.db.View(func(tx *bolt.Tx) error {
		bricks = EntryKeys(tx, BOLTDB_BUCKET_BRICK)
		volumes = EntryKeys(tx, BOLTDB_BUCKET_VOLUME)

		return nil
	})
	tests.Assert(t, err == nil)
	tests.Assert(t, len(bricks) == 0, bricks)
	tests.Assert(t, len(volumes) == 0)

}

func TestVolumeEntryCreateRunOutOfSpaceMaxBrickLimit(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Lots of nodes with little drives
	err := setupSampleDbWithTopology(app.db,
		1,  // clusters
		20, // nodes_per_cluster
		40, // devices_per_node,

		// Must be larger than the brick min size
		BRICK_MIN_SIZE*2, // disksize
	)
	tests.Assert(t, err == nil)

	// Create a volume who will be broken down to
	// Shouldn't be able to break it down enough to allocate volume
	v := createSampleVolumeEntry(BRICK_MAX_NUM * 2 * int(BRICK_MIN_SIZE/GB))
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == ErrNoSpace)

	// Check database volume does not exist
	err = app.db.View(func(tx *bolt.Tx) error {
		_, err := NewVolumeEntryFromId(tx, v.Info.Id)
		return err
	})
	tests.Assert(t, err == ErrNotFound)

	// Check no bricks or volumes exist
	var bricks []string
	var volumes []string
	err = app.db.View(func(tx *bolt.Tx) error {
		bricks = EntryKeys(tx, BOLTDB_BUCKET_BRICK)

		volumes = EntryKeys(tx, BOLTDB_BUCKET_VOLUME)
		return nil
	})
	tests.Assert(t, len(bricks) == 0)
	tests.Assert(t, len(volumes) == 0)

}

func TestVolumeEntryCreateFourBricks(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Lots of nodes with little drives
	err := setupSampleDbWithTopology(app.db,
		1,      // clusters
		4,      // nodes_per_cluster
		4,      // devices_per_node,
		500*GB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create a volume who will be broken down to
	// Shouldn't be able to break it down enough to allocate volume
	v := createSampleVolumeEntry(250)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Check database volume does not exist
	var info *VolumeInfoResponse
	err = app.db.View(func(tx *bolt.Tx) error {
		entry, err := NewVolumeEntryFromId(tx, v.Info.Id)
		if err != nil {
			return err
		}

		info, err = entry.NewInfoResponse(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == nil)

	// Check that it used only two bricks each with only two replicas
	tests.Assert(t, len(info.Bricks) == 4)
	tests.Assert(t, info.Bricks[0].Size == info.Bricks[1].Size)
	tests.Assert(t, info.Bricks[0].Size == info.Bricks[2].Size)
	tests.Assert(t, info.Bricks[0].Size == info.Bricks[3].Size)
	tests.Assert(t, info.Cluster == v.Info.Cluster)

	// :TODO: Check mount

}

func TestVolumeEntryCreateBrickDivision(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create 50TB of storage
	err := setupSampleDbWithTopology(app.db,
		1,      // clusters
		10,     // nodes_per_cluster
		10,     // devices_per_node,
		500*GB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create a volume who will be broken down to
	// Shouldn't be able to break it down enough to allocate volume
	v := createSampleVolumeEntry(2000)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Check database volume does not exist
	var info *VolumeInfoResponse
	err = app.db.View(func(tx *bolt.Tx) error {
		entry, err := NewVolumeEntryFromId(tx, v.Info.Id)
		if err != nil {
			return err
		}

		info, err = entry.NewInfoResponse(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == nil)

	// Will need 3 splits for a total of 8 bricks + replicas
	tests.Assert(t, len(info.Bricks) == 16)
	for b := 1; b < 16; b++ {
		tests.Assert(t, info.Bricks[0].Size == info.Bricks[b].Size, b)
	}
	tests.Assert(t, info.Cluster == v.Info.Cluster)

	// :TODO: Check mount

}

func TestVolumeEntryCreateMaxBrickSize(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create 50TB of storage
	err := setupSampleDbWithTopology(app.db,
		1,    // clusters
		10,   // nodes_per_cluster
		10,   // devices_per_node,
		5*TB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create a volume who will be broken down to
	// Shouldn't be able to break it down enough to allocate volume
	v := createSampleVolumeEntry(int(BRICK_MAX_SIZE / GB * 4))
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Check database volume does not exist
	var info *VolumeInfoResponse
	err = app.db.View(func(tx *bolt.Tx) error {
		entry, err := NewVolumeEntryFromId(tx, v.Info.Id)
		if err != nil {
			return err
		}

		info, err = entry.NewInfoResponse(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == nil)

	// Will need to split until less than BRICK_MAX_SIZE, then try to allocate
	// the bricks.
	tests.Assert(t, len(info.Bricks) == 8)
	for b := 1; b < 8; b++ {
		tests.Assert(t, info.Bricks[0].Size == info.Bricks[b].Size, b)
	}
	tests.Assert(t, info.Cluster == v.Info.Cluster)

	// :TODO: Check mount

}

func TestVolumeEntryCreateOnClustersRequested(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create 50TB of storage
	err := setupSampleDbWithTopology(app.db,
		10,   // clusters
		10,   // nodes_per_cluster
		10,   // devices_per_node,
		5*TB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Get a cluster list
	var clusters sort.StringSlice
	err = app.db.View(func(tx *bolt.Tx) error {
		var err error
		clusters, err = ClusterList(tx)
		return err
	})
	tests.Assert(t, err == nil)
	clusters.Sort()

	// Create a 1TB volume
	v := createSampleVolumeEntry(1024)

	// Set the clusters to the first two cluster ids
	v.Info.Clusters = []string{clusters[0]}

	// Create volume
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Check database volume does not exist
	var info *VolumeInfoResponse
	err = app.db.View(func(tx *bolt.Tx) error {
		entry, err := NewVolumeEntryFromId(tx, v.Info.Id)
		if err != nil {
			return err
		}

		info, err = entry.NewInfoResponse(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == nil)
	tests.Assert(t, info.Cluster == clusters[0])

	// Create a new volume on either of three clusters
	clusterset := clusters[2:5]
	v = createSampleVolumeEntry(1024)
	v.Info.Clusters = clusterset
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Check database volume exists
	err = app.db.View(func(tx *bolt.Tx) error {
		entry, err := NewVolumeEntryFromId(tx, v.Info.Id)
		if err != nil {
			return err
		}

		info, err = entry.NewInfoResponse(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == nil)
	tests.Assert(t, utils.SortedStringHas(clusterset, info.Cluster))

}

func TestVolumeEntryCreateCheckingClustersForSpace(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create 100 small clusters
	err := setupSampleDbWithTopology(app.db,
		10,    // clusters
		1,     // nodes_per_cluster
		1,     // devices_per_node,
		10*GB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create one large cluster
	cluster := createSampleClusterEntry()
	err = app.db.Update(func(tx *bolt.Tx) error {
		for n := 0; n < 100; n++ {
			node := createSampleNodeEntry()
			node.Info.ClusterId = cluster.Info.Id
			node.Info.Zone = n % 2

			cluster.NodeAdd(node.Info.Id)

			for d := 0; d < 10; d++ {
				device := createSampleDeviceEntry(node.Info.Id, 4*TB)
				node.DeviceAdd(device.Id())

				err := device.Save(tx)
				if err != nil {
					return err
				}
			}
			err := node.Save(tx)
			if err != nil {
				return err
			}
		}
		err := cluster.Save(tx)
		if err != nil {
			return err
		}

		return nil
	})

	// Create a 1TB volume
	v := createSampleVolumeEntry(1024)

	// Create volume
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Check database volume exists
	var info *VolumeInfoResponse
	err = app.db.View(func(tx *bolt.Tx) error {
		entry, err := NewVolumeEntryFromId(tx, v.Info.Id)
		if err != nil {
			return err
		}

		info, err = entry.NewInfoResponse(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == nil)
	tests.Assert(t, info.Cluster == cluster.Info.Id)
}

func TestVolumeEntryCreateWithSnapshot(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Lots of nodes with little drives
	err := setupSampleDbWithTopology(app.db,
		1,      // clusters
		4,      // nodes_per_cluster
		4,      // devices_per_node,
		500*GB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create a volume with a snapshot factor of 1.5
	// For a 200G vol, it would get a brick size of 100G, with a thin pool
	// size of 100G * 1.5 = 150GB.
	v := createSampleVolumeEntry(200)
	v.Info.Snapshot.Enable = true
	v.Info.Snapshot.Factor = 1.5

	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Check database volume exists
	var info *VolumeInfoResponse
	err = app.db.View(func(tx *bolt.Tx) error {
		entry, err := NewVolumeEntryFromId(tx, v.Info.Id)
		if err != nil {
			return err
		}

		info, err = entry.NewInfoResponse(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == nil)

	// Check that it used only two bricks each with only two replicas
	tests.Assert(t, len(info.Bricks) == 4)
	err = app.db.View(func(tx *bolt.Tx) error {
		for _, b := range info.Bricks {
			device, err := NewDeviceEntryFromId(tx, b.DeviceId)
			if err != nil {
				return err
			}

			tests.Assert(t, device.Info.Storage.Used >= uint64(1.5*float32(b.Size)))
		}

		return nil
	})
	tests.Assert(t, err == nil)
}

func TestVolumeEntryCreateBrickCreationFailure(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Lots of nodes with little drives
	err := setupSampleDbWithTopology(app.db,
		1,      // clusters
		4,      // nodes_per_cluster
		4,      // devices_per_node,
		500*GB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Cause a brick creation failure
	mockerror := errors.New("MOCK")
	app.xo.MockBrickCreate = func(host string, brick *executors.BrickRequest) (*executors.BrickInfo, error) {
		return nil, mockerror
	}

	// Create a volume with a snapshot factor of 1.5
	// For a 200G vol, it would get a brick size of 100G, with a thin pool
	// size of 100G * 1.5 = 150GB.
	v := createSampleVolumeEntry(200)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == mockerror)

	// Check database is still clean. No bricks and No volumes
	err = app.db.View(func(tx *bolt.Tx) error {
		volumes, err := VolumeList(tx)
		tests.Assert(t, err == nil)
		tests.Assert(t, len(volumes) == 0)

		bricks, err := BrickList(tx)
		tests.Assert(t, err == nil)
		tests.Assert(t, len(bricks) == 0)

		clusters, err := ClusterList(tx)
		tests.Assert(t, err == nil)
		tests.Assert(t, len(clusters) == 1)

		cluster, err := NewClusterEntryFromId(tx, clusters[0])
		tests.Assert(t, err == nil)
		tests.Assert(t, len(cluster.Info.Volumes) == 0)

		return nil

	})
	tests.Assert(t, err == nil)
}

func TestVolumeEntryCreateVolumeCreationFailure(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Lots of nodes with little drives
	err := setupSampleDbWithTopology(app.db,
		1,      // clusters
		4,      // nodes_per_cluster
		4,      // devices_per_node,
		500*GB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Cause a brick creation failure
	mockerror := errors.New("MOCK")
	app.xo.MockVolumeCreate = func(host string, volume *executors.VolumeRequest) (*executors.VolumeInfo, error) {
		return nil, mockerror
	}

	// Create a volume with a snapshot factor of 1.5
	// For a 200G vol, it would get a brick size of 100G, with a thin pool
	// size of 100G * 1.5 = 150GB.
	v := createSampleVolumeEntry(200)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == mockerror)

	// Check database is still clean. No bricks and No volumes
	err = app.db.View(func(tx *bolt.Tx) error {
		volumes, err := VolumeList(tx)
		tests.Assert(t, err == nil)
		tests.Assert(t, len(volumes) == 0)

		bricks, err := BrickList(tx)
		tests.Assert(t, err == nil)
		tests.Assert(t, len(bricks) == 0)

		clusters, err := ClusterList(tx)
		tests.Assert(t, err == nil)
		tests.Assert(t, len(clusters) == 1)

		cluster, err := NewClusterEntryFromId(tx, clusters[0])
		tests.Assert(t, err == nil)
		tests.Assert(t, len(cluster.Info.Volumes) == 0)

		return nil

	})
	tests.Assert(t, err == nil)
}

func TestVolumeEntryDestroy(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Lots of nodes with little drives
	err := setupSampleDbWithTopology(app.db,
		1,      // clusters
		4,      // nodes_per_cluster
		4,      // devices_per_node,
		500*GB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create a volume with a snapshot factor of 1.5
	// For a 200G vol, it would get a brick size of 100G, with a thin pool
	// size of 100G * 1.5 = 150GB.
	v := createSampleVolumeEntry(200)
	v.Info.Snapshot.Enable = true
	v.Info.Snapshot.Factor = 1.5

	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Destroy the volume
	err = v.Destroy(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Check database volume does not exist
	var bricks []string
	err = app.db.View(func(tx *bolt.Tx) error {
		bricks, err = BrickList(tx)
		return err
	})
	tests.Assert(t, err == nil)

	// Check that there are no bricks
	tests.Assert(t, len(bricks) == 0)

	// Check that the devices have no bricks
	err = app.db.View(func(tx *bolt.Tx) error {
		devices, err := DeviceList(tx)
		if err != nil {
			return err
		}

		for _, id := range devices {
			device, err := NewDeviceEntryFromId(tx, id)
			if err != nil {
				return err
			}
			tests.Assert(t, len(device.Bricks) == 0, id, device)
		}

		return err
	})
	tests.Assert(t, err == nil)

	// Check that the cluster has no volumes
	err = app.db.View(func(tx *bolt.Tx) error {
		clusters, err := ClusterList(tx)
		if err != nil {
			return err
		}

		tests.Assert(t, len(clusters) == 1)
		cluster, err := NewClusterEntryFromId(tx, clusters[0])
		tests.Assert(t, err == nil)
		tests.Assert(t, len(cluster.Info.Volumes) == 0)

		return nil
	})
	tests.Assert(t, err == nil)

}

func TestVolumeEntryExpandNoSpace(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create cluster
	err := setupSampleDbWithTopology(app.db,
		10,     // clusters
		1,      // nodes_per_cluster
		2,      // devices_per_node,
		600*GB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create large volume
	v := createSampleVolumeEntry(599)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Save a copy of the volume before expansion
	vcopy := &VolumeEntry{}
	*vcopy = *v

	// Asking for a large amount will require too many little bricks
	err = v.Expand(app.db, app.executor, 5000)
	tests.Assert(t, err == ErrMaxBricks, err)

	// Asking for a small amount will set the bricks too small
	err = v.Expand(app.db, app.executor, 10)
	tests.Assert(t, err == ErrMininumBrickSize)

	// Check db is the same as before expansion
	var entry *VolumeEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		var err error
		entry, err = NewVolumeEntryFromId(tx, v.Info.Id)

		return err
	})
	tests.Assert(t, err == nil, err)
	tests.Assert(t, reflect.DeepEqual(vcopy, entry))
}

func TestVolumeEntryExpandCreateBricksFailure(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create large cluster
	err := setupSampleDbWithTopology(app.db,
		10,     // clusters
		10,     // nodes_per_cluster
		20,     // devices_per_node,
		600*GB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create volume
	v := createSampleVolumeEntry(100)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Save a copy of the volume before expansion
	vcopy := &VolumeEntry{}
	*vcopy = *v

	// Mock create bricks to fail
	ErrMock := errors.New("MOCK")
	app.xo.MockBrickCreate = func(host string, brick *executors.BrickRequest) (*executors.BrickInfo, error) {
		return nil, ErrMock
	}

	// Expand volume
	err = v.Expand(app.db, app.executor, 500)
	tests.Assert(t, err == ErrMock)

	// Check db is the same as before expansion
	var entry *VolumeEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		var err error
		entry, err = NewVolumeEntryFromId(tx, v.Info.Id)

		return err
	})
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(vcopy, entry))
}

func TestVolumeEntryExpand(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create large cluster
	err := setupSampleDbWithTopology(app.db,
		1,    // clusters
		10,   // nodes_per_cluster
		20,   // devices_per_node,
		6*TB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create volume
	v := createSampleVolumeEntry(1024)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)
	tests.Assert(t, v.Info.Size == 1024)
	tests.Assert(t, len(v.Bricks) == 4)

	// Expand volume
	err = v.Expand(app.db, app.executor, 1234)
	tests.Assert(t, err == nil)
	tests.Assert(t, v.Info.Size == 1024+1234)
	tests.Assert(t, len(v.Bricks) == 8)

	// Check db
	var entry *VolumeEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		var err error
		entry, err = NewVolumeEntryFromId(tx, v.Info.Id)

		return err
	})
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(entry, v))
}
