# db edit commands

All db editing commands are provided in heketi binary. This is to highlight the restriction that db editing must be performed only when heketi server isn't running. If the commands are performed when the server is using the database, "Unable to open database: timeout" error is shown.

IMPORTANT NOTE:
All operations on the db must be performed when heketi server isn't operational.  
Procudure to be followed:
1. Shutdown the heketi service. In container platforms, do so by scale down pods to zero.
2. mount the gluster volume "heketidbstorage" on node.
3. Backup the heketi.db 
4. edit the db using commands given below.
5. umount gluster volume heketidbstorage
6. start heketi service. In container platforms, do so by scaling up the pod to 1.
```
# mount -t glusterfs NODEIP:/heketidbstorage /mnt
# cp /mnt/heketi.db /mnt/heketi.db.bak
# heketi db <commands ...>
# umount /mnt
```

## export command
export creates a JSON file from a db file. Use this to check contents of heketi db without using the API. Also note, this provides complete content of the db including DB attributes bucket that heketi uses internally to keep backward compatibility and to version the database. This also includes the contents of pending operations bucket that lists operations which are ongoing or got interrupted.

```
# heketi db export [flags]
```

### flags

```
--dbfile string     File path for db to be exported
--debug             Show debug logs on stdout
-h, --help          help for export
--jsonfile string   File path for JSON file to be created
```

dbfile and jsonfile flags are mandatory and the path for json file should not exist.

Examples

```
# heketi db export --jsonfile=heketi-db.json --dbfile=heketi.db
DB exported to heketi-db-single.json
```

This will create an unindented json file with contents that reflect the contents of the database. To get an indented json file use jq or json module of python as shown below

* cat heketi-db.json | jq "." > formatted.json

* cat heketi-db.json | python -m json.tool > formatted.json

```
{
  "clusterentries": {
    "36ac90baf26e1965feb15fe1d6e302c6": {
      "Info": {
        "id": "36ac90baf26e1965feb15fe1d6e302c6",
        "nodes": [
          "fb3862d51210e7fc5aff87a7aba87b63"
        ],
        "volumes": [
          "6955761dd774ceec809a9dce927da6e8"
        ],
        "block": true,
        "file": true,
        "blockvolumes": []
      }
    }
  },
  "volumeentries": {
    "6955761dd774ceec809a9dce927da6e8": {
      "Info": {
        "size": 5,
        "name": "vol_6955761dd774ceec809a9dce927da6e8",
        "durability": {
          "type": "none",
          "replicate": {
            "replica": 3
          },
          "disperse": {
            "data": 4,
            "redundancy": 2
          }
        },
        "snapshot": {
          "enable": false,
          "factor": 1
        },
        "id": "6955761dd774ceec809a9dce927da6e8",
        "cluster": "36ac90baf26e1965feb15fe1d6e302c6",
        "mount": {
          "glusterfs": {
            "hosts": [
              "192.168.55.14"
            ],
            "device": "192.168.55.14:vol_6955761dd774ceec809a9dce927da6e8",
            "options": {
              "backup-volfile-servers": ""
            }
          }
        },
        "blockinfo": {}
      },
      "Bricks": [
        "8435071a6acbfd905c00ff9ee627d940"
      ],
      "GlusterVolumeOptions": null,
      "Pending": {
        "Id": ""
      }
    }
  },
  "brickentries": {
    "8435071a6acbfd905c00ff9ee627d940": {
      "Info": {
        "id": "8435071a6acbfd905c00ff9ee627d940",
        "path": "/var/lib/heketi/mounts/vg_03532e41f85f7e211eac104e3c4096d7/brick_8435071a6acbfd905c00ff9ee627d940/brick",
        "device": "03532e41f85f7e211eac104e3c4096d7",
        "node": "fb3862d51210e7fc5aff87a7aba87b63",
        "volume": "6955761dd774ceec809a9dce927da6e8",
        "size": 5242880
      },
      "TpSize": 5242880,
      "PoolMetadataSize": 28672,
      "Pending": {
        "Id": ""
      }
    }
  },
  "nodeentries": {
    "fb3862d51210e7fc5aff87a7aba87b63": {
      "State": "online",
      "Info": {
        "zone": 1,
        "hostnames": {
          "manage": [
            "glusterblockcluster1node2.obnox_vagrant_dev.rastarnet"
          ],
          "storage": [
            "192.168.55.14"
          ]
        },
        "cluster": "36ac90baf26e1965feb15fe1d6e302c6",
        "id": "fb3862d51210e7fc5aff87a7aba87b63"
      },
      "Devices": [
        "03532e41f85f7e211eac104e3c4096d7"
      ]
    }
  },
  "deviceentries": {
    "03532e41f85f7e211eac104e3c4096d7": {
      "State": "online",
      "Info": {
        "name": "/dev/vdb",
        "storage": {
          "total": 524288000,
          "free": 519016448,
          "used": 5271552
        },
        "id": "03532e41f85f7e211eac104e3c4096d7"
      },
      "Bricks": [
        "8435071a6acbfd905c00ff9ee627d940"
      ],
      "NodeId": "fb3862d51210e7fc5aff87a7aba87b63",
      "ExtentSize": 4096
    }
  },
  "blockvolumeentries": {},
  "dbattributeentries": {
    "DB_CLUSTER_HAS_FILE_BLOCK_FLAG": {
      "Key": "DB_CLUSTER_HAS_FILE_BLOCK_FLAG",
      "Value": "yes"
    },
    "DB_GENERATION_ID": {
      "Key": "DB_GENERATION_ID",
      "Value": "c9e784e354e040d60b1d98efe8c3d419"
    },
    "DB_HAS_PENDING_OPS_BUCKET": {
      "Key": "DB_HAS_PENDING_OPS_BUCKET",
      "Value": "yes"
    }
  },
  "pendingoperations": {}
}
```

## import command
import creates a db file from JSON input

```
# heketi db import [flags]
```

### flags

```
--dbfile string     File path for db to be exported
--debug             Show debug logs on stdout
-h, --help          help for export
--jsonfile string   File path for JSON file to be created
```

dbfile and jsonfile flags are mandatory and the path for db file should not exist.

Examples

```
# heketi db import --jsonfile=heketi-db.json --dbfile=heketi-new.db  
DB imported to heketi-new.db
```

This will create a db file with contents that reflect the contents of the json file.

## delete-bricks-with-empty-path command
delete-bricks-with-empty-path command removes brick entries from db that have empty path. It also adds back free space into the device from which the brick is removed. In earlier versions of heketi if heketi process was killed during creation of a volume, it would have left bricks that were created but not part of any volume in heketi db. To clean such bricks you may use this command. To identify such bricks use the following command:

```
# heketi-cli topology info | grep ".*Path:[\ \]$"
                                Id:8ab5071a6acbfd905c00ff9ee627d940   Size (GiB):5       Path: 
```

```
# heketi db delete-bricks-with-empty-path [flags]
```

### flags


```
    --all                    if set true, then all bricks with empty path are removed
    --clusters stringSlice   comma separated list of cluster IDs
    --dbfile string          File path for db to operate on
    --debug                  Show debug logs on stdout
    --devices stringSlice    comma separated list of device IDs
-h, --help                   help for delete-bricks-with-empty-path
    --nodes stringSlice      comma separated list of node IDs
```


Examples

```
# heketi db delete-bricks-with-empty-path --dbfile ./heketi.db 
neither --all flag nor list of clusters/nodes/devices is given

# heketi db delete-bricks-with-empty-path --dbfile ./heketi.db --all
bricks with empty path removed

# heketi db delete-bricks-with-empty-path --dbfile ./heketi.db --clusters 36ac90baf26e1965feb15fe1d6e302c6
bricks with empty path removed 

# heketi db delete-bricks-with-empty-path --dbfile ./heketi.db --nodes 98ac90baf26e1965feb15fe1d6e302e3,acac90baf26e1965feb15fe1d6e302fd
bricks with empty path removed 

# heketi db delete-bricks-with-empty-path --dbfile ./heketi.db --devices 98ac90baf26e1965feb15fe1d6e302e3,acac90baf26e1965feb15fe1d6e302fd --clusters 36ac90baf26e1965feb15fe1d6e302c6
bricks with empty path removed 
```


## delete-pending-entries
delete-pending-entries removes entries from db that have pending attribute set. Heketi version 6.0 onward, if heketi process is terminated while an operation is being performed incomplete operations would be saved in pending operations bucket. When restarted, heketi process uses this information to differentiate completed objects from incomplete ones. Such pending objects can be removed from heketi database using this command and later cleaned up on the nodes. The command has no intelligence and removes only the objects that have pending attribute set without cleaning references to the object in other object entries. This will be fixed in upcoming releases.


```
#   heketi db delete-pending-entries [flags]
```

### flags


```
--dbfile string   File path for db to operate on
--debug           Show debug logs on stdout
--dry-run         Show actions that would be performed but don't perform them
--force           Clean entries even if they don't have an owner Pending Operation, incompatible with dry-run
-h, --help            help for delete-pending-entries
```


Examples

```
# heketi db delete-pending-entries --dbfile heketi.db --dry-run
# heketi db delete-pending-entries --dbfile heketi.db
```