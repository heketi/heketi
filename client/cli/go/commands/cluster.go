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
	flag "github.com/spf13/pflag"
	"fmt"
	"github.com/lpabon/godbc"
)

type ClusterCommand struct {
	Cmd
}

//function to create new cluster command
func NewClusterCommand(options *Options) *ClusterCommand {

	//require before we do any work
	godbc.Require(options != nil)

	//create ClusterCommand object
	cmd := &ClusterCommand{}
	cmd.name = "cluster"
	cmd.options = options

	//setup subcommands
	cmd.cmds = Commands{
		NewClusterCreateCommand(options),
		NewClusterInfoCommand(options),
		NewClusterListCommand(options),
		NewClusterDestroyCommand(options),
	}

	//create flags
	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)

	//usage on -help
	cmd.flags.Usage = func() {
		fmt.Println(`
Heketi cluster management

USAGE
  heketi-cli [options] cluster [commands]

COMMANDS
  create   Creates a new cluster for Heketi to manage.
  list     Returns a list of all clusters
  info     Returns information about a specific cluster.
  delete   Delete a cluster

Use "heketi-cli cluster [command] -help" for more information about a command
`)
	}

	//ensure before we return
	godbc.Ensure(cmd.flags != nil)
	godbc.Ensure(cmd.name == "cluster")
	return cmd
}
