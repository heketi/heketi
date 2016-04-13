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
	"os"

	"github.com/heketi/heketi/apps/glusterfs"
	client "github.com/heketi/heketi/client/api/go-client"
	"github.com/spf13/cobra"
)

var jsonConfigFile string

// Config file
type ConfigFileNode struct {
	Devices []string                 `json:"devices"`
	Node    glusterfs.NodeAddRequest `json:"node"`
}
type ConfigFileCluster struct {
	Nodes []ConfigFileNode `json:"nodes"`
}
type ConfigFile struct {
	Clusters []ConfigFileCluster `json:"clusters"`
}

func init() {
	RootCmd.AddCommand(loadCommand)
	loadCommand.Flags().StringVarP(&jsonConfigFile, "json", "j", "",
		"\n\tConfiguration containing devices, nodes, and clusters, in"+
			"\n\tJSON format.")
	loadCommand.SilenceUsage = true
}

var loadCommand = &cobra.Command{
	Use:     "load",
	Short:   "Add devices to Heketi from a configuration file",
	Long:    "Add devices to Heketi from a configuration file",
	Example: "  $ heketi-cli load --json=topology.json",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check arguments
		if jsonConfigFile == "" {
			return errors.New("Missing configuration file")
		}

		// Load config file
		fp, err := os.Open(jsonConfigFile)
		if err != nil {
			return errors.New("Unable to open config file")
		}
		defer fp.Close()

		configParser := json.NewDecoder(fp)
		var topology ConfigFile
		if err = configParser.Decode(&topology); err != nil {
			return errors.New("Unable to parse config file")
		}
		heketi := client.NewClient(options.Url, options.User, options.Key)
		for _, cluster := range topology.Clusters {

			fmt.Fprintf(stdout, "Creating cluster ... ")
			clusterInfo, err := heketi.ClusterCreate()
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "ID: %v\n", clusterInfo.Id)
			for _, node := range cluster.Nodes {

				fmt.Fprintf(stdout, "\tCreating node %v ... ", node.Node.Hostnames.Manage[0])
				node.Node.ClusterId = clusterInfo.Id
				nodeInfo, err := heketi.NodeAdd(&node.Node)
				if err != nil {
					return err
				}
				fmt.Fprintf(stdout, "ID: %v\n", nodeInfo.Id)

				for _, device := range node.Devices {
					fmt.Fprintf(stdout, "\t\tAdding device %v ... ", device)

					req := &glusterfs.DeviceAddRequest{}
					req.Name = device
					req.NodeId = nodeInfo.Id
					err := heketi.DeviceAdd(req)
					if err != nil {
						return nil
					}

					fmt.Fprintf(stdout, "OK\n")
				}
			}
		}
		return nil
	},
}
