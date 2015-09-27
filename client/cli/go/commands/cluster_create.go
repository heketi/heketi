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
	"flag"
	"fmt"
	client "github.com/heketi/heketi/client/api/go-client"
	"github.com/lpabon/godbc"
)

type ClusterCreateCommand struct {
	Cmd
}

func NewClusterCreateCommand(options *Options) *ClusterCreateCommand {

	godbc.Require(options != nil)

	cmd := &ClusterCreateCommand{}
	cmd.name = "create"
	cmd.options = options

	// Set flags
	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)

	//usage on -help
	cmd.flags.Usage = func() {
		fmt.Println(`
Create a cluster

A cluster is used to group a collection of nodes.  It also provides
the caller with the choice to specify clusters where volumes should
be created.

USAGE
  heketi-cli [options] cluster create

EXAMPLE
  $ heketi-cli cluster create

`)
	}

	godbc.Ensure(cmd.name == "create")

	return cmd
}

func (c *ClusterCreateCommand) Exec(args []string) error {

	//parse args
	c.flags.Parse(args)

	// Create a client to talk to Heketi
	heketi := client.NewClient(c.options.Url, c.options.User, c.options.Key)

	// Create cluster
	cluster, err := heketi.ClusterCreate()
	if err != nil {
		return err
	}

	// Check if JSON should be printed
	if c.options.Json {
		data, err := json.Marshal(cluster)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, string(data))
	} else {
		fmt.Fprintf(stdout, "Cluster id: %v", cluster.Id)
	}

	return nil
}
