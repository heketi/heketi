#!/bin/bash

show_help() {
	echo "$0 [options]"
	echo "  Options:"
	echo "    --test T  Run test suite T (can be specified multiple times)"
	echo "    --help    Display this help text"
	echo ""
}

TESTS=()

CLI="$(getopt -o h --long test:,help -n "$0" -- "$@")"
eval set -- "${CLI}"
while true ; do
	case "$1" in
		--test)
			TESTS+=("$2")
			shift
			shift
		;;
		-h|--help)
			show_help
			exit 0
		;;
		--)
			shift
			break
		;;
		*)
			echo "unknown option" >&2
			exit 2
		;;
	esac
done


if [[ "${#TESTS[@]}" -eq 0 ]]; then
	TESTS+=("TestSmokeTest")
	TESTS+=("TestVolumeNotDeletedWhenNodeIsDown")
	TESTS+=("TestVolumeSnapshotBehavior")
	TESTS+=("TestManyBricksVolume")
	TESTS+=("TestUpgrade")
fi

# install glide
if ! command -v glide ; then
	echo glide is not installed, please install it to continue
	echo 'get it from your package manager, or unsafely via: "curl https://glide.sh/get | sh"'
	exit 1
fi

fetch_golang() {
    # Download golang 1.8.3
    curl -O https://storage.googleapis.com/golang/go1.8.3.linux-amd64.tar.gz
    tar xzvf go1.8.3.linux-amd64.tar.gz
    GOROOT=$(pwd)/go
    export GOROOT
    export PATH=$GOROOT/bin:$PATH
}

vercheck() {
    # return true (0) if version number $2 is greater-or-equal to
    # version number $1
    r="$(echo -e "$1\n$2" | sort -V | head -n1)"
    if [[ "$r" == "$1" ]]; then
        return 0
    fi
    return 1
}

case "$HEKETI_TEST_SYSTEM_GO" in
    yes)
        echo "Using system go packages"
    ;;
    auto|"")
        gv="$(go version | awk '{print $3}')"
        gv="${gv/go/}"
        if [[ "${gv}" ]] && vercheck "1.8.3" "${gv}"; then
            echo "Using system go (version ${gv})"
        else
            fetch_golang
        fi
    ;;
    no)
        fetch_golang
    ;;
    *)
        echo "error: unknown value for HEKETI_TEST_SYSTEM_GO, need yes|no|auto" >&2
        exit 2
    ;;
esac

source ./lib.sh

teardown_all() {
    results=0
    for testDir in "${TESTS[@]}" ; do
        if [ -x "$testDir/teardown.sh" ] ; then
            println "TEARDOWN $testDir"
            cd "$testDir" || fail "Unable to 'cd $testDir'."
            teardown.sh
            cd ..
        fi
    done
}

### MAIN ###

# See https://bugzilla.redhat.com/show_bug.cgi?id=1327740
_sudo setenforce 0

starttime=$(date)
export PATH=$PATH:.

# Check go can build
if [ -z "$GOPATH" ] ; then
    fail "GOPATH must be specified"
fi

# Clean up
rm -f heketi-server > /dev/null 2>&1
teardown_all

# Check each dir for tests
tpassed=()
tfailed=()
for testDir in "${TESTS[@]}" ; do
    if [ -x "$testDir/run.sh" ] ; then
        println "TEST $testDir"
        cd "$testDir" || fail "Unable to 'cd $testDir'."

        # Run the command with a large timeout.
        # Just large enough so that it doesn't run forever.
        timeout 1h run.sh ; result=$?

        if [ $result -ne 0 ] ; then
            println "FAILED $testDir"
            println "TEARDOWN $testDir"
            teardown.sh
            tfailed+=("${testDir}")
        else
            println "PASSED $testDir"
            tpassed+=("${testDir}")
        fi

        cd ..
    fi
done

# Summary
println "Started $starttime"
println "Ended $(date)"
println "-- Passing Tests: ${tpassed[*]}"
if [[ ${#tfailed[@]} -eq 0 ]] ; then
    println "PASSED"
else
    println "-- Failing Tests: ${tfailed[*]}"
    println "FAILED"
fi

exit $results
