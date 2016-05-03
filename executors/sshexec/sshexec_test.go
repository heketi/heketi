//
// Copyright (c) 2016 The heketi Authors
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
	"os"
	"testing"

	"github.com/heketi/tests"
	"github.com/heketi/utils"
)

// Mock SSH calls
type FakeSsh struct {
	FakeConnectAndExec func(host string, commands []string, timeoutMinutes int) ([]string, error)
}

func NewFakeSsh() *FakeSsh {
	f := &FakeSsh{}

	f.FakeConnectAndExec = func(host string,
		commands []string,
		timeoutMinutes int) ([]string, error) {
		return []string{""}, nil
	}

	return f
}

func (f *FakeSsh) ConnectAndExec(host string,
	commands []string,
	timeoutMinutes int) ([]string, error) {
	return f.FakeConnectAndExec(host, commands, timeoutMinutes)

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
		Fstab:          "xfstab",
	}

	s := NewSshExecutor(config)
	tests.Assert(t, s != nil)
	tests.Assert(t, s.private_keyfile == config.PrivateKeyFile)
	tests.Assert(t, s.user == config.User)
	tests.Assert(t, s.port == config.Port)
	tests.Assert(t, s.fstab == config.Fstab)
	tests.Assert(t, s.exec != nil)
}

func TestNewSshExecDefaults(t *testing.T) {
	f := NewFakeSsh()
	defer tests.Patch(&sshNew,
		func(logger *utils.Logger, user string, file string) (Ssher, error) {
			return f, nil
		}).Restore()

	config := &SshConfig{}

	s := NewSshExecutor(config)
	tests.Assert(t, s != nil)
	tests.Assert(t, s.private_keyfile == os.Getenv("HOME")+"/.ssh/id_rsa")
	tests.Assert(t, s.user == "heketi")
	tests.Assert(t, s.port == "22")
	tests.Assert(t, s.fstab == "/etc/fstab")
	tests.Assert(t, s.exec != nil)

}

func TestNewSshExecBadPrivateKeyLocation(t *testing.T) {
	config := &SshConfig{
		PrivateKeyFile: "thereisnospoon",
	}

	s := NewSshExecutor(config)
	tests.Assert(t, s == nil)
}
