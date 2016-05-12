//
// Copyright (c) 2015 The heketi Authors
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

package cmds

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/heketi/heketi/apps/glusterfs"
	"github.com/heketi/heketi/client/api/go-client"
	"github.com/spf13/cobra"
)

var (
	device, nodeId string
)

func init() {
	RootCmd.AddCommand(DeviceCommand)
	DeviceCommand.AddCommand(DeviceAddCommand)
	DeviceCommand.AddCommand(DeviceDeleteCommand)
	DeviceCommand.AddCommand(DeviceInfoCommand)
	DeviceAddCommand.Flags().StringVar(&device, "name", "",
		"Name of device to add")
	DeviceAddCommand.Flags().StringVar(&nodeId, "node", "",
		"Id of the node which has this device")
	DeviceAddCommand.SilenceUsage = true
	DeviceDeleteCommand.SilenceUsage = true
	DeviceInfoCommand.SilenceUsage = true
}

var DeviceCommand = &cobra.Command{
	Use:   "device",
	Short: "Heketi device management",
	Long:  "Heketi Device Management",
}

var DeviceAddCommand = &cobra.Command{
	Use:   "add",
	Short: "Add new device to node to be managed by Heketi",
	Long:  "Add new device to node to be managed by Heketi",
	Example: `  $ heketi-cli device add \
      --name=/dev/sdb
      --node=3e098cb4407d7109806bb196d9e8f095 `,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check arguments
		if device == "" {
			return errors.New("Missing device name")
		}
		if nodeId == "" {
			return errors.New("Missing node id")
		}

		// Create request blob
		req := &glusterfs.DeviceAddRequest{}
		req.Name = device
		req.NodeId = nodeId

		// Create a client
		heketi := client.NewClient(options.Url, options.User, options.Key)

		// Add node
		err := heketi.DeviceAdd(req)
		if err != nil {
			return err
		} else {
			fmt.Fprintf(stdout, "Device added successfully\n")
		}

		return nil
	},
}

var DeviceDeleteCommand = &cobra.Command{
	Use:     "delete [device_id]",
	Short:   "Deletes a device from Heketi node",
	Long:    "Deletes a device from Heketi node",
	Example: "  $ heketi-cli device delete 886a86a868711bef83001",
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Flags().Args()

		//ensure proper number of args
		if len(s) < 1 {
			return errors.New("Device id missing")
		}

		//set clusterId
		deviceId := cmd.Flags().Arg(0)

		// Create a client
		heketi := client.NewClient(options.Url, options.User, options.Key)

		//set url
		err := heketi.DeviceDelete(deviceId)
		if err == nil {
			fmt.Fprintf(stdout, "Device %v deleted\n", deviceId)
		}

		return err
	},
}

var DeviceInfoCommand = &cobra.Command{
	Use:     "info [device_id]",
	Short:   "Retreives information about the device",
	Long:    "Retreives information about the device",
	Example: "  $ heketi-cli node info 886a86a868711bef83001",
	RunE: func(cmd *cobra.Command, args []string) error {
		//ensure proper number of args
		s := cmd.Flags().Args()
		if len(s) < 1 {
			return errors.New("Device id missing")
		}

		// Set node id
		deviceId := cmd.Flags().Arg(0)

		// Create a client to talk to Heketi
		heketi := client.NewClient(options.Url, options.User, options.Key)

		// Create cluster
		info, err := heketi.DeviceInfo(deviceId)
		if err != nil {
			return err
		}

		if options.Json {
			data, err := json.Marshal(info)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, string(data))
		} else {
			fmt.Fprintf(stdout, "Device Id: %v\n"+
				"Name: %v\n"+
				"Size (GiB): %v\n"+
				"Used (GiB): %v\n"+
				"Free (GiB): %v\n",
				info.Id,
				info.Name,
				info.Storage.Total/(1024*1024),
				info.Storage.Used/(1024*1024),
				info.Storage.Free/(1024*1024))

			fmt.Fprintf(stdout, "Bricks:\n")
			for _, d := range info.Bricks {
				fmt.Fprintf(stdout, "Id:%-35v"+
					"Size (GiB):%-8v"+
					"Path: %v\n",
					d.Id,
					d.Size/(1024*1024),
					d.Path)
			}
		}
		return nil

	},
}
