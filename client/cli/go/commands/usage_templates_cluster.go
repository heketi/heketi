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
