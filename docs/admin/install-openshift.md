# Overview

![overview](https://github.com/heketi/heketi/wiki/images/aplo_arch.png)

This guide enables the integration, deployment, and management of GlusterFS containerized storage nodes in an OpenShift cluster.  This enables OpenShift administrators to provide their users with reliable, shared storage.

# Demo
[![demo](https://github.com/heketi/heketi/wiki/images/aplo_demo.png)](https://asciinema.org/a/50531)

For simplicity, you can deploy an OpenShift cluster using the configured [Heketi Vagrant Demo](https://github.com/heketi/vagrant-heketi)

# Requirements

* OpenShift cluster must be up and running
* OpenShift router must be created and DNS setup
* A cluster-admin user created
* At least three OpenShift nodes must be storage nodes with at least one raw device per node
* Non-Atomic systems: All OpenShift nodes must have glusterfs-client RPM installed which should match the version of GlusterFS server running in the containers
* OpenShift nodes which are to be used with GlusterFS must have the correct ports opened for GlusterFS communication. On each Openshift minion that will host a glusterfs container, add the following rules to `/etc/sysconfig/iptables`:

```
-A OS_FIREWALL_ALLOW -p tcp -m state --state NEW -m tcp --dport 24007 -j ACCEPT
-A OS_FIREWALL_ALLOW -p tcp -m state --state NEW -m tcp --dport 24008 -j ACCEPT
-A OS_FIREWALL_ALLOW -p tcp -m state --state NEW -m tcp --dport 2222 -j ACCEPT
-A OS_FIREWALL_ALLOW -p tcp -m state --state NEW -m multiport --dports 49152:49251 -j ACCEPT
```

# Setup
The following setup assumes that you will be using a client machine to communicate with the OpenShift cluser as shown:

![setup](https://github.com/heketi/heketi/wiki/images/aplo_install.png)

## Client Setup
* Install the following packages in Fedora/CentOS(EPEL): `heketi-templates heketi-client`
* For other Linux systems or MacOS X, install from the [releases page](https://github.com/heketi/heketi/releases/tag/v2.0.6)

## Deployment

Create a project for the storage containers:

```
$ oc new-project aplo
```

Make sure to allow privileged containers in the new project (on Atomic, you may need to do this command on the master for the `aplo` project). On the master, run the following:

```
$ oc project aplo
$ oadm policy add-scc-to-user privileged -z default
```

Register the Heketi and GlusterFS OpenShift templates:

```
$ oc create -f /usr/share/heketi/templates
```

Show the node names:

```
$ oc get nodes
```

Deploy GlusterFS container by doing the following for each node which will run GlusterFS:

```
$ oc process glusterfs -v GLUSTERFS_NODE=<name of a node to use glusterfs as shown above> | oc create -f -
```

Deploy the bootstrap Heketi container which will be used to setup the database:

```
$ oc process deploy-heketi -v \
         HEKETI_KUBE_NAMESPACE=aplo \
         HEKETI_KUBE_APIHOST=https://<host of OpenShift master and port> \
         HEKETI_KUBE_INSECURE=y \
         HEKETI_KUBE_USER=<cluster admin username> \
         HEKETI_KUBE_PASSWORD=<cluster admin password> | oc create -f -
```

Wait until the deploy-heketi pod is running, then note the exported name for your the deploy-heketi service:

```
$ oc status
```

Wait until all services are up, check Heketi communication:

```
$ curl http://<address to deploy-heketi service>/hello
```

It should return:

```
Hello from Heketi
```

Register your [Topology](./topology.md) with the bootstrap Heketi:

```
$ heketi-cli -s http://<address to deploy-heketi service> topology load --json=<topology file>
```

Setup storage volume for Heketi database:

```
$ heketi-cli -s http://<address to deploy-heketi service> setup-openshift-heketi-storage
$ oc create -f heketi-storage.json
```

Wait until the job is complete then delete the bootstrap Heketi:

```
$ oc delete all,job,template,secret --selector="deploy-heketi"
```

Install Heketi service:

```
$ oc process heketi -v \
         HEKETI_KUBE_NAMESPACE=aplo \
         HEKETI_KUBE_APIHOST=https://<host of OpenShift master and port> \
         HEKETI_KUBE_INSECURE=y \
         HEKETI_KUBE_USER=<cluster admin username> \
         HEKETI_KUBE_PASSWORD=<cluster admin password> | oc create -f -
```

Wait until the Heketi service is up, then note the exported DNS name for Heketi:

```
$ oc status
```

# Usage Example

Normally you would need to setup another project for an applications, and setup the appropriate [endpoints and services](https://github.com/kubernetes/kubernetes/tree/master/examples/glusterfs).  As an example, you will create an application in the same project as above using the endpoints and service already registered.

Download the example application:

```
$ wget https://raw.githubusercontent.com/lpabon/aplo-demo/master/nginx.yml
```

Because nginx container sets up a USER in their Dockerfile, you will need to run the following command on the master:

```
$ oadm policy add-scc-to-group anyuid system:authenticated
```

Create a 100G volume for nginx:

```
$ export HEKETI_CLI_SERVER=http://<address to heketi service>
$ heketi-cli volume create --size=100 \
  --persistent-volume \
  --persistent-volume-endpoint=heketi-storage-endpoints | oc create -f -
```

Bring up nginx application:

```
$ cat nginx.yml | oc create -f -
```

Wait until the nginx application is up and running, then note the exported DNS name for the nginx service:

```
$ oc status
```

Create a sample `index.html` file:

```
$ oc rsh nginx
# df -h
# echo 'I love GlusterFS!' > /usr/share/nginx/html/index.html
# exit
$ curl http://<address to nginx service>
I love GlusterFS!
```
