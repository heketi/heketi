#!/bin/sh

TOP=../../..
CURRENT_DIR=`pwd`
FUNCTIONAL_DIR=${CURRENT_DIR}/..
RESOURCES_DIR=$CURRENT_DIR/resources
PATH=$PATH:$RESOURCES_DIR

source ${FUNCTIONAL_DIR}/lib.sh

wait_for_glusterfs_cluster() {
    # Wait for Heketi to be ready
    echo "..wait for Heketi container"
    wait_for_pod_ready "default" "heketi" 1

    # Wait for GlusterFS nodes to be ready
    echo "..wait for GlusterFS containers"
    wait_for_pod_ready "default" "gluster" 3

    # Wait for Cluster to be ready
    echo "..wait for cluster configuration"
    n=0
    until [ `kubectl get storagecluster gluster -o json | jq '.status.ready'` = "true" ] ; do
        n=$[$n+1]
        if [ $n -gt 600 ] ; then
            fail "Timed out waiting for storage cluster to be ready"
        fi
        sleep 1
    done
    echo "Cluster is now ready"
}

setup() {
    echo "Setup RBAC rules"
    kubectl create -f $RESOURCES_DIR/heketi-servicetoken-role-binding-k14.yaml
    kubectl create -f $RESOURCES_DIR/heketi-servicetoken-role.yaml

    echo "Deploy GlusterFS cluster using Quartermaster"
    kubectl create -f $RESOURCES_DIR/glusterfs-ci.yaml

    echo "Wait for GlusterFS cluster to become ready"
    wait_for_glusterfs_cluster

	echo "Create a service for deploy-heketi"
	kubectl expose deployment heketi --name="heketi-external" --port=8080 --type=NodePort || fail "Unable to expose heketi service"
	port=`get_node_port_from_service "default" "heketi-external"`

	echo -e "\nShow Topology"
	export HEKETI_CLI_SERVER="http://node2.example.com:${port}"
	heketi-cli topology info
}

teardown() {
    echo "Deleting cluster"
    kubectl delete storagecluster gluster
    kubectl delete deploy heketi
    kubectl delete svc heketi
    kubectl delete svc heketi-external
    kubectl delete serviceaccount heketi-service-account
    kubectl delete secret heketi-db-backup
}

tests() {
    ### TESTS ###
    for testDir in Test* ; do
        if [ -x $testDir/run.sh ] ; then
            println "START KUBETEST $testDir"
            cd $testDir

            ./run.sh ; result=$?

            if [ $result -ne 0 ] ; then
                println "FAILED KUBETEST $testDir"
                if [ -x teardown.sh ] ; then
                    println "TEARDOWN KUBETEST $testDir"
                    ./teardown.sh
                fi
                results=1
            else
                println "PASSED $testDir"
            fi

            cd ..
        fi
    done
}

result=0
tests
exit $result

# Ok now start test
