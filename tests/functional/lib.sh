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

HEKETI_PID=
start_heketi() {
    local config_filename=$1
    ( cd "$HEKETI_SERVER_BUILD_DIR" && make && cp heketi "$HEKETI_SERVER" )
    if [ $? -ne 0 ] ; then
        fail "Unable to build Heketi"
    fi

    if [ ! -f config/"${config_filename}" ]
    then
        config_filename="heketi.json"
    fi

    # Start server
    rm -f heketi.db > /dev/null 2>&1
    $HEKETI_SERVER --config=config/${config_filename} &
    HEKETI_PID=$!
    sleep 2
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
    for t in $(find . -name '*_test.go' | LC_COLLATE=C sort)
    do
        config_file=${t%.go}.json
        cd ..
        start_heketi "$config_file"
        cd tests || fail "Unable to 'cd tests'."
        testfuncs=$(grep "func Test.*(t\\ \\*testing.T)[\\ ]*{" ./* | awk '{print $2}' | cut -d"(" -f1)
        for testfunc in "${testfuncs[@]}"
        do
            go test -timeout=1h -tags functional -run "$testfunc"
            if [ $? -ne 0 ]
            then
                gotest_result=$?
            fi
        done
        testfuncs=""
        kill $HEKETI_PID
    done
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

    pause_test "$HEKETI_TEST_PAUSE_BEFORE"
    run_go_tests
    pause_test "$HEKETI_TEST_PAUSE_AFTER"

    if [[ "$HEKETI_TEST_CLEANUP" != "no" ]]
    then
        teardown
    fi

    exit $gotest_result
}

