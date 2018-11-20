FROM registry.centos.org/centos/centos:latest
MAINTAINER Heketi Developers <heketi-devel@gluster.org>

LABEL version="0.8"
LABEL description="Heketi based on the packages from the CentOS Storage SIG"

# install dependencies, build and cleanup
RUN yum -y install centos-release-gluster && \
    yum -y install heketi heketi-client && \
    yum -y clean all

# post install config from example file
RUN install -m 0600 -D /usr/share/heketi/container/heketi.json /etc/heketi/heketi.json

VOLUME /etc/heketi
VOLUME /var/lib/heketi

# expose port, set user and set entrypoint with config option
ENTRYPOINT ["/usr/bin/heketi-start.sh"]
EXPOSE 8080
