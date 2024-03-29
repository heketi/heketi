//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package admin

import (
	"os"
	"os/signal"

	"github.com/heketi/heketi/v10/pkg/glusterfs/api"
)

func ResetStateOnSignal(s *ServerState, sig ...os.Signal) {
	signalch := make(chan os.Signal, 1)
	signal.Notify(signalch, sig...)

	go func() {
		for {
			select {
			case <-signalch:
				s.Set(api.AdminStateNormal)
			}
		}
	}()
}
