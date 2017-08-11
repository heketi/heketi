#!/bin/sh

TOP=../../..
CURRENT_DIR=`pwd`
RESOURCES_DIR=$CURRENT_DIR/resources
FUNCTIONAL_DIR=${CURRENT_DIR}/..
DOCKERDIR=$TOP/extras/docker
CLIENTDIR=$TOP/client/cli/go
export PATH=$(pwd)/kubeup/bin:$CLIENTDIR:$PATH
export KUBECONFIG=$(pwd)/kubeup/matchbox/assets/auth/kubeconfig

source ${FUNCTIONAL_DIR}/lib.sh

teardown() {
    bash ./teardown.sh
}

display_information() {
	# Display information
	echo -e "\nVersions"
	kubectl version

	echo -e "\nShow nodes"
	kubectl get nodes
}


build_docker_file(){
    println "Start registry proxy"
    REGID=$(docker run -e REGISTRY_HOST=172.17.0.21 -e REGISTRY_PORT=5000 -p 5000:80 -d quay.io/deisci/registry-proxy:git-4cc19e2)

    println "Create Heketi Docker image"
    cd $DOCKERDIR/ci
    cp $TOP/heketi $DOCKERDIR/ci || fail "Unable to copy $TOP/heketi to $DOCKERDIR/ci"
    _sudo docker build --rm --tag localhost:5000/heketi . || fail "Unable to create docker container"

    println "Pushing container to Kubernetes"
    _sudo docker push localhost:5000/heketi || fail "Unable to push image to Kubernetes"

    println "Stop registry proxy"
    docker stop $REGID
    cd $CURRENT_DIR
}

build_heketi() {
    cd $TOP
    make || fail  "Unable to build heketi"
    cd $CURRENT_DIR
}

start_kubernetes() {
    println "Start Kubernetes"
    if [ ! -d kubeup/.git ] ; then
        git clone https://github.com/lpabon/kubeup.git
    fi
    ( cd kubeup && ./bootstrap.sh && ./up.sh )
    if [ $? -ne 0 ] ; then
        fail "Unable to setup kubernetes"
    fi

    # Wait until all pods are in Running state
    n=0
    while `kubectl -n kube-system get pods 2>/dev/null | grep -v AGE | grep -v Running > /dev/null 2>&1` ; do
        n=$[$n+1]
        if [ $n -gt 600 ] ; then
            fail "Timed out waiting for quartermaster to start"
        fi
        sleep 1
    done

}

deploy_quartermaster() {
    if ! kubectl -n kube-system get pods 2>/dev/null | grep quartermaster > /dev/null 2>&1 ; then
        println "Deploy Quartermaster"
        kubectl run -n kube-system quartermaster --image=quay.io/lpabon/qm || fail "Unable to start quartermaster"
    fi
}

wait_quartermaster() {
    wait_for_pod_ready "kube-system" "quartermaster" 1
    println "quartermaster Ready"
}

deploy_registry() {
    if ! kubectl -n kube-system get pods 2>/dev/null | grep registry > /dev/null 2>&1 ; then
        println "Deploy Registry"
        kubectl create -f registry.yaml
    fi
}

wait_registry() {
    wait_for_pod_ready "kube-system" "registry" 3
    println "registry Ready"
}

setup_kubernetes() {
    deploy_quartermaster
    deploy_registry
    wait_quartermaster
    wait_registry
}

setup() {
    start_kubernetes
    setup_kubernetes
    display_information
    build_heketi
    build_docker_file
}

### MAIN ###
setup
tests

./testMock.sh && ./testKubernetes.sh

