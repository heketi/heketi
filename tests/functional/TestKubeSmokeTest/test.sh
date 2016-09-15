#!/bin/sh

TOP=../../..
CURRENT_DIR=`pwd`
FUNCTIONAL_DIR=${CURRENT_DIR}/..
RESOURCES_DIR=$CURRENT_DIR/resources
PATH=$PATH:$RESOURCES_DIR

source ${FUNCTIONAL_DIR}/lib.sh

# Setup Docker environment
eval $(minikube docker-env) 

# Display information
echo -e "\nVersions"
kubectl version

echo -e "\nDocker containers running"
docker ps

echo -e "\nDocker images"
docker images

echo -e "\nShow nodes"
kubectl get nodes

# Start Heketi
echo -e "\nStart Heketi container"
kubectl run heketi --image=heketi/heketi --port=8080 || fail "Unable to start heketi container"
sleep 2

# This blocks until ready
kubectl expose deployment heketi --type=NodePort || fail "Unable to expose heketi service"
export HEKETI_CLI_SERVER=$(minikube service heketi --url)
ncat_server="echo $HEKETI_CLI_SERVER | sed -e 's#:# #'"

echo -e "\nShow Cluster List"
heketi-cli cluster list

