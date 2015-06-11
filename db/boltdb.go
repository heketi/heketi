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

package db

import (
	"github.com/boltdb/bolt"
	"github.com/lpabon/godbc"
	"time"
)

type BoltDB struct {
	db *bolt.DB
}

func NewBoltDB(dbpath string) *BoltDB {

	var err error

	db := &BoltDB{}

	db.db, err = bolt.Open(dbpath, 0600, &bolt.Options{Timeout: 3 * time.Second})
	godbc.Check(err == nil)

	err = db.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("heketi"))
		godbc.Check(err == nil)
		return nil
	})
	godbc.Check(err == nil)

	return db
}

func (c *BoltDB) Close() {
	c.db.Close()
}

func (c *BoltDB) Put(key, val []byte) (err error) {
	err = c.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte("heketi")).Put(key, val)
	})
	return
}

func (c *BoltDB) Get(key []byte) (val []byte, err error) {
	err = c.db.View(func(tx *bolt.Tx) error {
		val = tx.Bucket([]byte("heketi")).Get(key)
		return nil
	})
	return
}

func (c *BoltDB) Delete(key []byte) (err error) {
	err = c.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte("heketi")).Delete(key)
	})
	return
}

func (c *BoltDB) String() string {
	return ""
}
