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
	"github.com/heketi/heketi/client/go/utils"
)

type ArithCommand struct {
	// Generic stuff.  This is called
	// embedding.  In other words, the members in
	// the struct below are here also
	Cmd

	// Subcommands available to this command
	cmds Commands

	// Subcommand
	cmd Command
}

func NewArithCommand() *ArithCommand {
	cmd := &ArithCommand{}
	cmd.name = "arith"

	cmd.cmds = Commands{
		NewArithSubtractCommand(),
	}

	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)
	//cmd.flags.StringVar(&cmd.operation, "op", "a", "help message")
	cmd.flags.Usage = func() {
		fmt.Println("Hello from my usage")
	}

	return cmd
}

func (a *ArithCommand) Name() string {
	return a.name

}

func (a *ArithCommand) Parse(args []string) error {

	// Parse our flags here

	// Check which of the subcommands we need to call the .Parse function
	for _, cmd := range a.cmds {
		if args[0] == cmd.Name() {
			cmd.Parse(args[1:])
			// Save this command for later use
			a.cmd = cmd

			return nil
		}
	}

	// Done
	return errors.New("Command not found")
}

func (a *ArithCommand) add() int {
	args := a.flags.Args()[1 : len(a.flags.Args())-1]
	fmt.Println(args)
	sum := 0

	//convert string arr to int arr
	ret := utils.StrArrToIntArr(args)
	//sum all numbers
	for _, num := range ret {
		sum = sum + num
	}
	fmt.Println(sum)
	return sum
}

func (a *ArithCommand) subtract() int {
	if len(a.flags.Args()) > 3 {
		panic("Oops, I can only subtract 2 numbers!")
	}
	args := a.flags.Args()[1:3]

	ret := utils.StrArrToIntArr(args)

	difference := ret[0] - ret[1]
	fmt.Println(difference)
	return difference

}

func (a *ArithCommand) Do() error {

	// Call cmd.Do()

	return a.cmd.Do()

	/*
			fmt.Println(a.flags.Args())
			switch a.flags.Arg(0) {
			case "add":
				// if a.operation != "a" {
				a.add()
				// }
			case "subtract":
				a.subtract()
			default:
				fmt.Println("NAH")
			}
			fmt.Println(a.flags.Arg(1))
			fmt.Println("Options:")
			fmt.Println(a.operation)

		return nil
	*/
}
