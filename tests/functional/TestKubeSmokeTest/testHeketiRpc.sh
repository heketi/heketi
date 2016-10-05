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

  kubectl get nodes --show-labels

  echo -e "\nCreate a ServiceAccount"
	kubectl create -f ServiceAccount.yaml || fail "Unable to create a serviceAccount"

	KUBESEC=$(kubectl get secrets | grep seracc | awk 'NR==1{print $1}')

	KUBEAPI=https://$(minikube ip):8443

	# Start Heketi
	echo -e "\nStart Heketi container"
  sed 's\<ApiHost>\'"$KUBEAPI"'\g; s\<SecretName>\'"$KUBESEC"'\g' test-heketi-deployment.json | kubectl create -f - --validate=false || fail "Unable to start heketi container"
	sleep 30

	# This blocks until ready
	kubectl expose deployment heketi --type=NodePort || fail "Unable to expose heketi service"

	echo -e "\nShow Topology"
	export HEKETI_CLI_SERVER=$(minikube service heketi --url)
	heketi-cli --user=admin --secret="My Secret" topology info

  echo -e "\nStart gluster container"
	sed 's\<hostname>\minikubevm\g' glusterfs-mock.json | kubectl create -f - --validate=false || fail "Unable to start gluster1"

	kubectl run gluster2 --image=ashiq/glusterfs-mock-container --labels=glusterfs-node=gluster2 || fail "Unable to start gluster2"

}

test_peer_probe() {
  echo -e "\nGet the Heketi server connection"
	heketi-cli --user=admin --secret="My Secret" cluster create || fail "Unable to create cluster"

	CLUSTERID=$(heketi-cli --user=admin --secret="My Secret" cluster list | sed -e '$!d')

  echo -e "\nAdd First Node"
	heketi-cli --user=admin --secret="My Secret" node add --zone=1 --cluster=$CLUSTERID --management-host-name=minikubevm --storage-host-name=minikubevm || fail "Unable to add gluster1"

  echo -e "\nAdd Second Node"
	heketi-cli --user=admin --secret="My Secret" node add --zone=2 --cluster=$CLUSTERID --management-host-name=gluster2 --storage-host-name=gluster2 || fail "Unable to add gluster2"

	echo -e "\nShow Topology"
	heketi-cli --user=admin --secret="My Secret" topology info
}




display_information
setup_all_pods

echo -e "\n*** Start tests ***"
test_peer_probe

# Ok now start test
