#!/bin/sh

CURRENT_DIR=`pwd`
HEKETI_SERVER_BUILD_DIR=../../..
FUNCTIONAL_DIR=${CURRENT_DIR}/..
HEKETI_SERVER=${FUNCTIONAL_DIR}/heketi-server

source ${FUNCTIONAL_DIR}/lib.sh
vagrant_heketi_docker=$CURRENT_DIR/vagrant/roles/cluster/files/$HEKETI_DOCKER_IMG

teardown_vagrant
force_cleanup_libvirt_disks
rm -rf $vagrant_heketi_docker \
    vagrant/roles/client/files/heketi-cli \
    vagrant/templates > /dev/null 2>&1

