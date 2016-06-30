#!/bin/sh

fail() {
    echo $1
    exit 1
}

println() {
    echo "==> $1"
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
force_cleaup_libvirt_disks

# Check each dir for tests
results=0
for testDir in * ; do
    if [ -x $testDir/run.sh ] ; then
        println "TEST $testDir"
        cd $testDir 
          run.sh
        result=$?
        cd ..

        if [ $result -ne 0 ] ; then
            println "FAILED $testDir"
            results=1
        else
            println "PASSED $testDir"
        fi
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
