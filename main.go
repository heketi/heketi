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

//
// This route style comes from the tutorial on
// http://thenewstack.io/make-a-restful-json-api-go/
//
type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

type Routes []Route

var routes = Routes{

	// Volume Routes
	Route{"VolumeList", "GET", "/volumes", models.VolumeListHandler},
	Route{"VolumeCreate", "POST", "/volumes", models.VolumeCreateHandler},
	Route{"VolumeInfo", "GET", "/volumes/{volid:[0-9]+}", models.VolumeInfoHandler},
	Route{"VolumeDelete", "DELETE", "/volumes/{volid:[0-9]+}", models.VolumeDeleteHandler},

	// Node Routes

}

func main() {

	// Create a router and do not allow any routes
	// unless defined.
	router := mux.NewRouter().StrictSlash(true)
	for _, route := range routes {

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
