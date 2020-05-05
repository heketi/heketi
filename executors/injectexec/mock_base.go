//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package injectexec

import (
	"github.com/heketi/heketi/executors"
	"github.com/heketi/heketi/executors/mockexec"
)

var NotSupportedError = executors.NotSupportedError

// newMockBase returns a mock executor set up for use as the first "dummy"
// executor in the error inject executor's stack.
// This functions on this executor can be overridden directly for test
// purposes.
func newMockBase() *mockexec.MockExecutor {
	m, _ := mockexec.NewMockExecutor()
	m.DefaultMockFuncError = NotSupportedError
	return m
}
