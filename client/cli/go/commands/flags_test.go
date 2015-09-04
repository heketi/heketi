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

package commands

import (
	"bytes"
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/heketi/heketi/apps/glusterfs"
	"github.com/heketi/heketi/tests"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestJsonFlagsCreate(t *testing.T) {
	db := tests.Tempfile()
	defer os.Remove(db)

	// Create the app
	app := glusterfs.NewTestApp(db)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	//set options
	options := &Options{
		Url:  ts.URL,
		Json: true,
	}

	//create b to get values of stdout
	var b bytes.Buffer
	defer tests.Patch(&stdout, &b).Restore()

	//create mock cluster and mock destroy
	mockCluster := NewClusterCreateCommand(options)

	//create new cluster and assert json
	err := mockCluster.Exec([]string{})
	tests.Assert(t, err == nil)
	var clusterInfoResCreate glusterfs.ClusterInfoResponse
	err = json.Unmarshal(b.Bytes(), &clusterInfoResCreate)
	tests.Assert(t, err == nil, err)
	tests.Assert(t, strings.Contains(b.String(), clusterInfoResCreate.Id), clusterInfoResCreate)
	b.Reset()

	//get cluster info and assert json
	clusterInfo := NewClusterInfoCommand(options)
	args := []string{clusterInfoResCreate.Id}
	err = clusterInfo.Exec(args)
	tests.Assert(t, err == nil)
	tests.Assert(t, strings.Contains(b.String(), clusterInfoResCreate.Id))
	b.Reset()

	//get cluster list and assert json
	clusterList := NewClusterListCommand(options)
	err = clusterList.Exec([]string{})
	tests.Assert(t, err == nil)
	var clusterListResList glusterfs.ClusterInfoResponse
	err = json.Unmarshal(b.Bytes(), &clusterListResList)
	tests.Assert(t, strings.Contains(b.String(), clusterListResList.Id))
	b.Reset()

	//destroy cluster and assert proper json response
	clusterDestroy := NewClusterDestroyCommand(options)
	err = clusterDestroy.Exec(args)
	tests.Assert(t, err != nil)
	tests.Assert(t, strings.Contains(b.String(), ""))

}
