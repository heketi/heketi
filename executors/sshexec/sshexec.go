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
	"os"
	"sync"

	"github.com/heketi/utils"
	"github.com/heketi/utils/ssh"
	"github.com/lpabon/godbc"
)

type Ssher interface {
	ConnectAndExec(host string, commands []string, timeoutMinutes int) ([]string, error)
}

type SshExecutor struct {
	private_keyfile string
	user            string
	throttlemap     map[string]chan bool
	lock            sync.Mutex
	exec            Ssher
	config          *SshConfig
	port            string
	fstab           string
}

type SshConfig struct {
	PrivateKeyFile string `json:"keyfile"`
	User           string `json:"user"`
	Port           string `json:"port"`
	Fstab          string `json:"fstab"`

	// Experimental Settings
	RebalanceOnExpansion bool `json:"rebalance_on_expansion"`
}

var (
	logger           = utils.NewLogger("[sshexec]", utils.LEVEL_DEBUG)
	ErrSshPrivateKey = errors.New("Unable to read private key file")
	sshNew           = func(logger *utils.Logger, user string, file string) (Ssher, error) {
		s := ssh.NewSshExecWithKeyFile(logger, user, file)
		if s == nil {
			return nil, ErrSshPrivateKey
		}
		return s, nil
	}
)

func NewSshExecutor(config *SshConfig) *SshExecutor {
	godbc.Require(config != nil)

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

	if config.Port == "" {
		s.port = "22"
	} else {
		s.port = config.Port
	}

	if config.Fstab == "" {
		s.fstab = "/etc/fstab"
	} else {
		s.fstab = config.Fstab
	}

	// Save the configuration
	s.config = config

	// Show experimental settings
	if s.config.RebalanceOnExpansion {
		logger.Warning("Rebalance on volume expansion has been enabled.  This is an EXPERIMENTAL feature")
	}

	// Setup key
	var err error
	s.exec, err = sshNew(logger, s.user, s.private_keyfile)
	if err != nil {
		logger.Err(err)
		return nil
	}

	godbc.Ensure(s != nil)
	godbc.Ensure(s.config == config)
	godbc.Ensure(s.user != "")
	godbc.Ensure(s.private_keyfile != "")
	godbc.Ensure(s.port != "")
	godbc.Ensure(s.fstab != "")

	return s
}

func (s *SshExecutor) accessConnection(host string) {

	var (
		c  chan bool
		ok bool
	)

	s.lock.Lock()
	if c, ok = s.throttlemap[host]; !ok {
		c = make(chan bool, 1)
		s.throttlemap[host] = c
	}
	s.lock.Unlock()

	c <- true
}

func (s *SshExecutor) freeConnection(host string) {
	s.lock.Lock()
	c := s.throttlemap[host]
	s.lock.Unlock()

	<-c
}

func (s *SshExecutor) sshExec(host string, commands []string, timeoutMinutes int) ([]string, error) {

	// Throttle
	s.accessConnection(host)
	defer s.freeConnection(host)

	// Execute
	return s.exec.ConnectAndExec(host+":"+s.port, commands, timeoutMinutes)
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
