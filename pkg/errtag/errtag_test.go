//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package errtag

import (
	"testing"

	"github.com/heketi/tests"
	"github.com/pkg/errors"
)

func TestEmittedErrorEqualsID(t *testing.T) {
	etag := NewTag("test")
	err := etag.Err()
	tests.Assert(t, etag.In(err))
}

func TestTwoEmittedErrorsAreNotEqual(t *testing.T) {
	etag := NewTag("test")
	err1 := etag.Err()
	err2 := etag.Err()
	tests.Assert(t, err1 != err2)
}

func TestEmittedErrorsContainStacktrace(t *testing.T) {
	etag := NewTag("test")
	err := etag.Err()
	type stackTracer interface {
		StackTrace() errors.StackTrace
	}
	_, ok := err.(stackTracer)
	tests.Assert(t, ok)
}

func TestEmittedErrorsContainCause(t *testing.T) {
	etag := NewTag("test")
	err := etag.Err()
	type causer interface {
		Cause() error
	}
	_, ok := err.(causer)
	tests.Assert(t, ok)
}

func TestTwoIDsWithSameStringAreNotEqual(t *testing.T) {
	etag1 := NewTag("test")
	etag2 := NewTag("test")
	tests.Assert(t, etag1 != etag2)
	err1 := etag1.Err()
	err2 := etag2.Err()
	tests.Assert(t, err1 != err2)
	tests.Assert(t, !etag1.In(err2))
	tests.Assert(t, !etag2.In(err1))
}
