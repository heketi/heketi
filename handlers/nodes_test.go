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
	"bytes"
	"github.com/lpabon/heketi/plugins/mock"
	"github.com/lpabon/heketi/requests"
	"github.com/lpabon/heketi/tests"
	"github.com/lpabon/heketi/utils"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNodeListHandlerEmpty(t *testing.T) {

	var msg requests.NodeListResponse

	n := NewNodeServer(mock.NewMockPlugin())
	ts := httptest.NewServer(http.HandlerFunc(n.NodeListHandler))
	defer ts.Close()

	// Request
	r, err := http.Get(ts.URL)
	tests.Assert(t, err == nil)

	// Check body
	err = utils.GetJsonFromResponse(r, &msg)
	tests.Assert(t, err == nil)

	tests.Assert(t, len(msg.Nodes) == 0)
}

func TestNodeAddHandler(t *testing.T) {

	var msg requests.NodeInfoResp

	// Instead of coding our own JSON here,
	// create the JSON message as a string to test the handler
	request := []byte(`{
        "name" : "test_name",
        "zone" : "test_zone",
        "lvm" : {
            "volumegroup" : "test_vg"
        }
    }`)

	n := NewNodeServer(mock.NewMockPlugin())
	ts := httptest.NewServer(http.HandlerFunc(n.NodeAddHandler))
	defer ts.Close()

	// Request
	r, err := http.Post(ts.URL, "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)

	// Check body
	err = utils.GetJsonFromResponse(r, &msg)
	tests.Assert(t, err == nil)

	tests.Assert(t, msg.Name == "test_name")
	tests.Assert(t, msg.Zone == "test_zone")
	/* Add when vgs are supported
	tests.Assert(t, len(msg.VolumeGroups) == 1)
	tests.Assert(t, msg.VolumeGroups[0].Name == "test_vg")
	*/
}
