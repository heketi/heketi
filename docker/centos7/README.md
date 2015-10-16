dockerfile-centos7-heketi
========================

CentOS 7 dockerfile for heketi

To build:

Copy the sources down -

    # docker build --rm --tag <username>/heketi:centos7 .

To run:

    # docker run -d -p 8080:8080 <username>/heketi:centos7
