//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package cmdexec

import (
	"sync"

	"github.com/heketi/heketi/pkg/utils"
)

var (
	logger = utils.NewLogger("[cmdexec]", utils.LEVEL_DEBUG)
)

type RemoteCommandTransport interface {
	RemoteCommandExecute(host string, commands []string, timeoutMinutes int) ([]string, error)
	RebalanceOnExpansion() bool
	SnapShotLimit() int
}

type CmdExecutor struct {
	Throttlemap map[string]chan bool
	Lock        sync.Mutex

	RemoteExecutor RemoteCommandTransport
	Fstab          string
}

func (s *CmdExecutor) AccessConnection(host string) {
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

func (s *CmdExecutor) FreeConnection(host string) {
	s.Lock.Lock()
	c := s.Throttlemap[host]
	s.Lock.Unlock()

	<-c
}

func (s *CmdExecutor) SetLogLevel(level string) {
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

func (s *CmdExecutor) Logger() *utils.Logger {
	return logger
}
