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
	"encoding/gob"
	"github.com/lpabon/godbc"
	"github.com/lpabon/heketi/db"
	"github.com/lpabon/heketi/requests"
)

type GlusterFSDB struct {
	nodes    map[string]*Node
	volumes  map[string]*Volume
	db       db.HeketiDB
	nodelist ModelNodeList
}

type ModelNode struct {
	Resp *requests.NodeInfoResp
}

type ModelNodeList struct {
	Nodes map[string]bool
}

func dbEncode(e interface{}) ([]byte, error) {
	var buffer bytes.Buffer

	enc := gob.NewEncoder(&buffer)
	err := enc.Encode(e)
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func dbDecode(e interface{}, buffer []byte) error {

	dec := gob.NewDecoder(bytes.NewBuffer(buffer))
	err := dec.Decode(e)
	if err != nil {
		return err
	}
	return nil
}

func NewGlusterFSDB() *GlusterFSDB {

	gfsdb := &GlusterFSDB{}

	gfsdb.db = db.NewBoltDB("heketi.db")
	gfsdb.nodes = make(map[string]*Node)
	gfsdb.volumes = make(map[string]*Volume)
	gfsdb.nodelist.Nodes = make(map[string]bool)
	godbc.Check(gfsdb != nil)

	// load node list
	buf, err := gfsdb.db.Get([]byte("nodelist"))
	if len(buf) > 0 && err == nil {
		err = dbDecode(&gfsdb.nodelist, buf)
		if err != nil {
			return nil
		}
	}

	return gfsdb
}

func (g *GlusterFSDB) Close() {
	g.db.Close()
}

func (g *GlusterFSDB) SaveNode(node *ModelNode) error {

	buffer, err := dbEncode(node)
	if err != nil {
		return err
	}

	err = g.db.Put([]byte(node.Resp.Id), buffer)
	if err != nil {
		return err
	}

	g.nodelist.Nodes[node.Resp.Id] = true
	buffer, err = dbEncode(&g.nodelist)
	if err != nil {
		return err
	}

	err = g.db.Put([]byte("nodelist"), buffer)
	if err != nil {
		return err
	}

	return nil
}

func (g *GlusterFSDB) Node(id string) (*ModelNode, error) {

	buf, err := g.db.Get([]byte(id))
	if err != nil {
		return nil, err
	}

	node := &ModelNode{}
	err = dbDecode(&node, buf)
	if err != nil {
		return nil, err
	}

	return node, nil
}

func (g *GlusterFSDB) Nodes() map[string]bool {
	return g.nodelist.Nodes
}
