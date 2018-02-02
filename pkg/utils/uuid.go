//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.

package utils

// From http://www.ashishbanerjee.com/home/go/go-generate-uuid

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"io"
	"sync"

	"github.com/lpabon/godbc"
)

type IdSource struct {
	io.Reader
}

var (
	Randomness = rand.Reader
)

func (s IdSource) ReadUUID() string {
	uuid := make([]byte, 16)
	n, err := s.Read(uuid)
	godbc.Check(n == len(uuid), n, len(uuid))
	godbc.Check(err == nil, err)

	return hex.EncodeToString(uuid)
}

// ShortID returns a unique-as-possible ID of length l.
// The length l must be a multiple of 2, greater than 1 and less than or
// equal to 32. The function will panic if l is invalid.
func (s IdSource) ShortID(l int) string {
	godbc.Require(l <= 32)
	godbc.Require(l > 1)
	godbc.Require((l & 1) != 1)
	id := make([]byte, l/2)
	n, err := s.Read(id)

	godbc.Check(n == len(id), n, len(id))
	godbc.Check(err == nil, err)

	return hex.EncodeToString(id)
}

// Return a 16-byte uuid
func GenUUID() string {
	return IdSource{Randomness}.ReadUUID()
}

// ShortID returns a unique-as-possible ID of length l.
// The length l must be a multiple of 2, greater than 1 and less than or
// equal to 32. The function will panic if l is invalid.
func ShortID(l int) string {
	return IdSource{Randomness}.ShortID(l)
}

type NonRandom struct {
	count uint64
	lock  sync.Mutex
}

func (n *NonRandom) Count() (curr uint64) {
	n.lock.Lock()
	defer n.lock.Unlock()
	curr = n.count
	n.count++
	return
}

func (n *NonRandom) Read(p []byte) (s int, err error) {
	switch {
	case len(p) > 8:
		offset := len(p) - 8
		binary.BigEndian.PutUint64(p[offset:], n.Count())
	case len(p) < 8:
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, n.Count())
		offset := 8 - len(p)
		copy(p, b[offset:])
	default:
		binary.BigEndian.PutUint64(p, n.Count())
	}
	s = len(p)
	return
}
