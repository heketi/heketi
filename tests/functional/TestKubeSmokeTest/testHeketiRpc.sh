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

start_mock_gluster_container() {
# Use a busybox container
  kubectl run gluster$1 \
	  --restart=Never \
		--image=busybox \
		--labels=glusterfs-node=gluster$1 \
		--command -- sleep 10000 || fail "Unable to start gluster$1"

	# Wait until it is running
	while ! kubectl get pods | grep gluster$1 | grep "1/1" > /dev/null ; do
		sleep 1
	done

	# Create fake gluster file
	kubectl exec gluster$1 -- sh -c "echo '#!/bin/sh' > /bin/gluster" || fail "Unable to create /bin/gluster"
	kubectl exec gluster$1 -- chmod +x /bin/gluster || fail "Unable to chmod +x /bin/gluster"

	# Create fake bash file
	kubectl exec gluster$1 -- sh -c "echo '#!/bin/sh' > /bin/bash" || fail "Unable to create /bin/bash"
	kubectl exec gluster$1 -- chmod +x /bin/bash || fail "Unable to chmod +x /bin/bash"
}

setup_all_pods() {

	kubectl get nodes --show-labels

	echo -e "\nCreate a ServiceAccount"
	kubectl create -f $RESOURCES_DIR/heketi-service-account.yaml || fail "Unable to create a serviceAccount"

	KUBESEC=$(kubectl get sa heketi-service-account -o="go-template" --template="{{range .secrets}}{{.name}}{{end}}")
	KUBEAPI=https://$(minikube ip):8443

	echo -e "\nSecret is = $KUBESEC"
	if [ -z "$KUBESEC" ] ; then
		fail "Secret is empty"
	fi

	# Start Heketi
	echo -e "\nStart Heketi container"
    sed -e "s#heketi/heketi:dev#heketi/heketi:ci#" \
        -e "s#Always#IfNotPresent#" \
        -e "s#<HEKETI_KUBE_SECRETNAME>#\"$KUBESEC\"#" \
        -e "s#<HEKETI_KUBE_APIHOST>#\"$KUBEAPI\"#" \
        $RESOURCES_DIR/deploy-heketi-deployment.json | kubectl create -f -

	# Wait until it is running
	echo "Wait until deploy-heketi is ready"
	while ! kubectl get pods | grep heketi | grep "1/1" > /dev/null ; do
		echo -n "."
		sleep 1
	done

	echo "Delete the cluster service because it cannot be used in minikube"
	kubectl delete service deploy-heketi

	echo "Create a service for deploy-heketi"
	kubectl expose deployment deploy-heketi --port=8080 --type=NodePort || fail "Unable to expose heketi service"

	echo -e "\nShow Topology"
	export HEKETI_CLI_SERVER=$(minikube service deploy-heketi --url)
	heketi-cli topology info

	echo -e "\nStart gluster mock container"
	start_mock_gluster_container 1
	start_mock_gluster_container 2
}

test_peer_probe() {
	echo -e "\nGet the Heketi server connection"
	heketi-cli cluster create || fail "Unable to create cluster"

	CLUSTERID=$(heketi-cli cluster list | sed -e '$!d')

	echo -e "\nAdd First Node"
	heketi-cli node add --zone=1 --cluster=$CLUSTERID --management-host-name=gluster1 --storage-host-name=gluster1 || fail "Unable to add gluster1"

	echo -e "\nAdd Second Node"
	heketi-cli node add --zone=2 --cluster=$CLUSTERID --management-host-name=gluster2 --storage-host-name=gluster2 || fail "Unable to add gluster2"

	echo -e "\nShow Topology"
	heketi-cli topology info
}




display_information
setup_all_pods

echo -e "\n*** Start tests ***"
test_peer_probe

# Ok now start test
