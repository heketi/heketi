# Unit Tests

To run the unit tests, execute `make test`. This runs the
[test.sh](../../test.sh) shell script that is a wrapper for all the unit tests
and also some miscellaneous tests.

# Functional Tests To run the functional tests, execute `make test-functional`.
This runs the [run.sh](../../tests/functional/run.sh) shell script that is a
wrapper for all the functional tests.

The functional tests are run in a VM environment. A trick to reduce the time
spent in setting up and tearing down is to reuse the VM environment. Setting the
env HEKETI_TEST_CLEANUP=no disables the teardown of the VMs. Setting the env
HEKETI_TEST_VAGRANT=no skips the VM creation. For example, run the tests for
the first time with cleanup disabled `HEKETI_TEST_CLEANUP=no
./tests/functional/run.sh` and there onwards run the tests thereon with both VM creation
and cleanup disabled `HEKETI_TEST_CLEANUP=no HEKETI_TEST_VAGRANT=no
./tests/functional/run.sh`.

hek-ft-restore; HEKETI_TEST_CLEANUP=no HEKETI_TEST_VAGRANT=no
HEKETI_TEST_GO_TEST_RUN=TestServerStartUnknownDbAttrs gtgw
./tests/functional/TestErrorHandling/run.sh  2>&1 | tee x

# Unit Tests integration with GitHub

# Functional Tests integration with GitHub
Automated tests run using CentOS infrastructure and the results are available here:
https://ci.centos.org/view/Gluster/job/gluster_heketi-functional/

## Commands
When a new pull request is opened in the project and the author of the pull request isn't white-listed, builder will ask "Can one of the admins verify this patch?".

    "ok to test" to accept this pull request for testing
    "test this please" for a one time test run
    "add to whitelist" to add the author to the whitelist

If the build fails for other various reasons you can rebuild.

    "retest this please" to start a new build


# Container
A container built using the latest unstable master is available after every
merge at the Docker Hub. Please see
https://github.com/heketi/heketi/tree/master/extras/docker/fromsource/Dockerfile
