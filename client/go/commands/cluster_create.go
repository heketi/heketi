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
	"errors"
	"flag"
	"fmt"
	"github.com/heketi/heketi/apps/glusterfs"
	"github.com/heketi/heketi/utils"
	"github.com/lpabon/godbc"
	"net/http"
	"os"
)

type ClusterCreateCommand struct {
	Cmd
	options *Options
}

func NewClusterCreateCommand(options *Options) *ClusterCreateCommand {

	godbc.Require(options != nil)

	cmd := &ClusterCreateCommand{}
	cmd.name = "create"
	cmd.options = options
	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)

	//usage on -help
	cmd.flags.Usage = func() {
		fmt.Println(usageTemplateClusterCreate)
	}

	godbc.Ensure(cmd.flags != nil)
	godbc.Ensure(cmd.name == "create")

	return cmd
}

func (a *ClusterCreateCommand) Name() string {
	return a.name

}

func (a *ClusterCreateCommand) Exec(args []string) error {

	//parse args
	a.flags.Parse(args)

	//ensure we have Url
	if a.options.Url == "" {
		fmt.Fprintf(stdout, "You need a server!\n")
		os.Exit(1)
	}

	s := a.flags.Args()
	//ensure length
	if len(s) > 0 {
		return errors.New("Too many arguments!")
	}

	//set url
	url := a.options.Url

	//do http POST and check if sent to server
	r, err := http.Post(url+"/clusters", "application/json", bytes.NewBuffer([]byte("{}")))
	if err != nil {
		fmt.Fprintf(stdout, "Error: Unable to send command to server: %v", err)
		return err
	}

	//check status code
	if r.StatusCode != http.StatusCreated {
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
		//if all is well, print stuff
		fmt.Fprintf(stdout, "Cluster id: %v", body.Id)
	}
	return nil

}
