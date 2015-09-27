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

type VolumeInfoCommand struct {
	Cmd
}

func NewVolumeInfoCommand(options *Options) *VolumeInfoCommand {

	godbc.Require(options != nil)

	cmd := &VolumeInfoCommand{}
	cmd.name = "info"
	cmd.options = options
	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)

	//usage on -help
	cmd.flags.Usage = func() {
		fmt.Println(`
Retreives information about the volume 

USAGE
  heketi-cli [options] volume info [id]

  Where "id" is the id of the volume

EXAMPLE
  $ heketi-cli volume info 886a86a868711bef83001
`)
	}

	godbc.Ensure(cmd.flags != nil)
	godbc.Ensure(cmd.name == "info")

	return cmd
}

func (n *VolumeInfoCommand) Exec(args []string) error {

	n.flags.Parse(args)

	//ensure proper number of args
	s := n.flags.Args()
	if len(s) < 1 {
		return errors.New("Volume id missing")
	}

	// Set volume id
	volumeId := n.flags.Arg(0)

	// Create a client to talk to Heketi
	heketi := client.NewClient(n.options.Url, n.options.User, n.options.Key)

	// Create cluster
	info, err := heketi.VolumeInfo(volumeId)
	if err != nil {
		return err
	}

	if n.options.Json {
		data, err := json.Marshal(info)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, string(data))
	} else {
		fmt.Fprintf(stdout, "%v", info)
	}
	return nil

}
