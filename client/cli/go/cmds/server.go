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

package cmds

import (
	"os"
	"text/template"

	"github.com/spf13/cobra"
)

var serverCommand = &cobra.Command{
	Use:   "server",
	Short: "Heketi Server Management",
	Long:  "Heketi Server Information & Management",
}

var operationsCommand = &cobra.Command{
	Use:   "operations",
	Short: "Manage ongoing server operations",
	Long:  "Manage ongoing server operations",
}

var opInfoTemplate = `Operation Counts:
  Total: {{.Total}}
  In-Flight: {{.InFlight}}
  New: {{.New}}
  Stale: {{.Stale}}
`

var operationsInfoCommand = &cobra.Command{
	Use:     "info",
	Short:   "Get a summary of server operations",
	Long:    "Get a summary of server operations",
	Example: `  $ heketi-cli server operations info`,
	RunE: func(cmd *cobra.Command, args []string) error {
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}
		t, err := template.New("opInfo").Parse(opInfoTemplate)
		if err != nil {
			return err
		}
		opInfo, err := heketi.OperationsInfo()
		if err == nil {
			t.Execute(os.Stdout, opInfo)
		}
		return err
	},
}

func init() {
	RootCmd.AddCommand(serverCommand)
	serverCommand.AddCommand(operationsCommand)
	operationsCommand.SilenceUsage = true
	operationsCommand.AddCommand(operationsInfoCommand)
	operationsInfoCommand.SilenceUsage = true
}
