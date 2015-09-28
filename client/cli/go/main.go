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

package main

import (
	"flag"
	"fmt"
	"github.com/heketi/heketi/client/cli/go/commands"
	"io"
	"os"
)

var (
	HEKETI_CLI_VERSION           = "(dev)"
	stdout             io.Writer = os.Stdout
	stderr             io.Writer = os.Stderr
	options            commands.Options
	version            bool
)

func init() {

	flag.StringVar(&options.Url, "server", "",
		"\n\tHeketi server. Can also be set using the"+
			"\n\tenvironment variable HEKETI_CLI_SERVER")
	flag.StringVar(&options.Key, "secret", "",
		"\n\tSecret key for specified user.  Can also be"+
			"\n\tset using the environment variable HEKETI_CLI_KEY")
	flag.StringVar(&options.User, "user", "",
		"\n\tHeketi user.  Can also be set using the"+
			"\n\tenvironment variable HEKETI_CLI_USER")
	flag.BoolVar(&options.Json, "json", false,
		"\n\tPrint response as JSON")
	flag.BoolVar(&version, "version", false,
		"\n\tPrint version")

	flag.Usage = func() {
		fmt.Println(`
Command line program for Heketi.

USAGE
  heketi-cli [options] command [arguments]

OPTIONS`)
		flag.PrintDefaults()
		fmt.Println(`
COMMANDS
  load       Load topology configuration file
  cluster    Manage a cluster (a set of storage nodes)
  node       Manage a storage node
  device     Manage devices in a node
  volume     Manage GlusterFS volumes

EXAMPLES
  $ export HEKETI_CLI_SERVER=http://localhost:8080
  $ heketi-cli volume list

Use "heketi-cli [command] -help" for more information about a command
`)
	}
}

// ------ Main
func main() {

	// Parse command line
	flag.Parse()

	// Check if user asked for version
	if version {
		fmt.Printf("heketi-cli %v\n", HEKETI_CLI_VERSION)
		return
	}

	// Check server
	if options.Url == "" {
		options.Url = os.Getenv("HEKETI_CLI_SERVER")
		if options.Url == "" {
			fmt.Fprintf(stderr, "Server must be provided\n")
			os.Exit(3)
		}
	}

	// Check user
	if options.Key == "" {
		options.Key = os.Getenv("HEKETI_CLI_KEY")
	}

	// Check key
	if options.User == "" {
		options.User = os.Getenv("HEKETI_CLI_USER")
	}

	// All first level commands go here (cluster, node, device, volume)
	cmds := commands.Commands{
		commands.NewClusterCommand(&options),
		commands.NewNodeCommand(&options),
		commands.NewDeviceCommand(&options),
		commands.NewVolumeCommand(&options),
		commands.NewLoadCommand(&options),
	}

	// Find command
	for _, cmd := range cmds {
		if flag.Arg(0) == cmd.Name() {

			err := cmd.Exec(flag.Args()[1:])
			if err != nil {
				fmt.Fprintf(stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	fmt.Fprintf(stderr, "Command %v not recognized\n", flag.Arg(0))
	os.Exit(1)
}
