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
	client "github.com/heketi/heketi/client/api/go-client"
	"github.com/lpabon/godbc"
	"strings"
)

type ClusterInfoCommand struct {
	Cmd
}

func NewClusterInfoCommand(options *Options) *ClusterInfoCommand {

	godbc.Require(options != nil)

	cmd := &ClusterInfoCommand{}
	cmd.name = "info"
	cmd.options = options
	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)

	//usage on -help
	cmd.flags.Usage = func() {
		fmt.Println(`
Retreives information about the cluster

USAGE
  heketi-cli [options] cluster info [id]

  Where "id" is the id of the cluster

EXAMPLE
  $ heketi-cli cluster info 886a86a868711bef83001

`)
	}

	godbc.Ensure(cmd.flags != nil)
	godbc.Ensure(cmd.name == "info")

	return cmd
}

func (c *ClusterInfoCommand) Exec(args []string) error {

	//parse args
	c.flags.Parse(args)

	//ensure proper number of args
	s := c.flags.Args()
	if len(s) < 1 {
		return errors.New("Cluster id missing")
	}

	//set clusterId
	clusterId := c.flags.Arg(0)

	// Create a client to talk to Heketi
	heketi := client.NewClient(c.options.Url, c.options.User, c.options.Key)

	// Create cluster
	info, err := heketi.ClusterInfo(clusterId)
	if err != nil {
		return err
	}

	// Check if JSON should be printed
	if c.options.Json {
		data, err := json.Marshal(info)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, string(data))
	} else {
		fmt.Fprintf(stdout, "Cluster id: %v\n", info.Id)
		fmt.Fprintf(stdout, "Nodes:\n%v", strings.Join(info.Nodes, "\n"))
		fmt.Fprintf(stdout, "\nVolumes:\n%v", strings.Join(info.Volumes, "\n"))
	}

	return nil
}
