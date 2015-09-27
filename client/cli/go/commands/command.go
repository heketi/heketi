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
	"io"
	"os"
)

//make stdout "global" to command package
var (
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

// Main arguments
type Options struct {
	Url, Key, User string
	Json           bool
}

type Command interface {
	Name() string
	Exec([]string) error
}

type Commands []Command

type Cmd struct {
	name    string
	flags   *flag.FlagSet
	options *Options
	cmds    Commands
}

func (c *Cmd) Name() string {
	return c.name
}

func (c *Cmd) Exec(args []string) error {
	c.flags.Parse(args)

	//check number of args
	if len(c.flags.Args()) < 1 {
		return errors.New("Not enough arguments")
	}

	// Check which of the subcommands we need to call the .Parse function
	for _, cmd := range c.cmds {
		if c.flags.Arg(0) == cmd.Name() {
			err := cmd.Exec(c.flags.Args()[1:])
			if err != nil {
				return err
			}
			return nil
		}
	}

	// Done
	return errors.New("Command not found")
}
