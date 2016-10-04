#!/bin/sh

TOP=../../..
CURRENT_DIR=`pwd`
FUNCTIONAL_DIR=${CURRENT_DIR}/..
RESOURCES_DIR=$CURRENT_DIR/resources
PATH=$PATH:$RESOURCES_DIR

source ${FUNCTIONAL_DIR}/lib.sh

# Setup Docker environment
eval $(minikube docker-env)

display_information() {
	# Display information
	echo -e "\nVersions"
	kubectl version

	echo -e "\nDocker containers running"
	docker ps

	echo -e "\nDocker images"
	docker images

	echo -e "\nShow nodes"
	kubectl get nodes
}

setup_all_pods() {

  echo -e "\nCreate a ServiceAccount"
	kubectl create -f ServiceAccount.yaml || fail "Unable to create a serviceAccount"

	KUBESEC=$(kubectl get secrets | grep seracc | awk 'NR==1{print $1}')

	KUBEAPI=https://$(minikube ip):8443

	# Start Heketi
	echo -e "\nStart Heketi container"
  sed 's\<ApiHost>\'"$KUBEAPI"'\g; s\<SecretName>\'"$KUBESEC"'\g' test-heketi-deployment.json | kubectl creat -f - --validate=false || fail "Unable to start heketi container"
	sleep 2

	# This blocks until ready
	kubectl expose deployment heketi --type=NodePort || fail "Unable to expose heketi service"

	echo -e "\nShow Topology"
	export HEKETI_CLI_SERVER=$(minikube service heketi --url)
	heketi-cli topology info

  echo -e "\nStart gluster container"
	kubectl run gluster1 --image=ashiq/glusterfs-mock-container --labels=glusterfs-node=gluster1 || fail "Unable to start gluster1"

	kubectl run gluster2 --image=ashiq/glusterfs-mock-container --labels=glusterfs-node=gluster2 || fail "Unable to start gluster2"

}

test_peer_probe() {
  echo -e "Get the Heketi server connection"
	heketi-cli cluster create || fail "Unable to create cluster"

	CLUSTERID=$(heketi-cli cluster list | sed -e '$!d')

	heketi-cli node add --zone=1 --cluster=$CLUSTERID --management-host-name=gluster1 --storage-host-name=gluster1 || fail "Unable to add gluster1"

	heketi-cli node add --zone=2 --cluster=$CLUSTERID --management-host-name=gluster2 --storage-host-name=gluster2 || fail "Unable to add gluster2"

}




display_information
setup_all_pods

echo -e "\n*** Start tests ***"
test_peer_probe

# Ok now start test
