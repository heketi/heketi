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
	"github.com/gorilla/mux"
	"github.com/heketi/heketi/apps/glusterfs"
	"github.com/heketi/heketi/tests"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

/*** GENERAL COMMAND LINE TESTS BEGIN ***/

//tests object creation
func TestNewNodeCommand(t *testing.T) {

	options := &Options{
		Url: "soaps",
	}

	//assert object creation is correct
	c := NewNodeCommand(options)
	tests.Assert(t, c.options == options)
	tests.Assert(t, c.name == "node")
	tests.Assert(t, c.flags != nil)
	tests.Assert(t, len(c.cmds) == 3)
}

//tests too little args
func TestNodeCommandTooLittleArguments(t *testing.T) {
	defer os.Remove("heketi.db")

	// Create the app
	app := glusterfs.NewApp()
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	//set options
	options := &Options{
		Url: ts.URL,
	}

	//create b to get values of stdout
	var b bytes.Buffer
	defer tests.Patch(&stdout, &b).Restore()

	NodeCommand := NewNodeCommand(options)

	//too little args
	err := NodeCommand.Exec([]string{})

	//make sure not enough args
	tests.Assert(t, err != nil, err)
	tests.Assert(t, strings.Contains(err.Error(), "Not enough arguments"), err.Error())

}

//tests too many arguments
func TestNodeCommandTooManyArguments(t *testing.T) {
	defer os.Remove("heketi.db")

	// Create the app
	app := glusterfs.NewApp()
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	//set options
	options := &Options{
		Url: ts.URL,
	}

	//create b to get values of stdout
	var b bytes.Buffer
	defer tests.Patch(&stdout, &b).Restore()

	NodeCommand := NewNodeCommand(options)

	//add too many args
	var str = []string{"info", "one", "two", "three"}
	err := NodeCommand.Exec(str)

	//make sure too many args
	tests.Assert(t, err != nil, err)
	tests.Assert(t, strings.Contains(err.Error(), "Too many arguments"), err.Error())

}

//tests command not found
func TestNodeCommandNotFound(t *testing.T) {
	defer os.Remove("heketi.db")

	// Create the app
	app := glusterfs.NewApp()
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	//set options
	options := &Options{
		Url: ts.URL,
	}

	//create b to get values of stdout
	var b bytes.Buffer
	defer tests.Patch(&stdout, &b).Restore()

	NodeCommand := NewNodeCommand(options)

	//make first arg not a recognized command
	var str = []string{"NotACommand"}
	err := NodeCommand.Exec(str)

	//make sure command not found
	tests.Assert(t, err != nil, err)
	tests.Assert(t, strings.Contains(err.Error(), "Command not found"), err.Error())

}

//tests Node info and destroy
func TestNewGetNodeAddAndInfoAndDestroy(t *testing.T) {
	defer os.Remove("heketi.db")

	// Create the app
	app := glusterfs.NewApp()
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	//set options
	options := &Options{
		Url: ts.URL,
	}

	//create b to get values of stdout
	var b bytes.Buffer
	defer tests.Patch(&stdout, &b).Restore()

	//create mock cluster and mock destroy
	mockCluster := NewClusterCreateCommand(options)

	//create new cluster
	err := mockCluster.Exec([]string{})
	tests.Assert(t, err == nil)

	//get cluster id
	MockClusterIdArray := strings.SplitAfter(b.String(), "id: ")
	tests.Assert(t, len(MockClusterIdArray) >= 1)
	MockClusterId := MockClusterIdArray[1]
	b.Reset()

	//create mock Node
	mockNode := NewNodeAddCommand(options)

	//create new Node
	mockNode.zone = 1
	mockNode.clusterId = MockClusterId
	mockNode.storageHostNames = "storage.hostname.com"
	mockNode.managmentHostNames = "manage.hostname.com"
	err = mockNode.Exec([]string{})
	tests.Assert(t, err == nil)
	b.Reset()

	//get Node id
	mockClusterNodeId := NewClusterInfoCommand(options)
	err = mockClusterNodeId.Exec([]string{MockClusterId})
	tests.Assert(t, err == nil)
	nodeIdArray := strings.SplitAfter(b.String(), "\n")
	tests.Assert(t, len(nodeIdArray) >= 2)
	nodeId := strings.TrimSpace(nodeIdArray[2])

	// //assert that Node info Exec succeeds and prints correctly
	nodeInfo := NewNodeInfoCommand(options)
	args := []string{nodeId}
	err = nodeInfo.Exec(args)
	tests.Assert(t, err == nil, err)
	tests.Assert(t, strings.Contains(b.String(), "Zone: "), b.String())

	// //create destroy struct and destroy node
	mockNodeDestroy := NewNodeDestroyCommand(options)
	args = []string{nodeId}
	err = mockNodeDestroy.Exec(args)
	tests.Assert(t, err == nil)

	// //assert that we cannot get info on destroyed Node
	err = nodeInfo.Exec([]string{})
	tests.Assert(t, err != nil)

}

//tests for bad id
func TestNewGetNodeInfoBadID(t *testing.T) {
	defer os.Remove("heketi.db")

	// Create the app
	app := glusterfs.NewApp()
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	//set options
	options := &Options{
		Url: ts.URL,
	}

	//create b to get values of stdout
	var b bytes.Buffer
	defer tests.Patch(&stdout, &b).Restore()

	//set destroy id to our id
	nodeInfo := NewNodeInfoCommand(options)
	nodeId := "penguins are the key to something"

	//assert that cluster info Exec FAILS and with bad id
	args := []string{nodeId}
	err := nodeInfo.Exec(args)
	tests.Assert(t, err != nil, err)
	tests.Assert(t, err.Error() != "")

}

func TestNodePostFailure(t *testing.T) {
	defer os.Remove("heketi.db")

	// Create the app
	app := glusterfs.NewApp()
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	options := &Options{
		Url: "http://nottherightthing:8080",
	}

	//create b so we can see stdout
	var b bytes.Buffer
	defer tests.Patch(&stdout, &b).Restore()

	//create cluster
	cluster := NewNodeAddCommand(options)
	tests.Assert(t, cluster != nil)

	//execute
	err := cluster.Exec([]string{})
	tests.Assert(t, err != nil)
	tests.Assert(t, strings.Contains(b.String(), "Unable to send "))
}
