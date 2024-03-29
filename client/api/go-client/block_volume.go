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

	"github.com/heketi/heketi/v10/pkg/glusterfs/api"
	"github.com/heketi/heketi/v10/pkg/utils"
)

func (c *Client) BlockVolumeCreate(request *api.BlockVolumeCreateRequest) (
	*api.BlockVolumeInfoResponse, error) {

	buffer, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST",
		c.host+"/blockvolumes",
		bytes.NewBuffer(buffer))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	err = c.setToken(req)
	if err != nil {
		return nil, err
	}

	r, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusAccepted {
		return nil, utils.GetErrorFromResponse(r)
	}

	r, err = c.pollResponse(r)
	if err != nil {
		return nil, err
	}
	if r.StatusCode != http.StatusOK {
		return nil, utils.GetErrorFromResponse(r)
	}

	var blockvolume api.BlockVolumeInfoResponse
	err = utils.GetJsonFromResponse(r, &blockvolume)
	if err != nil {
		return nil, err
	}

	return &blockvolume, nil

}

func (c *Client) BlockVolumeList() (*api.BlockVolumeListResponse, error) {
	req, err := http.NewRequest("GET", c.host+"/blockvolumes", nil)
	if err != nil {
		return nil, err
	}

	err = c.setToken(req)
	if err != nil {
		return nil, err
	}

	r, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return nil, utils.GetErrorFromResponse(r)
	}

	var blockvolumes api.BlockVolumeListResponse
	err = utils.GetJsonFromResponse(r, &blockvolumes)
	if err != nil {
		return nil, err
	}

	return &blockvolumes, nil
}

func (c *Client) BlockVolumeInfo(id string) (*api.BlockVolumeInfoResponse, error) {
	req, err := http.NewRequest("GET", c.host+"/blockvolumes/"+id, nil)
	if err != nil {
		return nil, err
	}

	err = c.setToken(req)
	if err != nil {
		return nil, err
	}

	r, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return nil, utils.GetErrorFromResponse(r)
	}

	var blockvolume api.BlockVolumeInfoResponse
	err = utils.GetJsonFromResponse(r, &blockvolume)
	if err != nil {
		return nil, err
	}

	return &blockvolume, nil
}

func (c *Client) BlockVolumeDelete(id string) error {
	req, err := http.NewRequest("DELETE", c.host+"/blockvolumes/"+id, nil)
	if err != nil {
		return err
	}

	err = c.setToken(req)
	if err != nil {
		return err
	}

	r, err := c.do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusAccepted {
		return utils.GetErrorFromResponse(r)
	}

	r, err = c.pollResponse(r)
	if err != nil {
		return err
	}
	if r.StatusCode != http.StatusNoContent {
		return utils.GetErrorFromResponse(r)
	}

	return nil
}

func (c *Client) BlockVolumeExpand(id string, request *api.BlockVolumeExpandRequest) (
	*api.BlockVolumeInfoResponse, error) {

	// Marshal request to JSON
	buffer, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	// Create a request
	req, err := http.NewRequest("POST",
		c.host+"/blockvolumes/"+id+"/expand",
		bytes.NewBuffer(buffer))
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
	defer r.Body.Close()
	if r.StatusCode != http.StatusAccepted {
		return nil, utils.GetErrorFromResponse(r)
	}

	// Wait for response
	r, err = c.pollResponse(r)
	if err != nil {
		return nil, err
	}
	if r.StatusCode != http.StatusOK {
		return nil, utils.GetErrorFromResponse(r)
	}

	// Read JSON response
	var blockvolume api.BlockVolumeInfoResponse
	err = utils.GetJsonFromResponse(r, &blockvolume)
	if err != nil {
		return nil, err
	}

	return &blockvolume, nil
}
