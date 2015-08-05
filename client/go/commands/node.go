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
	"errors"
	"flag"
	"fmt"
	"github.com/lpabon/godbc"
)

type NodeCommand struct {
	Cmd
	cmds    Commands
	options *Options
	cmd     Command
}

//function to create new cluster command
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
		fmt.Println(usageTemplateNode)
	}

	godbc.Ensure(cmd.flags != nil)
	godbc.Ensure(cmd.name == "node")
	return cmd
}

func (a *NodeCommand) Name() string {
	return a.name

}

func (a *NodeCommand) Exec(args []string) error {
	a.flags.Parse(args)

	//check number of args
	if len(a.flags.Args()) < 1 {
		return errors.New("Not enough arguments")
	}

	// Check which of the subcommands we need to call the .Parse function
	for _, cmd := range a.cmds {
		if a.flags.Arg(0) == cmd.Name() {
			err := cmd.Exec(a.flags.Args()[1:])
			if err != nil {
				return err
			}
			a.cmd = cmd

			return nil
		}
	}

	// Done
	return errors.New("Command not found")
}
