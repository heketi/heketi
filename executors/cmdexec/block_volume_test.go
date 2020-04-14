//
// Copyright (c) 2020 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package cmdexec

import (
	"testing"

	"github.com/heketi/tests"
)

func TestToGigaBytes(t *testing.T) {
	var (
		input string
		size  int
		err   error
	)

	input = ""
	size, err = toGigaBytes(input)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
	tests.Assert(t, size == 0, "expected: size == 0, got:", size)

	input = "."
	size, err = toGigaBytes(input)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
	tests.Assert(t, size == 0, "expected: size == 0, got:", size)

	input = " "
	size, err = toGigaBytes(input)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
	tests.Assert(t, size == 0, "expected: size == 0, got:", size)

	input = "xyz"
	size, err = toGigaBytes(input)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
	tests.Assert(t, size == 0, "expected: size == 0, got:", size)

	input = "1.0 V"
	size, err = toGigaBytes(input)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
	tests.Assert(t, size == 0, "expected: size == 0, got:", size)

	input = "1.0 GiB"
	size, err = toGigaBytes(input)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, size == 1, "expected: size == 1, got:", size)

	input = "2    GiB"
	size, err = toGigaBytes(input)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, size == 2, "expected: size == 2, got:", size)

	input = "3  G"
	size, err = toGigaBytes(input)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, size == 3, "expected: size == 3, got:", size)

	input = "10.0 GB"
	size, err = toGigaBytes(input)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, size == 10, "expected: size == 10, got:", size)

	input = "1.0 TiB"
	size, err = toGigaBytes(input)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, size == 1024, "expected: size == 1024, got:", size)

	input = "5   T"
	size, err = toGigaBytes(input)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, size == 5120, "expected: size == 5120, got:", size)

	input = "1.0 P"
	size, err = toGigaBytes(input)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, size == 1048576, "expected: size == 1048576, got:", size)

	input = "6   EiB"
	size, err = toGigaBytes(input)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, size == 6442450944, "expected: size == 6442450944, got:", size)
}
