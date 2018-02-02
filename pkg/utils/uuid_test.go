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
	"strings"
	"testing"

	"github.com/heketi/tests"
)

// test actual generation of a uuid, we can only test the length
// of the output as we are relying on an actual random source here
func TestGenUUID(t *testing.T) {
	uuid := GenUUID()
	tests.Assert(t, len(uuid) == 32, "bad length", len(uuid), 32)
}

// test actual output by specifying our own source of "randomness"
func TestFakeUUID(t *testing.T) {
	r := strings.NewReader("heketiheketiheketi")
	uuid := IdSource{r}.ReadUUID()
	tests.Assert(t, len(uuid) == 32, "bad length", len(uuid), 32)
	tests.Assert(t, uuid == "68656b65746968656b65746968656b65")
}

func TestNonRandomUUID(t *testing.T) {
	s := IdSource{&NonRandom{}}

	uuid := s.ReadUUID()
	tests.Assert(t, len(uuid) == 32, "expected len(uuid) == 32, got:", len(uuid))
	tests.Assert(t, uuid == "00000000000000000000000000000000", "got:", uuid)
	uuid = s.ReadUUID()
	tests.Assert(t, len(uuid) == 32, "expected len(uuid) == 32, got:", len(uuid))
	tests.Assert(t, uuid == "00000000000000000000000000000001", "got:", uuid)
	uuid = s.ReadUUID()
	tests.Assert(t, len(uuid) == 32, "expected len(uuid) == 32, got:", len(uuid))
	tests.Assert(t, uuid == "00000000000000000000000000000002", "got:", uuid)

	for i := 0; i < 106; i++ {
		s.ReadUUID()
	}
	uuid = s.ReadUUID()
	tests.Assert(t, len(uuid) == 32, "expected len(uuid) == 32, got:", len(uuid))
	tests.Assert(t, uuid == "0000000000000000000000000000006d", "got:", uuid)
}

func TestReplaceRandomness(t *testing.T) {
	before := Randomness

	n := &NonRandom{}
	Randomness = n
	defer func() {
		Randomness = before
	}()

	// now we're using a non-random source. we should have predictable values
	uuid := GenUUID()
	tests.Assert(t, len(uuid) == 32, "expected len(uuid) == 32, got:", len(uuid))
	tests.Assert(t, uuid == "00000000000000000000000000000000", "got:", uuid)

	uuid = GenUUID()
	tests.Assert(t, len(uuid) == 32, "expected len(uuid) == 32, got:", len(uuid))
	tests.Assert(t, uuid == "00000000000000000000000000000001", "got:", uuid)

	// restore the random source back to what it was before
	Randomness = before

	// uuid should NOT be the next non-random number
	uuid = GenUUID()
	tests.Assert(t, len(uuid) == 32, "expected len(uuid) == 32, got:", len(uuid))
	tests.Assert(t, uuid != "00000000000000000000000000000002", "got:", uuid)
}

func TestShortID(t *testing.T) {
	s := IdSource{Randomness}

	for i := 2; i < 34; i += 2 {
		id := s.ShortID(i)
		tests.Assert(t, len(id) == i, "expected length ", i, ", got:", len(id))
	}
}

func TestGlobalShortID(t *testing.T) {
	for i := 2; i < 34; i += 2 {
		id := ShortID(i)
		tests.Assert(t, len(id) == i, "expected length ", i, ", got:", len(id))
	}
}

func TestShortIDTooShort(t *testing.T) {
	defer func() {
		r := recover()
		tests.Assert(t, r != nil, "expected r to be not nil, got nil")
	}()

	s := IdSource{Randomness}
	s.ShortID(1)

	t.Fatalf("should not reach this line")
}

func TestShortIDTooLong(t *testing.T) {
	defer func() {
		r := recover()
		tests.Assert(t, r != nil, "expected r to be not nil, got nil")
	}()

	s := IdSource{Randomness}
	id := s.ShortID(32)
	tests.Assert(t, len(id) == 32, "expected length ", 32, ", got:", len(id))

	s.ShortID(34)
	t.Fatalf("should not reach this line")
}

func TestShortIDOddLen(t *testing.T) {
	defer func() {
		r := recover()
		tests.Assert(t, r != nil, "expected r to be not nil, got nil")
	}()

	s := IdSource{Randomness}
	id := s.ShortID(4)
	tests.Assert(t, len(id) == 4, "expected length ", 4, ", got:", len(id))

	s.ShortID(5)
	t.Fatalf("should not reach this line")
}

func TestShortIDNonRandom(t *testing.T) {
	s := IdSource{&NonRandom{}}

	id := s.ShortID(16)
	tests.Assert(t, id == "0000000000000000",
		"expected id == 0000000000000000, got:", id)

	id = s.ShortID(8)
	tests.Assert(t, id == "00000001",
		"expected id == 00000001, got:", id)
	id = s.ShortID(8)
	tests.Assert(t, id == "00000002",
		"expected id == 00000002, got:", id)

	id = s.ShortID(2)
	tests.Assert(t, id == "03",
		"expected id == 03, got:", id)

	id = s.ShortID(4)
	tests.Assert(t, id == "0004",
		"expected id == 0004, got:", id)

	id = s.ShortID(6)
	tests.Assert(t, id == "000005",
		"expected id == 000005, got:", id)

	id = s.ShortID(18)
	tests.Assert(t, id == "000000000000000006",
		"expected id == 000000000000000006, got:", id)
}

// NOTE: the Original GenUUID function aborts the applicaion
// when conditions are not met. This was carried over into the
// version with selectable random sources so we dont actually
// do any of that testing here or the unit test runner would abort
