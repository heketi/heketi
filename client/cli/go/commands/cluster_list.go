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
	"strings"
)

type ClusterListCommand struct {
	Cmd
}

func NewClusterListCommand(options *Options) *ClusterListCommand {

	godbc.Require(options != nil)

	cmd := &ClusterListCommand{}
	cmd.name = "list"
	cmd.options = options
	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)

	//usage on -help
	cmd.flags.Usage = func() {
		fmt.Println(`
Lists the clusters managed by Heketi

USAGE
  heketi-cli [options] cluster list

EXAMPLE
  $ heketi-cli cluster list

`)
	}

	godbc.Ensure(cmd.name == "list")

	return cmd
}

func (c *ClusterListCommand) Exec(args []string) error {

	//parse args
	c.flags.Parse(args)

	// Create a client
	heketi := client.NewClient(c.options.Url, c.options.User, c.options.Key)

	// List clusters
	list, err := heketi.ClusterList()
	if err != nil {
		return err
	}

	if c.options.Json {
		data, err := json.Marshal(list)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, string(data))
	} else {
		output := strings.Join(list.Clusters, "\n")
		fmt.Fprintf(stdout, "Clusters:\n%v\n", output)
	}

	return nil
}
