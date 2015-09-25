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
	"strings"
)

type VolumeCreateCommand struct {
	Cmd
	size            int
	volname         string
	durability      string
	replica         int
	disperse_data   int
	redundancy      int
	snapshot_factor float64
	clusters        string
}

func NewVolumeCreateCommand(options *Options) *VolumeCreateCommand {

	godbc.Require(options != nil)

	cmd := &VolumeCreateCommand{}
	cmd.name = "create"
	cmd.options = options
	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)
	cmd.flags.IntVar(&cmd.size, "size", -1,
		"\n\tSize of volume in GB")
	cmd.flags.StringVar(&cmd.volname, "name", "",
		"\n\tOptional: Name of volume. Only set if really necessary")
	cmd.flags.StringVar(&cmd.durability, "durability", "replicate",
		"\n\tOptional: Durability type.  Values are:"+
			"\n\t\tnone: No durability.  Distributed volume only."+
			"\n\t\treplicate: (Default) Distributed-Replica volume."+
			"\n\t\tdisperse: Distributed-Erasure Coded volume.")
	cmd.flags.IntVar(&cmd.replica, "replica", 3,
		"\n\tReplica value for durability type 'replicate'."+
			"\n\tDefault is 3")
	cmd.flags.IntVar(&cmd.disperse_data, "disperse-data", 4,
		"\n\tOptional: Dispersion value for durability type 'disperse'."+
			"\n\tDefault is 4")
	cmd.flags.IntVar(&cmd.redundancy, "redundancy", 2,
		"\n\tOptional: Redundancy value for durability type 'disperse'."+
			"\n\tDefault is 2")
	cmd.flags.Float64Var(&cmd.snapshot_factor, "snapshot-factor", 1.0,
		"\n\tOptional: Amount of storage to allocate for snapshot support."+
			"\n\tMust be greater 1.0.  For example if a 10TiB volume requires 5TiB of"+
			"\n\tsnapshot storage, then snapshot-factor would be set to 1.5.  If the"+
			"\n\tvalue is set to 1, then snapshots will not be enabled for this volume")
	cmd.flags.StringVar(&cmd.clusters, "clusters", "",
		"\n\tOptional: Comma separated list of cluster ids where this volume"+
			"\n\tmust be allocated. If ommitted, Heketi will allocate the volume"+
			"\n\ton any of the configured clusters which have the available space."+
			"\n\tProviding a set of clusters will ensure Heketi allocates storage"+
			"\n\tfor this volume only in the clusters specified.")

	//usage on -help
	cmd.flags.Usage = func() {
		fmt.Println(`
Create a GlusterFS volume

USAGE
  heketi volume create [options]

OPTIONS`)

		//print flags
		cmd.flags.PrintDefaults()
		fmt.Println(`
EXAMPLES
  * Create a 100GB replica 3 volume:
      $ heketi volume create -size=100

  * Create a 100GB replica 3 volume specifying two specific clusters:
      $ heketi volume create -size=100 \
          -clusters=0995098e1284ddccb46c7752d142c832,60d46d518074b13a04ce1022c8c7193c

  * Create a 100GB replica 2 volume with 50GB of snapshot storage:
      $ heketi volume create -size=100 -snapshot-factor=1.5 -replica=2 

  * Create a 100GB distributed volume
      $ heketi volume create -size=100 -durabilty=none

  * Create a 100GB erasure coded 4+2 volume with 25GB snapshot storage:
      $ heketi volume create -size=100 -durability=disperse -snapshot-factor=1.25

  * Create a 100GB erasure coded 8+3 volume with 25GB snapshot storage:
      $ heketi volume create -size=100 -durability=disperse -snapshot-factor=1.25 \
          -disperse-data=8 -redundancy=3
`)
	}
	godbc.Ensure(cmd.name == "create")

	return cmd
}

func (v *VolumeCreateCommand) Exec(args []string) error {

	// Parse args
	v.flags.Parse(args)

	// Check volume size
	if v.size == -1 {
		return errors.New("Missing volume size")
	}

	// Check clusters
	var clusters []string
	if v.clusters != "" {
		clusters = strings.Split(v.clusters, ",")
	}

	// Create request blob
	req := &glusterfs.VolumeCreateRequest{}
	req.Size = v.size
	req.Clusters = clusters
	req.Durability.Type = v.durability
	req.Durability.Replicate.Replica = v.replica
	req.Durability.Disperse.Data = v.disperse_data
	req.Durability.Disperse.Redundancy = v.redundancy

	if v.volname != "" {
		req.Name = v.volname
	}

	if v.snapshot_factor > 1.0 {
		req.Snapshot.Factor = float32(v.snapshot_factor)
		req.Snapshot.Enable = true
	}

	// Create a client
	heketi := client.NewClient(v.options.Url, v.options.User, v.options.Key)

	// Add volume
	volume, err := heketi.VolumeCreate(req)
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
