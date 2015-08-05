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
	"github.com/heketi/heketi/utils"
	"github.com/lpabon/godbc"
	"net/http"
	"os"
	"strconv"
)

type GetNodeInfoCommand struct {
	Cmd
	options *Options
	nodeId  string
}

func NewNodeInfoCommand(options *Options) *GetNodeInfoCommand {

	godbc.Require(options != nil)

	cmd := &GetNodeInfoCommand{}
	cmd.name = "info"
	cmd.options = options
	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)

	//usage on -help
	cmd.flags.Usage = func() {
		fmt.Println(usageTemplateNodeInfo)
	}

	godbc.Ensure(cmd.flags != nil)
	godbc.Ensure(cmd.name == "info")

	return cmd
}

func (a *GetNodeInfoCommand) Name() string {
	return a.name

}

func (a *GetNodeInfoCommand) Exec(args []string) error {

	a.flags.Parse(args)

	//ensure we have Url
	if a.options.Url == "" {
		fmt.Fprintf(stdout, "You need a server!\n")
		os.Exit(1)
	}

	if len(args) < 1 {
		return errors.New("Not enough arguments!")
	}
	if len(args) >= 2 {
		return errors.New("Too many arguments!")
	}
	a.nodeId = a.flags.Arg(0)
	url := a.options.Url

	//do http GET and check if sent to server
	r, err := http.Get(url + "/nodes/" + a.nodeId)
	if err != nil {
		fmt.Fprintf(stdout, "Unable to send command to server: %v", err)
		return err
	}

	//check status code
	if r.StatusCode != http.StatusOK {
		s, err := utils.GetStringFromResponse(r)
		if err != nil {
			return err
		}
		return errors.New(s)
	}

	if a.options.Json {
		// Print JSON body
		s, err := utils.GetStringFromResponse(r)
		if err != nil {
			return err
		}
		fmt.Fprint(stdout, s)
	} else {

		//check json response
		var body glusterfs.NodeInfoResponse
		err = utils.GetJsonFromResponse(r, &body)
		if err != nil {
			fmt.Println("Error: Bad json response from server")
			return err
		}
		//print revelent results
		s := "Node: " + a.nodeId + "\n\nZone: " + strconv.Itoa(body.Zone) + "\n\nCluster: " + body.ClusterId + "\n\nManage hostnames:\n"
		for _, hostname := range body.Hostnames.Manage {
			s += hostname + "\n"
		}
		s += "\nStorage hostnames:\n"
		for _, hostname := range body.Hostnames.Storage {
			s += hostname + "\n"
		}
		s += "\nDevices:\n"
		for _, device := range body.DevicesInfo {
			s += device.Name + "\n"

		}

		fmt.Fprintf(stdout, s)
	}
	return nil

}
