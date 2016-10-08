#!/bin/sh

source ./lib.sh

teardown_all() {
    results=0
    for testDir in * ; do
        if [ -x $testDir/teardown.sh ] ; then
            println "TEARDOWN $testDir"
            cd $testDir
            teardown.sh
            cd ..
        fi
    done
}

### MAIN ###

starttime=`date`
export PATH=$PATH:.

# Check go can build
if [ -z $GOPATH ] ; then
    fail "GOPATH must be specified"
fi

# Clean up
rm -f heketi-server > /dev/null 2>&1
teardown_all

# Check each dir for tests
results=0
for testDir in * ; do
    if [ -x $testDir/run.sh ] ; then
        println "TEST $testDir"
        cd $testDir
        run.sh
        result=$?

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
