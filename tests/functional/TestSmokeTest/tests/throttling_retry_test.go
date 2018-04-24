// +build functional

//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package functional

import (
	"testing"

	client "github.com/heketi/heketi/client/api/go-client"
)

//Test with Throttling enabled with count 15
//Pass throttling.json as server config for this
func TestTrottlingRetryNewClient(t *testing.T) {
	heketi = client.NewClientWithRetry(heketiUrl, "", "", 1000)
	setupCluster(t, 4, 8)
	defer teardownCluster(t)
	t.Run("throttlingcreatevolume", throttlingcreatevolume)

}
