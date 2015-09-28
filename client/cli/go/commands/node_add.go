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
)

type NodeAddCommand struct {
	Cmd
	zone               int
	managmentHostNames string
	storageHostNames   string
	clusterId          string
}

func NewNodeAddCommand(options *Options) *NodeAddCommand {

	godbc.Require(options != nil)

	cmd := &NodeAddCommand{}
	cmd.name = "add"
	cmd.options = options
	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)
	cmd.flags.IntVar(&cmd.zone, "zone", -1, "The zone in which the node should reside")
	cmd.flags.StringVar(&cmd.clusterId, "cluster", "", "The cluster in which the node should reside")
	cmd.flags.StringVar(&cmd.managmentHostNames, "management-host-name", "", "Managment host name")
	cmd.flags.StringVar(&cmd.storageHostNames, "storage-host-name", "", "Storage host name")

	//usage on -help
	cmd.flags.Usage = func() {
		fmt.Println(`
Add new node to be managed by Heketi

USAGE
  heketi-cli node add [options]

OPTIONS`)

		//print flags
		cmd.flags.PrintDefaults()
		fmt.Println(`
EXAMPLES
  $ heketi-cli node add \
      -zone=3 \
      -cluster=3e098cb4407d7109806bb196d9e8f095 \
      -managment-host-name=node1-manage.gluster.lab.com \
      -storage-host-name=node1-storage.gluster.lab.com
`)
	}
	godbc.Ensure(cmd.name == "add")

	return cmd
}

func (n *NodeAddCommand) Exec(args []string) error {

	// Parse args
	n.flags.Parse(args)

	// Check arguments
	if n.zone == -1 {
		return errors.New("Missing zone")
	}
	if n.managmentHostNames == "" {
		return errors.New("Missing management hostname")
	}
	if n.storageHostNames == "" {
		return errors.New("Missing management hostname")
	}
	if n.clusterId == "" {
		return errors.New("Missing cluster id")
	}

	// Create request blob
	req := &glusterfs.NodeAddRequest{}
	req.ClusterId = n.clusterId
	req.Hostnames.Manage = []string{n.managmentHostNames}
	req.Hostnames.Storage = []string{n.storageHostNames}
	req.Zone = n.zone

	// Create a client
	heketi := client.NewClient(n.options.Url, n.options.User, n.options.Key)

	// Add node
	node, err := heketi.NodeAdd(req)
	if err != nil {
		return err
	}

	if n.options.Json {
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
}
