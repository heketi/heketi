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
)

type GetClusterInfoCommand struct {
	Cmd
	options *Options
}

func NewGetClusterInfoCommand(options *Options) *GetClusterInfoCommand {

	godbc.Require(options != nil)
	godbc.Require(options.Url != "")

	cmd := &GetClusterInfoCommand{}
	cmd.name = "info"
	cmd.options = options
	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)

	godbc.Ensure(cmd.flags != nil)
	godbc.Ensure(cmd.name == "info")

	return cmd
}

func (a *GetClusterInfoCommand) Name() string {
	return a.name

}

func (a *GetClusterInfoCommand) Exec(args []string) error {
	//parse flags and set id
	a.flags.Parse(args)

	s := a.flags.Args()

	//ensure correct number of args
	if len(s) < 1 {
		return errors.New("Not enough arguments!")
	}
	if len(s) >= 2 {
		return errors.New("Too many arguments!")
	}

	clusterId := a.flags.Arg(0)

	url := a.options.Url

	//do http GET and check if sent to server
	r, err := http.Get(url + "/clusters/" + clusterId)
	if err != nil {
		fmt.Fprintf(stdout, "Error: Unable to send command to server: %v", err)
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

	//check json response
	var body glusterfs.ClusterInfoResponse
	err = utils.GetJsonFromResponse(r, &body)
	if err != nil {
		fmt.Println("Error: Bad json response from server")
		return err
	}

	//print revelent results
	s := "Cluster: " + clusterId + " \n" + "Nodes: \n"
	for _, node := range body.Nodes {
		s += node + "\n"
	}

	s += "Volumes: \n"
	for _, volume := range body.Volumes {
		s += volume + "\n"
	}

	fmt.Fprintf(stdout, s)
	return nil

}
