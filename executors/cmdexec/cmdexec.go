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
	"fmt"
	"os"
	"regexp"
	"strconv"
	"sync"

	"github.com/heketi/heketi/v10/pkg/logging"
	rex "github.com/heketi/heketi/v10/pkg/remoteexec"
)

var (
	logger           = logging.NewLogger("[cmdexec]", logging.LEVEL_DEBUG)
	preallocRe       = regexp.MustCompile("^[a-zA-Z0-9-_]+$")
	DefaultMountOpts = "rw,inode64,noatime,nouuid"
)

type RemoteCommandTransport interface {
	ExecCommands(host string, commands rex.Cmds, timeoutMinutes int) (rex.Results, error)
	RebalanceOnExpansion() bool
	SnapShotLimit() int
	GlusterCliTimeout() uint32
	PVDataAlignment() string
	VGPhysicalExtentSize() string
	LVChunkSize() string
	XfsSw() int
	XfsSu() int
}

type CmdExecutor struct {
	config      *CmdConfig
	Throttlemap map[string]chan bool
	Lock        sync.Mutex

	RemoteExecutor RemoteCommandTransport
	Fstab          string
	MountOpts      string
	BackupLVM      bool
}

func (c *CmdExecutor) glusterCommand() string {
	return fmt.Sprintf("gluster --mode=script --timeout=%v", c.GlusterCliTimeout())
}

// When running Gluster inside a container, the LVM commands need to be
// executed on the host. The Gluster Server containers provide a wrapper script
// for `nsenter` and its arguments as `exec-on-host`.
//
// All LVM commands will get prefixed with `/usr/sbin/lvm` in order to make
// things a little more secure (hopefully).
func (c *CmdExecutor) lvmCommand() string {
	if wrapper := c.LVMWrapper(); wrapper != "" {
		return wrapper + " /usr/sbin/lvm"
	}

	return "/usr/sbin/lvm"
}

func (c *CmdExecutor) udevCommand() string {
	if wrapper := c.LVMWrapper(); wrapper != "" {
		return wrapper + " /usr/bin/udevadm"
	}

	return "/usr/bin/udevadm"
}

func setWithEnvVariables(config *CmdConfig) {
	var env string

	env = os.Getenv("HEKETI_GLUSTER_CLI_TIMEOUT")
	if env != "" {
		value, err := strconv.ParseUint(env, 10, 32)
		if err != nil {
			logger.LogError("Error: While parsing HEKETI_GLUSTER_CLI_TIMEOUT: %v", err)
		} else {
			config.GlusterCliTimeout = uint32(value)
		}
	}

	env = os.Getenv("HEKETI_DEBUG_UMOUNT_FAILURES")
	if env != "" {
		value, err := strconv.ParseBool(env)
		if err != nil {
			logger.LogError("Error: While parsing HEKETI_DEBUG_UMOUNT_FAILURES: %v", err)
		} else {
			config.DebugUmountFailures = value
		}
	}

	env = os.Getenv("HEKETI_BLOCK_VOLUME_DEFAULT_PREALLOC")
	if env != "" {
		config.BlockVolumePrealloc = env
	}

	env = os.Getenv("HEKETI_LVM_WRAPPER")
	if env != "" {
		config.LVMWrapper = env
	}
}

func (c *CmdExecutor) Init(config *CmdConfig) {
	c.Throttlemap = make(map[string]chan bool)
	c.config = config

	setWithEnvVariables(config)
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
		logger.SetLevel(logging.LEVEL_NOLOG)
	case "critical":
		logger.SetLevel(logging.LEVEL_CRITICAL)
	case "error":
		logger.SetLevel(logging.LEVEL_ERROR)
	case "warning":
		logger.SetLevel(logging.LEVEL_WARNING)
	case "info":
		logger.SetLevel(logging.LEVEL_INFO)
	case "debug":
		logger.SetLevel(logging.LEVEL_DEBUG)
	}
}

func (s *CmdExecutor) Logger() *logging.Logger {
	return logger
}

func (c *CmdExecutor) GlusterCliTimeout() uint32 {
	if c.config.GlusterCliTimeout == 0 {
		// Use a longer timeout (10 minutes) than gluster cli's default
		// of 2 minutes, because some commands take longer in a system
		// with many volumes.
		return 600
	}

	return c.config.GlusterCliTimeout
}

// The timeout, in minutes, for the command execution.
// It used to be 10 minutes (or sometimes 5, for some simple commands),
// but now it needs to be longer than the gluster cli timeout at
// least where calling the gluster cli.
func (c *CmdExecutor) GlusterCliExecTimeout() int {
	timeout := 1 + (int(c.GlusterCliTimeout())+1)/60

	if timeout < 10 {
		timeout = 10
	}

	return timeout
}

func (c *CmdExecutor) PVDataAlignment() string {
	if c.config.PVDataAlignment == "" {
		return "256K"
	}
	return c.config.PVDataAlignment
}

func (c *CmdExecutor) VGPhysicalExtentSize() string {
	if c.config.VGPhysicalExtentSize == "" {
		return "4M"
	}
	return c.config.VGPhysicalExtentSize
}

func (c *CmdExecutor) LVChunkSize() string {
	if c.config.LVChunkSize == "" {
		return "256K"
	}
	return c.config.LVChunkSize
}

func (c *CmdExecutor) XfsSw() int {
	return c.config.XfsSw
}

func (c *CmdExecutor) XfsSu() int {
	return c.config.XfsSu
}

func (c *CmdExecutor) DebugUmountFailures() bool {
	return c.config.DebugUmountFailures
}

func (c *CmdExecutor) BlockVolumeDefaultPrealloc() string {
	defaultValue := "full"
	if c.config.BlockVolumePrealloc == "" {
		return defaultValue
	}
	if !preallocRe.MatchString(c.config.BlockVolumePrealloc) {
		logger.Warning(
			"invalid value for prealloc option [%v], using default",
			c.config.BlockVolumePrealloc)
		return defaultValue
	}
	return c.config.BlockVolumePrealloc
}

func (c *CmdExecutor) LVMWrapper() string {
	return c.config.LVMWrapper
}
