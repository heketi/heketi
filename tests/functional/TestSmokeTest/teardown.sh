#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "${0}")" && pwd)"
export HEKETI_SERVER_BUILD_DIR=../../..
FUNCTIONAL_DIR="${SCRIPT_DIR}/.."
export HEKETI_SERVER="${FUNCTIONAL_DIR}/heketi-server"

source "${FUNCTIONAL_DIR}/lib.sh"

teardown

