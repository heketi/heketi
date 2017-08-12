#!/bin/sh

CURRENT_DIR=`pwd`

source ${FUNCTIONAL_DIR}/lib.sh

setup_heketi() {
	println "Setup Heketi"

	# Start Heketi
	echo "Start Heketi container"
	kubectl run heketi --image=localhost:5000/heketi --port=8080 || fail "Unable to start heketi container"
	wait_for_pod_ready "default" "heketi" 1

	# This blocks until ready
	kubectl expose deployment heketi --type=NodePort || fail "Unable to expose heketi service"
	port=`get_node_port_from_service "default" "heketi"`

	echo "Show Topology"
	export HEKETI_CLI_SERVER=http://node2.example.com:${port}
	heketi-cli topology info

	echo "Load mock topology"
	heketi-cli topology load --json=mock-topology.json || fail "Unable to load topology"

	echo "Show Topology"
	heketi-cli topology info

	echo -e "\nRegister storage class"
	sed -e \
	"s#%%URL%%#${HEKETI_CLI_SERVER}#" \
	storageclass.yaml.sed > ${RESOURCES_DIR}/sc.yaml
    kubectl create -f ${RESOURCES_DIR}/sc.yaml || fail "Unable to register storage class"
}

test_create() {
	echo "--> Test Create"
	echo "Assert no volumes available"
	if heketi-cli volume list | grep Id ; then
        heketi-cli volume list
		fail "Incorrect number of volumes in Heketi"
	fi

	echo "Submit PVC for 100GiB"
	kubectl create -f pvc.json || fail "Unable to submit PVC"

	# Wait until pvc bound
    wait_for_pvc_bound "default" "claim1"

	echo "Assert only one volume created in Heketi"
	if ! heketi-cli volume list | grep Id | wc -l | grep 1 ; then
		fail "Incorrect number of volumes in Heketi"
	fi

	echo "Assert volume size is 100GiB"
	id=`heketi-cli volume list | grep Id | awk '{print $1}' | cut -d: -f2`
    if ! heketi-cli volume info ${id} | grep Size | cut -d: -f2 | grep 100 ; then
		fail "Invalid size"
	fi
}

test_delete() {
	echo "--> Delete PVC"
	kubectl delete pvc claim1 || fail "Unable to delete claim1"

	# Wait until pvc unbound
    wait_for_pvc_deleted "default" "claim1"

	echo "Assert no volumes available"
	if heketi-cli volume list | grep Id ; then
        heketi-cli volume list
		fail "Incorrect number of volumes in Heketi"
	fi
}

teardown_heketi() {
	echo "--> Cleanup"
	kubectl delete svc heketi
	kubectl delete deploy heketi
	kubectl delete storageclass slow
}

## MAIN
setup_heketi
test_create
test_delete
teardown_heketi

