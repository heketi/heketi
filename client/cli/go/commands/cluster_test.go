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
func TestNewClusterCommand(t *testing.T) {

	options := &Options{
		Url: "soaps",
	}

	//assert object creation is correct
	c := NewClusterCommand(options)
	tests.Assert(t, c.options == options)
	tests.Assert(t, c.name == "cluster")
	tests.Assert(t, c.flags != nil)
	tests.Assert(t, len(c.cmds) == 4)
}

//tests too little args
func TestClusterCommandTooLittleArguments(t *testing.T) {
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
		Url: ts.URL,
	}

	//create b to get values of stdout
	var b bytes.Buffer
	defer tests.Patch(&stdout, &b).Restore()

	ClusterCommand := NewClusterCommand(options)

	//too little args
	err := ClusterCommand.Exec([]string{})

	//make sure not enough args
	tests.Assert(t, err != nil, err)
	tests.Assert(t, strings.Contains(err.Error(), "Not enough arguments"), err.Error())

}

//tests too many arguments
func TestClusterCommandTooManyArguments(t *testing.T) {
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
		Url: ts.URL,
	}

	//create b to get values of stdout
	var b bytes.Buffer
	defer tests.Patch(&stdout, &b).Restore()

	ClusterCommand := NewClusterCommand(options)

	//add too many args
	var str = []string{"create", "one", "two", "three"}
	err := ClusterCommand.Exec(str)

	//make sure too many args
	tests.Assert(t, err != nil, err)
	tests.Assert(t, strings.Contains(err.Error(), "Too many arguments"), err.Error())

}

//tests command not found
func TestClusterCommandNotFound(t *testing.T) {
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
		Url: ts.URL,
	}

	//create b to get values of stdout
	var b bytes.Buffer
	defer tests.Patch(&stdout, &b).Restore()

	ClusterCommand := NewClusterCommand(options)

	//make first arg not a recognized command
	var str = []string{"NotACommand"}
	err := ClusterCommand.Exec(str)

	//make sure command not found
	tests.Assert(t, err != nil, err)
	tests.Assert(t, strings.Contains(err.Error(), "Command not found"), err.Error())

}

/*** GENERAL COMMAND LINE TESTS END ***/

/*** MAIN TESTS BEGIN ***/

//tests cluster info and destroy
func TestNewGetClusterInfoAndDestroy(t *testing.T) {
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

	//set destroy id to our id
	clusterInfo := NewClusterInfoCommand(options)

	//assert that cluster info Exec succeeds and prints correctly
	args := []string{MockClusterId}
	err = clusterInfo.Exec(args)
	tests.Assert(t, err == nil, err)
	tests.Assert(t, strings.Contains(b.String(), "Cluster: "), b.String())

	//create destroy struct and destroy it
	mockClusterDestroy := NewClusterDestroyCommand(options)
	args = []string{MockClusterId}
	err = mockClusterDestroy.Exec(args)
	tests.Assert(t, err == nil)

	//assert that we cannot get info on destroyed cluster
	err = clusterInfo.Exec([]string{})
	tests.Assert(t, err != nil)

}

//tests for bad id
func TestNewGetClusterInfoBadID(t *testing.T) {
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
		Url: ts.URL,
	}

	//create b to get values of stdout
	var b bytes.Buffer
	defer tests.Patch(&stdout, &b).Restore()

	//set destroy id to our id
	clusterInfo := NewClusterInfoCommand(options)
	clusterId := "penguins are the key to something"

	//assert that cluster info Exec FAILS and with bad id
	args := []string{clusterId}
	err := clusterInfo.Exec(args)
	tests.Assert(t, err != nil, err)
	tests.Assert(t, err.Error() != "")

}

// test cluster list
func TestNewGetClusterList(t *testing.T) {
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

	//assert cluster was created
	tests.Assert(t, strings.Contains(b.String(), "Cluster id: "), b.String())
	b.Reset()

	//create new list command
	listCommand := NewClusterListCommand(options)
	err = listCommand.Exec([]string{})
	tests.Assert(t, err == nil)

	//asert stdout is correct
	tests.Assert(t, strings.Contains(b.String(), "Clusters: "), b.String())
	tests.Assert(t, len(b.String()) > len("Clusters : "))
}

//test cluster create
func TestClusterPostSuccess(t *testing.T) {
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

	options := &Options{
		Url: ts.URL,
	}

	//create bytes.Buffer so we can read stdout
	var b bytes.Buffer
	defer tests.Patch(&stdout, &b).Restore()

	//assert cluster creation
	cluster := NewClusterCreateCommand(options)
	tests.Assert(t, cluster != nil)

	//execute and assert works
	err := cluster.Exec([]string{})
	tests.Assert(t, err == nil)
	tests.Assert(t, strings.Contains(b.String(), "Cluster id:"), b.String())
}

func TestClusterPostFailure(t *testing.T) {
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

	options := &Options{
		Url: "http://nottherightthing:8080",
	}

	//create b so we can see stdout
	var b bytes.Buffer
	defer tests.Patch(&stdout, &b).Restore()

	//create cluster
	cluster := NewClusterCreateCommand(options)
	tests.Assert(t, cluster != nil)

	//execute
	err := cluster.Exec([]string{})
	tests.Assert(t, err != nil)
	tests.Assert(t, strings.Contains(b.String(), "Unable to send "))
}

func TestNewClusterDestroyNoServer(t *testing.T) {
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
		Url: "",
	}

	//create b to get values of stdout
	var b bytes.Buffer
	defer tests.Patch(&stdout, &b).Restore()

	clusterDestroy := NewClusterDestroyCommand(options)
	err := clusterDestroy.Exec([]string{})
	tests.Assert(t, err != nil)
	tests.Assert(t, err.Error() == "You need a server!\n")

}

func TestNewClusterDestroyFailTooManyArgs(t *testing.T) {
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
		Url: ts.URL,
	}

	//create b to get values of stdout
	var b bytes.Buffer
	defer tests.Patch(&stdout, &b).Restore()

	clusterDestroy := NewClusterDestroyCommand(options)
	err := clusterDestroy.Exec([]string{"too", "many", "args"})
	tests.Assert(t, err != nil)
	tests.Assert(t, err.Error() == "Too many arguments!")

}

func TestNewClusterDestroyFailTooLittleArgs(t *testing.T) {
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
		Url: ts.URL,
	}

	//create b to get values of stdout
	var b bytes.Buffer
	defer tests.Patch(&stdout, &b).Restore()

	clusterDestroy := NewClusterDestroyCommand(options)
	err := clusterDestroy.Exec([]string{})
	tests.Assert(t, err != nil)
	tests.Assert(t, err.Error() == "Not enough arguments!")

}

func TestNewClusterDestroyBadStatus(t *testing.T) {
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
		Url: ts.URL,
	}

	//create b to get values of stdout
	var b bytes.Buffer
	defer tests.Patch(&stdout, &b).Restore()

	//create mock add node
	clusterDestroy := NewClusterDestroyCommand(options)

	err := clusterDestroy.Exec([]string{"nah"})
	tests.Assert(t, err != nil)
	tests.Assert(t, strings.Contains(err.Error(), "404"), err.Error())

}

func TestClusterPostFailureClusterDestroy(t *testing.T) {
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

	options := &Options{
		Url: "http://nottherightthing:8080",
	}

	//create b so we can see stdout
	var b bytes.Buffer
	defer tests.Patch(&stdout, &b).Restore()

	//create cluster
	clusterDestroy := NewClusterDestroyCommand(options)
	tests.Assert(t, clusterDestroy != nil)

	//execute
	err := clusterDestroy.Exec([]string{"a"})
	tests.Assert(t, err != nil)
	tests.Assert(t, strings.Contains(b.String(), "Unable to send "), err.Error())
}

func TestNewClusterCreateNoServer(t *testing.T) {
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
		Url: "",
	}

	//create b to get values of stdout
	var b bytes.Buffer
	defer tests.Patch(&stdout, &b).Restore()

	clusterCreate := NewClusterCreateCommand(options)
	err := clusterCreate.Exec([]string{})
	tests.Assert(t, err != nil)
	tests.Assert(t, err.Error() == "You need a server!\n")

}

func TestNewClusterInfoNoServer(t *testing.T) {
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
		Url: "",
	}

	//create b to get values of stdout
	var b bytes.Buffer
	defer tests.Patch(&stdout, &b).Restore()

	clusterInfo := NewClusterInfoCommand(options)
	err := clusterInfo.Exec([]string{"ba"})
	tests.Assert(t, err != nil)
	tests.Assert(t, err.Error() == "You need a server!\n")

}

func TestNewClusterInfoFailTooManyArgs(t *testing.T) {
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
		Url: ts.URL,
	}

	//create b to get values of stdout
	var b bytes.Buffer
	defer tests.Patch(&stdout, &b).Restore()

	clusterInfo := NewClusterInfoCommand(options)
	err := clusterInfo.Exec([]string{"too", "many", "args"})
	tests.Assert(t, err != nil)
	tests.Assert(t, err.Error() == "Too many arguments!")

}

func TestNewClusterListNoServer(t *testing.T) {
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
		Url: "",
	}

	//create b to get values of stdout
	var b bytes.Buffer
	defer tests.Patch(&stdout, &b).Restore()

	clusterList := NewClusterListCommand(options)
	err := clusterList.Exec([]string{})
	tests.Assert(t, err != nil)
	tests.Assert(t, err.Error() == "You need a server!\n")

}

func TestNewClusterListFailTooManyArgs(t *testing.T) {
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
		Url: ts.URL,
	}

	//create b to get values of stdout
	var b bytes.Buffer
	defer tests.Patch(&stdout, &b).Restore()

	clusterList := NewClusterListCommand(options)
	err := clusterList.Exec([]string{"too", "many", "args"})
	tests.Assert(t, err != nil)
	tests.Assert(t, err.Error() == "Too many arguments!")

}
