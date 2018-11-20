#!/bin/sh
#
# Preparation for building container images.
#
# - artifacts need to be in this WORKDIR, symlinks won't work
#

# error out if anything fails
set -e

# the local directory where the Dockerfile is located
WORKDIR=$(dirname "${0}")

cp -f "${WORKDIR}"/../../container/heketi.json "${WORKDIR}"
cp -f "${WORKDIR}"/../../container/heketi-start.sh "${WORKDIR}"
