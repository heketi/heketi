#!/bin/bash

fail() {
    echo "==> ERROR: $*"
    exit 1
}

println() {
    echo "==> $1"
}

_sudo() {
    if [[ ${UID} = 0 || "$HEKETI_TEST_USE_SUDO" = "no" ]]; then
        "${@}"
    else
        sudo -E "${@}"
    fi
}

wait_for_heketi() {
    for i in $(seq 0 30); do
        sleep 1
        ss -tlnp "( sport = :8080 )" | grep -q heketi
        if [[ $? -eq 0 ]]; then
            return 0
        fi
    done
    return 1
}

build_heketi() {
    ( cd "$HEKETI_SERVER_BUILD_DIR" && make server && cp heketi "$HEKETI_SERVER" )
    if [ $? -ne 0 ] ; then
        fail "Unable to build Heketi"
    fi
}

HEKETI_PID=
start_heketi() {
    HEKETI_PID=
    build_heketi

    # Start server
    rm -f heketi.db > /dev/null 2>&1
    $HEKETI_SERVER --config=config/heketi.json &
    HEKETI_PID=$!

    wait_for_heketi
    if [[ $? -ne 0 ]] ; then
        echo "ERROR: heketi failed to listen on port 8080" >&2
        return 1
    fi
}

stop_heketi() {
    if [[ -z "$HEKETI_PID" ]]; then
        # heketi pid was not set, nothing to stop
        return 0
    fi

    kill "$HEKETI_PID"
    sleep 0.2
    for i in $(seq 1 5); do
        if [[ ! -d "/proc/${HEKETI_PID}" ]]; then
            break
        fi
        echo "WARNING: Heketi server may still be running."
        ps -f "$HEKETI_PID"
        kill "$HEKETI_PID"
        sleep 1
    done
}

start_vagrant() {
    cd vagrant || fail "Unable to 'cd vagrant'."
    _sudo ./up.sh || fail "unable to start vagrant virtual machines"
    cd ..
}

teardown_vagrant() {
    cd vagrant || fail "Unable to 'cd vagrant'."
    _sudo vagrant destroy -f
    cd ..
}

run_go_tests() {
    cd tests || fail "Unable to 'cd tests'."
    targs=()
    if [[ "$HEKETI_TEST_GO_TEST_RUN" ]]; then
        targs+=("-run")
        targs+=("$HEKETI_TEST_GO_TEST_RUN")
    fi
    export HEKETI_PID
    time go test -timeout=2h -tags functional -v "${targs[@]}"
    gotest_result=$?
    echo "~~~ go test exited with ${gotest_result}"
    cd ..
}

force_cleanup_libvirt_disks() {
    # Sometimes disks are not deleted
    for i in $(_sudo virsh vol-list default | grep '\.disk' | awk '{print $1}') ; do
        _sudo virsh vol-delete --pool default "${i}" || fail "Unable to delete disk $i"
    done
}

teardown() {
    if [[ "$HEKETI_TEST_VAGRANT" != "no" ]]
    then
        teardown_vagrant
        force_cleanup_libvirt_disks
    fi
    rm -f heketi.db > /dev/null 2>&1
}

setup_test_paths() {
    cd "$SCRIPT_DIR" || return 0
    if [[ -z "${FUNCTIONAL_DIR}" ]]; then
        echo "error: env var FUNCTIONAL_DIR not set" >&2
        exit 2
    fi
    : "${HEKETI_SERVER_BUILD_DIR:=$FUNCTIONAL_DIR/../..}"
    : "${HEKETI_SERVER:=${FUNCTIONAL_DIR}/heketi-server}"
}

pause_test() {
    if [[ "$1" = "yes" ]]; then
        read -r -p "Press ENTER to continue. "
    fi
}

functional_tests() {
    setup_test_paths
    if [[ "$HEKETI_TEST_VAGRANT" != "no" ]]
    then
        start_vagrant
    fi
    if [[ "$HEKETI_TEST_SERVER" == "no" ]]; then
        build_heketi
        # make sure pid is unset so stop_heketi does nothing
        HEKETI_PID=
    else
        start_heketi
    fi

    pause_test "$HEKETI_TEST_PAUSE_BEFORE"
    run_go_tests
    pause_test "$HEKETI_TEST_PAUSE_AFTER"

    stop_heketi
    if [[ "$HEKETI_TEST_CLEANUP" != "no" ]]
    then
        teardown
    fi

    exit $gotest_result
}

