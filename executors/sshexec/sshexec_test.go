//
// Copyright (c) 2016 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package sshexec

import (
	"testing"

	"github.com/heketi/heketi/pkg/utils"
	"github.com/heketi/tests"
)

// Mock SSH calls
type FakeSsh struct {
	FakeConnectAndExec func(host string,
		commands []string,
		timeoutMinutes int,
		useSudo bool) ([]string, error)
}

func NewFakeSsh() *FakeSsh {
	f := &FakeSsh{}

	f.FakeConnectAndExec = func(host string,
		commands []string,
		timeoutMinutes int,
		useSudo bool) ([]string, error) {
		return []string{""}, nil
	}

	return f
}

func (f *FakeSsh) ConnectAndExec(host string,
	commands []string,
	timeoutMinutes int,
	useSudo bool) ([]string, error) {
	return f.FakeConnectAndExec(host, commands, timeoutMinutes, useSudo)

}

func TestNewSshExec(t *testing.T) {

	f := NewFakeSsh()
	defer tests.Patch(&sshNew,
		func(logger *utils.Logger, user string, file string) (Ssher, error) {
			return f, nil
		}).Restore()

	config := &SshConfig{
		PrivateKeyFile: "xkeyfile",
		User:           "xuser",
		Port:           "100",
		CLICommandConfig: CLICommandConfig{
			Fstab: "xfstab",
		},
	}

	s, err := NewSshExecutor(config)
	tests.Assert(t, err == nil)
	tests.Assert(t, s != nil)
	tests.Assert(t, s.private_keyfile == config.PrivateKeyFile)
	tests.Assert(t, s.user == config.User)
	tests.Assert(t, s.port == config.Port)
	tests.Assert(t, s.Fstab == config.Fstab)
	tests.Assert(t, s.exec != nil)
}

func TestSshExecRebalanceOnExpansion(t *testing.T) {

	f := NewFakeSsh()
	defer tests.Patch(&sshNew,
		func(logger *utils.Logger, user string, file string) (Ssher, error) {
			return f, nil
		}).Restore()

	config := &SshConfig{
		PrivateKeyFile: "xkeyfile",
		User:           "xuser",
		Port:           "100",
		CLICommandConfig: CLICommandConfig{
			Fstab: "xfstab",
		},
	}

	s, err := NewSshExecutor(config)
	tests.Assert(t, err == nil)
	tests.Assert(t, s != nil)
	tests.Assert(t, s.private_keyfile == config.PrivateKeyFile)
	tests.Assert(t, s.user == config.User)
	tests.Assert(t, s.port == config.Port)
	tests.Assert(t, s.Fstab == config.Fstab)
	tests.Assert(t, s.exec != nil)
	tests.Assert(t, s.RebalanceOnExpansion() == false)

	config = &SshConfig{
		PrivateKeyFile: "xkeyfile",
		User:           "xuser",
		Port:           "100",
		CLICommandConfig: CLICommandConfig{
			Fstab:                "xfstab",
			RebalanceOnExpansion: true,
		},
	}

	s, err = NewSshExecutor(config)
	tests.Assert(t, err == nil)
	tests.Assert(t, s != nil)
	tests.Assert(t, s.private_keyfile == config.PrivateKeyFile)
	tests.Assert(t, s.user == config.User)
	tests.Assert(t, s.port == config.Port)
	tests.Assert(t, s.Fstab == config.Fstab)
	tests.Assert(t, s.exec != nil)
	tests.Assert(t, s.RebalanceOnExpansion() == true)

}

func TestNewSshExecDefaults(t *testing.T) {
	f := NewFakeSsh()
	defer tests.Patch(&sshNew,
		func(logger *utils.Logger, user string, file string) (Ssher, error) {
			return f, nil
		}).Restore()

	config := &SshConfig{
		PrivateKeyFile: "xkeyfile",
	}

	s, err := NewSshExecutor(config)
	tests.Assert(t, err == nil)
	tests.Assert(t, s != nil)
	tests.Assert(t, s.private_keyfile == "xkeyfile")
	tests.Assert(t, s.user == "heketi")
	tests.Assert(t, s.port == "22")
	tests.Assert(t, s.Fstab == "/etc/fstab")
	tests.Assert(t, s.exec != nil)

}

func TestNewSshExecBadPrivateKeyLocation(t *testing.T) {
	config := &SshConfig{}

	s, err := NewSshExecutor(config)
	tests.Assert(t, s == nil)
	tests.Assert(t, err != nil)
}
