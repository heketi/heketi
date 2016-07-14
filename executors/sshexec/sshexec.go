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
	"fmt"
	"sync"

	"github.com/heketi/heketi/pkg/utils"
	"github.com/heketi/heketi/pkg/utils/ssh"
	"github.com/lpabon/godbc"
)

type RemoteCommandTransport interface {
	RemoteCommandExecute(host string, commands []string, timeoutMinutes int) ([]string, error)
}

type Ssher interface {
	ConnectAndExec(host string, commands []string, timeoutMinutes int, useSudo bool) ([]string, error)
}

type SshExecutor struct {
	// "Public"
	Throttlemap    map[string]chan bool
	Lock           sync.Mutex
	RemoteExecutor RemoteCommandTransport
	Fstab          string

	// Private
	private_keyfile string
	user            string
	exec            Ssher
	config          *SshConfig
	port            string
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

func NewSshExecutor(config *SshConfig) (*SshExecutor, error) {
	godbc.Require(config != nil)

	s := &SshExecutor{}
	s.RemoteExecutor = s
	s.Throttlemap = make(map[string]chan bool)

	// Set configuration
	if config.PrivateKeyFile == "" {
		return nil, fmt.Errorf("Missing ssh private key file in configuration")
	}
	s.private_keyfile = config.PrivateKeyFile

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
		s.Fstab = "/etc/fstab"
	} else {
		s.Fstab = config.Fstab
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
		return nil, err
	}

	godbc.Ensure(s != nil)
	godbc.Ensure(s.config == config)
	godbc.Ensure(s.user != "")
	godbc.Ensure(s.private_keyfile != "")
	godbc.Ensure(s.port != "")
	godbc.Ensure(s.Fstab != "")

	return s, nil
}

func (s *SshExecutor) SetLogLevel(level string) {
	switch level {
	case "none":
		logger.SetLevel(utils.LEVEL_NOLOG)
	case "critical":
		logger.SetLevel(utils.LEVEL_CRITICAL)
	case "error":
		logger.SetLevel(utils.LEVEL_ERROR)
	case "warning":
		logger.SetLevel(utils.LEVEL_WARNING)
	case "info":
		logger.SetLevel(utils.LEVEL_INFO)
	case "debug":
		logger.SetLevel(utils.LEVEL_DEBUG)
	}
}

func (s *SshExecutor) AccessConnection(host string) {

	var (
		c  chan bool
		ok bool
	)

	s.Lock.Lock()
	if c, ok = s.Throttlemap[host]; !ok {
		c = make(chan bool, 1)
		s.Throttlemap[host] = c
	}
	s.Lock.Unlock()

	c <- true
}

func (s *SshExecutor) FreeConnection(host string) {
	s.Lock.Lock()
	c := s.Throttlemap[host]
	s.Lock.Unlock()

	<-c
}

func (s *SshExecutor) RemoteCommandExecute(host string,
	commands []string,
	timeoutMinutes int) ([]string, error) {

	// Throttle
	s.AccessConnection(host)
	defer s.FreeConnection(host)

	// Execute
	return s.exec.ConnectAndExec(host+":"+s.port, commands, timeoutMinutes, s.config.Sudo)
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
