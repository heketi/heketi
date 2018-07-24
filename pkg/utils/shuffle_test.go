//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package utils

import (
	"math/rand"
	"testing"
	"time"

	"github.com/heketi/tests"
)

func TestShuffle(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	a := []string{
		"bobcat",
		"lion",
		"puma",
		"cheetah",
		"lynx",
	}
	a2 := make([]string, len(a))
	copy(a2, a)
	Shuffle(r, len(a2), func(i, j int) {
		a2[i], a2[j] = a2[j], a2[i]
	})
	same := true
	for i := 0; i < len(a); i++ {
		same = same && (a[i] == a2[i])
	}
	tests.Assert(t, !same, a, a2)
}

func TestSeededShuffle(t *testing.T) {
	a := []string{
		"bobcat",
		"lion",
		"puma",
		"cheetah",
		"lynx",
	}
	a2 := make([]string, len(a))
	dedup := map[string]int{}
	copy(a2, a)
	SeededShuffle(len(a2), func(i, j int) {
		a2[i], a2[j] = a2[j], a2[i]
	})
	same := true
	for i := 0; i < len(a); i++ {
		dedup[a2[i]] += 1
		same = same && (a[i] == a2[i])
	}
	tests.Assert(t, !same, a, a2)
	tests.Assert(t, len(dedup) == len(a),
		"expected len(dedup) == len(a), got:", dedup, a)
}
