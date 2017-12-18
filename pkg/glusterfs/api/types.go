//
// Copyright (c) 2016 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), as published by the Free Software Foundation,
// or under the Apache License, Version 2.0 <LICENSE-APACHE2 or
// http://www.apache.org/licenses/LICENSE-2.0>.
//
// You may not use this file except in compliance with those terms.
//

//
// Please see https://github.com/heketi/heketi/wiki/API
// for documentation
//
package api

import (
	"fmt"
	"regexp"
	"sort"

	"github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"
)

var (
	// Restricting the deviceName to much smaller subset of Unix Path
	// as unix path takes almost everything except NULL
	deviceNameRe = regexp.MustCompile("^/[a-zA-Z0-9_./-]+$")

	// Volume name constraints decided by looking at
	// "cli_validate_volname" function in cli-cmd-parser.c of gluster code
	volumeNameRe = regexp.MustCompile("^[a-zA-Z0-9_-]+$")

	blockVolNameRe = regexp.MustCompile("^[a-zA-Z0-9_-]+$")
)

// ValidateUUID is written this way because heketi UUID does not
// conform to neither UUID v4 nor v5.
func ValidateUUID(value interface{}) error {
	s, _ := value.(string)
	err := validation.Validate(s, validation.RuneLength(32, 32), is.Hexadecimal)
	if err != nil {
		return fmt.Errorf("%v is not a valid UUID", s)
	}
	return nil
}

// State
type EntryState string

const (
	EntryStateUnknown EntryState = ""
	EntryStateOnline  EntryState = "online"
	EntryStateOffline EntryState = "offline"
	EntryStateFailed  EntryState = "failed"
)

func ValidateEntryState(value interface{}) error {
	s, _ := value.(EntryState)
	err := validation.Validate(s, validation.Required, validation.In(EntryStateOnline, EntryStateOffline, EntryStateFailed))
	if err != nil {
		return fmt.Errorf("%v is not valid state", s)
	}
	return nil
}

type DurabilityType string

const (
	DurabilityReplicate      DurabilityType = "replicate"
	DurabilityDistributeOnly DurabilityType = "none"
	DurabilityEC             DurabilityType = "disperse"
)

func ValidateDurabilityType(value interface{}) error {
	s, _ := value.(DurabilityType)
	err := validation.Validate(s, validation.Required, validation.In(DurabilityReplicate, DurabilityDistributeOnly, DurabilityEC))
	if err != nil {
		return fmt.Errorf("%v is not a valid durability type", s)
	}
	return nil
}

// Common
type StateRequest struct {
	State EntryState `json:"state"`
}

func (statereq StateRequest) Validate() error {
	return validation.ValidateStruct(&statereq,
		validation.Field(&statereq.State, validation.Required, validation.By(ValidateEntryState)),
	)
}

// Storage values in KB
type StorageSize struct {
	Total uint64 `json:"total"`
	Free  uint64 `json:"free"`
	Used  uint64 `json:"used"`
}

type HostAddresses struct {
	Manage  sort.StringSlice `json:"manage"`
	Storage sort.StringSlice `json:"storage"`
}

func ValidateManagementHostname(value interface{}) error {
	s, _ := value.(sort.StringSlice)
	for _, fqdn := range s {
		err := validation.Validate(fqdn, validation.Required, is.Host)
		if err != nil {
			return fmt.Errorf("%v is not a valid manage hostname", s)
		}
	}
	return nil
}

func ValidateStorageHostname(value interface{}) error {
	s, _ := value.(sort.StringSlice)
	for _, ip := range s {
		err := validation.Validate(ip, validation.Required, is.Host)
		if err != nil {
			return fmt.Errorf("%v is not a valid storage hostname", s)
		}
	}
	return nil
}

func (hostadd HostAddresses) Validate() error {
	return validation.ValidateStruct(&hostadd,
		validation.Field(&hostadd.Manage, validation.Required, validation.By(ValidateManagementHostname)),
		validation.Field(&hostadd.Storage, validation.Required, validation.By(ValidateStorageHostname)),
	)
}

// Brick
type BrickInfo struct {
	Id       string `json:"id"`
	Path     string `json:"path"`
	DeviceId string `json:"device"`
	NodeId   string `json:"node"`
	VolumeId string `json:"volume"`

	// Size in KB
	Size uint64 `json:"size"`
}

// Device
type Device struct {
	Name string `json:"name"`
}

func (dev Device) Validate() error {
	return validation.ValidateStruct(&dev,
		validation.Field(&dev.Name, validation.Required, validation.Match(deviceNameRe)),
	)
}

type DeviceAddRequest struct {
	Device
	NodeId string `json:"node"`
}

func (devAddReq DeviceAddRequest) Validate() error {
	return validation.ValidateStruct(&devAddReq,
		validation.Field(&devAddReq.Device, validation.Required),
		validation.Field(&devAddReq.NodeId, validation.Required, validation.By(ValidateUUID)),
	)
}

type DeviceInfo struct {
	Device
	Storage StorageSize `json:"storage"`
	Id      string      `json:"id"`
}

type DeviceInfoResponse struct {
	DeviceInfo
	State  EntryState  `json:"state"`
	Bricks []BrickInfo `json:"bricks"`
}

// Node
type NodeAddRequest struct {
	Zone      int           `json:"zone"`
	Hostnames HostAddresses `json:"hostnames"`
	ClusterId string        `json:"cluster"`
}

func (req NodeAddRequest) Validate() error {
	return validation.ValidateStruct(&req,
		validation.Field(&req.Zone, validation.Required, validation.Min(1)),
		validation.Field(&req.Hostnames, validation.Required),
		validation.Field(&req.ClusterId, validation.Required, validation.By(ValidateUUID)),
	)
}

type NodeInfo struct {
	NodeAddRequest
	Id string `json:"id"`
}

type NodeInfoResponse struct {
	NodeInfo
	State       EntryState           `json:"state"`
	DevicesInfo []DeviceInfoResponse `json:"devices"`
}

// Cluster
type Cluster struct {
	Volumes []VolumeInfoResponse `json:"volumes"`
	Nodes   []NodeInfoResponse   `json:"nodes"`
	Id      string               `json:"id"`
}

type TopologyInfoResponse struct {
	ClusterList []Cluster `json:"clusters"`
}

type ClusterInfoResponse struct {
	Id      string           `json:"id"`
	Nodes   sort.StringSlice `json:"nodes"`
	Volumes sort.StringSlice `json:"volumes"`
}

type ClusterListResponse struct {
	Clusters []string `json:"clusters"`
}

// Durabilities
type ReplicaDurability struct {
	Replica int `json:"replica,omitempty"`
}

type DisperseDurability struct {
	Data       int `json:"data,omitempty"`
	Redundancy int `json:"redundancy,omitempty"`
}

// Volume
type VolumeDurabilityInfo struct {
	Type      DurabilityType     `json:"type,omitempty"`
	Replicate ReplicaDurability  `json:"replicate,omitempty"`
	Disperse  DisperseDurability `json:"disperse,omitempty"`
}

type VolumeCreateRequest struct {
	// Size in GB
	Size                 int                  `json:"size"`
	Clusters             []string             `json:"clusters,omitempty"`
	Name                 string               `json:"name"`
	Durability           VolumeDurabilityInfo `json:"durability,omitempty"`
	Gid                  int64                `json:"gid,omitempty"`
	GlusterVolumeOptions []string             `json:"glustervolumeoptions,omitempty"`
	Snapshot             struct {
		Enable bool    `json:"enable"`
		Factor float32 `json:"factor"`
	} `json:"snapshot"`
}

func (volCreateRequest VolumeCreateRequest) Validate() error {
	return validation.ValidateStruct(&volCreateRequest,
		validation.Field(&volCreateRequest.Size, validation.Required, validation.Min(1)),
		validation.Field(&volCreateRequest.Clusters, validation.By(ValidateUUID)),
		validation.Field(&volCreateRequest.Name, validation.Match(volumeNameRe)),
		validation.Field(&volCreateRequest.Durability, validation.Skip),
		validation.Field(&volCreateRequest.Gid, validation.Skip),
		validation.Field(&volCreateRequest.GlusterVolumeOptions, validation.Skip),
		// This is possibly a bug in validation lib, ignore next two lines for now
		// validation.Field(&volCreateRequest.Snapshot.Enable, validation.In(true, false)),
		// validation.Field(&volCreateRequest.Snapshot.Factor, validation.Min(1.0)),
	)
}

type VolumeInfo struct {
	VolumeCreateRequest
	Id      string `json:"id"`
	Cluster string `json:"cluster"`
	Mount   struct {
		GlusterFS struct {
			Hosts      []string          `json:"hosts"`
			MountPoint string            `json:"device"`
			Options    map[string]string `json:"options"`
		} `json:"glusterfs"`
	} `json:"mount"`
}

type VolumeInfoResponse struct {
	VolumeInfo
	Bricks []BrickInfo `json:"bricks"`
}

type VolumeListResponse struct {
	Volumes []string `json:"volumes"`
}

type VolumeExpandRequest struct {
	Size int `json:"expand_size"`
}

func (volExpandReq VolumeExpandRequest) Validate() error {
	return validation.ValidateStruct(&volExpandReq,
		validation.Field(&volExpandReq.Size, validation.Required, validation.Min(1)),
	)
}

// Constructors

func NewVolumeInfoResponse() *VolumeInfoResponse {

	info := &VolumeInfoResponse{}
	info.Mount.GlusterFS.Options = make(map[string]string)
	info.Bricks = make([]BrickInfo, 0)

	return info
}

// String functions
func (v *VolumeInfoResponse) String() string {
	s := fmt.Sprintf("Name: %v\n"+
		"Size: %v\n"+
		"Volume Id: %v\n"+
		"Cluster Id: %v\n"+
		"Mount: %v\n"+
		"Mount Options: backup-volfile-servers=%v\n"+
		"Durability Type: %v\n",
		v.Name,
		v.Size,
		v.Id,
		v.Cluster,
		v.Mount.GlusterFS.MountPoint,
		v.Mount.GlusterFS.Options["backup-volfile-servers"],
		v.Durability.Type)

	switch v.Durability.Type {
	case DurabilityEC:
		s += fmt.Sprintf("Disperse Data: %v\n"+
			"Disperse Redundancy: %v\n",
			v.Durability.Disperse.Data,
			v.Durability.Disperse.Redundancy)
	case DurabilityReplicate:
		s += fmt.Sprintf("Distributed+Replica: %v\n",
			v.Durability.Replicate.Replica)
	}

	if v.Snapshot.Enable {
		s += fmt.Sprintf("Snapshot Factor: %.2f\n",
			v.Snapshot.Factor)
	}

	/*
		s += "\nBricks:\n"
		for _, b := range v.Bricks {
			s += fmt.Sprintf("Id: %v\n"+
				"Path: %v\n"+
				"Size (GiB): %v\n"+
				"Node: %v\n"+
				"Device: %v\n\n",
				b.Id,
				b.Path,
				b.Size/(1024*1024),
				b.NodeId,
				b.DeviceId)
		}
	*/

	return s
}
