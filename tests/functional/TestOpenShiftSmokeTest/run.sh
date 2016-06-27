#!/bin/sh

CURRENT_DIR=`pwd`
TOP=../../..
HEKETI_SERVER_BUILD_DIR=$TOP
FUNCTIONAL_DIR=${CURRENT_DIR}/..
HEKETI_SERVER=${FUNCTIONAL_DIR}/heketi-server
HEKETI_DOCKER_IMG=heketi-docker-ci.img
DOCKERDIR=$TOP/extras/docker/ci
CLIENTDIR=$TOP/client/cli/go

source ${FUNCTIONAL_DIR}/lib.sh


build_docker_file(){
    vagrant_heketi_docker=$CURRENT_DIR/vagrant/roles/cluster/files/$HEKETI_DOCKER_IMG
    if [ ! -f "$vagrant_heketi_docker" ] ; then
        cd $DOCKERDIR
        cp $TOP/heketi $DOCKERDIR
        _sudo docker build --rm --tag heketi/heketi:ci . || fail "Unable to create docker container"
        _sudo docker save -o $HEKETI_DOCKER_IMG heketi/heketi:ci || fail "Unable to save docker image"
        cp $HEKETI_DOCKER_IMG $vagrant_heketi_docker
        cd $CURRENT_DIR
    fi
}


build_heketi() {
    cd $TOP
    make || fail  "Unable to build heketi"
    cd $CURRENT_DIR
}

copy_client_files() {
    cp $CLIENTDIR/heketi-cli vagrant/roles/client/files
    cp -r $TOP/extras/openshift/template vagrant
}

deploy_heketi_glusterfs() {
    cd tests/deploy
    _sudo ./run.sh || fail "Unable to deploy"
    cd $CURRENT_DIR
}

teardown_vagrant
build_heketi
copy_client_files
build_docker_file
start_vagrant
deploy_heketi_glusterfs
teardown_vagrant
