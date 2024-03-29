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
	"github.com/heketi/heketi/v10/executors/cmdexec"
)

type SshConfig struct {
	cmdexec.CmdConfig

	PrivateKeyFile string `json:"keyfile"`
	User           string `json:"user"`
	Port           string `json:"port"`
}
