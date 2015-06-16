//
// Copyright (c) 2014 The heketi Authors
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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

type BrickNode struct {
	node   string `json:"meta"`
	device string `json:"device"`
}
type BrickNodes []BrickNode

type RingOutput struct {
	nodes     BrickNodes `json:"nodes"`
	partition int        `json:"partition"`
}

type GlusterRing struct {
	db *GlusterFSDB
}

func NewGlusterRing(db *GlusterFSDB) *GlusterRing {
	return &GlusterRing{
		db: db,
	}
}

func (g *GlusterRing) CreateRing() error {

	os.Remove("heketi.builder")
	os.Remove("heketi.ring.gz")

	args := []string{
		"heketi.builder",
		"create",
		"16",
		"2",
		"1",
	}

	// Create new ring
	err := exec.Command("swift-ring-builder", args...).Run()
	if err != nil {
		return errors.New("Unable to create brick placement db")
	}

	// Add all devices
	for nodeid, node := range g.db.nodes {
		for devid, dev := range node.Info.Devices {
			args := []string{
				"heketi.builder",
				"add",
				fmt.Sprintf("r1z%v-%v:80/%v_%v",
					node.Info.Zone,
					node.Info.Name,
					devid,
					nodeid),
				fmt.Sprintf("%v", dev.Weight),
			}
			err := exec.Command("swift-ring-builder", args...).Run()
			if err != nil {
				return err
			}
		}
	}

	// Rebalance
	return exec.Command("swift-ring-builder", "heketi.builder", "rebalance").Run()
}

func (g *GlusterRing) GetNodes(brick *Brick) (BrickNodes, error) {

	var out bytes.Buffer
	var nodes RingOutput

	cmd := exec.Command("./ring.py", brick.Id)
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(out.Bytes(), &nodes); err != nil {
		return nil, err
	}

	return nodes.nodes, nil
}
