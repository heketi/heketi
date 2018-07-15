//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package cmds

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
)

// snapshotCmd represents the snapshot command
var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Heketi Snapshot Management",
	Long:  "Heketi Snapshot Management",
}

func init() {
	RootCmd.AddCommand(snapshotCmd)
	snapshotCmd.AddCommand(snapshotListCommand)
	snapshotCmd.AddCommand(snapshotInfoCommand)
	snapshotCmd.AddCommand(snapshotDeleteCommand)
	snapshotListCommand.SilenceUsage = true
}

var snapshotListCommand = &cobra.Command{
	Use:     "list",
	Short:   "Lists the snapshots managed by Heketi",
	Long:    "Lists the snapshots managed by Heketi",
	Example: "  $ heketi-cli snapshot list",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create a client
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}

		// List volumes
		list, err := heketi.SnapshotList()
		if err != nil {
			return err
		}

		if options.Json {
			data, err := json.Marshal(list)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, string(data))
		} else {
			for _, id := range list.Snapshots {
				snapshot, err := heketi.SnapshotInfo(id)
				if err != nil {
					return err
				}

				fmt.Fprintf(stdout, "Id:%-35v Name:%v Description:%v \n",
					id,
					snapshot.Name,
					snapshot.Description)
			}
		}

		return nil
	},
}

var snapshotInfoCommand = &cobra.Command{
	Use:     "info",
	Short:   "get the snapshots info managed by Heketi",
	Long:    "get the snapshots info managed by Heketi",
	Example: "  $ heketi-cli snapshot info <snapshotid>",
	RunE: func(cmd *cobra.Command, args []string) error {
		//ensure proper number of args
		s := cmd.Flags().Args()
		if len(s) < 1 {
			return errors.New("snapshot id missing")
		}
		snapshotID := cmd.Flags().Arg(0)

		// Create a client
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}

		// get snapshot info
		snapshot, err := heketi.SnapshotInfo(snapshotID)
		if err != nil {
			return err
		}

		if options.Json {
			data, err := json.Marshal(snapshot)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, string(data))
		} else {
			fmt.Fprintf(stdout, "Id:%-35v Name:%v Description:%v \n",
				id,
				snapshot.Name,
				snapshot.Description)
		}
		return nil
	},
}

var snapshotDeleteCommand = &cobra.Command{
	Use:     "delete",
	Short:   "delete the snapshots info managed by Heketi",
	Long:    "delete the snapshots info managed by Heketi",
	Example: "  $ heketi-cli snapshot delete <snapshotid>",
	RunE: func(cmd *cobra.Command, args []string) error {
		//ensure proper number of args
		s := cmd.Flags().Args()
		if len(s) < 1 {
			return errors.New("snapshot id missing")
		}
		snapshotID := cmd.Flags().Arg(0)

		// Create a client
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}

		// get snapshot info
		err = heketi.SnapshotDelete(snapshotID)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Snapshot %v deleted\n", snapshotID)
		return nil
	},
}
