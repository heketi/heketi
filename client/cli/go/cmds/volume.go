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
	"os"
	"strings"

	"github.com/heketi/heketi/apps/glusterfs"
	"github.com/heketi/heketi/client/api/go-client"
	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/api/resource"
	api "k8s.io/kubernetes/pkg/api/v1"
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
	kubePvFile     string
	kubePvEndpoint string
	kubePv         bool
)

func init() {
	RootCmd.AddCommand(VolumeCommand)
	VolumeCommand.AddCommand(VolumeCreateCommand)
	VolumeCommand.AddCommand(VolumeDeleteCommand)
	VolumeCommand.AddCommand(VolumeExpandCommand)
	VolumeCommand.AddCommand(VolumeInfoCommand)
	VolumeCommand.AddCommand(VolumeListCommand)

	VolumeCreateCommand.Flags().IntVar(&size, "size", -1,
		"\n\tSize of volume in GB")
	VolumeCreateCommand.Flags().StringVar(&volname, "name", "",
		"\n\tOptional: Name of volume. Only set if really necessary")
	VolumeCreateCommand.Flags().StringVar(&durability, "durability", "replicate",
		"\n\tOptional: Durability type.  Values are:"+
			"\n\t\tnone: No durability.  Distributed volume only."+
			"\n\t\treplicate: (Default) Distributed-Replica volume."+
			"\n\t\tdisperse: Distributed-Erasure Coded volume.")
	VolumeCreateCommand.Flags().IntVar(&replica, "replica", 3,
		"\n\tReplica value for durability type 'replicate'."+
			"\n\tDefault is 3")
	VolumeCreateCommand.Flags().IntVar(&disperseData, "disperse-data", 4,
		"\n\tOptional: Dispersion value for durability type 'disperse'."+
			"\n\tDefault is 4")
	VolumeCreateCommand.Flags().IntVar(&redundancy, "redundancy", 2,
		"\n\tOptional: Redundancy value for durability type 'disperse'."+
			"\n\tDefault is 2")
	VolumeCreateCommand.Flags().Float64Var(&snapshotFactor, "snapshot-factor", 1.0,
		"\n\tOptional: Amount of storage to allocate for snapshot support."+
			"\n\tMust be greater 1.0.  For example if a 10TiB volume requires 5TiB of"+
			"\n\tsnapshot storage, then snapshot-factor would be set to 1.5.  If the"+
			"\n\tvalue is set to 1, then snapshots will not be enabled for this volume")
	VolumeCreateCommand.Flags().StringVar(&clusters, "clusters", "",
		"\n\tOptional: Comma separated list of cluster ids where this volume"+
			"\n\tmust be allocated. If ommitted, Heketi will allocate the volume"+
			"\n\ton any of the configured clusters which have the available space."+
			"\n\tProviding a set of clusters will ensure Heketi allocates storage"+
			"\n\tfor this volume only in the clusters specified.")
	volumeCreateCommand.Flags().BoolVar(&kubePv, "persistent-volume", false,
		"\n\tOptional: Output to standard out a peristent volume JSON file for OpenShift or"+
			"\n\tKubernetes with the name provided.")
	volumeCreateCommand.Flags().StringVar(&kubePvFile, "persistent-volume-file", "",
		"\n\tOptional: Create a peristent volume JSON file for OpenShift or"+
			"\n\tKubernetes with the name provided.")
	volumeCreateCommand.Flags().StringVar(&kubePvEndpoint, "persistent-volume-endpoint", "",
		"\n\tOptional: Endpoint name for the persistent volume")
	volumeExpandCommand.Flags().IntVar(&expandSize, "expand-size", -1,
		"\n\tAmount in GB to add to the volume")
	VolumeExpandCommand.Flags().StringVar(&id, "volume", "",
		"\n\tId of volume to expand")
	VolumeCreateCommand.SilenceUsage = true
	VolumeDeleteCommand.SilenceUsage = true
	VolumeExpandCommand.SilenceUsage = true
	VolumeInfoCommand.SilenceUsage = true
	VolumeListCommand.SilenceUsage = true
}

var VolumeCommand = &cobra.Command{
	Use:   "volume",
	Short: "Heketi Volume Management",
	Long:  "Heketi Volume Management",
}

var VolumeCreateCommand = &cobra.Command{
	Use:   "create",
	Short: "Create a GlusterFS volume",
	Long:  "Create a GlusterFS volume",
	Example: `  * Create a 100GB replica 3 volume:
      $ heketi-cli volume create --size=100

  * Create a 100GB replica 3 volume specifying two specific clusters:
      $ heketi-cli volume create --size=100 \
        --clusters=0995098e1284ddccb46c7752d142c832,60d46d518074b13a04ce1022c8c7193c

  * Create a 100GB replica 2 volume with 50GB of snapshot storage:
      $ heketi-cli volume create --size=100 --snapshot-factor=1.5 --replica=2

  * Create a 100GB distributed volume
      $ heketi-cli volume create --size=100 --durability=none

  * Create a 100GB erasure coded 4+2 volume with 25GB snapshot storage:
      $ heketi-cli volume create --size=100 --durability=disperse --snapshot-factor=1.25

  * Create a 100GB erasure coded 8+3 volume with 25GB snapshot storage:
      $ heketi-cli volume create --size=100 --durability=disperse --snapshot-factor=1.25 \
        --disperse-data=8 --redundancy=3
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check volume size
		if size == -1 {
			return errors.New("Missing volume size")
		}

		if kubePv && kubePvEndpoint == "" {
			fmt.Fprintf(stderr, "--persistent-volume-endpoint must be provided "+
				"when using --persistent-volume\n")
			return fmt.Errorf("Missing endpoing")
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

		// Check if we need to print out a PV
		if kubePvFile != "" || kubePv {

			// Initialize object
			pv := &api.PersistentVolume{}
			pv.Kind = "PersistentVolume"
			pv.APIVersion = "v1"
			pv.Spec.PersistentVolumeReclaimPolicy = api.PersistentVolumeReclaimRecycle
			pv.Spec.AccessModes = []api.PersistentVolumeAccessMode{
				api.ReadWriteMany,
			}
			pv.Spec.Capacity = make(api.ResourceList)
			pv.Spec.Glusterfs = &api.GlusterfsVolumeSource{}

			// Set values
			pv.ObjectMeta.Name = "glusterfs-" + volume.Id[:8]
			pv.Spec.Capacity[api.ResourceStorage] =
				resource.MustParse(fmt.Sprintf("%vGi", volume.Size))
			pv.Spec.Glusterfs.Path = volume.Name

			// Set endpoint
			if kubePvEndpoint == "" {
				pv.Spec.Glusterfs.EndpointsName = "TYPE ENDPOINT HERE"
			} else {
				pv.Spec.Glusterfs.EndpointsName = kubePvEndpoint
			}

			data, err := json.MarshalIndent(pv, "", "  ")
			if err != nil {
				return err
			}

			if kubePv {
				fmt.Fprintln(stdout, string(data))
			} else {

				f, err := os.Create(kubePvFile)
				if err != nil {
					fmt.Fprintf(stderr, "Unable to write to file %v\n", kubePvFile)
					return err
				}
				f.Write(data)
				f.Close()
			}

		} else {
			if options.Json {
				data, err := json.Marshal(volume)
				if err != nil {
					return err
				}
				fmt.Fprintf(stdout, string(data))
			} else {
				fmt.Fprintf(stdout, "%v", volume)
			}
		}

		return nil
	},
}

var VolumeDeleteCommand = &cobra.Command{
	Use:     "delete",
	Short:   "Deletes the volume",
	Long:    "Deletes the volume",
	Example: "  $ heketi-cli volume delete 886a86a868711bef83001",
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

var VolumeExpandCommand = &cobra.Command{
	Use:   "expand",
	Short: "Expand a volume",
	Long:  "Expand a volume",
	Example: `  * Add 10GB to a volume
    $ heketi-cli volume expand --volume=60d46d518074b13a04ce1022c8c7193c --expand-size=10
`,
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

var VolumeInfoCommand = &cobra.Command{
	Use:     "info",
	Short:   "Retreives information about the volume",
	Long:    "Retreives information about the volume",
	Example: "  $ heketi-cli volume info 886a86a868711bef83001",
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

var VolumeListCommand = &cobra.Command{
	Use:     "list",
	Short:   "Lists the volumes managed by Heketi",
	Long:    "Lists the volumes managed by Heketi",
	Example: "  $ heketi-cli volume list",
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
