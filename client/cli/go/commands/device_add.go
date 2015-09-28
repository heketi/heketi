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
	"github.com/heketi/heketi/apps/glusterfs"
	client "github.com/heketi/heketi/client/api/go-client"
	"github.com/lpabon/godbc"
)

type DeviceAddCommand struct {
	Cmd
	device, nodeId string
}

func NewDeviceAddCommand(options *Options) *DeviceAddCommand {
	godbc.Require(options != nil)

	cmd := &DeviceAddCommand{}
	cmd.name = "add"
	cmd.options = options
	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)
	cmd.flags.StringVar(&cmd.device, "name", "", "Name of device to add")
	cmd.flags.StringVar(&cmd.nodeId, "node", "", "Id of the node which has this device")

	//usage on -help
	cmd.flags.Usage = func() {
		fmt.Println(`
Add new device to node to be managed by Heketi

USAGE
  heketi-cli device add [options]

OPTIONS`)

		//print flags
		cmd.flags.PrintDefaults()
		fmt.Println(`
EXAMPLES
  $ heketi-cli device add \
      -name=/dev/sdb
      -node=3e098cb4407d7109806bb196d9e8f095 
`)
	}

	godbc.Ensure(cmd.name == "add")

	return cmd
}

func (d *DeviceAddCommand) Exec(args []string) error {

	// Parse args
	d.flags.Parse(args)

	// Check arguments
	if d.name == "" {
		return errors.New("Missing device name")
	}
	if d.nodeId == "" {
		return errors.New("Missing node id")
	}

	// Create request blob
	req := &glusterfs.DeviceAddRequest{}
	req.Name = d.device
	req.NodeId = d.nodeId
	req.Weight = 100

	// Create a client
	heketi := client.NewClient(d.options.Url, d.options.User, d.options.Key)

	// Add node
	err := heketi.DeviceAdd(req)
	if err != nil {
		return err
	} else {
		fmt.Fprintf(stdout, "Device added successfully\n")
	}

	return nil
}
