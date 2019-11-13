//
// Copyright (c) 2019 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package cmds

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(brickCommand)
	brickCommand.AddCommand(brickEvictCommand)
	brickCommand.SilenceUsage = true
}

var brickCommand = &cobra.Command{
	Use:   "brick",
	Short: "Heketi Brick Management",
	Long:  "Heketi Brick Management",
}

var brickEvictCommand = &cobra.Command{
	Use:     "evict",
	Short:   "Evict (remove) a brick from a volume",
	Long:    "Evict (remove) a brick from a volume",
	Example: "  $ heketi-cli brick evict ee6a86a868711bef8300c",
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error

		if len(args) < 1 {
			return fmt.Errorf("Brick id missing")
		} else if len(args) > 1 {
			return fmt.Errorf("Too many arguments provided")
		}
		brickId := args[0]

		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}

		err = heketi.BrickEvict(brickId, nil)
		if err == nil {
			fmt.Fprintf(stdout, "Brick %v evicted\n", brickId)
		}
		return err
	},
}
