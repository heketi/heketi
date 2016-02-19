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

package commands

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/heketi/heketi/apps/glusterfs"
	client "github.com/heketi/heketi/client/api/go-client"
	"github.com/lpabon/godbc"
	"os"
)

type LoadCommand struct {
	Cmd
	jsonConfigFile string
}

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

func NewLoadCommand(options *Options) *LoadCommand {

	//require before we do any work
	godbc.Require(options != nil)

	//create ClusterCommand object
	cmd := &LoadCommand{}
	cmd.name = "load"
	cmd.options = options

	//create flags
	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)
	cmd.flags.StringVar(&cmd.jsonConfigFile, "json", "",
		"\n\tConfiguration containing devices, nodes, and clusters, in"+
			"\n\tJSON format.")

	//usage on -help
	cmd.flags.Usage = func() {
		fmt.Println(`
Add devices to Heketi from a configuration file

USAGE
  heketi-cli load [options]

OPTIONS`)

		//print flags
		cmd.flags.PrintDefaults()
		fmt.Println(`
EXAMPLES
  $ heketi-cli load -json=topology.json 
`)
	}

	godbc.Ensure(cmd.name == "load")

	return cmd
}

func (l *LoadCommand) Exec(args []string) error {

	// Parse args
	l.flags.Parse(args)

	// Check arguments
	if l.jsonConfigFile == "" {
		return errors.New("Missing configuration file")
	}

	// Load config file
	fp, err := os.Open(l.jsonConfigFile)
	if err != nil {
		return errors.New("Unable to open config file")
	}
	defer fp.Close()

	configParser := json.NewDecoder(fp)
	var topology ConfigFile
	if err = configParser.Decode(&topology); err != nil {
		return errors.New("Unable to parse config file")
	}

	heketi := client.NewClient(l.options.Url, l.options.User, l.options.Key)
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

}
