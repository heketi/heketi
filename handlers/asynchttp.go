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
	"github.com/gorilla/mux"
	"github.com/heketi/heketi/utils"
	"github.com/lpabon/godbc"
	"net/http"
	"sync"
)

type AsyncHttpHandler struct {
	err          error
	completed    bool
	manager      *AsyncHttpManager
	location, id string
}

type AsyncHttpManager struct {
	lock     sync.RWMutex
	route    string
	handlers map[string]*AsyncHttpHandler
}

func NewAsyncHttpManager(route string) *AsyncHttpManager {
	return &AsyncHttpManager{
		route:    route,
		handlers: make(map[string]*AsyncHttpHandler),
	}
}

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

func (a *AsyncHttpManager) HandlerStatus(w http.ResponseWriter, r *http.Request) {
	// Get the id from the URL
	vars := mux.Vars(r)
	id := vars["id"]

	a.lock.Lock()
	defer a.lock.Unlock()

	if handler, ok := a.handlers[id]; ok {
		if handler.completed {
			if handler.err != nil {
				http.Error(w, handler.err.Error(), http.StatusInternalServerError)
			} else {
				if handler.location != "" {
					http.Redirect(w, r, handler.location, http.StatusSeeOther)
				} else {
					w.WriteHeader(http.StatusNoContent)
				}
			}

			// It has been completed, we can now remove it from the map
			delete(a.handlers, id)
		} else {
			// Still pending
			// Could add a JSON body here later
			w.WriteHeader(http.StatusOK)
		}

	} else {
		http.Error(w, "Id not found", http.StatusNotFound)
	}
}

func (h *AsyncHttpHandler) Url() string {
	h.manager.lock.RLock()
	defer h.manager.lock.RUnlock()

	return h.manager.route + "/" + h.id
}

func (h *AsyncHttpHandler) CompletedWithError(err error) {

	h.manager.lock.RLock()
	defer h.manager.lock.RUnlock()

	godbc.Require(h.completed == false)

	h.err = err
	h.completed = true

	godbc.Ensure(h.completed == true)
}

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

func (h *AsyncHttpHandler) Completed() {

	h.manager.lock.RLock()
	defer h.manager.lock.RUnlock()

	godbc.Require(h.completed == false)

	h.completed = true

	godbc.Ensure(h.completed == true)
	godbc.Ensure(h.location == "")
	godbc.Ensure(h.err == nil)
}
