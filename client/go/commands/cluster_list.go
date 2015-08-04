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

type GetClusterListCommand struct {
	Cmd
	options *Options
}

func NewGetClusterListCommand(options *Options) *GetClusterListCommand {

	godbc.Require(options != nil)
	godbc.Require(options.Url != "")

	cmd := &GetClusterListCommand{}
	cmd.name = "list"
	cmd.options = options
	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)

	godbc.Ensure(cmd.flags != nil)
	godbc.Ensure(cmd.name == "list")

	return cmd
}

func (a *GetClusterListCommand) Name() string {
	return a.name

}

func (a *GetClusterListCommand) Exec(args []string) error {

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
		s, err := utils.GetStringFromResponse(r)
		if err != nil {
			return err
		}
		return errors.New(s)
	}

	//check json response
	var body glusterfs.ClusterListResponse
	err = utils.GetJsonFromResponse(r, &body)
	if err != nil {
		fmt.Println("Error: Bad json response from server")
		return err
	}

	//if all is well, print stuff
	str := "Clusters: \n"
	for _, cluster := range body.Clusters {
		str += cluster + "\n"
	}
	fmt.Fprintf(stdout, str)
	return nil

}
