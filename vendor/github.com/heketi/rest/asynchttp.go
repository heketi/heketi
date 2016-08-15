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

package rest

import (
	"github.com/gorilla/mux"
	"github.com/heketi/utils"
	"github.com/lpabon/godbc"
	"net/http"
	"sync"
	"time"
)

var (
	logger = utils.NewLogger("[asynchttp]", utils.LEVEL_INFO)
)

// Contains information about the asynchronous operation
type AsyncHttpHandler struct {
	err          error
	completed    bool
	manager      *AsyncHttpManager
	location, id string
}

// Manager of asynchronous operations
type AsyncHttpManager struct {
	lock     sync.RWMutex
	route    string
	handlers map[string]*AsyncHttpHandler
}

// Creates a new manager
func NewAsyncHttpManager(route string) *AsyncHttpManager {
	return &AsyncHttpManager{
		route:    route,
		handlers: make(map[string]*AsyncHttpHandler),
	}
}

// Use to create a new asynchronous operation handler.
// Only use this function if you need to do every step by hand.
// It is recommended to use AsyncHttpRedirectFunc() instead
func (a *AsyncHttpManager) NewHandler() *AsyncHttpHandler {
	handler := &AsyncHttpHandler{
		manager: a,
		id:      utils.GenUUID(),
	}

	a.lock.Lock()
	defer a.lock.Unlock()

	a.handlers[handler.id] = handler

	return handler
}

// Create an asynchronous operation handler and return the appropiate
// information the caller.
// This function will call handlerfunc() in a new go routine, then
// return to the caller a HTTP status 202 setting up the `Location` header
// to point to the new asynchronous handler.
//
// If handlerfunc() returns failure, the asynchronous handler will return
// an http status of 500 and save the error string in the body.
// If handlerfunc() is successful and returns a location url path in "string",
// the asynchronous handler will return 303 (See Other) with the Location
// header set to the value returned in the string.
// If handlerfunc() is successful and returns an empty string, then the
// asynchronous handler will return 204 to the caller.
//
// Example:
//      package rest
//		import (
//			"github.com/gorilla/mux"
//          "github.com/heketi/rest"
//			"net/http"
//			"net/http/httptest"
//			"time"
//		)
//
//		// Setup asynchronous manager
//		route := "/x"
//		manager := rest.NewAsyncHttpManager(route)
//
//		// Setup the route
//		router := mux.NewRouter()
//	 	router.HandleFunc(route+"/{id}", manager.HandlerStatus).Methods("GET")
//		router.HandleFunc("/result", func(w http.ResponseWriter, r *http.Request) {
//			w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
//			w.WriteHeader(http.StatusOK)
//			fmt.Fprint(w, "HelloWorld")
//		}).Methods("GET")
//
//		router.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
//			manager.AsyncHttpRedirectFunc(w, r, func() (string, error) {
//				time.Sleep(100 * time.Millisecond)
//				return "/result", nil
//			})
//		}).Methods("GET")
//
//		// Setup the server
//		ts := httptest.NewServer(router)
//		defer ts.Close()
//
func (a *AsyncHttpManager) AsyncHttpRedirectFunc(w http.ResponseWriter,
	r *http.Request,
	handlerfunc func() (string, error)) {

	handler := a.NewHandler()
	go func() {
		logger.Info("Started job %v", handler.id)

		ts := time.Now()
		url, err := handlerfunc()
		logger.Info("Completed job %v in %v", handler.id, time.Since(ts))

		if err != nil {
			handler.CompletedWithError(err)
		} else if url != "" {
			handler.CompletedWithLocation(url)
		} else {
			handler.Completed()
		}
	}()
	http.Redirect(w, r, handler.Url(), http.StatusAccepted)
}

// Handler for asynchronous operation status
// Register this handler with a router like Gorilla Mux
//
// Returns the following HTTP status codes
// 		200 Operation is still pending
//		404 Id requested does not exist
//		500 Operation finished and has failed.  Body will be filled in with the
//			error in plain text.
//		303 Operation finished and has setup a new location to retreive data.
//		204 Operation finished and has no data to return
//
// Example:
//      package rest
//		import (
//			"github.com/gorilla/mux"
//          "github.com/heketi/rest"
//			"net/http"
//			"net/http/httptest"
//			"time"
//		)
//
//		// Setup asynchronous manager
//		route := "/x"
//		manager := rest.NewAsyncHttpManager(route)
//
//		// Setup the route
//		router := mux.NewRouter()
//	 	router.HandleFunc(route+"/{id:[A-Fa-f0-9]+}", manager.HandlerStatus).Methods("GET")
//
//		// Setup the server
//		ts := httptest.NewServer(router)
//		defer ts.Close()
//
func (a *AsyncHttpManager) HandlerStatus(w http.ResponseWriter, r *http.Request) {
	// Get the id from the URL
	vars := mux.Vars(r)
	id := vars["id"]

	a.lock.Lock()
	defer a.lock.Unlock()

	// Check the id is in the map
	if handler, ok := a.handlers[id]; ok {

		if handler.completed {
			if handler.err != nil {

				// Return 500 status
				http.Error(w, handler.err.Error(), http.StatusInternalServerError)
			} else {
				if handler.location != "" {

					// Redirect to new location
					http.Redirect(w, r, handler.location, http.StatusSeeOther)
				} else {

					// Return 204 status
					w.WriteHeader(http.StatusNoContent)
				}
			}

			// It has been completed, we can now remove it from the map
			delete(a.handlers, id)
		} else {
			// Still pending
			// Could add a JSON body here later
			w.Header().Add("X-Pending", "true")
			w.WriteHeader(http.StatusOK)
		}

	} else {
		http.Error(w, "Id not found", http.StatusNotFound)
	}
}

// Returns the url for the specified asynchronous handler
func (h *AsyncHttpHandler) Url() string {
	h.manager.lock.RLock()
	defer h.manager.lock.RUnlock()

	return h.manager.route + "/" + h.id
}

// Registers that the handler has completed with an error
func (h *AsyncHttpHandler) CompletedWithError(err error) {

	h.manager.lock.RLock()
	defer h.manager.lock.RUnlock()

	godbc.Require(h.completed == false)

	h.err = err
	h.completed = true

	godbc.Ensure(h.completed == true)
}

// Registers that the handler has completed and has provided a location
// where information can be retreived
func (h *AsyncHttpHandler) CompletedWithLocation(location string) {

	h.manager.lock.RLock()
	defer h.manager.lock.RUnlock()

	godbc.Require(h.completed == false)

	h.location = location
	h.completed = true

	godbc.Ensure(h.completed == true)
	godbc.Ensure(h.location == location)
	godbc.Ensure(h.err == nil)
}

// Registers that the handler has completed and no data needs to be returned
func (h *AsyncHttpHandler) Completed() {

	h.manager.lock.RLock()
	defer h.manager.lock.RUnlock()

	godbc.Require(h.completed == false)

	h.completed = true

	godbc.Ensure(h.completed == true)
	godbc.Ensure(h.location == "")
	godbc.Ensure(h.err == nil)
}
