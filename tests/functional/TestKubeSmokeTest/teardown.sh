#!/bin/sh

TOP=../../..
CURRENT_DIR=`pwd`
RESOURCES_DIR=$CURRENT_DIR/resources
FUNCTIONAL_DIR=${CURRENT_DIR}/..

source ${FUNCTIONAL_DIR}/lib.sh


if [ -d kubeup/.git ] ; then
    ( cd kubeup && ./down.sh )
fi
rm -rf $RESOURCES_DIR > /dev/null
