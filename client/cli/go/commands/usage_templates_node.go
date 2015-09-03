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

var usageTemplateNode = `Node is a command used for managing Heketi nodes.

Usage:
    heketi -server [server] [options] node [subcommand]

The subcommands are:
    add        Adds a new node to the designated cluster.
    info       Returns information about the specific node.
    destroy    Destroys node with specified id.

Use "heketi node [subcommand] -help" for more information about a subcommand

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
