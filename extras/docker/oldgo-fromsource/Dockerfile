# set author and base
FROM fedora
MAINTAINER Heketi Developers <heketi-devel@gluster.org>
ARG HEKETI_REPO="https://github.com/heketi/heketi.git"
ARG HEKETI_BRANCH="master"
ARG GO_VERSION=1.13.15

LABEL version="1.3.1"
LABEL description="Development build"

# let's setup all the necessary environment variables
ENV BUILD_HOME=/build
ENV GOPATH=$BUILD_HOME/golang
ENV PATH=$GOPATH/bin:$PATH
# where to clone from
ENV HEKETI_REPO=$HEKETI_REPO
ENV HEKETI_BRANCH=$HEKETI_BRANCH
ENV GO111MODULE=off

# install dependencies, build and cleanup
RUN mkdir $BUILD_HOME $GOPATH && \
    dnf -y install git make mercurial findutils && \
    dnf -y clean all && \
    curl -o /tmp/go.tar.gz "https://storage.googleapis.com/golang/go${GO_VERSION}.linux-amd64.tar.gz" && \
    tar -xzv -C /opt/  -f /tmp/go.tar.gz && \
    export GOROOT=/opt/go && \
    export PATH=$GOROOT/bin:$PATH && \
    mkdir -p $GOPATH/src/github.com/heketi && \
    cd $GOPATH/src/github.com/heketi && \
    git clone -b $HEKETI_BRANCH $HEKETI_REPO && \
    cd $GOPATH/src/github.com/heketi/heketi && \
    make && \
    mkdir -p /etc/heketi /var/lib/heketi && \
    make install prefix=/usr && \
    cp /usr/share/heketi/container/heketi-start.sh /usr/bin/heketi-start.sh && \
    cp /usr/share/heketi/container/heketi.json /etc/heketi/heketi.json && \
    cd && rm -rf $BUILD_HOME && \
    rm -rf /opt/go && \
    dnf -y remove git mercurial && \
    dnf -y autoremove && \
    dnf -y clean all

VOLUME /etc/heketi /var/lib/heketi

# expose port, set user and set entrypoint with config option
ENTRYPOINT ["/usr/bin/heketi-start.sh"]
EXPOSE 8080
