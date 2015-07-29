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
	"net/http"
)

type GetClusterInfoCommand struct {
	Cmd
	options   *Options
	clusterId string
}

func NewGetClusterInfoCommand(options *Options) *GetClusterInfoCommand {
	cmd := &GetClusterInfoCommand{}
	cmd.name = "info"
	cmd.options = options
	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)

	return cmd
}

func (a *GetClusterInfoCommand) Name() string {
	return a.name

}

func (a *GetClusterInfoCommand) Exec(args []string) error {

	//ensure correct number of args
	if len(args) < 1 {
		return errors.New("Not enough arguments!")
	}
	if len(args) >= 2 {
		return errors.New("Too many arguments!")
	}

	//parse flags and set id
	a.flags.Parse(args)
	a.clusterId = a.flags.Arg(0)

	url := a.options.Url

	//do http GET and check if sent to server
	r, err := http.Get(url + "/clusters/" + a.clusterId)
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

	//check json response
	var body glusterfs.ClusterInfoResponse
	err = utils.GetJsonFromResponse(r, &body)
	if err != nil {
		fmt.Println("Error: Bad json response from server")
		return err
	}

	//print revelent results
	s := "For cluster: " + a.clusterId + " \n" + "Nodes are: \n"
	for _, node := range body.Nodes {
		s += node + "\n"
	}

	s += "Volumes are: \n"
	for _, volume := range body.Volumes {
		s += volume + "\n"
	}

	fmt.Fprintf(stdout, s)
	return nil

}
