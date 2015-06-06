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
	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"github.com/lpabon/heketi/models"
	"log"
	"net/http"
)

func main() {

	// Create a router and do not allow any routes
	// unless defined.
	router := mux.NewRouter().StrictSlash(true)
	for _, route := range models.HandlerRoutes() {

		// Add routes from the table
		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(route.HandlerFunc)

	}

	// Use negroni to add middleware.  Here we add two
	// middlewares: Recovery and Logger, which come with
	// Negroni
	n := negroni.New(negroni.NewRecovery(), negroni.NewLogger())
	n.UseHandler(router)

	// Start the server.
	log.Fatal(http.ListenAndServe(":8080", n))

}
