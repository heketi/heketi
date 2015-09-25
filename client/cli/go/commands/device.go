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

type DeviceCommand struct {
	Cmd
}

//function to create new cluster command
func NewDeviceCommand(options *Options) *DeviceCommand {

	//require before we do any work
	godbc.Require(options != nil)

	//create DeviceCommand object
	cmd := &DeviceCommand{}
	cmd.name = "device"
	cmd.options = options

	//setup subcommands
	cmd.cmds = Commands{
		NewDeviceAddCommand(options),
		NewDeviceInfoCommand(options),
		NewDeviceDeleteCommand(options),
	}

	//create flags
	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)

	//usage on -help
	cmd.flags.Usage = func() {
		fmt.Println(`
Heketi device management

USAGE
  heketi-cli [options] device [commands]

COMMANDS
  add      Adds a new raw device
  info     Information about a device
  delete   Delete device from being managed by Heketi

Use "heketi-cli device [command] -help" for more information about a command
`)
	}

	//ensure before we return
	godbc.Ensure(cmd.name == "device")
	return cmd
}
