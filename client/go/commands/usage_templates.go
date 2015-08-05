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

var usageTemplateCluster = `Cluster is a command used for managing heketi clusters.

Usage:

    heketi -server [server] [options] cluster [subcommand]

The subcommands are:
    
    create         Creates a new cluster for Heketi to manage.
    list           Returns a list of all clusters on the specified server.
    info [id]      Returns information about a specific cluster.
    destroy [id]   Destroys cluster with specified id. 

Use "heketi cluster [subcommand] -help" for more information about a subcommand

`

var usageTemplateClusterDestroy = `cluster destroy is a command used for destroying heketi clusters.

Usage:

    heketi -server [server] [options] cluster destroy [args] 

The args are:

    cluster id    The id of the cluster you want to destroy

Example:

    heketi -server http://localhost:8080 cluster destroy 0854b5f5405cac5280c7dc479cd0e7fb

`

var usageTemplateClusterCreate = `cluster create is a command used for creating heketi clusters.

Usage:

    heketi -server [server] [options] cluster create 

This command takes no arguments.

Example:

    heketi -server http://localhost:8080 cluster create

`

var usageTemplateClusterInfo = `cluster info is a command used for getting more information about a cluster.

Usage:

    heketi -server [server] [options] cluster info [args] 

The args are:

    cluster id    The id of the cluster you want more information about.

Example:

    heketi -server http://localhost:8080 cluster info 0854b5f5405cac5280c7dc479cd0e7fb

`

var usageTemplateClusterList = `cluster list is a command used for getting a list of clusters on the specified server.

Usage:

    heketi -server [server] [options] cluster list 

This command takes no arguments.

Example:

    heketi -server http://localhost:8080 cluster list

`
var usageTemplateNode = `Node is a command used for managing Heketi nodes.

Usage:
    heketi -server [server] [options] node [subcommand]

The subcommands are:
    add        Adds a new node to the designated cluster.
    info       Returns information about the specific node.
    destroy    Destroys node with specified id.

Use "heketi node [subcommand] -help" for more information about a subcommand

`
var usageTemplateNodeAdd = `node add is a command used for adding a node to the specified cluster.

Usage:
    heketi -server [server] [options] node add [flags]

The flags are:
    -zone                  The zone in which the node should reside.
    -cluster               The cluster in which the node should reside.
    -managment-host-name   List of node managment hostnames.
    -storage-host-name     List of node storage hostnames.

Example:

    heketi -server http://localhost:8080 node add -zone 3 -cluster 3e098cb4407d7109806bb196d9e8f095 -managment-host-name node1-manage.gluster.lab.com -storage-host-name node1-storage.gluster.lab.com
`
var usageTemplateNodeDestroy = `node destroy is a command used for destroying a node.

Usage:
    heketi -server [server] [options] node add [args]

The args are:
    node id     The id of the node you want to destroy

Example:

    heketi -server http://localhost:8080 node destroy 22d4d9fe40ac1d24805af036dc820657

`
var usageTemplateNodeInfo = `node info is a command used for getting more information about a node.

Usage:

    heketi -server [server] [options] node info [args] 

The args are:

    node id    The id of the node you want to to get more information about

Example:

    heketi -server http://localhost:8080 node info 22d4d9fe40ac1d24805af036dc820657

`
