## Overview

This guide enables the integration, deployment, and management of GlusterFS containerized storage nodes in a Kubernetes cluster. This enables Kubernetes administrators to provide their users with reliable, shared storage.

There are other guides available, as well. The [gluster-kubernetes](https://github.com/gluster/gluster-kubernetes), includes a [setup guide](https://github.com/gluster/gluster-kubernetes/blob/master/docs/setup-guide.md) for running most of the directions in this guide through a deployment tool called [gk-deploy](https://github.com/gluster/gluster-kubernetes/blob/master/deploy/gk-deploy). It also includes a [Hello World](https://github.com/gluster/gluster-kubernetes/tree/master/docs/examples/hello_world) featuring an nginx pod using a dynamically-provisioned GlusterFS volume for storage. For those looking to test or play around with this quickly, follow the Quickstart instructions found in the main [README](https://github.com/gluster/gluster-kubernetes) for gluster-kubernetes

## Infrastructure Requirements

* A running Kubernetes cluster with at least three Kubernetes worker nodes that each have an available raw block device attached (like an EBS Volume or a local disk).
* The three Kubernetes nodes intended to run the GlusterFS Pods must have the appropriate ports opened for GlusterFS communication. Run the following commands on each of the nodes.
```
iptables -N HEKETI
iptables -A HEKETI -p tcp -m state --state NEW -m tcp --dport 24007 -j ACCEPT
iptables -A HEKETI -p tcp -m state --state NEW -m tcp --dport 24008 -j ACCEPT
iptables -A HEKETI -p tcp -m state --state NEW -m tcp --dport 2222 -j ACCEPT
iptables -A HEKETI -p tcp -m state --state NEW -m multiport --dports 49152:49251 -j ACCEPT
service iptables save
```

## Client Setup

Heketi provides a CLI that provides users with a means to administer the deployment and configuration of GlusterFS in Kubernetes. [Download and install the heketi-cli](https://github.com/heketi/heketi/releases) on your client machine (usually your laptop).

## Kubernetes Deployment
All the following files are located under `extras/kubernetes`.

* Deploy the GlusterFS DaemonSet

```
$ kubectl create -f glusterfs-daemonset.json
```

* Get node name by running:

```
$ kubectl get nodes
```

* Deploy gluster container onto specified node by setting the label `storagenode=glusterfs` on that node

```
$ kubectl label node <...node...> storagenode=glusterfs
```

Repeat as needed.  Verify that the pods are running on the nodes.

```
$ kubectl get pods
```

* Next we need to deploy the Pod and Service Heketi Service Interface to the GlusterFS cluster. In the repo you cloned, there will be a deploy-heketi-deployment.json file. 

Submit the file and verify everything is running properly as demonstrated below:

```
# kubectl create -f deploy-heketi-deployment.json 
service "deploy-heketi" created
deployment "deploy-heketi" created

# kubectl get pods
NAME                                                      READY     STATUS    RESTARTS   AGE
deploy-heketi-1211581626-2jotm                            1/1       Running   0          35m
glusterfs-ip-172-20-0-217.ec2.internal-1217067810-4gsvx   1/1       Running   0          1h
glusterfs-ip-172-20-0-218.ec2.internal-2001140516-i9dw9   1/1       Running   0          1h
glusterfs-ip-172-20-0-219.ec2.internal-2785213222-q3hba   1/1       Running   0          1h
```

* Now that the Bootstrap Heketi Service is running we are going to configure port-fowarding so that we can communicate with the service using the Heketi CLI. Using the name of the Heketi pod, run the command below:

`kubectl port-forward deploy-heketi-1211581626-2jotm :8080`

Now verify that the port forwarding is working, by running a sample query againt the Heketi Service. The command should return a local port that it will be forwarding from. Incorporate that into a localhost query to test the service, as demonstrated below:

```
curl http://localhost:57598/hello
Handling connection for 57598
Hello from Heketi
```

Lastly, set an environment variable for the Heketi CLI client so that it knows how to reach the Heketi Server.

`export HEKETI_CLI_SERVER=http://localhost:57598`

* Next we are going to provide Heketi with the information about the GlusterFS cluster it is to manage. We provide this information via [a topology file](./topology.md). There is a sample topology file within the repo you cloned called topology-sample.json. Topologies primarily specify what Kubernetes Nodes the GlusterFS containers are to run on as well as the corresponding available raw block device for each of the nodes.

> NOTE: Make sure that `hostnames/manage` points to the exact name as shown under `kubectl get nodes`, and `hostnames/storage` is the ip address of the storage network.

Modify the topology file to reflect the choices you have made and then deploy it as demonstrated below:

```
heketi-client/bin/heketi-cli topology load --json=topology-sample.json
Handling connection for 57598
	Found node ip-172-20-0-217.ec2.internal on cluster e6c063ba398f8e9c88a6ed720dc07dd2
		Adding device /dev/xvdg ... OK
	Found node ip-172-20-0-218.ec2.internal on cluster e6c063ba398f8e9c88a6ed720dc07dd2
		Adding device /dev/xvdg ... OK
	Found node ip-172-20-0-219.ec2.internal on cluster e6c063ba398f8e9c88a6ed720dc07dd2
		Adding device /dev/xvdg ... OK
```

* Next we are going to use Heketi to provision a volume for it to storage its database:

```
# heketi-client/bin/heketi-cli setup-openshift-heketi-storage
# kubectl create -f heketi-storage.json
```

Wait until the job is complete then delete the bootstrap Heketi:

```
# kubectl delete all,service,jobs,deployment,secret --selector="deploy-heketi" 
```

* Submit Heketi:

```
# kubectl create -f heketi-deployment.json 
service "heketi" created
deployment "heketi" created
```

# Usage Example

There are two ways to provision storage.  The primary way is to setup StorageClass which lets Kubernetes automatically provision storage for an PersistentVolumeClaim submitted.  Another is to manually submit and create the volumes.
