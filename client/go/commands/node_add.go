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
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/heketi/heketi/apps/glusterfs"
	"github.com/heketi/heketi/utils"
	"github.com/lpabon/godbc"
	"net/http"
	"strconv"
	"time"
)

type NodeAddCommand struct {
	Cmd
	options            *Options
	zone               int
	managmentHostNames string
	storageHostNames   string
	clusterId          string
}

func NewNodeAddCommand(options *Options) *NodeAddCommand {

	godbc.Require(options != nil)

	cmd := &NodeAddCommand{}
	cmd.name = "add"
	cmd.options = options
	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)
	cmd.flags.IntVar(&cmd.zone, "zone", 0, "The zone in which the node should reside")
	cmd.flags.StringVar(&cmd.clusterId, "cluster", "", "The cluster in which the node should reside")
	cmd.flags.StringVar(&cmd.managmentHostNames, "managment-host-name", "", "Managment host name")
	cmd.flags.StringVar(&cmd.storageHostNames, "storage-host-name", "", "Storage host name")

	//usage on -help
	cmd.flags.Usage = func() {
		fmt.Println(`
Node add is a command used for adding a node to the specified cluster.

USAGE
	heketi node add [options]

OPTIONS`)

		//print flags
		cmd.flags.PrintDefaults()
		fmt.Println(`
EXAMPLES
	Add a node to heketi
		$ heketi -server http://localhost:8080 node add \
	  		 -zone 3 \
	  		 -cluster 3e098cb4407d7109806bb196d9e8f095 \
	  		 -managment-host-name node1-manage.gluster.lab.com \
	  		 -storage-host-name node1-storage.gluster.lab.com`)
	}
	godbc.Ensure(cmd.flags != nil)
	godbc.Ensure(cmd.name == "add")

	return cmd
}

func (a *NodeAddCommand) Name() string {
	return a.name

}

func (a *NodeAddCommand) Exec(args []string) error {

	//parse args
	a.flags.Parse(args)

	//ensure we have Url
	if a.options.Url == "" {
		return errors.New("You need a server!\n")
	}

	s := a.flags.Args()
	if len(s) != 0 {
		return errors.New("Too many arguments!")
	}
	//set url
	url := a.options.Url

	//create request blob
	requestBlob := glusterfs.NodeAddRequest{}
	requestBlob.ClusterId = a.clusterId
	requestBlob.Hostnames.Manage = []string{a.managmentHostNames}
	requestBlob.Hostnames.Storage = []string{a.storageHostNames}
	requestBlob.Zone = a.zone

	//marshal blob to get []byte to be Post'd
	request, err := json.Marshal(requestBlob)
	if err != nil {
		return errors.New("json marshaling did not work")
	}
	//do Post and check if sent to server
	r, err := http.Post(url+"/nodes", "application/json", bytes.NewBuffer(request))
	if err != nil {
		fmt.Fprintf(stdout, "Error: Unable to send command to server: %v", err)
		return err
	}

	//check status code
	if r.StatusCode != http.StatusAccepted {
		return utils.GetErrorFromResponse(r)
	}

	//Query queue until finished
	location, err := r.Location()
	for {
		r, err := http.Get(location.String())
		if err != nil {
			return err
		}
		if r.Header.Get("X-Pending") == "true" {
			if r.StatusCode == http.StatusOK {
				time.Sleep(time.Millisecond * 3)
				continue
			} else {
				return utils.GetErrorFromResponse(r)
			}
		} else {
			if r.StatusCode == http.StatusOK {
				if a.options.Json {
					s, err := utils.GetStringFromResponse(r)
					if err != nil {
						return err
					}
					fmt.Fprint(stdout, s)
				} else {
					var body glusterfs.NodeInfoResponse
					err = utils.GetJsonFromResponse(r, &body)
					zone := strconv.Itoa(body.Zone)
					fmt.Fprintf(stdout, `Successfully created node!
ZONE:
	%v

CLUSTER ID:
	%v

MANAGMENT HOSTNAME:
	%v

STORAGE HOSTNAME:
	%v

NODE ID:
	%v
`, zone, body.ClusterId, body.Hostnames.Manage[0], body.Hostnames.Storage[0], body.Id)
				}
				break
			} else {
				return utils.GetErrorFromResponse(r)
			}
		}
	}
	return nil
}
