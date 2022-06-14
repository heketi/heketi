FROM docker.io/centos:7
MAINTAINER Heketi Developers <heketi-devel@gluster.org>
ARG HEKETI_REPO="https://github.com/heketi/heketi.git"
ARG HEKETI_BRANCH="release/10"
ARG GO_VERSION=1.18.1


# let's setup all the necessary environment variables
ENV BUILD_HOME=/build
ENV GOPATH=$BUILD_HOME/golang
ENV PATH=$GOPATH/bin:$PATH
ENV GOTAR=go${GO_VERSION}.linux-amd64.tar.gz

# install dependencies, build and cleanup
RUN mkdir $BUILD_HOME $GOPATH && \
    yum -y install git make mercurial && \
    mkdir -p $GOPATH/src/github.com/heketi && \
    curl -o /tmp/$GOTAR https://storage.googleapis.com/golang/$GOTAR && \
    tar xzf /tmp/$GOTAR -C /usr/local && \
    cd $GOPATH/src/github.com/heketi && \
    git clone -b $HEKETI_BRANCH $HEKETI_REPO && \
    cd $GOPATH/src/github.com/heketi/heketi && \
    true

WORKDIR $GOPATH/src/github.com/heketi/heketi
ENV PATH=${PATH}:/usr/local/go/bin
