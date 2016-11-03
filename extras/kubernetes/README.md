# Overview
Kubernetes templates for Heketi and Gluster. The following documentation is setup
to deploy the containers in Kubernetes.  It is not a full setup.  For full
documentation, please visit the Heketi wiki page.

# Usage

## Deploy Gluster

* Get node name by running:

```
$ kubectl get nodes
```

* Deploy gluster container onto specified node:

```
$ sed -e \
   's#<GLUSTERFS_NODE>#..type your node name here..#' \
   glusterfs-deployment.json | kubectl create -f -
```

Repeat as needed.

## Deploy Heketi

* Create a service account for Heketi

```
$ kubectl create -f heketi-service-account.json
```

* Note the secret for the service account 

```
$ heketi_secret=$(kubectl get sa heketi-service-account -o="go-template" --template="{{(index .secrets 0).name}}")
```

* Deploy deploy-heketi.  Before deploying you will need to determine the Kubernetes API endpoint and namespace.

In this example, we will use `https://1.1.1.1:443` as our Kubernetes API endpoint

```
$ sed -e "s#<HEKETI_KUBE_SECRETNAME>#\"$heketi_secret\"#" \
      -e "s#<HEKETI_KUBE_APIHOST>#\"http://1.1.1.1:443\"#" deploy-heketi-deployment.json | kubectl create -f -
```

Please refer to the wiki Kubernetes Deployment page for more information

