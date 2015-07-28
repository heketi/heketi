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
	"math"
)

type ArithSubtractCommand struct {
	// Generic stuff.  This is called
	// embedding.  In other words, the members in
	// the struct below are here also
	Cmd

	// Now we can add stuff that specific to this
	// structure
	values  []int
	abs_val string
}

func NewArithSubtractCommand() *ArithSubtractCommand {
	cmd := &ArithSubtractCommand{}
	cmd.name = "subtract"

	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)
	cmd.flags.StringVar(&cmd.abs_val, "abs_val", "no", "get abs value")
	cmd.flags.Usage = func() {
		fmt.Println("Hello from my subtract")
	}

	return cmd
}

func (a *ArithSubtractCommand) Name() string {
	return a.name

}

// func (a *ArithSubtractCommand) Exec(args []string) error {
// 	a.flags.Parse(args)
// 	a.values = utils.StrArrToIntArr(a.flags.Args())

// 	fmt.Println(a.abs_val)
// 	fmt.Printf("Total: %v\n", a.values[0]-a.values[1])

// 	return nil

// }

func (a *ArithSubtractCommand) Parse(args []string) error {
	a.flags.Parse(args)
	a.values = utils.StrArrToIntArr(a.flags.Args())
	return nil
}

func (a *ArithSubtractCommand) Do() error {
	val := a.values[0] - a.values[1]
	switch a.abs_val {
	case "yes":
		fmt.Printf("Total: %v\n", math.Abs(float64(val)))
	case "no":
		fmt.Println("Total:", val)
	default:
		fmt.Println("Invalid option for flag abs-val")
	}
	return nil
}
