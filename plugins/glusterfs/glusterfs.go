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
	"github.com/lpabon/godbc"
	"github.com/lpabon/heketi/utils/ssh"
	"sync"
)

type GlusterFSPlugin struct {
	db      *GlusterFSDB
	sshexec *ssh.SshExec
	rwlock  sync.RWMutex
}

func NewGlusterFSPlugin() *GlusterFSPlugin {
	m := &GlusterFSPlugin{}
	m.db = NewGlusterFSDB("heketi.db")

	// Just for now, it will work wih https://github.com/lpabon/vagrant-gfsm
	m.sshexec = ssh.NewSshExecWithKeyFile("vagrant", "insecure_private_key")
	godbc.Check(m.sshexec != nil)

	return m
}

func (g *GlusterFSPlugin) Close() {
	g.db.Close()
}
