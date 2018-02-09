# Fix database orphan bricks

This python script will help you to remove entries of orphan bricks in your heketi database. To make it efficiently, you'll need to follow instructions.

## Use latest heketi build

You will need "db" command to export/import database in json. This command was added to heketi in #959 pull request. It was merged into master branch.

Clone or update your repository inside your GOPATH:

```
# if you didn't already cloned heketi
$ mkdir -p $GOPATH/src/github.com/heketi/heketi
$ git clone https://github.com/heketi/heketi.git $GOPATH/src/github.com/heketi/heketi
$ cd $GOPATH/src/github.com/heketi/heketi
$ make server
```

You should now have `heketi` server binary.

## Copy your heketi database

First, you need to stop heketi database. If you are using it on OpenShift/Kubernetes, you can set replicas number to 0 on the deployment or deploymentConfig:

```
# ocp
oc scale dc heketi --replicas=0
# k8s
kubectl scale deployment heketi --replicas=0
```

When heketi Pod is terminated you may now copy database somewhere you will work. That could be a server or your personnal computer.

Heketi database resides in `heketidbstorage` volume that you can mount with glusterfs. So, you can mount that volume on you computer, eg:

```
$ mkdir /tmp/heketidbstorage
$ mount -t glusterfs 192.168.1.2:heketidbstorage /tmp/heketidbstorage
$ ls /tmp/heketidbstorage
heketi.db
```

Now, copy that database:
```
mkdir ~/heketi
cp /tmp/heketidbstorage/heketi.db ~/heketi
cd ~/heketi
```

Then, you may export database in json:

```
$GOPATH/src/github.com/heketi/heketi/heketi db export --dbfile heketi.db --jsonfile heketi-from.json 
```


## Fix entries and remove LVM volumes

Now, you may use the python script to fixup database. You'll need to keep logs in a file if you want to fixum LVM in Gluster peers.

```
$ python $GOPATH/src/github.com/heketi/heketi/extras/misc/fix-orphans/fix.py --input jeketi-from.json --ouput heketi-fixed.json 
```

The output will show you Ids of bricks and devices where resides that orphans brick. What you need to do is to remove that logical volumes in peers:

```
# eg
$ BRICK_ID="49deb577a3c3d798ae12e6af8d896df5"
$ lvs | grep ${BRICK_ID}
$ umount /var/lib/heketi/mounts/brick_${BRICK_ID}
$ lvremove -f /dev/mapper/vg_xxxxxx/tp_${BRICK_ID}
$ lvremove -f /dev/mapper/vg_xxxxxx/brick_${BRICK_ID}
$ sed -i.save '/${BRICK_ID}/d' /var/lib/heketi/fstab
```

You must to do this (check if brick exists, umount, remove LV, remove fstab entry...) in **each** gluster peer.

## Rebuild datase and restart heketi

Now you can rebuild database and replace the old one:

```
$ heketi db import --jsonfile heketi-fixed.json --dbfile heketi-new.db
$ cp heketi-new.db /tmp/heketidbstorage/heketi.db
$ umount /tmp/heketidbstorage
```

Restart heketi or rescale to 1 deployment or deploymentConfig:

```
# ocp
oc scale dc heketi --replicas=1
# k8s
kubectl scale deployment heketi --replicas=1
```

Wait for heketi to be started and then:

- check topology - you should not have bricks with empty path and/or no volume
- check that each node is "online" (if state is offline, you may use `heketi node enable $ID`)
- create a volume and check logs. Then remove that volume and recheck logs one more time


