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
	zone               int
	managmentHostNames string
	storageHostNames   string
	clusterId          string
)

func init() {
	RootCmd.AddCommand(NodeCommand)
	NodeCommand.AddCommand(NodeAddCommand)
	NodeCommand.AddCommand(NodeDeleteCommand)
	NodeCommand.AddCommand(NodeInfoCommand)
	NodeAddCommand.Flags().IntVar(&zone, "zone", -1, "The zone in which the node should reside")
	NodeAddCommand.Flags().StringVar(&clusterId, "cluster", "", "The cluster in which the node should reside")
	NodeAddCommand.Flags().StringVar(&managmentHostNames, "management-host-name", "", "Managment host name")
	NodeAddCommand.Flags().StringVar(&storageHostNames, "storage-host-name", "", "Storage host name")
	NodeAddCommand.SilenceUsage = true
	NodeDeleteCommand.SilenceUsage = true
	NodeInfoCommand.SilenceUsage = true
}

var NodeCommand = &cobra.Command{
	Use:   "node",
	Short: "Heketi Node Management",
	Long:  "Heketi Node Management",
}

var NodeAddCommand = &cobra.Command{
	Use:   "add",
	Short: "Add new node to be managed by Heketi",
	Long:  "Add new node to be managed by Heketi",
	Example: `  $ heketi-cli node add \
      --zone=3 \
      --cluster=3e098cb4407d7109806bb196d9e8f095 \
      --management-host-name=node1-manage.gluster.lab.com \
      --storage-host-name=node1-storage.gluster.lab.com
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check arguments
		if zone == -1 {
			return errors.New("Missing zone")
		}
		if managmentHostNames == "" {
			return errors.New("Missing management hostname")
		}
		if storageHostNames == "" {
			return errors.New("Missing storage hostname")
		}
		if clusterId == "" {
			return errors.New("Missing cluster id")
		}

		// Create request blob
		req := &glusterfs.NodeAddRequest{}
		req.ClusterId = clusterId
		req.Hostnames.Manage = []string{managmentHostNames}
		req.Hostnames.Storage = []string{storageHostNames}
		req.Zone = zone

		// Create a client
		heketi := client.NewClient(options.Url, options.User, options.Key)

		// Add node
		node, err := heketi.NodeAdd(req)
		if err != nil {
			return err
		}

		if options.Json {
			data, err := json.Marshal(node)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, string(data))
		} else {
			fmt.Fprintf(stdout, "Node information:\n"+
				"Id: %v\n"+
				"Cluster Id: %v\n"+
				"Zone: %v\n"+
				"Management Hostname %v\n"+
				"Storage Hostname %v\n",
				node.Id,
				node.ClusterId,
				node.Zone,
				node.Hostnames.Manage[0],
				node.Hostnames.Storage[0])
		}
		return nil
	},
}

var NodeDeleteCommand = &cobra.Command{
	Use:     "delete [node_id]",
	Short:   "Deletes a node from Heketi management",
	Long:    "Deletes a node from Heketi management",
	Example: "  $ heketi-cli node delete 886a86a868711bef83001",
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Flags().Args()

		//ensure proper number of args
		if len(s) < 1 {
			return errors.New("Node id missing")
		}

		//set clusterId
		nodeId := cmd.Flags().Arg(0)

		// Create a client
		heketi := client.NewClient(options.Url, options.User, options.Key)

		//set url
		err := heketi.NodeDelete(nodeId)
		if err == nil {
			fmt.Fprintf(stdout, "Node %v deleted\n", nodeId)
		}

		return err
	},
}

var NodeInfoCommand = &cobra.Command{
	Use:     "info [node_id]",
	Short:   "Retreives information about the node",
	Long:    "Retreives information about the node",
	Example: "  $ heketi-cli node info 886a86a868711bef83001",
	RunE: func(cmd *cobra.Command, args []string) error {
		//ensure proper number of args
		s := cmd.Flags().Args()
		if len(s) < 1 {
			return errors.New("Node id missing")
		}

		// Set node id
		nodeId := cmd.Flags().Arg(0)

		// Create a client to talk to Heketi
		heketi := client.NewClient(options.Url, options.User, options.Key)

		// Create cluster
		info, err := heketi.NodeInfo(nodeId)
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
			fmt.Fprintf(stdout, "Node Id: %v\n"+
				"Cluster Id: %v\n"+
				"Zone: %v\n"+
				"Management Hostname: %v\n"+
				"Storage Hostname: %v\n",
				info.Id,
				info.ClusterId,
				info.Zone,
				info.Hostnames.Manage[0],
				info.Hostnames.Storage[0])
			fmt.Fprintf(stdout, "Devices:\n")
			for _, d := range info.DevicesInfo {
				fmt.Fprintf(stdout, "Id:%-35v"+
					"Name:%-20v"+
					"Size (GiB):%-8v"+
					"Used (GiB):%-8v"+
					"Free (GiB):%-8v\n",
					d.Id,
					d.Name,
					d.Storage.Total/(1024*1024),
					d.Storage.Used/(1024*1024),
					d.Storage.Free/(1024*1024))
			}
		}
		return nil
	},
}
