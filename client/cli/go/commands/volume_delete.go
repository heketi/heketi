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
	"fmt"

	client "github.com/heketi/heketi/client/api/go-client"
	"github.com/lpabon/godbc"
	flag "github.com/spf13/pflag"
)

type VolumeDeleteCommand struct {
	Cmd
}

func NewVolumeDeleteCommand(options *Options) *VolumeDeleteCommand {

	godbc.Require(options != nil)

	cmd := &VolumeDeleteCommand{}
	cmd.name = "delete"
	cmd.options = options
	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)

	//usage on -help
	cmd.flags.Usage = func() {
		fmt.Println(`
Deletes the volume

USAGE
  heketi-cli [options] volume delete [id]

  Where "id" is the id of the volume to be deleted

EXAMPLE
  $ heketi-cli volume delete 886a86a868711bef83001

`)
	}

	godbc.Ensure(cmd.flags != nil)
	godbc.Ensure(cmd.name == "delete")

	return cmd
}

func (c *VolumeDeleteCommand) Exec(args []string) error {

	//parse args
	c.flags.Parse(args)

	s := c.flags.Args()

	//ensure proper number of args
	if len(s) < 1 {
		return errors.New("Volume id missing")
	}

	//set volumeId
	volumeId := c.flags.Arg(0)

	// Create a client
	heketi := client.NewClient(c.options.Url, c.options.User, c.options.Key)

	//set url
	err := heketi.VolumeDelete(volumeId)
	if err == nil {
		fmt.Fprintf(stdout, "Volume %v deleted\n", volumeId)
	}

	return err
}
