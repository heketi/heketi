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

type VolumeCommand struct {
	Cmd
}

//function to create new cluster command
func NewVolumeCommand(options *Options) *VolumeCommand {
	godbc.Require(options != nil)

	cmd := &VolumeCommand{}
	cmd.name = "volume"
	cmd.options = options
	cmd.cmds = Commands{
		NewVolumeCreateCommand(options),
		NewVolumeInfoCommand(options),
		NewVolumeListCommand(options),
		NewVolumeDeleteCommand(options),
		NewVolumeExpandCommand(options),
	}

	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)

	//usage on -help
	cmd.flags.Usage = func() {
		fmt.Println(`
Heketi volume management

USAGE
  heketi-cli [options] volume [commands]

COMMANDS
  create    Creates a new volume
  info      Returns information about a specific volume.
  list      List all volumes
  delete    Delete volume
  expand    Expand volume

Use "heketi-cli volume [command] -help" for more information about a command
`)
	}

	godbc.Ensure(cmd.name == "volume")
	return cmd
}
