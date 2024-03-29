//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package apps

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/heketi/heketi/v10/pkg/glusterfs/api"
)

type Application interface {
	SetRoutes(router *mux.Router) error
	TopologyInfo() (*api.TopologyInfoResponse, error)
	Close()
	Auth(w http.ResponseWriter, r *http.Request, next http.HandlerFunc)
	AppOperationsInfo() (*api.OperationsInfo, error)
}
