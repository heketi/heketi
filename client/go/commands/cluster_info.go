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
)

type ClusterInfoCommand struct {
	Cmd
	options *Options
}

func NewClusterInfoCommand(options *Options) *ClusterInfoCommand {

	godbc.Require(options != nil)

	cmd := &ClusterInfoCommand{}
	cmd.name = "info"
	cmd.options = options
	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)

	//usage on -help
	cmd.flags.Usage = func() {
		fmt.Println(usageTemplateClusterInfo)
	}

	godbc.Ensure(cmd.flags != nil)
	godbc.Ensure(cmd.name == "info")

	return cmd
}

func (a *ClusterInfoCommand) Name() string {
	return a.name

}

func (a *ClusterInfoCommand) Exec(args []string) error {
	//parse flags and set id
	a.flags.Parse(args)

	//ensure we have Url
	if a.options.Url == "" {
		fmt.Fprintf(stdout, "You need a server!\n")
		os.Exit(1)
	}

	s := a.flags.Args()
	fmt.Println(len(s))

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
		utils.GetStringFromResponseCheck(r)
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
		var body glusterfs.ClusterInfoResponse
		err = utils.GetJsonFromResponse(r, &body)
		if err != nil {
			fmt.Println("Error: Bad json response from server")
			return err
		}

		//print revelent results
		str := "Cluster: " + clusterId + " \n" + "Nodes: \n"
		for _, node := range body.Nodes {
			str += node + "\n"
		}

		str += "Volumes: \n"
		for _, volume := range body.Volumes {
			str += volume + "\n"
		}
		fmt.Fprintf(stdout, str)
	}
	return nil

}
