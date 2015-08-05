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
	"encoding/json"
	"flag"
	"fmt"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"github.com/heketi/heketi/apps"
	"github.com/heketi/heketi/apps/glusterfs"
	"log"
	"net/http"
	"os"
	"os/signal"
)

type Config struct {
	Port string `json:"port"`
}

var (
	HEKETI_VERSION = "(dev)"
	configfile     string
	showVersion    bool
)

func init() {
	flag.StringVar(&configfile, "config", "", "Configuration file")
	flag.BoolVar(&showVersion, "version", false, "Show version")
}

func printVersion() {
	fmt.Printf("Heketi %v\n", HEKETI_VERSION)
}

func main() {
	flag.Parse()
	printVersion()

	// Quit here if all we needed to do was show version
	if showVersion {
		return
	}

	// Check configuration file was given
	if configfile == "" {
		fmt.Fprintln(os.Stderr, "Please provide configuration file")
		os.Exit(1)
	}

	// Read configuration
	fp, err := os.Open(configfile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to open config file %v: %v\n",
			configfile,
			err.Error())
		os.Exit(1)
	}
	defer fp.Close()

	configParser := json.NewDecoder(fp)
	var options Config
	if err = configParser.Decode(&options); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to parse %v: %v\n",
			configfile,
			err.Error())
		os.Exit(1)
	}

	// Go to the beginning of the file when we pass it
	// to the application
	fp.Seek(0, os.SEEK_SET)

	// Setup a new GlusterFS application
	var app apps.Application
	app = glusterfs.NewApp(fp)
	if app == nil {
		fmt.Println("Unable to start application")
		os.Exit(1)
	}

	// Create a router and do not allow any routes
	// unless defined.
	router := mux.NewRouter().StrictSlash(true)
	err = app.SetRoutes(router)
	if err != nil {
		fmt.Println("Unable to create http server endpoints")
		os.Exit(1)
	}

	// Use negroni to add middleware.  Here we add two
	// middlewares: Recovery and Logger, which come with
	// Negroni
	n := negroni.New(negroni.NewRecovery(), negroni.NewLogger())
	n.UseHandler(router)

	// Shutdown on CTRL-C signal
	// For a better cleanup, we should shutdown the server and
	signalch := make(chan os.Signal, 1)
	signal.Notify(signalch, os.Interrupt)
	go func() {
		select {
		case <-signalch:
			fmt.Printf("Shutting down...\n")
			// :TODO: Need to stop the server before closing the app

			// Close the application
			app.Close()

			// Quit
			os.Exit(0)
		}
	}()

	// Start the server.
	log.Fatal(http.ListenAndServe(":"+options.Port, n))

}
