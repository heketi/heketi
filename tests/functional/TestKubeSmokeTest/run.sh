#!/bin/sh

TOP=../../..
CURRENT_DIR=`pwd`
RESOURCES_DIR=$CURRENT_DIR/resources
FUNCTIONAL_DIR=${CURRENT_DIR}/..
HEKETI_DOCKER_IMG=heketi-docker-ci.img
DOCKERDIR=$TOP/extras/docker
CLIENTDIR=$TOP/client/cli/go

source ${FUNCTIONAL_DIR}/lib.sh



copy_docker_files() {
    (
        eval $(minikube docker-env) 
        docker load -i $heketi_docker || fail "Unable to load Heketi docker image"
        docker tag heketi/heketi:ci heketi/heketi:latest || fail "Unable to retag Heketi container"
    )
}

build_docker_file(){
    echo "Create Heketi Docker image"
    heketi_docker=$RESOURCES_DIR/$HEKETI_DOCKER_IMG
    if [ ! -f "$heketi_docker" ] ; then
        cd $DOCKERDIR/ci
        cp $TOP/heketi $DOCKERDIR/ci || fail "Unable to copy $TOP/heketi to $DOCKERDIR/ci"
        _sudo docker build --rm --tag heketi/heketi:ci . || fail "Unable to create docker container"
        _sudo docker save -o $HEKETI_DOCKER_IMG heketi/heketi:ci || fail "Unable to save docker image"
        cp $HEKETI_DOCKER_IMG $heketi_docker || fail "Unable to copy image"
        _sudo docker rmi heketi/heketi:ci
        cd $CURRENT_DIR
    fi
    copy_docker_files
}

build_heketi() {
    cd $TOP
    make || fail  "Unable to build heketi"
    cd $CURRENT_DIR
}

copy_client_files() {
    cp $CLIENTDIR/heketi-cli $RESOURCES_DIR || fail "Unable to copy client files"
}

teardown() {
    if [ -x /usr/local/bin/minikube ] ; then
        minikube stop
        minikube delete
    fi
    rm -rf $RESOURCES_DIR > /dev/null
}

setup_minikube() {
    if [ ! -d $RESOURCES_DIR ] ; then
        mkdir $RESOURCES_DIR
    fi

	if [ ! -x /usr/local/bin/docker-machine ] ; then
		curl -Lo docker-machine https://github.com/docker/machine/releases/download/v0.8.1/docker-machine-Linux-x86_64 || fail "Unable to get docker-machine"
		chmod +x docker-machine
		_sudo mv docker-machine /usr/local/bin
	fi

	if [ ! -x /usr/local/bin/docker-machine-driver-kvm ] ; then
		curl -Lo docker-machine-driver-kvm \
			https://github.com/dhiltgen/docker-machine-kvm/releases/download/v0.7.0/docker-machine-driver-kvm || fail "Unable to get docker-machine-driver-kvm"
		chmod +x docker-machine-driver-kvm
		_sudo mv docker-machine-driver-kvm /usr/local/bin
    fi

	_sudo usermod -a -G libvirt $(whoami)
	#newgrp libvirt

	if [ ! -x /usr/local/bin/minikube ] ; then
		curl -Lo minikube \
			https://storage.googleapis.com/minikube/releases/v0.9.0/minikube-linux-amd64 || fail "Unable to get minikube"
		chmod +x minikube
		_sudo mv minikube /usr/local/bin
	fi

	if [ ! -x /usr/local/bin/kubectl ] ; then
		curl -Lo kubectl \
			http://storage.googleapis.com/kubernetes-release/release/v1.3.0/bin/linux/amd64/kubectl || fail "Unable to get kubectl"
		chmod +x kubectl
		_sudo mv kubectl /usr/local/bin
	fi

}

start_minikube() {
	minikube start \
		--iso-url=https://github.com/kubernetes/minikube/releases/download/v0.9.0/minikube.iso \
		--cpus=2 \
		--memory=2048 \
		--vm-driver=kvm \
		--kubernetes-version="v1.4.0-beta.1" || fail "Unable to start minikube"
}



teardown

setup_minikube
start_minikube

build_heketi
copy_client_files
build_docker_file

kubectl get nodes

./test.sh

teardown

