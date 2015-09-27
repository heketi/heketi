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
	"github.com/heketi/heketi/apps/glusterfs"
	client "github.com/heketi/heketi/client/api/go-client"
	"github.com/lpabon/godbc"
)

type VolumeExpandCommand struct {
	Cmd
	expand_size int
	id          string
}

func NewVolumeExpandCommand(options *Options) *VolumeExpandCommand {

	godbc.Require(options != nil)

	cmd := &VolumeExpandCommand{}
	cmd.name = "expand"
	cmd.options = options
	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)
	cmd.flags.IntVar(&cmd.expand_size, "expand-size", -1,
		"\n\tAmount in GB to add to the volume")
	cmd.flags.StringVar(&cmd.id, "volume", "",
		"\n\tId of volume to expand")

	//usage on -help
	cmd.flags.Usage = func() {
		fmt.Println(`
Expand a volume

USAGE
  heketi volume expand [options]

OPTIONS`)

		//print flags
		cmd.flags.PrintDefaults()
		fmt.Println(`
EXAMPLES
  * Add 10GB to a volume
      $ heketi volume expand -volume=60d46d518074b13a04ce1022c8c7193c -expand-size=10
`)
	}
	godbc.Ensure(cmd.name == "expand")

	return cmd
}

func (v *VolumeExpandCommand) Exec(args []string) error {

	// Parse args
	v.flags.Parse(args)

	// Check volume size
	if v.expand_size == -1 {
		return errors.New("Missing volume amount to expand")
	}

	if v.id == "" {
		return errors.New("Missing volume id")
	}

	// Create request
	req := &glusterfs.VolumeExpandRequest{}
	req.Size = v.expand_size

	// Create client
	heketi := client.NewClient(v.options.Url, v.options.User, v.options.Key)

	// Expand volume
	volume, err := heketi.VolumeExpand(v.id, req)
	if err != nil {
		return err
	}

	if v.options.Json {
		data, err := json.Marshal(volume)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, string(data))
	} else {
		fmt.Fprintf(stdout, "%v", volume)
	}
	return nil
}
