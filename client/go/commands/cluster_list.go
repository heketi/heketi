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

type ClusterListCommand struct {
	Cmd
	options *Options
}

func NewClusterListCommand(options *Options) *ClusterListCommand {

	godbc.Require(options != nil)

	cmd := &ClusterListCommand{}
	cmd.name = "list"
	cmd.options = options
	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)

	//usage on -help
	cmd.flags.Usage = func() {
		fmt.Println(usageTemplateClusterList)
	}

	godbc.Ensure(cmd.flags != nil)
	godbc.Ensure(cmd.name == "list")

	return cmd
}

func (a *ClusterListCommand) Name() string {
	return a.name

}

func (a *ClusterListCommand) Exec(args []string) error {

	//parse args
	a.flags.Parse(args)

	//ensure we have Url
	if a.options.Url == "" {
		return errors.New("You need a server!\n")
	}

	s := a.flags.Args()

	//ensure number of args
	if len(s) > 0 {
		return errors.New("Too many arguments!")

	}
	//set url
	url := a.options.Url

	//do http GET and check if sent to server
	r, err := http.Get(url + "/clusters")
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
		var body glusterfs.ClusterListResponse
		err = utils.GetJsonFromResponse(r, &body)
		if err != nil {
			fmt.Println("Error: Bad json response from server")
			return err
		}

		// Print to user cluster lists
		str := "Clusters: \n"
		for _, cluster := range body.Clusters {
			str += cluster + "\n"
		}
		fmt.Fprintf(stdout, str)
	}
	return nil

}
