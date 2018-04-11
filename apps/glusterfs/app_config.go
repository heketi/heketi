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
	"encoding/json"
	"io"

	"github.com/heketi/heketi/executors/kubeexec"
	"github.com/heketi/heketi/executors/sshexec"
)

type GlusterFSConfig struct {
	DBfile     string              `json:"db"`
	Executor   string              `json:"executor"`
	Allocator  string              `json:"allocator"`
	SshConfig  sshexec.SshConfig   `json:"sshexec"`
	KubeConfig kubeexec.KubeConfig `json:"kubeexec"`
	Loglevel   string              `json:"loglevel"`

	// advanced settings
	BrickMaxSize int `json:"brick_max_size_gb"`
	BrickMinSize int `json:"brick_min_size_gb"`
	BrickMaxNum  int `json:"max_bricks_per_volume"`

	//block settings
	CreateBlockHostingVolumes bool `json:"auto_create_block_hosting_volume"`
	BlockHostingVolumeSize    int  `json:"block_hosting_volume_size"`

	// server behaviors
	IgnoreStaleOperations          bool   `json:"ignore_stale_operations"`
	RefreshTimeMonitorGlusterNodes uint32 `json:"refresh_time_monitor_gluster_nodes"`
	StartTimeMonitorGlusterNodes   uint32 `json:"start_time_monitor_gluster_nodes"`
}

type ConfigFile struct {
	GlusterFS GlusterFSConfig `json:"glusterfs"`
}

func loadConfiguration(configIo io.Reader) *GlusterFSConfig {
	configParser := json.NewDecoder(configIo)

	var config ConfigFile
	if err := configParser.Decode(&config); err != nil {
		logger.LogError("Unable to parse config file: %v\n",
			err.Error())
		return nil
	}
	return &config.GlusterFS
}
