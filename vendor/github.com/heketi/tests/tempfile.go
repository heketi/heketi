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
	"os"
)

func tempfile_generate() func() string {
	counter := 0
	return func() string {
		counter++
		return fmt.Sprintf("/tmp/gounittest.%d-%d",
			os.Getpid(), counter)
	}
}

var genname = tempfile_generate()

// Return a filename string in the form of
// /tmp/gounittest.<Process Id>-<Counter>
func Tempfile() string {
	return genname()
}

// We could use Fallocate, but some test CI systems
// do not support it, like Travis-ci.org.
func CreateFile(filename string, size int64) error {

	buf := make([]byte, size)

	// Create the file store some data
	fp, err := os.Create(filename)
	if err != nil {
		return err
	}

	// Write the buffer
	_, err = fp.Write(buf)

	// Cleanup
	fp.Close()

	return err
}
