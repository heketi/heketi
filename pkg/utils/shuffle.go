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
	"time"
)

// Shuffle pseudo-randomizes the order of elements.
// This shuffle is based on that from Go 1.10. As heketi currently
// supports older versions of Go we need our own shuffle. Once,
// only Go 1.10 or higher is supported this can be dropped.
func Shuffle(r *rand.Rand, n int, swap func(i, j int)) {
	if n < 0 {
		panic("invalid argument to Shuffle")
	}

	// Fisher-Yates shuffle. Shamelessly stolen from Golang 1.10
	// math/rand package. See go docs for details.
	i := n - 1
	for ; i > 1<<31-1-1; i-- {
		j := int(r.Int63n(int64(i + 1)))
		swap(i, j)
	}
	for ; i > 0; i-- {
		j := int(r.Int31n(int32(i + 1)))
		swap(i, j)
	}
}

// SeededShuffle pseudo-randomizes the order of elements
// using a private PRNG instance seeded from the clock.
func SeededShuffle(n int, swap func(i, j int)) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	Shuffle(r, n, swap)
}
