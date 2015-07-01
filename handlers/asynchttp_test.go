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

package handlers

import (
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/heketi/heketi/tests"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {

	manager := NewAsyncHttpManager("/x")

	tests.Assert(t, len(manager.handlers) == 0)
	tests.Assert(t, manager.route == "/x")

}

func TestNewHandler(t *testing.T) {

	manager := NewAsyncHttpManager("/x")

	handler := manager.NewHandler()
	tests.Assert(t, handler.location == "")
	tests.Assert(t, handler.id != "")
	tests.Assert(t, handler.completed == false)
	tests.Assert(t, handler.err == nil)
	tests.Assert(t, manager.handlers[handler.id] == handler)
}

func TestHandlerUrl(t *testing.T) {
	manager := NewAsyncHttpManager("/x")
	handler := manager.NewHandler()

	// overwrite id value
	handler.id = "12345"
	tests.Assert(t, handler.Url() == "/x/12345")
}

func TestHandlerNotFound(t *testing.T) {

	// Setup asynchronous manager
	route := "/x"
	manager := NewAsyncHttpManager(route)

	// Setup the route
	router := mux.NewRouter()
	router.HandleFunc(route+"/{id}", manager.HandlerStatus).Methods("GET")

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Request
	r, err := http.Get(ts.URL + route + "/12345")
	tests.Assert(t, r.StatusCode == http.StatusNotFound)
	tests.Assert(t, err == nil)
}

func TestHandlerCompletions(t *testing.T) {

	// Setup asynchronous manager
	route := "/x"
	manager := NewAsyncHttpManager(route)
	handler := manager.NewHandler()

	// Setup the route
	router := mux.NewRouter()
	router.HandleFunc(route+"/{id}", manager.HandlerStatus).Methods("GET")
	router.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Test", "HelloWorld")
		w.WriteHeader(http.StatusOK)
	}).Methods("GET")

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Request
	r, err := http.Get(ts.URL + handler.Url())
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, err == nil)

	// Handler completion without data
	handler.Completed()
	r, err = http.Get(ts.URL + handler.Url())
	tests.Assert(t, r.StatusCode == http.StatusNoContent)
	tests.Assert(t, err == nil)

	// Check that it was removed from the map
	_, ok := manager.handlers[handler.id]
	tests.Assert(t, ok == false)

	// Create new handler
	handler = manager.NewHandler()

	// Request
	r, err = http.Get(ts.URL + handler.Url())
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, err == nil)

	// Complete with error
	error_string := "This is a test"
	handler.CompletedWithError(errors.New(error_string))
	r, err = http.Get(ts.URL + handler.Url())
	tests.Assert(t, r.StatusCode == http.StatusInternalServerError)
	tests.Assert(t, err == nil)

	// Check body has error string
	body, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	tests.Assert(t, string(body) == error_string+"\n")

	// Create new handler
	handler = manager.NewHandler()

	// Request
	r, err = http.Get(ts.URL + handler.Url())
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, err == nil)

	// Complete with SeeOther to Location
	handler.CompletedWithLocation("/test")

	// http.Get() looks at the Location header
	// and automatically redirects to the new location
	r, err = http.Get(ts.URL + handler.Url())
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, r.Header.Get("X-Test") == "HelloWorld")
	tests.Assert(t, err == nil)

}

func TestHandlerConcurrency(t *testing.T) {

	// Setup asynchronous manager
	route := "/x"
	manager := NewAsyncHttpManager(route)

	// Setup the route
	router := mux.NewRouter()
	router.HandleFunc(route+"/{id}", manager.HandlerStatus).Methods("GET")
	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	var wg sync.WaitGroup
	errorsch := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			handler := manager.NewHandler()
			go func() {
				time.Sleep(10 * time.Millisecond)
				handler.Completed()
			}()

			for {
				r, err := http.Get(ts.URL + handler.Url())
				if err != nil {
					errorsch <- errors.New("Unable to get data from handler")
					return
				}
				if r.StatusCode == http.StatusNoContent {
					return
				} else if r.StatusCode == http.StatusOK {
					time.Sleep(time.Millisecond)
				} else {
					errorsch <- errors.New(fmt.Sprintf("Bad status returned: %d\n", r.StatusCode))
					return
				}
			}
		}()
	}
	wg.Wait()
	tests.Assert(t, len(errorsch) == 0)
}

func TestHandlerApplication(t *testing.T) {

	// Setup asynchronous manager
	route := "/x"
	manager := NewAsyncHttpManager(route)

	// Setup the route
	router := mux.NewRouter()
	router.HandleFunc(route+"/{id}", manager.HandlerStatus).Methods("GET")
	router.HandleFunc("/result", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "HelloWorld")
	}).Methods("GET")
	router.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
		handler := manager.NewHandler()
		go func() {
			time.Sleep(100 * time.Millisecond)
			handler.CompletedWithLocation("/result")
		}()

		http.Redirect(w, r, handler.Url(), http.StatusAccepted)
	}).Methods("GET")

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Get /app url
	r, err := http.Get(ts.URL + "/app")
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	tests.Assert(t, err == nil)
	location, err := r.Location()
	tests.Assert(t, err == nil)

	for {
		// Since Get automatically redirects, we will
		// just keep asking until we get a body
		r, err := http.Get(location.String())
		tests.Assert(t, err == nil)
		tests.Assert(t, r.StatusCode == http.StatusOK)
		if r.ContentLength > 0 {
			body, err := ioutil.ReadAll(r.Body)
			r.Body.Close()
			tests.Assert(t, err == nil)
			tests.Assert(t, string(body) == "HelloWorld")
			break
		}
	}

}
