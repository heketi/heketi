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
	"flag"
	"fmt"
	"github.com/lpabon/godbc"
)

type NodeCommand struct {
	Cmd
}

//function to create new node command
func NewNodeCommand(options *Options) *NodeCommand {
	godbc.Require(options != nil)

	cmd := &NodeCommand{}
	cmd.name = "node"
	cmd.options = options
	cmd.cmds = Commands{
		NewNodeAddCommand(options),
		NewNodeInfoCommand(options),
		NewNodeDestroyCommand(options),
	}

	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)

	//usage on -help
	cmd.flags.Usage = func() {
		fmt.Println(`
Heketi node management

USAGE
  heketi-cli [options] node [commands]

COMMANDS
  add     Adds a node for Heketi to manage.
  info    Returns information about a specific node.
  delete  Delete node with specified id. 

Use "heketi-cli node [command] -help" for more information about a command
`)
	}

	godbc.Ensure(cmd.name == "node")
	return cmd
}
