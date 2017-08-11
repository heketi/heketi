
fail() {
    echo "==> ERROR: $@"
    exit 1
}

println() {
    echo "==> $1"
}

_sudo() {
    if [ ${UID} = 0 ] ; then
        ${@}
    else
        sudo -E ${@}
    fi
}

HEKETI_PID=
start_heketi() {
    ( cd $HEKETI_SERVER_BUILD_DIR && make && cp heketi $HEKETI_SERVER )
    if [ $? -ne 0 ] ; then
        fail "Unable to build Heketi"
    fi

    # Start server
    rm -f heketi.db > /dev/null 2>&1
    $HEKETI_SERVER --config=config/heketi.json &
    HEKETI_PID=$!
    sleep 2
}

start_vagrant() {
    cd vagrant
    _sudo ./up.sh || fail "unable to start vagrant virtual machines"
    cd ..
}

teardown_vagrant() {
    cd vagrant
    _sudo vagrant destroy -f
    cd ..
}

run_tests() {
    cd tests
    go test -timeout=1h -tags functional
    gotest_result=$?
    cd ..
}

force_cleanup_libvirt_disks() {
    # Sometimes disks are not deleted
    for i in `_sudo virsh vol-list default | grep "*.disk" | awk '{print $1}'` ; do
        _sudo virsh vol-delete --pool default "${i}" || fail "Unable to delete disk $i"
    done
}

teardown() {
    teardown_vagrant
    force_cleanup_libvirt_disks
    rm -f heketi.db > /dev/null 2>&1
}

functional_tests() {
    start_vagrant
    start_heketi

    run_tests

    kill $HEKETI_PID
    teardown

    exit $gotest_result
}

# Kubernetes functions

# $1 namespace
# $2 name
get_node_port_from_service() {
    kubectl -n "$1" get svc "$2" -o json | jq '.spec.ports[0].nodePort'
}

# $1 namespace
# $2 name
# $3 number of expected running pods
wait_for_pod_ready() {
    # Wait until all pods are in Running state
    n=0
    until [ `kubectl -n "$1" get pods 2>/dev/null | grep "$2" | grep Running | wc -l` -eq $3 ] ; do
        n=$[$n+1]
        if [ $n -gt 600 ] ; then
            fail "Timed out waiting for $2 to start"
        fi
        sleep 1
    done
}

# $1 namespace
# $2 name
wait_for_pvc_bound() {
	n=0
    until `kubectl -n "$1" get pvc 2>/dev/null | grep "$2" | grep Bound > /dev/null 2>&1` ; do
        n=$[$n+1]
        if [ $n -gt 600 ] ; then
            fail "Timed out waiting for pvc to be deleted"
        fi
        sleep 1
    done
}

# $1 namespace
# $2 name
wait_for_pvc_deleted() {
	n=0
    while `kubectl -n "$1" get pvc 2>/dev/null | grep "$2" > /dev/null 2>&1` ; do
        n=$[$n+1]
        if [ $n -gt 600 ] ; then
            fail "Timed out waiting for pvc to be deleted"
        fi
        sleep 1
    done
    sleep 10
}
