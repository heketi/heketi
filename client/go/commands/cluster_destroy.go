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
	"github.com/heketi/heketi/utils"
	"github.com/lpabon/godbc"
	"net/http"
)

type DestroyClusterCommand struct {
	Cmd
	options *Options
}

func NewDestroyClusterCommand(options *Options) *DestroyClusterCommand {

	godbc.Require(options != nil)
	godbc.Require(options.Url != "")

	cmd := &DestroyClusterCommand{}
	cmd.name = "destroy"
	cmd.options = options
	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)

	godbc.Ensure(cmd.flags != nil)
	godbc.Ensure(cmd.name == "destroy")

	return cmd
}

func (a *DestroyClusterCommand) Name() string {
	return a.name

}

func (a *DestroyClusterCommand) Exec(args []string) error {

	//parse args
	a.flags.Parse(args)

	s := a.flags.Args()

	//ensure proper number of args
	if len(s) < 1 {
		return errors.New("Not enough arguments!")
	}
	if len(s) >= 2 {
		return errors.New("Too many arguments!")
	}

	//set clusterId
	clusterId := a.flags.Arg(0)

	//set url
	url := a.options.Url

	//create destroy request object
	req, err := http.NewRequest("DELETE", url+"/clusters/"+clusterId, nil)
	if err != nil {
		fmt.Fprintf(stdout, "Error: Unable to initiate destroy: %v", err)
		return err
	}

	//destroy cluster
	r, err := http.DefaultClient.Do(req)
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

	//if all is well, print stuff
	fmt.Fprintf(stdout, "Successfully destroyed cluster with id: %v ", clusterId)

	return nil

}
