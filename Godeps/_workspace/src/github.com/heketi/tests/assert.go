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
	"fmt"
	"runtime"
)

type Tester interface {
	Errorf(format string, args ...interface{})
	FailNow()
}

// Simple assert call for unit and functional tests
func Assert(t Tester, b bool, message ...interface{}) {
	if !b {
		pc, file, line, _ := runtime.Caller(1)
		caller_func_info := runtime.FuncForPC(pc)

		error_string := fmt.Sprintf("\n\rASSERT:\tfunc (%s) 0x%x\n\r\tFile %s:%d",
			caller_func_info.Name(),
			pc,
			file,
			line)

		if len(message) > 0 {
			error_string += fmt.Sprintf("\n\r\tInfo: %+v", message)
		}

		t.Errorf(error_string)
		t.FailNow()
	}
}
