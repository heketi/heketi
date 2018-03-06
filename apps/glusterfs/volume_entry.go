//
// Copyright (c) 2015 The heketi Authors
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
	"fmt"
	"sort"

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/executors"
	wdb "github.com/heketi/heketi/pkg/db"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/utils"
	"github.com/lpabon/godbc"
)

const (

	// Byte values in KB
	KB = 1
	MB = KB * 1024
	GB = MB * 1024
	TB = GB * 1024

	// Default values
	DEFAULT_REPLICA               = 2
	DEFAULT_EC_DATA               = 4
	DEFAULT_EC_REDUNDANCY         = 2
	DEFAULT_THINP_SNAPSHOT_FACTOR = 1.5
)

// VolumeEntry struct represents a volume in heketi. Serialization is done using
// gob when written to db and using json package when exportdb/importdb is used
// There are two reasons I skip Durability field for json pkg
//   1. Durability is used in some places in code, however, it represents the
//      same info that is in Info.Durability.
//   2. I wasn't able to serialize interface type to json in a straightfoward
//      way.
// Chose to skip writing redundant data than adding kludgy code
type VolumeEntry struct {
	Info                 api.VolumeInfo
	Bricks               sort.StringSlice
	Durability           VolumeDurability `json:"-"`
	GlusterVolumeOptions []string
	Pending              PendingItem
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

func NewVolumeEntryFromRequest(req *api.VolumeCreateRequest) *VolumeEntry {
	godbc.Require(req != nil)

	vol := NewVolumeEntry()
	vol.Info.Gid = req.Gid
	vol.Info.Id = utils.GenUUID()
	vol.Info.Durability = req.Durability
	vol.Info.Snapshot = req.Snapshot
	vol.Info.Size = req.Size
	vol.Info.Block = req.Block

	if vol.Info.Block {
		vol.Info.BlockInfo.FreeSize = req.Size
		vol.GlusterVolumeOptions = []string{"group gluster-block"}

	}

	// Set default durability values
	durability := vol.Info.Durability.Type
	switch {

	case durability == api.DurabilityReplicate:
		logger.Debug("[%v] Replica %v",
			vol.Info.Id,
			vol.Info.Durability.Replicate.Replica)
		vol.Durability = NewVolumeReplicaDurability(&vol.Info.Durability.Replicate)

	case durability == api.DurabilityEC:
		logger.Debug("[%v] EC %v + %v ",
			vol.Info.Id,
			vol.Info.Durability.Disperse.Data,
			vol.Info.Durability.Disperse.Redundancy)
		vol.Durability = NewVolumeDisperseDurability(&vol.Info.Durability.Disperse)

	case durability == api.DurabilityDistributeOnly || durability == "":
		logger.Debug("[%v] Distributed", vol.Info.Id)
		vol.Durability = NewNoneDurability()

	default:
		panic(fmt.Sprintf("BUG: Unknown type: %v\n", vol.Info.Durability))
	}

	// Set the default values accordingly
	vol.Durability.SetDurability()

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

	// If it is zero, then no volume options are set.
	vol.GlusterVolumeOptions = req.GlusterVolumeOptions

	// If it is zero, then it will be assigned during volume creation
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

func (v *VolumeEntry) NewInfoResponse(tx *bolt.Tx) (*api.VolumeInfoResponse, error) {
	godbc.Require(tx != nil)

	info := api.NewVolumeInfoResponse()
	info.Id = v.Info.Id
	info.Cluster = v.Info.Cluster
	info.Mount = v.Info.Mount
	info.Snapshot = v.Info.Snapshot
	info.Size = v.Info.Size
	info.Durability = v.Info.Durability
	info.Name = v.Info.Name
	info.GlusterVolumeOptions = v.GlusterVolumeOptions
	info.Block = v.Info.Block
	info.BlockInfo = v.Info.BlockInfo

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

func (v *VolumeEntry) Create(db wdb.DB,
	executor executors.Executor) (e error) {

	return RunOperation(
		NewVolumeCreateOperation(v, db),
		executor)
}

func (v *VolumeEntry) tryAllocateBricks(
	db wdb.DB,
	possibleClusters []string) (brick_entries []*BrickEntry, err error) {

	for _, cluster := range possibleClusters {
		// Check this cluster for space
		brick_entries, err = v.allocBricksInCluster(db, cluster, v.Info.Size)

		if err == nil {
			v.Info.Cluster = cluster
			logger.Debug("Volume to be created on cluster %v", cluster)
			break
		} else if err == ErrNoSpace ||
			err == ErrMaxBricks ||
			err == ErrMinimumBrickSize {
			logger.Debug("Cluster %v can not accommodate volume "+
				"(%v), trying next cluster", cluster, err)
			continue
		} else {
			// A genuine error occurred - bail out
			logger.LogError("Error calling v.allocBricksInCluster: %v", err)
			return
		}
	}
	return
}

func (v *VolumeEntry) cleanupCreateVolume(db wdb.DB,
	executor executors.Executor,
	brick_entries []*BrickEntry) error {

	// from a quick read its "safe" to unconditionally try to delete
	// bricks. TODO: find out if that is true with functional tests
	DestroyBricks(db, executor, brick_entries)
	return db.Update(func(tx *bolt.Tx) error {
		for _, brick := range brick_entries {
			v.removeBrickFromDb(tx, brick)
		}
		if v.Info.Cluster != "" {
			cluster, err := NewClusterEntryFromId(tx, v.Info.Cluster)
			if err == nil {
				cluster.VolumeDelete(v.Info.Id)
				cluster.Save(tx)
			}
		}
		v.Delete(tx)
		return nil
	})
}

func (v *VolumeEntry) createOneShot(db wdb.DB,
	executor executors.Executor) (e error) {

	var brick_entries []*BrickEntry
	// On any error, remove the volume
	defer func() {
		if e != nil {
			v.cleanupCreateVolume(db, executor, brick_entries)
		}
	}()

	brick_entries, e = v.createVolumeComponents(db)
	if e != nil {
		return e
	}
	return v.createVolumeExec(db, executor, brick_entries)
}

func (v *VolumeEntry) createVolumeComponents(db wdb.DB) (
	brick_entries []*BrickEntry, e error) {

	// Get list of clusters
	var possibleClusters []string
	if len(v.Info.Clusters) == 0 {
		err := db.View(func(tx *bolt.Tx) error {
			var err error
			possibleClusters, err = ClusterList(tx)
			return err
		})
		if err != nil {
			return brick_entries, err
		}
	} else {
		possibleClusters = v.Info.Clusters
	}

	cr := ClusterReq{v.Info.Block, v.Info.Name}
	possibleClusters, err := eligibleClusters(db, cr, possibleClusters)
	if err != nil {
		return brick_entries, err
	}
	if len(possibleClusters) == 0 {
		logger.LogError("No clusters eligible to satisfy create volume request")
		return brick_entries, ErrNoSpace
	}
	logger.Debug("Using the following clusters: %+v", possibleClusters)

	return v.saveCreateVolume(db, possibleClusters)
}

func (v *VolumeEntry) createVolumeExec(db wdb.DB,
	executor executors.Executor,
	brick_entries []*BrickEntry) (e error) {

	// Create the bricks on the nodes
	e = CreateBricks(db, executor, brick_entries)
	if e != nil {
		return
	}

	// Create GlusterFS volume
	return v.createVolume(db, executor, brick_entries)
}

func (v *VolumeEntry) saveCreateVolume(db wdb.DB,
	possibleClusters []string) (brick_entries []*BrickEntry, err error) {

	err = db.Update(func(tx *bolt.Tx) error {
		txdb := wdb.WrapTx(tx)
		// For each cluster look for storage space for this volume
		brick_entries, err = v.tryAllocateBricks(txdb, possibleClusters)
		if err != nil || brick_entries == nil {
			// Map all 'valid' errors to NoSpace here:
			// Only the last such error could get propagated down,
			// so it does not make sense to hand the granularity on.
			// But for other callers (Expand), we keep it.
			return ErrNoSpace
		}

		err = v.updateMountInfo(txdb)
		if err != nil {
			return err
		}

		// Save volume information
		if v.Info.Block {
			v.Info.BlockInfo.FreeSize = v.Info.Size
		}
		err := v.Save(tx)
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
	return
}

func (v *VolumeEntry) deleteVolumeExec(db wdb.RODB,
	executor executors.Executor,
	brick_entries []*BrickEntry,
	sshhost string) error {

	// Determine if we can destroy the volume
	err := executor.VolumeDestroyCheck(sshhost, v.Info.Name)
	if err != nil {
		logger.Err(err)
		return err
	}

	// Determine if the bricks can be destroyed
	err = v.checkBricksCanBeDestroyed(db, executor, brick_entries)
	if err != nil {
		logger.Err(err)
		return err
	}

	// :TODO: What if the host is no longer available, we may need to try others
	// Stop volume
	err = executor.VolumeDestroy(sshhost, v.Info.Name)
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

	return nil
}

func (v *VolumeEntry) saveDeleteVolume(db wdb.DB,
	brick_entries []*BrickEntry) error {

	// Remove from entries from the db
	return db.Update(func(tx *bolt.Tx) error {
		for _, brick := range brick_entries {
			err := v.removeBrickFromDb(tx, brick)
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
}

func (v *VolumeEntry) manageHostFromBricks(db wdb.DB,
	brick_entries []*BrickEntry) (sshhost string, err error) {

	err = db.View(func(tx *bolt.Tx) error {
		for _, brick := range brick_entries {
			node, err := NewNodeEntryFromId(tx, brick.Info.NodeId)
			if err != nil {
				return err
			}
			sshhost = node.ManageHostName()
			return nil
		}
		return fmt.Errorf("Unable to get management host from bricks")
	})
	return
}

func (v *VolumeEntry) deleteVolumeComponents(
	db wdb.RODB) (brick_entries []*BrickEntry, e error) {

	e = db.View(func(tx *bolt.Tx) error {
		for _, id := range v.BricksIds() {
			brick, err := NewBrickEntryFromId(tx, id)
			if err != nil {
				logger.LogError("Brick %v not found in db: %v", id, err)
				return err
			}
			brick_entries = append(brick_entries, brick)
		}
		return nil
	})
	return
}

func (v *VolumeEntry) Destroy(db wdb.DB, executor executors.Executor) error {
	logger.Info("Destroying volume %v", v.Info.Id)

	return RunOperation(
		NewVolumeDeleteOperation(v, db),
		executor)
}

func (v *VolumeEntry) expandVolumeComponents(db wdb.DB,
	sizeGB int,
	setSize bool) (brick_entries []*BrickEntry, e error) {

	e = db.Update(func(tx *bolt.Tx) error {
		// Allocate new bricks in the cluster
		txdb := wdb.WrapTx(tx)
		var err error
		brick_entries, err = v.allocBricksInCluster(txdb, v.Info.Cluster, sizeGB)
		if err != nil {
			return err
		}

		// Increase the recorded volume size
		if setSize {
			v.Info.Size += sizeGB
		}

		// Save brick entries
		for _, brick := range brick_entries {
			err := brick.Save(tx)
			if err != nil {
				return err
			}
		}

		return v.Save(tx)
	})
	return
}

func (v *VolumeEntry) cleanupExpandVolume(db wdb.DB,
	executor executors.Executor,
	brick_entries []*BrickEntry,
	origSize int) (e error) {

	logger.Debug("Error detected, cleaning up")
	DestroyBricks(db, executor, brick_entries)

	// Remove from db
	return db.Update(func(tx *bolt.Tx) error {
		for _, brick := range brick_entries {
			v.removeBrickFromDb(tx, brick)
		}
		v.Info.Size = origSize
		err := v.Save(tx)
		godbc.Check(err == nil)

		return nil
	})
}

func (v *VolumeEntry) expandVolumeExec(db wdb.DB,
	executor executors.Executor,
	brick_entries []*BrickEntry) (e error) {

	// Create bricks
	err := CreateBricks(db, executor, brick_entries)
	if err != nil {
		return err
	}

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

	return err
}

func (v *VolumeEntry) Expand(db wdb.DB,
	executor executors.Executor,
	sizeGB int) (e error) {

	return RunOperation(
		NewVolumeExpandOperation(v, db, sizeGB),
		executor)
}

func (v *VolumeEntry) BricksIds() sort.StringSlice {
	ids := make(sort.StringSlice, len(v.Bricks))
	copy(ids, v.Bricks)
	return ids
}

func (v *VolumeEntry) checkBricksCanBeDestroyed(db wdb.RODB,
	executor executors.Executor,
	brick_entries []*BrickEntry) error {

	sg := utils.NewStatusGroup()

	// Create a goroutine for each brick
	for _, brick := range brick_entries {
		sg.Add(1)
		go func(b *BrickEntry) {
			defer sg.Done()
			sg.Err(b.DestroyCheck(db, executor))
		}(brick)
	}

	// Wait here until all goroutines have returned.  If
	// any of errored, it would be cought here
	err := sg.Result()
	if err != nil {
		logger.Err(err)
	}
	return err
}

func VolumeEntryUpgrade(tx *bolt.Tx) error {
	return nil
}

func (v *VolumeEntry) BlockVolumeAdd(id string) {
	v.Info.BlockInfo.BlockVolumes = append(v.Info.BlockInfo.BlockVolumes, id)
	v.Info.BlockInfo.BlockVolumes.Sort()
}

func (v *VolumeEntry) BlockVolumeDelete(id string) {
	v.Info.BlockInfo.BlockVolumes = utils.SortedStringsDelete(v.Info.BlockInfo.BlockVolumes, id)
}

// Visible returns true if this volume is meant to be visible to
// API calls.
func (v *VolumeEntry) Visible() bool {
	return v.Pending.Id == ""
}

func volumeNameExistsInCluster(tx *bolt.Tx, cluster *ClusterEntry,
	name string) (found bool, e error) {
	for _, volumeId := range cluster.Info.Volumes {
		volume, err := NewVolumeEntryFromId(tx, volumeId)
		if err != nil {
			return false, err
		}
		if name == volume.Info.Name {
			found = true
			return
		}
	}

	return
}

type ClusterReq struct {
	Block bool
	Name  string
}

func eligibleClusters(db wdb.RODB, req ClusterReq,
	possibleClusters []string) ([]string, error) {
	//
	// If the request carries the Block flag, consider only
	// those clusters that carry the Block flag if there are
	// any, otherwise consider all clusters.
	// If the request does *not* carry the Block flag, consider
	// only those clusters that do not carry the Block flag.
	//
	candidateClusters := []string{}
	err := db.View(func(tx *bolt.Tx) error {
		for _, clusterId := range possibleClusters {
			c, err := NewClusterEntryFromId(tx, clusterId)
			if err != nil {
				return err
			}
			switch {
			case req.Block && c.Info.Block:
			case !req.Block && c.Info.File:
			case !(c.Info.Block || c.Info.File):
				// possibly bad cluster config
				logger.Info("Cluster %v lacks both block and file flags",
					clusterId)
				continue
			default:
				continue
			}
			if req.Name != "" {
				found, err := volumeNameExistsInCluster(tx, c, req.Name)
				if err != nil {
					return err
				}
				if found {
					logger.LogError("Name %v already in use in cluster %v",
						req.Name, clusterId)
					continue
				}
			}
			candidateClusters = append(candidateClusters, clusterId)
		}
		return nil
	})

	return candidateClusters, err
}
