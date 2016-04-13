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

package cmds

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/heketi/heketi/apps/glusterfs"
	"github.com/heketi/heketi/client/api/go-client"
	"github.com/spf13/cobra"
)

var (
	size           int
	volname        string
	durability     string
	replica        int
	disperseData   int
	redundancy     int
	snapshotFactor float64
	clusters       string
	expandSize     int
	id             string
)

func init() {
	RootCmd.AddCommand(volumeCommand)
	volumeCommand.AddCommand(volumeCreateCommand)
	volumeCommand.AddCommand(volumeDeleteCommand)
	volumeCommand.AddCommand(volumeExpandCommand)
	volumeCommand.AddCommand(volumeInfoCommand)
	volumeCommand.AddCommand(volumeListCommand)

	volumeCreateCommand.Flags().IntVar(&size, "size", -1,
		"\n\tSize of volume in GB")
	volumeCreateCommand.Flags().StringVar(&volname, "name", "",
		"\n\tOptional: Name of volume. Only set if really necessary")
	volumeCreateCommand.Flags().StringVar(&durability, "durability", "replicate",
		"\n\tOptional: Durability type.  Values are:"+
			"\n\t\tnone: No durability.  Distributed volume only."+
			"\n\t\treplicate: (Default) Distributed-Replica volume."+
			"\n\t\tdisperse: Distributed-Erasure Coded volume.")
	volumeCreateCommand.Flags().IntVar(&replica, "replica", 3,
		"\n\tReplica value for durability type 'replicate'."+
			"\n\tDefault is 3")
	volumeCreateCommand.Flags().IntVar(&disperseData, "disperse-data", 4,
		"\n\tOptional: Dispersion value for durability type 'disperse'."+
			"\n\tDefault is 4")
	volumeCreateCommand.Flags().IntVar(&redundancy, "redundancy", 2,
		"\n\tOptional: Redundancy value for durability type 'disperse'."+
			"\n\tDefault is 2")
	volumeCreateCommand.Flags().Float64Var(&snapshotFactor, "snapshot-factor", 1.0,
		"\n\tOptional: Amount of storage to allocate for snapshot support."+
			"\n\tMust be greater 1.0.  For example if a 10TiB volume requires 5TiB of"+
			"\n\tsnapshot storage, then snapshot-factor would be set to 1.5.  If the"+
			"\n\tvalue is set to 1, then snapshots will not be enabled for this volume")
	volumeCreateCommand.Flags().StringVar(&clusters, "clusters", "",
		"\n\tOptional: Comma separated list of cluster ids where this volume"+
			"\n\tmust be allocated. If ommitted, Heketi will allocate the volume"+
			"\n\ton any of the configured clusters which have the available space."+
			"\n\tProviding a set of clusters will ensure Heketi allocates storage"+
			"\n\tfor this volume only in the clusters specified.")

	volumeExpandCommand.Flags().IntVar(&expandSize, "expand-size", -1,
		"\n\tAmount in GB to add to the volume")
	volumeExpandCommand.Flags().StringVar(&id, "volume", "",
		"\n\tId of volume to expand")
}

var volumeCommand = &cobra.Command{
	Use:   "volume",
	Short: "Heketi Volume Management",
	Long:  "Heketi Volume Management",
}

var volumeCreateCommand = &cobra.Command{
	Use:   "create",
	Short: "Create a GlusterFS volume",
	Long:  "Create a GlusterFS volume",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check volume size
		if size == -1 {
			return errors.New("Missing volume size")
		}

		// Check clusters
		var clusters_ []string
		if clusters != "" {
			clusters_ = strings.Split(clusters, ",")
		}

		// Create request blob
		req := &glusterfs.VolumeCreateRequest{}
		req.Size = size
		req.Clusters = clusters_
		req.Durability.Type = durability
		req.Durability.Replicate.Replica = replica
		req.Durability.Disperse.Data = disperseData
		req.Durability.Disperse.Redundancy = redundancy

		if volname != "" {
			req.Name = volname
		}

		if snapshotFactor > 1.0 {
			req.Snapshot.Factor = float32(snapshotFactor)
			req.Snapshot.Enable = true
		}

		// Create a client
		heketi := client.NewClient(options.Url, options.User, options.Key)

		// Add volume
		volume, err := heketi.VolumeCreate(req)
		if err != nil {
			return err
		}

		if options.Json {
			data, err := json.Marshal(volume)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, string(data))
		} else {
			fmt.Fprintf(stdout, "%v", volume)
		}
		return nil
	},
}

var volumeDeleteCommand = &cobra.Command{
	Use:   "delete",
	Short: "Deletes the volume",
	Long:  "Deletes the volume",
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Flags().Args()

		//ensure proper number of args
		if len(s) < 1 {
			return errors.New("Volume id missing")
		}

		//set volumeId
		volumeId := cmd.Flags().Arg(0)

		// Create a client
		heketi := client.NewClient(options.Url, options.User, options.Key)

		//set url
		err := heketi.VolumeDelete(volumeId)
		if err == nil {
			fmt.Fprintf(stdout, "Volume %v deleted\n", volumeId)
		}

		return err
	},
}

var volumeExpandCommand = &cobra.Command{
	Use:   "expand",
	Short: "Expand a volume",
	Long:  "Expand a volume",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check volume size
		if expandSize == -1 {
			return errors.New("Missing volume amount to expand")
		}

		if id == "" {
			return errors.New("Missing volume id")
		}

		// Create request
		req := &glusterfs.VolumeExpandRequest{}
		req.Size = expandSize

		// Create client
		heketi := client.NewClient(options.Url, options.User, options.Key)

		// Expand volume
		volume, err := heketi.VolumeExpand(id, req)
		if err != nil {
			return err
		}

		if options.Json {
			data, err := json.Marshal(volume)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, string(data))
		} else {
			fmt.Fprintf(stdout, "%v", volume)
		}
		return nil
	},
}

var volumeInfoCommand = &cobra.Command{
	Use:   "info",
	Short: "Retreives information about the volume",
	Long:  "Retreives information about the volume",
	RunE: func(cmd *cobra.Command, args []string) error {
		//ensure proper number of args
		s := cmd.Flags().Args()
		if len(s) < 1 {
			return errors.New("Volume id missing")
		}

		// Set volume id
		volumeId := cmd.Flags().Arg(0)

		// Create a client to talk to Heketi
		heketi := client.NewClient(options.Url, options.User, options.Key)

		// Create cluster
		info, err := heketi.VolumeInfo(volumeId)
		if err != nil {
			return err
		}

		if options.Json {
			data, err := json.Marshal(info)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, string(data))
		} else {
			fmt.Fprintf(stdout, "%v", info)
		}
		return nil

	},
}

var volumeListCommand = &cobra.Command{
	Use:   "list",
	Short: "Lists the volumes managed by Heketi",
	Long:  "Lists the volumes managed by Heketi",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create a client
		heketi := client.NewClient(options.Url, options.User, options.Key)

		// List volumes
		list, err := heketi.VolumeList()
		if err != nil {
			return err
		}

		if options.Json {
			data, err := json.Marshal(list)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, string(data))
		} else {
			output := strings.Join(list.Volumes, "\n")
			fmt.Fprintf(stdout, "Volumes:\n%v\n", output)
		}

		return nil
	},
}
