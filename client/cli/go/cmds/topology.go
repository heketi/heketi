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

const (
	DURABILITY_STRING_REPLICATE       = "replicate"
	DURABILITY_STRING_DISTRIBUTE_ONLY = "none"
	DURABILITY_STRING_EC              = "disperse"
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
	RootCmd.AddCommand(topologyCommand)
	topologyCommand.AddCommand(topologyLoadCommand)
	topologyCommand.AddCommand(topologyInfoCommand)
	topologyLoadCommand.Flags().StringVarP(&jsonConfigFile, "json", "j", "",
		"\n\tConfiguration containing devices, nodes, and clusters, in"+
			"\n\tJSON format.")
	topologyLoadCommand.SilenceUsage = true
	topologyInfoCommand.SilenceUsage = true
}

var topologyCommand = &cobra.Command{
	Use:   "topology",
	Short: "Heketi Topology Management",
	Long:  "Heketi Topology management",
}

var topologyLoadCommand = &cobra.Command{
	Use:     "load",
	Short:   "Add devices to Heketi from a configuration file",
	Long:    "Add devices to Heketi from a configuration file",
	Example: " $ heketi-cli topology load --json=topo.json",
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

var topologyInfoCommand = &cobra.Command{
	Use:     "info",
	Short:   "Retreives information about the current Topology",
	Long:    "Retreives information about the current Topology",
	Example: " $ heketi-cli topology info",
	RunE: func(cmd *cobra.Command, args []string) error {

		// Create a client to talk to Heketi
		heketi := client.NewClient(options.Url, options.User, options.Key)

		// Create Topology
		topoinfo, err := heketi.TopologyInfo()
		if err != nil {
			return err
		}

		// Check if JSON should be printed
		if options.Json {
			data, err := json.Marshal(topoinfo)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, string(data))
		} else {

			// Get the cluster list and iterate over
			for i, _ := range topoinfo.ClusterList {
				fmt.Fprintf(stdout, "\nCluster Id: %v\n", topoinfo.ClusterList[i].Id)
				fmt.Fprintf(stdout, "\n    %s\n", "Volumes:")
				for k, _ := range topoinfo.ClusterList[i].Volumes {

					// Format and print volumeinfo  on this cluster
					v := topoinfo.ClusterList[i].Volumes[k]
					s := fmt.Sprintf("\n\tName: %v\n"+
						"\tSize: %v\n"+
						"\tId: %v\n"+
						"\tCluster Id: %v\n"+
						"\tMount: %v\n"+
						"\tMount Options: backup-volfile-servers=%v\n"+
						"\tDurability Type: %v\n",
						v.Name,
						v.Size,
						v.Id,
						v.Cluster,
						v.Mount.GlusterFS.MountPoint,
						v.Mount.GlusterFS.Options["backup-volfile-servers"],
						v.Durability.Type)

					switch v.Durability.Type {
					case DURABILITY_STRING_EC:
						s += fmt.Sprintf("\tDisperse Data: %v\n"+
							"\tDisperse Redundancy: %v\n",
							v.Durability.Disperse.Data,
							v.Durability.Disperse.Redundancy)
					case DURABILITY_STRING_REPLICATE:
						s += fmt.Sprintf("\tReplica: %v\n",
							v.Durability.Replicate.Replica)
					}
					if v.Snapshot.Enable {
						s += fmt.Sprintf("\tSnapshot: Enabled\n"+
							"\tSnapshot Factor: %.2f\n",
							v.Snapshot.Factor)
					} else {
						s += "\tSnapshot: Disabled\n"
					}
					s += "\n\t\tBricks:\n"
					for _, b := range v.Bricks {
						s += fmt.Sprintf("\t\t\tId: %v\n"+
							"\t\t\tPath: %v\n"+
							"\t\t\tSize (GiB): %v\n"+
							"\t\t\tNode: %v\n"+
							"\t\t\tDevice: %v\n\n",
							b.Id,
							b.Path,
							b.Size/(1024*1024),
							b.NodeId,
							b.DeviceId)
					}
					fmt.Fprintf(stdout, "%s", s)
				}

				// format and print each Node information on this cluster
				fmt.Fprintf(stdout, "\n    %s\n", "Nodes:")
				for j, _ := range topoinfo.ClusterList[i].Nodes {
					info := topoinfo.ClusterList[i].Nodes[j]
					fmt.Fprintf(stdout, "\n\tNode Id: %v\n"+
						"\tCluster Id: %v\n"+
						"\tZone: %v\n"+
						"\tManagement Hostname: %v\n"+
						"\tStorage Hostname: %v\n",
						info.Id,
						info.ClusterId,
						info.Zone,
						info.Hostnames.Manage[0],
						info.Hostnames.Storage[0])
					fmt.Fprintf(stdout, "\tDevices:\n")

					// format and print the device info
					for j, d := range info.DevicesInfo {
						fmt.Fprintf(stdout, "\t\tId:%-35v"+
							"Name:%-20v"+
							"Size (GiB):%-8v"+
							"Used (GiB):%-8v"+
							"Free (GiB):%-8v\n",
							d.Id,
							d.Name,
							d.Storage.Total/(1024*1024),
							d.Storage.Used/(1024*1024),
							d.Storage.Free/(1024*1024))

						// format and print the brick information
						fmt.Fprintf(stdout, "\t\t\tBricks:\n")
						for _, d := range info.DevicesInfo[j].Bricks {
							fmt.Fprintf(stdout, "\t\t\t\tId:%-35v"+
								"Size (GiB):%-8v"+
								"Path: %v\n",
								d.Id,
								d.Size/(1024*1024),
								d.Path)
						}
					}
				}
			}

		}

		return nil
	},
}
