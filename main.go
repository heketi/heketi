//
// Copyright (c) 2014 The heketi Authors
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
	"fmt"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"os"
	"os/signal"
)

func main() {

	// Create a router and do not allow any routes
	// unless defined.
	router := mux.NewRouter().StrictSlash(true)
	/*

		for _, route := range r {

			// Add routes from the table
			router.
				Methods(route.Method).
				Path(route.Pattern).
				Name(route.Name).
				Handler(route.HandlerFunc)

		}
	*/

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "HelloWorld")
	}).Methods("GET")

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
			fmt.Printf("Shutting down...")
			os.Exit(0)
		}
	}()

	// Start the server.
	log.Fatal(http.ListenAndServe(":8080", n))

}
