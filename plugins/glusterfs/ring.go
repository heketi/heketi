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
	"sync"
	"time"
)

type BrickNode struct {
	NodeId   string `json:"meta"`
	DeviceId string `json:"device"`
}
type BrickNodes []BrickNode

type RingOutput struct {
	Nodes     BrickNodes `json:"nodes"`
	Partition int        `json:"partition"`
}

type GlusterRing struct {
	db           *GlusterFSDB
	ringCreateCh chan bool
	lock         sync.RWMutex
}

func (g *GlusterRing) createServer() {

	for {
		// Wait for a command to start creating the ring
		select {
		case <-g.ringCreateCh:
			// This is not very efficient, but it works for now
			g.lock.Lock()
		}

		// Once we get the ring command.  Wait a maximum
		// of 5 seconds for more requests.  That way we only
		// do it once
		for created := false; !created; {
			timeout := time.After(time.Second * 5)
			select {
			case <-g.ringCreateCh:
				continue
			case <-timeout:
				err := g.createRing()

				// :TODO: Log
				if err != nil {
					// :TODO: May want to try again a few times if we fail.
					fmt.Println(err)
				}
				created = true
			}
		}

		g.lock.Unlock()
	}
}

func NewGlusterRing(db *GlusterFSDB) *GlusterRing {
	g := &GlusterRing{
		db:           db,
		ringCreateCh: make(chan bool),
	}

	go g.createServer()
	return g
}

func (g *GlusterRing) CreateRing() {
	g.ringCreateCh <- true
}

func (g *GlusterRing) createRing() error {

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

	err = g.db.Reader(func() error {
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

		return nil
	})
	if err != nil {
		return nil
	}

	// Rebalance
	return exec.Command("swift-ring-builder", "heketi.builder", "rebalance").Run()
}

func (g *GlusterRing) GetNodes(brick_num int, id string) (BrickNodes, error) {

	var out bytes.Buffer
	var nodes RingOutput

	g.lock.RLock()
	defer g.lock.RUnlock()

	args := []string{
		fmt.Sprintf("%v", brick_num),
		id,
	}

	cmd := exec.Command("./ring.py", args...)
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(out.Bytes(), &nodes); err != nil {
		return nil, err
	}

	return nodes.Nodes, nil
}
