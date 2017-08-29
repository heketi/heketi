package glusterfs

import (
	"fmt"

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/executors"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/lpabon/godbc"
)

func (v *VolumeEntry) GeoReplicationAction(db *bolt.DB,
	executor executors.Executor,
	host string,
	msg api.GeoReplicationRequest) error {

	logger.Debug("In GeoReplicationAction")

	godbc.Require(db != nil)

	geoRep := &executors.GeoReplicationRequest{
		ActionParams: msg.ActionParams,
		SlaveVolume:  msg.SlaveVolume,
		SlaveHost:    msg.SlaveHost,
		SlaveSSHPort: msg.SlaveSSHPort,
	}

	switch msg.Action {
	case api.GeoReplicationActionCreate:
		logger.Info("Creating geo-replication session for volume %s", v.Info.Id)
		if err := executor.GeoReplicationCreate(host, v.Info.Name, geoRep); err != nil {
			return err
		}
	case api.GeoReplicationActionConfig:
		logger.Info("Configuring geo-replication session for volume %s", v.Info.Id)
		if err := executor.GeoReplicationConfig(host, v.Info.Name, geoRep); err != nil {
			return err
		}
	case api.GeoReplicationActionStart, api.GeoReplicationActionStop, api.GeoReplicationActionPause, api.GeoReplicationActionResume, api.GeoReplicationActionDelete:
		action := string(msg.Action)
		logger.Info("Executing action %s geo-replication session for volume %s", action, v.Info.Id)
		if err := executor.GeoReplicationAction(host, v.Info.Name, action, geoRep); err != nil {
			return err
		}
	default:
		logger.LogError("Unsupported action %s", msg.Action)
	}

	return nil
}

//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

// NewGeoReplicationStatusResponse returns a geo-replication status response
// populated with data from the executor
func (v *VolumeEntry) NewGeoReplicationStatusResponse(executor executors.Executor,
	host string) (resp *api.GeoReplicationStatus, err error) {

	status, err := executor.GeoReplicationVolumeStatus(host, v.Info.Name)
	if err != nil {
		return nil, err
	}

	if len(status.Volume) < 1 {
		return nil, fmt.Errorf("Could not get replication status for volume %s", v.Info.Id)
	}

	volume := newGeoReplicationVolume(status.Volume[0])

	resp = &api.GeoReplicationStatus{
		Volumes: []api.GeoReplicationVolume{
			volume,
		},
	}

	return resp, nil
}

// NewGeoReplicationStatusResponse returns a geo-replication status response
// populated with data from the executor
func (n *NodeEntry) NewGeoReplicationStatusResponse(executor executors.Executor) (resp *api.GeoReplicationStatus, err error) {

	status, err := executor.GeoReplicationStatus(n.ManageHostName())
	if err != nil {
		return nil, err
	}

	resp = &api.GeoReplicationStatus{
		Volumes: []api.GeoReplicationVolume{},
	}

	for _, volume := range status.Volume {
		v := newGeoReplicationVolume(volume)
		resp.Volumes = append(resp.Volumes, v)
	}

	return resp, nil
}

func newGeoReplicationVolume(v executors.GeoReplicationVolume) api.GeoReplicationVolume {
	result := api.GeoReplicationVolume{
		VolumeName: v.VolumeName,
		Sessions:   api.GeoReplicationSessions{},
	}

	for _, session := range v.Sessions.SessionList {
		p := []api.GeoReplicationPair{}
		for _, pair := range session.Pairs {
			p = append(p, api.GeoReplicationPair{
				MasterNode:               pair.MasterNode,
				MasterBrick:              pair.MasterBrick,
				SlaveUser:                pair.SlaveUser,
				Slave:                    pair.Slave,
				SlaveNode:                pair.SlaveNode,
				Status:                   pair.Status,
				CrawlStatus:              pair.CrawlStatus,
				Entry:                    pair.Entry,
				Data:                     pair.Data,
				Meta:                     pair.Meta,
				Failures:                 pair.Failures,
				CheckpointCompleted:      pair.CheckpointCompleted,
				MasterNodeUUID:           pair.MasterNodeUUID,
				LastSynced:               pair.LastSynced,
				CheckpointTime:           pair.CheckpointTime,
				CheckpointCompletionTime: pair.CheckpointCompletionTime,
			})
		}

		result.Sessions.SessionList = append(result.Sessions.SessionList, api.GeoReplicationSession{
			SessionSlave: session.SessionSlave,
			Pairs:        p,
		})
	}
	return result
}
