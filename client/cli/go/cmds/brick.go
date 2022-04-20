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
	"time"

	"github.com/heketi/heketi/v10/pkg/glusterfs/api"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(brickCommand)
	brickCommand.AddCommand(brickEvictCommand)
	brickEvictCommand.Flags().Bool("expert-option-disable-heal-check", false,
		"[DANGEROUS] Skip the heal check while brick replace.")
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

		skipHeal, err := cmd.Flags().GetBool("expert-option-disable-heal-check")
		if err != nil {
			return err
		}

		if skipHeal {
			fmt.Println(
				"Skipping the heal check may be dangerous and increase the risk of data loss.\n",
				"Press CTRL-C within 10 seconds to cancel this action.")
			time.Sleep(10 * time.Second)
		}

		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}

		req := &api.BrickEvictOptions{}
		if skipHeal {
			req.HealCheck = api.HealCheckDisable
		}

		err = heketi.BrickEvict(brickId, req)
		if err == nil {
			fmt.Fprintf(stdout, "Brick %v evicted\n", brickId)
		}
		return err
	},
}
