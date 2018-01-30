#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${0}")" && pwd)"

cd "${SCRIPT_DIR}/functional/TestDbExportImport" || exit 1


require_heketi_binaries() {
	if [ ! -x heketi-server ] || [ ! -x heketi-cli ] ; then
		make -C ../../../  &> /dev/null
		cp ../../../heketi heketi-server
		cp ../../../client/cli/go/heketi-cli heketi-cli
	fi
}

start_server() {
	rm -f heketi.db &> /dev/null
	./heketi-server --config="./heketi.json" &> heketi.log &
	server_pid=$!
	sleep 2
}

restart_server() {
	./heketi-server --config="./heketi.json" &>> heketi.log &
	server_pid=$!
	sleep 2
}


kill_server() {
        if [[ -n $server_pid ]]
        then
                kill "${server_pid}"
                server_pid=""
        fi
}

show_err() {
        if [[ $? -ne 0 ]]
        then
                echo "failure/error on line $1"
        fi
}

cleanup() {
        kill_server
	rm -f heketi.db* &> /dev/null
	rm -f db.json.* &> /dev/null
	rm -f heketi.log &> /dev/null
	rm -f heketi-server &> /dev/null
	rm -f heketi-cli &> /dev/null
	rm -f topologyinfo.* &> /dev/null
}


require_heketi_binaries
start_server
trap 'cleanup $LINENO' EXIT
trap 'show_err $LINENO' ERR

# populate db
./heketi-cli --server "http://127.0.0.1:8080" topology load --json topology.json &> /dev/null
./heketi-cli --server "http://127.0.0.1:8080" volume create --size 100 --block=true &> /dev/null
./heketi-cli --server "http://127.0.0.1:8080" volume create --size 100 --snapshot-factor=1.25 &> /dev/null
./heketi-cli --server "http://127.0.0.1:8080" blockvolume create --size 1 &> /dev/null
./heketi-cli --server "http://127.0.0.1:8080" volume create --size 2 --durability=disperse  --disperse-data=2 --redundancy=1 &> /dev/null
./heketi-cli --server "http://127.0.0.1:8080" volume create --size 2 --durability=none  --gluster-volume-options="performance.rda-cache-limit 10MB" &> /dev/null
./heketi-cli --server "http://127.0.0.1:8080" topology info > topologyinfo.original

# tool should not open db in use
if ./heketi-server db export --jsonfile db.json.failcase --dbfile heketi.db &> /dev/null
then
        echo "FAILED: tool could open the db file when in use"
        exit 1
fi

# stop server and free db
kill_server

# test one cycle of export and import
./heketi-server db export --jsonfile db.json.original --dbfile heketi.db &> /dev/null
./heketi-server db import --jsonfile db.json.original --dbfile heketi.db.new &> /dev/null
./heketi-server db export --jsonfile db.json.new --dbfile heketi.db.new &> /dev/null
diff db.json.original db.json.new &> /dev/null

# existing json file should not be overwritten
if ./heketi-server db export --jsonfile db.json.original --dbfile heketi.db &> /dev/null
then
        echo "FAILED: overwrote the json file"
        exit 1
fi

# existing db file should not be overwritten
if ./heketi-server db import --jsonfile db.json.original --dbfile heketi.db.new &> /dev/null
then
        echo "FAILED: overwrote the db file"
        exit 1
fi

restart_server
./heketi-cli --server "http://127.0.0.1:8080" topology info > topologyinfo.new
diff topologyinfo.original topologyinfo.new &> /dev/null
