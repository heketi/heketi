//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package cmdexec

type CommandFaker struct {
	FakeConnectAndExec func(host string,
		commands []string,
		timeoutMinutes int,
		useSudo bool) ([]string, error)
}

func NewCommandFaker() *CommandFaker {
	f := &CommandFaker{}
	f.FakeConnectAndExec = func(
		host string, commands []string,
		timeoutMinutes int, useSudo bool) ([]string, error) {
		return []string{}, nil
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
	t.Throttlemap = make(map[string]chan bool)
	t.fake = f
	t.Fstab = "/my/fstab"
	t.portStr = "22"
	return t, nil
}

func (s *FakeExecutor) RemoteCommandExecute(host string,
	commands []string,
	timeoutMinutes int) ([]string, error) {

	s.AccessConnection(host)
	defer s.FreeConnection(host)

	return s.fake.FakeConnectAndExec(
		host+":"+s.portStr, commands, timeoutMinutes, s.useSudo)
}

func (s *FakeExecutor) RebalanceOnExpansion() bool {
	return false
}

func (s *FakeExecutor) SnapShotLimit() int {
	return s.snapShotLimit
}
