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
)

type DeviceInfoCommand struct {
	Cmd
}

func NewDeviceInfoCommand(options *Options) *DeviceInfoCommand {
	godbc.Require(options != nil)

	cmd := &DeviceInfoCommand{}
	cmd.name = "info"
	cmd.options = options
	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)

	//usage on -help
	cmd.flags.Usage = func() {
		fmt.Println(`
Retreives information about the device

USAGE
  heketi-cli [options] node device [id]

  Where "id" is the id of the device 

EXAMPLE
  $ heketi-cli node info 886a86a868711bef83001
`)
	}

	godbc.Ensure(cmd.name == "info")

	return cmd
}

func (d *DeviceInfoCommand) Exec(args []string) error {

	d.flags.Parse(args)

	//ensure proper number of args
	s := d.flags.Args()
	if len(s) < 1 {
		return errors.New("Device id missing")
	}

	// Set node id
	deviceId := d.flags.Arg(0)

	// Create a client to talk to Heketi
	heketi := client.NewClient(d.options.Url, d.options.User, d.options.Key)

	// Create cluster
	info, err := heketi.DeviceInfo(deviceId)
	if err != nil {
		return err
	}

	if d.options.Json {
		data, err := json.Marshal(info)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, string(data))
	} else {
		fmt.Fprintf(stdout, "Device Id: %v\n"+
			"Name: %v\n"+
			"Size (GiB): %v\n"+
			"Used (GiB): %v\n"+
			"Free (GiB): %v\n",
			info.Id,
			info.Name,
			info.Storage.Total/(1024*1024),
			info.Storage.Used/(1024*1024),
			info.Storage.Free/(1024*1024))

		fmt.Fprintf(stdout, "Bricks:\n")
		for _, d := range info.Bricks {
			fmt.Fprintf(stdout, "Id:%-35v"+
				"Size (GiB):%-8v"+
				"Path: %v\n",
				d.Id,
				d.Size/(1024*1024),
				d.Path)
		}
	}
	return nil

}
