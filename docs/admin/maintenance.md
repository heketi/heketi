# Contents
* [Overview](#overview)
* [Adding Capacity](#adding-capacity)
    * [Adding new devices](#adding-new-devices)
    * [Increasing cluster size](#increasing-cluster-size)
    * [Adding a new cluster](#adding-a-new-cluster)
* [Reducing Capacity](#reducing-capacity)
* [Replacing Nodes or Devices](#replacing-nodes-or-devices)


# Overview

Heketi allows administrators to add and remove storage capacity by managing
one or more GlusterFS clusters.

# Adding Capacity

There are multiple ways to add additional storage capacity using Heketi.
One can add new devices, increase the cluster size, or add an entirely
new cluster.

## Adding new devices

When adding more devices, please keep in mind to add devices as a set.
For example, if volumes are using replica 2 you should add a device to two
nodes (one device per node). If using replica 3, then add a device to three
nodes.

Devices can be added to nodes by directly accessing the
[Heketi API](../api/api.md).

Using the Heketi cli, a single device can be added to a node:

```
$ heketi-cli device add \
      --name=/dev/sdb \
      --node=3e098cb4407d7109806bb196d9e8f095
```

A much simpler way to add many devices at once is to add the new device
to the node description in your topology file used to setup the cluster.
Then rerun the command to load the new topology.
Here is an example where we added a new `/dev/sdj` drive to the node:

```
$ heketi-cli topology load --json=topology.json
...
        Found node 192.168.10.100 on cluster 3e21671bc4f290fca6bce464ae7bb6e7
                Found device /dev/sdb
                Found device /dev/sdc
                Found device /dev/sdd
                Found device /dev/sde
                Found device /dev/sdf
                Found device /dev/sdg
                Found device /dev/sdh
                Found device /dev/sdi
                Adding device /dev/sdj ... OK
...
```

## Increasing cluster size

In addition to adding new devices to existing nodes new nodes can be
added to the cluster. As with devices one can add a new node to an
existing cluster by either using the [API](../api/api.md), using the cli,
or modifying your topology file.

The following shows an example of how to add a new node using the cli:

```
$ heketi-cli node add \
      --zone=3 \
      --cluster=3e21671bc4f290fca6bce464ae7bb6e7 \
      --management-host-name=node1-manage.gluster.lab.com \
      --storage-host-name=172.18.10.53

Node information:
Id: e0017385b683c10e4166492e78832d09
State: online
Cluster Id: 3e21671bc4f290fca6bce464ae7bb6e7
Zone: 3
Management Hostname node1-manage.gluster.lab.com
Storage Hostname 172.18.10.53

$ heketi-cli device add \
      --name=/dev/sdb \
      --node=e0017385b683c10e4166492e78832d09
Device added successfully

$ heketi-cli device add \
      --name=/dev/sdc \
      --node=e0017385b683c10e4166492e78832d09
Device added successfully
```

## Adding a new cluster

Storage capacity can also be increased by adding new clusters of GlusterFS.
Just as before, one can use the [API](../api/api.md) directly, use the
`heketi-cli` to manually add clusters, nodes and devices, or create another
topology file to define the new nodes and devices which will compose this
cluster.

# Reducing Capacity

Heketi also supports the reduction of storage capacity. This is possible
by deleting devices, nodes, and clusters. These changes can be
performed  using the [API](../api/api.md) directly or by using `heketi-cli`.
Here is an example of how to delete devices with no volumes from Heketi:

```
sh-4.2$ heketi-cli topology info
 
Cluster Id: 6fe4dcffb9e077007db17f737ed999fe
 
    Volumes:
 
    Nodes:
 
        Node Id: 61d019bb0f717e04ecddfefa5555bc41
        State: online
        Cluster Id: 6fe4dcffb9e077007db17f737ed999fe
        Zone: 1
        Management Hostname: gprfc053.o.internal
        Storage Hostname: 172.18.10.53
        Devices:
                Id:e4805400ffa45d6da503da19b26baad6   Name:/dev/sdc            State:online    Size (GiB):279     Used (GiB):0       Free (GiB):279
                        Bricks:
                Id:ecc3c65e4d22abf3980deba4ae90238c   Name:/dev/sdd            State:online    Size (GiB):279     Used (GiB):0       Free (GiB):279
                        Bricks:
 
        Node Id: e97d77d0191c26089376c78202ee2f20
        State: online
        Cluster Id: 6fe4dcffb9e077007db17f737ed999fe
        Zone: 2
        Management Hostname: gprfc054.o.internal
        Storage Hostname: 172.18.10.54
        Devices:
                Id:3dc3b3f0dfd749e8dc4ee98ed2cc4141   Name:/dev/sdd            State:online    Size (GiB):279     Used (GiB):0       Free (GiB):279
                        Bricks:
                Id:4122bdbbe28017944a44e42b06755b1c   Name:/dev/sdc            State:online    Size (GiB):279     Used (GiB):0       Free (GiB):279
                        Bricks:
                Id:b5333d93446565243f1a7413be45292a   Name:/dev/sdb            State:online    Size (GiB):279     Used (GiB):0       Free (GiB):279
                        Bricks:
sh-4.2$
sh-4.2$ d=`heketi-cli topology info | grep Size | awk '{print $1}' | cut -d: -f 2`
sh-4.2$ for i in $d ; do
> heketi-cli device delete $i
> done
Device e4805400ffa45d6da503da19b26baad6 deleted
Device ecc3c65e4d22abf3980deba4ae90238c deleted
Device 3dc3b3f0dfd749e8dc4ee98ed2cc4141 deleted
Device 4122bdbbe28017944a44e42b06755b1c deleted
Device b5333d93446565243f1a7413be45292a deleted
sh-4.2$ heketi-cli node delete $node1
Node 61d019bb0f717e04ecddfefa5555bc41 deleted
sh-4.2$ heketi-cli node delete $node2
Node e97d77d0191c26089376c78202ee2f20 deleted
sh-4.2$ heketi-cli cluster delete $cluster
Cluster 6fe4dcffb9e077007db17f737ed999fe deleted
```

# Replacing Nodes or Devices

A node or device can be replaced if it has failed or needs to be swapped out
as part of proactive maintenance. All bricks located on the node or device will
be replaced with new bricks on different devices. There must be enough free
space on other devices to support these new bricks and the same constraints on
volume replication count must be obeyed. For example, trying to remove a device
from a node in a three node cluster, where each node has exactly one device
will fail. This is due to the fact that a replica-3 volume requires the bricks
(in each replica set) to be hosted by a different node. It may be necessary to
increase overall cluster capacity before running through the replacement
process.

If you are going to be replacing multiple nodes or devices together or in
quick succession it is typically better to mark all of the nodes and devices
that you do not want to be used for new bricks as 'offline' prior to
removing any one device or node. This is especially true if performing
maintenance and the devices are functioning and could accept new bricks.
Heketi will not place a new brick on a node or device in the 'offline' state.
Putting all unwanted devices offline together prevents the scenario where
Heketi moves a brick from device a to b, and then from b to c if both
a and b are being removed.

Removing a node is effectively the same as removing all the devices on
the node in one pass. Setting a node offline or failed will make all the
devices attached to that node offline or failed.

## Remove a single device

To remove a device, replacing all the bricks on that device with new ones
on other devices, the device must first be put in the offline state:
```
heketi-cli device disable <device-id>
```

An offline device is not allowed to accept new bricks but Heketi makes
no other changes to the device.

To fully remove the bricks from the device, execute:
```
heketi-cli device remove <device-id>
```

When this command succeeds all bricks should have been removed from the
device. This can be confirmed using `heketi-cli topology info`.
If the command fails it may be that there was insufficient space on the
cluster or that some of the constraints on brick placement
(insufficient nodes, etc.) could not be met. Check the error message and/or
the Heketi log for more details.

## Remove a single node

To remove a node, replacing all the bricks on all devices on that node with new
ones on other devices, the node must first be put in the offline state:
```
heketi-cli node disable <node-id>
```

An offline node is not allowed to accept new bricks but Heketi makes
no other changes to the node or it's devices.

To fully remove the bricks from the node, execute:
```
heketi-cli node remove <node-id>
```

When this command succeeds all bricks should have been removed from the
node. This can be confirmed using `heketi-cli topology info`.
If the command fails it may be that there was insufficient space on the
cluster or that some of the constraints on brick placement
(insufficient nodes, etc.) could not be met. Check the error message and/or
the Heketi log for more details.

## Replace a single brick

In some circumstances it may be useful to replace only a single specific brick.
In this scenario the brick id can be acquired using `heketi-cli topology info`
and then passed to the 'brick eviction' command. A brick may be evicted from
a device that is online, but Heketi will always obey the same rules noted above
and will not place the brick on an offline device or node and will conform
to all placement constraints. Brick eviction is new in Heketi version 10.

To evict a given brick, run:
```
heketi-cli brick evict <brick-id>
```

Please note that brick eviction only allows the user to directly control what
brick to remove. The brick will automatically be replaced just like the device
remove case.

## Deleting nodes and devices

When a device is free of bricks the device can be removed from the cluster
topology in the Heketi database. When a node is free of devices it can be
removed from the cluster topology in the Heketi database.

To delete a device, run:
```
heketi-cli device delete <device-id>
```

To delete a node, run:
```
heketi-cli node delete <node-id>
```

