# Automated Functional Tests
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
A container built using the latest unstable master is available after every merge at the Docker Hub. Please see https://github.com/heketi/heketi/tree/master/extras/docker/unstable
