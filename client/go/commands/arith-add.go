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
	"flag"
	"fmt"
	"github.com/heketi/heketi/client/go/utils"
)

type ArithAddCommand struct {
	// Generic stuff.  This is called
	// embedding.  In other words, the members in
	// the struct below are here also
	Cmd

	// Now we can add stuff that specific to this
	// structure
	values []int
	double string
}

func NewArithAddCommand() *ArithAddCommand {
	cmd := &ArithAddCommand{}
	cmd.name = "add"

	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)
	cmd.flags.StringVar(&cmd.double, "double", "no", "doubles the sum")
	cmd.flags.Usage = func() {
		fmt.Println("Hello from my add")
	}

	return cmd
}

func (a *ArithAddCommand) Name() string {
	return a.name

}

func (a *ArithAddCommand) Parse(args []string) error {
	a.flags.Parse(args)
	a.values = utils.StrArrToIntArr(a.flags.Args())
	return nil
}

func (a *ArithAddCommand) Do() error {
	sum := 0
	for _, val := range a.values {
		sum = sum + val
	}

	switch a.double {
	case "yes":
		fmt.Printf("Total: %v\n", sum*2)
	case "no":
		fmt.Printf("Total: %v\n", sum)
	default:
		fmt.Println("Invalid value for flag: double")
	}

	return nil
}
