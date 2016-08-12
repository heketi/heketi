//
// Copyright (c) 2015 The heketi Authors
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

package tests

import (
	"strings"
	"testing"
)

type FakeTester struct {
	testing.T
	failed bool
}

func (f *FakeTester) FailNow() {
	f.failed = true
}

func (f *FakeTester) Failed() bool {
	return f.failed
}

func TestAssertPass(t *testing.T) {
	test := &FakeTester{}
	Assert(test, true)

	if test.Failed() {
		t.Fail()
	}
}

func TestAssertFail(t *testing.T) {

	test := &FakeTester{}
	Assert(test, false)

	if !test.Failed() {
		t.Fail()
	}
}

func TestTempfile(t *testing.T) {
	s1 := Tempfile()
	Assert(t, strings.Contains(s1, "gounittest"))
	Assert(t, strings.Contains(s1, "-1"))
	Assert(t, !strings.Contains(s1, "-2"))

	s2 := Tempfile()
	Assert(t, strings.Contains(s2, "gounittest"))
	Assert(t, !strings.Contains(s2, "-1"))
	Assert(t, strings.Contains(s2, "-2"))
}
