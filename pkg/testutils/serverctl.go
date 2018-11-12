// +build functional

//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), as published by the Free Software Foundation,
// or under the Apache License, Version 2.0 <LICENSE-APACHE2 or
// http://www.apache.org/licenses/LICENSE-2.0>.
//
// You may not use this file except in compliance with those terms.
//

package testutils

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"syscall"
	"time"
)

type ServerCfg struct {
	ServerDir string
	HeketiBin string
	LogPath   string
	ConfPath  string
	DbPath    string
	KeepDB    bool
}

type ServerCtl struct {
	ServerCfg

	// the real stuff
	cmd       *exec.Cmd
	cmdExited bool
	cmdErr    error
	logF      *os.File
}

func getEnvValue(k, val string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return val
}

func NewServerCfgFromEnv(dirDefault string) *ServerCfg {
	return &ServerCfg{
		ServerDir: getEnvValue("HEKETI_SERVER_DIR", dirDefault),
		HeketiBin: getEnvValue("HEKETI_SERVER", "./heketi-server"),
		LogPath:   getEnvValue("HEKETI_LOG", ""),
		DbPath:    getEnvValue("HEKETI_DB_PATH", "./heketi.db"),
		ConfPath:  getEnvValue("HEKETI_CONF_PATH", "heketi.json"),
	}
}

func NewServerCtlFromEnv(dirDefault string) *ServerCtl {
	return NewServerCtl(NewServerCfgFromEnv(dirDefault))
}

func NewServerCtl(cfg *ServerCfg) *ServerCtl {
	return &ServerCtl{ServerCfg: *cfg}
}

func (s *ServerCtl) Start() error {
	if !s.KeepDB {
		// do not preserve the heketi db between server instances
		os.Remove(path.Join(s.ServerDir, s.DbPath))
	}
	if s.LogPath == "" {
		s.logF = nil
	} else {
		f, err := os.OpenFile(s.LogPath, os.O_TRUNC|os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			return err
		}
		s.logF = f
	}
	s.cmd = exec.Command(s.HeketiBin, fmt.Sprintf("--config=%v", s.ConfPath))
	s.cmd.Dir = s.ServerDir
	if s.logF == nil {
		s.cmd.Stdout = os.Stdout
		s.cmd.Stderr = os.Stderr
	} else {
		s.cmd.Stdout = s.logF
		s.cmd.Stderr = s.logF
	}
	if err := s.cmd.Start(); err != nil {
		return err
	}
	go func() {
		s.cmdErr = s.cmd.Wait()
		s.cmdExited = true
	}()
	time.Sleep(300 * time.Millisecond)
	if !s.IsAlive() {
		return errors.New("server exited early")
	}
	return nil
}

func (s *ServerCtl) IsAlive() bool {
	if s.cmd == nil {
		// no s.cmd object so server was never started
		// needed when this function is called prior to Start
		return false
	}
	if err := s.cmd.Process.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	return true
}

func (s *ServerCtl) Stop() error {
	// close the log file fd after stopping heketi (or if stop fails)
	// this is needed in case the process has already died for some reason
	defer s.logF.Close()
	if err := s.cmd.Process.Signal(os.Interrupt); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	if !s.cmdExited {
		if err := s.cmd.Process.Kill(); err != nil {
			return err
		}
	}
	return nil
}

// Tester is an interface that can be expose a minimal
// set of functionally from testing.T.
type Tester interface {
	Fatalf(format string, args ...interface{})
}

// ServerStarted asserts that the server s is in the started
// state regardless of state prior to the call. If
// the server fails to start the function triggers a test
// failure (through the Tester interface).
func ServerStarted(t Tester, s *ServerCtl) {
	if s.IsAlive() {
		return
	}
	if err := s.Start(); err != nil {
		t.Fatalf("heketi server is not started: %v", err)
	}
}

// ServerStopped asserts that the server s is in the stopped
// state regardless of state prior to the call. If
// the server fails to stop the function triggers a test
// failure (through the Tester interface).
func ServerStopped(t Tester, s *ServerCtl) {
	if !s.IsAlive() {
		return
	}
	if err := s.Stop(); err != nil {
		t.Fatalf("heketi server is not stopped: %v", err)
	}
}

// ServerRestarted asserts that the server is started but
// that any existing instance is first stopped. If any
// steps fails the function triggers a test failure
// (through the TestSuite interface).
func ServerRestarted(t Tester, s *ServerCtl) {
	if s.IsAlive() {
		ServerStopped(t, s)
	}
	if s.IsAlive() {
		t.Fatalf("heketi server should have been stopped")
	}
	ServerStarted(t, s)
}
