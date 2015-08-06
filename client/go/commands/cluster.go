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

type ClusterCommand struct {
	Cmd
	cmds    Commands
	options *Options
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
		NewCreateNewClusterCommand(options),
		NewGetClusterInfoCommand(options),
		NewGetClusterListCommand(options),
		NewDestroyClusterCommand(options),
	}

	//create flags
	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)

	//usage on -help
	cmd.flags.Usage = func() {
		fmt.Println(usageTemplateCluster)
	}

	//ensure before we return
	godbc.Ensure(cmd.flags != nil)
	godbc.Ensure(cmd.name == "cluster")
	return cmd
}

func (a *ClusterCommand) Name() string {
	return a.name

}

func (a *ClusterCommand) Exec(args []string) error {
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
			return nil
		}
	}

	// Done
	return errors.New("Command not found")
}
