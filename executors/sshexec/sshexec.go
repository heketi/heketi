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
//

package sshexec

import (
	"errors"
	"github.com/heketi/utils"
	"github.com/heketi/utils/ssh"
	"github.com/lpabon/godbc"
	"os"
	"sync"
)

const (
	DEFAULT_MAX_CONNECTIONS = 8
)

type SshExecutor struct {
	private_keyfile string
	user            string
	throttlemap     map[string]chan bool
	lock            sync.Mutex
	exec            *ssh.SshExec
	config          *SshConfig
}

type SshConfig struct {
	PrivateKeyFile string `json:"keyfile"`
	User           string `json:"user"`

	// Experimental Settings
	RebalanceOnExpansion bool `json:"rebalance_on_expansion"`
}

var (
	logger           = utils.NewLogger("[sshexec]", utils.LEVEL_DEBUG)
	ErrSshPrivateKey = errors.New("Unable to read private key file")
)

func NewSshExecutor(config *SshConfig) *SshExecutor {
	godbc.Require(config != nil)
	godbc.Require(DEFAULT_MAX_CONNECTIONS > 1)

	s := &SshExecutor{}
	s.throttlemap = make(map[string]chan bool)

	// Set configuration
	if config.PrivateKeyFile == "" {
		s.private_keyfile = os.Getenv("HOME") + "/.ssh/id_rsa"
	} else {
		s.private_keyfile = config.PrivateKeyFile
	}

	if config.User == "" {
		s.user = "heketi"
	} else {
		s.user = config.User
	}
	s.config = config

	// Show experimental settings
	if s.config.RebalanceOnExpansion {
		logger.Warning("Rebalance on volume expansion has been enabled.  This is an EXPERIMENTAL feature")
	}

	// Setup key
	s.exec = ssh.NewSshExecWithKeyFile(logger, s.user, s.private_keyfile)
	if s.exec == nil {
		logger.LogError("Unable to load ssh user and private keyfile")
		return nil
	}

	godbc.Ensure(s != nil)
	godbc.Ensure(s.config == config)
	godbc.Ensure(s.user != "")
	godbc.Ensure(s.private_keyfile != "")

	return s
}

func (s *SshExecutor) accessConnection(host string) {
	var (
		c  chan bool
		ok bool
	)

	s.lock.Lock()
	if c, ok = s.throttlemap[host]; !ok {
		c = make(chan bool, DEFAULT_MAX_CONNECTIONS)
		s.throttlemap[host] = c

		// We know we are not going to pend here
		// Do this inside the lock to remove the race
		// condition of creating the channel, unlocking,
		// then having freeConnection() lock, notice that the channel
		// is empty and then remove it.  When this function tries
		// to do c <- true, it will notice that the channel no longer exists
		c <- true

		s.lock.Unlock()

		// Return here
		return
	}
	s.lock.Unlock()

	// Do this outside the locks to pend here
	c <- true
}

func (s *SshExecutor) freeConnection(host string) {
	s.lock.Lock()
	c := s.throttlemap[host]

	<-c

	if len(c) == 0 {
		close(c)
		delete(s.throttlemap, host)
	}
	s.lock.Unlock()
}

func (s *SshExecutor) sshExec(host string, commands []string, timeoutMinutes int) ([]string, error) {

	// Throttle
	s.accessConnection(host)
	defer s.freeConnection(host)

	// Execute
	return s.exec.ConnectAndExec(host+":22", commands, timeoutMinutes)
}

func (s *SshExecutor) vgName(vgId string) string {
	return "vg_" + vgId
}

func (s *SshExecutor) brickName(brickId string) string {
	return "brick_" + brickId
}

func (s *SshExecutor) tpName(brickId string) string {
	return "tp_" + brickId
}
