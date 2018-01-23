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

// NOTE: the Original GenUUID function aborts the applicaion
// when conditions are not met. This was carried over into the
// version with selectable random sources so we dont actually
// do any of that testing here or the unit test runner would abort
