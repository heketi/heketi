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
	"fmt"
	"os"
)

type GlusterFSDbOnDisk struct {
	Nodes   map[string]*NodeDB
	Volumes map[string]*VolumeDB
}

type GlusterFSDB struct {
	nodes      map[string]*NodeDB
	volumes    map[string]*VolumeDB
	dbfilename string
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

func NewGlusterFSDB(dbfile string) *GlusterFSDB {

	gfsdb := &GlusterFSDB{}

	gfsdb.nodes = make(map[string]*NodeDB)
	gfsdb.volumes = make(map[string]*VolumeDB)
	gfsdb.dbfilename = dbfile

	// Load db
	if _, err := os.Stat(gfsdb.dbfilename); err == nil {
		err := gfsdb.Load()
		if err != nil {
			fmt.Printf("Unable to load metadata: %s", err)
			return nil
		}
	}

	return gfsdb
}

func (g *GlusterFSDB) Commit() error {
	ondisk := &GlusterFSDbOnDisk{
		Nodes:   g.nodes,
		Volumes: g.volumes,
	}
	fi, err := os.Create(g.dbfilename)
	if err != nil {
		return err
	}
	defer fi.Close()

	encoder := gob.NewEncoder(fi)
	err = encoder.Encode(&ondisk)
	if err != nil {
		return err
	}

	return nil
}

func (g *GlusterFSDB) Load() error {
	ondisk := &GlusterFSDbOnDisk{}

	fi, err := os.Open(g.dbfilename)
	if err != nil {
		return err
	}
	defer fi.Close()

	decoder := gob.NewDecoder(fi)
	err = decoder.Decode(&ondisk)
	if err != nil {
		return err
	}

	g.nodes = ondisk.Nodes
	g.volumes = ondisk.Volumes

	return nil
}

func (g *GlusterFSDB) Close() {
	// Nothing to do since we commit on every change
}
