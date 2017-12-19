#!/bin/bash

# Runs the "unit" test suite of the python api client
# Sadly, this needs an actual heketi server running

set -e

SCRIPT_DIR="$(cd "$(dirname "${0}")" && pwd)"

cd "${SCRIPT_DIR}/../client/api/python" || exit 1

require_server() {
	if [ ! -x heketi-server ] ; then
		make -C ../../../
		cp ../../../heketi heketi-server
	fi
}

start_server() {
	rm -f heketi.db &> /dev/null
	./heketi-server --config=test/unit/heketi.json &> heketi.log &
	server_pid=$!
	echo "---- Started heketi server, pid=${server_pid}"
	sleep 2
	echo "---- Heketi server ready, pid=${server_pid}"
}

cleanup_server() {
	echo "---- Terminating heketi server, pid=${server_pid}"
	kill "${server_pid}"
	rm -f heketi.db &> /dev/null
}


if ! command -v tox &>/dev/null; then
	echo "warning: tox not installed... skipping tests" >&2
	exit 0
fi

require_server
start_server
trap cleanup_server EXIT

tox -e py27,py35,py36 --skip-missing-interpreters
