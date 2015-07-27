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
)

type EchoCommand struct {
	// Generic stuff.  This is called
	// embedding.  In other words, the values in
	// the struct below are here also
	Cmd

	// it echoes anything after the command
	strings []string
}

func NewEchoCommand() *EchoCommand {
	cmd := &EchoCommand{}
	cmd.name = "echo"

	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)
	cmd.flags.Usage = func() {
		fmt.Println("Hello from my usage")
	}

	return cmd
}

func (e *EchoCommand) Name() string {
	return e.name

}

func (e *EchoCommand) Parse(args []string) error {
	err := e.flags.Parse(args)
	if err != nil {
		return err
	}
	e.strings = e.flags.Args()

	return nil
}

func (e *EchoCommand) Do() error {
	fmt.Println("Echo")
	fmt.Println(e.strings)
	return nil
}
