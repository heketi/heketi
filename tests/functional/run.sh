#!/bin/sh

TESTS="TestSmokeTest"
TESTS="TestVolumeNotDeletedWhenNodeIsDown $TESTS"
TESTS="TestVolumeSnapshotBehavior $TESTS"
TESTS="TestManyBricksVolume $TESTS"
TESTS="TestKubeSmokeTest $TESTS"

source ./lib.sh

install_go() {
    # Download golang 1.8.3
    GOLANGDIR=$HOME/.golang
    current_dir=$(pwd)
    if [ ! -d $GOLANGDIR ] ; then
        mkdir $GOLANGDIR
    fi
    cd $GOLANGDIR
    curl -O https://storage.googleapis.com/golang/go1.8.3.linux-amd64.tar.gz || fail "Unable to get Go binary"
    tar xzf go1.8.3.linux-amd64.tar.gz || fail "Unable to untar go"
    export GOROOT=$GOLANGDIR/go
    export PATH=$GOROOT/bin:$PATH

    cd $current_dir
}

teardown_all() {
    results=0
    for testDir in $TESTS ; do
        if [ -x $testDir/teardown.sh ] ; then
            println "TEARDOWN $testDir"
            cd $testDir
            teardown.sh
            cd ..
        fi
    done
}

### MAIN ###

# See https://bugzilla.redhat.com/show_bug.cgi?id=1327740
_sudo setenforce 0

starttime=`date`
export PATH=$PATH:.

# install glide
if ! command -v glide ; then
    curl https://glide.sh/get | sh
fi

# install packages needed
if grep "CentOS" /etc/redhat-release > /dev/null 2>&1 ; then
    _sudo yum -y install jq
elif grep "Fedora" /etc/redhat-release > /dev/null 2>&1 ; then
    _sudo dnf -y install jq
fi

install_go

# Check go can build
if [ -z $GOPATH ] ; then
    fail "GOPATH must be specified"
fi

# Clean up
rm -f heketi-server > /dev/null 2>&1
teardown_all

# Check each dir for tests
results=0
for testDir in $TESTS ; do
    if [ -x $testDir/run.sh ] ; then
        println "TEST $testDir"
        cd $testDir

        # Run the command with a large timeout.
        # Just large enough so that it doesn't run forever.
        timeout 1h run.sh ; result=$?

        if [ $result -ne 0 ] ; then
            println "FAILED $testDir"
            println "TEARDOWN $testDir"
            teardown.sh
            results=1
        else
            println "PASSED $testDir"
        fi

        cd ..
    fi
done

# Summary
println "Started $starttime"
println "Ended `date`"
if [ $results -eq 0 ] ; then
    println "PASSED"
else
    println "FAILED"
fi

exit $results
