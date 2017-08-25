//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), as published by the Free Software Foundation,
// or under the Apache License, Version 2.0 <LICENSE-APACHE2 or
// http://www.apache.org/licenses/LICENSE-2.0>.
//
// You may not use this file except in compliance with those terms.
//

package client

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/utils"
)

func (c *Client) GeoReplicationPostAction(id string, request *api.GeoReplicationRequest) (*api.GeoReplicationStatus, error) {
	// Marshal request to JSON
	buffer, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	// Create a request
	req, err := http.NewRequest("POST", c.host+"/volumes/"+id+"/georeplication", bytes.NewBuffer(buffer))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// Set token
	err = c.setToken(req)
	if err != nil {
		return nil, err
	}

	// Send request
	r, err := c.do(req)
	if err != nil {
		return nil, err
	}
	if r.StatusCode != http.StatusAccepted {
		return nil, utils.GetErrorFromResponse(r)
	}

	// Wait for response
	r, err = c.waitForResponseWithTimer(r, time.Second)
	if err != nil {
		return nil, err
	}
	if r.StatusCode != http.StatusOK {
		return nil, utils.GetErrorFromResponse(r)
	}

	// Read JSON response
	var resp api.GeoReplicationStatus
	err = utils.GetJsonFromResponse(r, &resp)
	r.Body.Close()
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *Client) GeoReplicationVolumeStatus(id string) (*api.GeoReplicationStatus, error) {
	// Create request
	req, err := http.NewRequest("GET", c.host+"/volumes/"+id+"/georeplication", nil)
	if err != nil {
		return nil, err
	}

	// Set token
	err = c.setToken(req)
	if err != nil {
		return nil, err
	}

	// Get status
	r, err := c.do(req)
	if err != nil {
		return nil, err
	}
	if r.StatusCode != http.StatusOK {
		return nil, utils.GetErrorFromResponse(r)
	}

	// Read JSON response
	var status api.GeoReplicationStatus
	err = utils.GetJsonFromResponse(r, &status)
	r.Body.Close()
	if err != nil {
		return nil, err
	}

	return &status, nil
}

func (c *Client) GeoReplicationStatus() (*api.GeoReplicationStatus, error) {
	// Create request
	req, err := http.NewRequest("GET", c.host+"/georeplication", nil)
	if err != nil {
		return nil, err
	}

	// Set token
	err = c.setToken(req)
	if err != nil {
		return nil, err
	}

	// Get status
	r, err := c.do(req)
	if err != nil {
		return nil, err
	}
	if r.StatusCode != http.StatusOK {
		return nil, utils.GetErrorFromResponse(r)
	}

	// Read JSON response
	var status api.GeoReplicationStatus
	err = utils.GetJsonFromResponse(r, &status)
	r.Body.Close()
	if err != nil {
		return nil, err
	}

	return &status, nil
}
