//
// Copyright (c) 2014 The heketi Authors
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
//

package glusterfs

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func (m *GlusterFSPlugin) vgSize(host string, vg string) (uint64, error) {

	commands := []string{
		fmt.Sprintf("sudo vgdisplay -c %v", vg),
	}

	b, err := m.sshexec.ConnectAndExec(host+":22", commands, nil)
	if err != nil {
		return 0, err
	}
	for k, v := range b {
		fmt.Printf("[%v] ==\n%v\n", k, v)
	}

	vginfo := strings.Split(b[0], ":")
	if len(vginfo) < 12 {
		return 0, errors.New("vgdisplay returned an invalid string")
	}

	return strconv.ParseUint(vginfo[11], 10, 64)

}
