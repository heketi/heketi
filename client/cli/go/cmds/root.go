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

package cmds

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

var (
	HEKETI_CLI_VERSION           = "(dev)"
	stdout             io.Writer = os.Stdout
	stderr             io.Writer = os.Stderr
	options            Options
	version            bool
)

// Main arguments
type Options struct {
	Url, Key, User string
	Json           bool
}

var RootCmd = &cobra.Command{
	Use:   "heketi-cli",
	Short: "Command line program for Heketi",
	Long:  "Command line program for Heketi",
	Run: func(cmd *cobra.Command, args []string) {
		if version {
			fmt.Printf("heketi-cli %v\n", HEKETI_CLI_VERSION)
		}
	},
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err) //should be used for logging
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	RootCmd.PersistentFlags().StringVarP(&options.Url, "server", "s", "",
		"\n\tHeketi server. Can also be set using the"+
			"\n\tenvironment variable HEKETI_CLI_SERVER")
	RootCmd.PersistentFlags().StringVar(&options.Key, "secret", "",
		"\n\tSecret key for specified user.  Can also be"+
			"\n\tset using the environment variable HEKETI_CLI_KEY")
	RootCmd.PersistentFlags().StringVar(&options.User, "user", "",
		"\n\tHeketi user.  Can also be set using the"+
			"\n\tenvironment variable HEKETI_CLI_USER")
	RootCmd.PersistentFlags().BoolVar(&options.Json, "json", false,
		"\n\tPrint response as JSON")
	RootCmd.Flags().BoolVarP(&version, "version", "v", false,
		"\n\tPrint version")
}

func initConfig() {
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
}
