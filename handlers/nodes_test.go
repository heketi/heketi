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
	"github.com/lpabon/heketi/plugins/mock"
	"github.com/lpabon/heketi/requests"
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
	if err != nil {
		t.Fatal(err)
	}

	// Check body
	err = utils.GetJsonFromResponse(r, &msg)
	if err != nil {
		t.Fatal(err)
	}

	if len(msg.Nodes) > 0 {
		t.Error("Nodes has more than one value")
	}

}
