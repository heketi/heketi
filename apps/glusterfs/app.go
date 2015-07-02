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

package glusterfs

import (
	"fmt"
	"github.com/heketi/heketi/rest"
	"net/http"
)

type App struct {
	hello string
}

func NewApp() *App {
	return &App{}
}

// Interface rest.App
func (a *App) GetRoutes() rest.Routes {

	return rest.Routes{

		// HelloWorld
		rest.Route{"Hello", "GET", "/hello", a.Hello},

		// Cluster

		// Node
		/*
		   Route{"NodeList", "GET", "/nodes", n.NodeListHandler},
		   Route{"NodeAdd", "POST", "/nodes", n.NodeAddHandler},
		   Route{"NodeInfo", "GET", "/nodes/{id:[A-Fa-f0-9]+}", n.NodeInfoHandler},
		   Route{"NodeDelete", "DELETE", "/nodes/{id:[A-Fa-f0-9]+}", n.NodeDeleteHandler},
		   Route{"NodeAddDevice", "POST", "/nodes/{id:[A-Fa-f0-9]+}/devices", n.NodeAddDeviceHandler},
		   //Route{"NodeDeleteDevice", "DELETE", "/nodes/{id:[A-Fa-f0-9]+}/devices/{devid:[A-Fa-f0-9]+}", n.NodeDeleteDeviceHandler},
		*/

		// Volume
	}
}

func (a *App) Hello(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "HelloWorld from GlusterFS Application")
}
