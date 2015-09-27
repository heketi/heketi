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
	client "github.com/heketi/heketi/client/api/go-client"
	"github.com/lpabon/godbc"
)

type DeviceDeleteCommand struct {
	Cmd
}

func NewDeviceDeleteCommand(options *Options) *DeviceDeleteCommand {

	godbc.Require(options != nil)

	cmd := &DeviceDeleteCommand{}
	cmd.name = "delete"
	cmd.options = options
	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)

	//usage on -help
	cmd.flags.Usage = func() {
		fmt.Println(`
Deletes a device from Heketi node

USAGE
  heketi-cli [options] device delete [id]

  Where "id" is the id of the cluster

EXAMPLE
  $ heketi-cli device delete 886a86a868711bef83001
`)
	}

	godbc.Ensure(cmd.name == "delete")

	return cmd
}

func (n *DeviceDeleteCommand) Exec(args []string) error {

	//parse args
	n.flags.Parse(args)

	s := n.flags.Args()

	//ensure proper number of args
	if len(s) < 1 {
		return errors.New("Device id missing")
	}

	//set clusterId
	deviceId := n.flags.Arg(0)

	// Create a client
	heketi := client.NewClient(n.options.Url, n.options.User, n.options.Key)

	//set url
	err := heketi.DeviceDelete(deviceId)
	if err == nil {
		fmt.Fprintf(stdout, "Device %v deleted\n", deviceId)
	}

	return err

}
