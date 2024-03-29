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
	rex "github.com/heketi/heketi/v10/pkg/remoteexec"
)

type CommandFaker struct {
	FakeConnectAndExec func(host string,
		commands []string,
		timeoutMinutes int,
		useSudo bool) (rex.Results, error)
}

func NewCommandFaker() *CommandFaker {
	f := &CommandFaker{}
	f.FakeConnectAndExec = func(
		host string, commands []string,
		timeoutMinutes int, useSudo bool) (rex.Results, error) {
		return rex.Results{}, nil
	}
	return f
}

type FakeExecutor struct {
	CmdExecutor

	fake          *CommandFaker
	portStr       string
	snapShotLimit int
	useSudo       bool
}

func NewFakeExecutor(f *CommandFaker) (*FakeExecutor, error) {
	t := &FakeExecutor{}
	t.RemoteExecutor = t
	config := &CmdConfig{}
	config.GlusterCliTimeout = 42
	config.LVChunkSize = "256K"
	config.XfsSu = 0
	config.XfsSw = 0
	t.CmdExecutor.Init(config)
	t.fake = f
	t.Fstab = "/my/fstab"
	t.portStr = "22"
	t.MountOpts = DefaultMountOpts
	return t, nil
}

func (s *FakeExecutor) RemoteCommandExecute(host string,
	commands []string,
	timeoutMinutes int) ([]string, error) {

	s.AccessConnection(host)
	defer s.FreeConnection(host)

	r, err := s.fake.FakeConnectAndExec(
		host+":"+s.portStr, commands, timeoutMinutes, s.useSudo)
	if err != nil {
		return nil, err
	}
	return r.SquashErrors()
}

func (s *FakeExecutor) ExecCommands(host string,
	commands rex.Cmds,
	timeoutMinutes int) (rex.Results, error) {

	s.AccessConnection(host)
	defer s.FreeConnection(host)

	c := make([]string, len(commands))
	for i, v := range commands {
		c[i] = v.String()
	}
	return s.fake.FakeConnectAndExec(
		host+":"+s.portStr, c, timeoutMinutes, s.useSudo)
}

func (s *FakeExecutor) RebalanceOnExpansion() bool {
	return false
}

func (s *FakeExecutor) SnapShotLimit() int {
	return s.snapShotLimit
}
