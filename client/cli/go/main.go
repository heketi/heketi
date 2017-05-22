//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package main

import (
	"io"
	"os"

	"github.com/heketi/heketi/client/cli/go/cmds"
)

var (
	HEKETI_CLI_VERSION           = "(dev)"
	stdout             io.Writer = os.Stdout
	stderr             io.Writer = os.Stderr
	version            bool
)

func main() {
	stdout, _ = os.OpenFile("/var/log/heketi/heketi.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	cmd := cmds.NewHeketiCli(HEKETI_CLI_VERSION, stderr, stdout)
	if err := cmd.Execute(); err != nil {
		//fmt.Println(err) //Should be used for logging
		os.Exit(-1)
	}
}
