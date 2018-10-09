# Dockerfile to build a container image from the current working tree. This
# includes any non-committed changes.
#
# Build from the root of the repository with:
#
#  $ buildah bud --file=extras/docker/fromsource/Dockerfile .
#
#
# Alternatively, the legacy way of building the image based on the master
# branch of the heketi github repository is still functional:
#
#  $ buildah bud --file=extras/docker/fromsource
#

# need to use fedora 27 until glide is fixed in fedora 28
# (or we get an alternative glide)
FROM fedora:27 AS build
MAINTAINER Heketi Developers <heketi-devel@gluster.org>

LABEL version="1.3.1"
LABEL description="Development build"

# let's setup all the necessary environment variables
ENV BUILD_HOME=/build
ENV GOPATH=$BUILD_HOME/golang
ENV PATH=$GOPATH/bin:$PATH
ENV HEKETI_REPO=https://github.com/heketi/heketi.git
ENV HEKETI_BRANCH=master

# we'll copy the contents of the working directory, depending on the files that
# may (not) exist, the way of building the binaries is decided
COPY . $GOPATH/src/github.com/heketi/heketi/

# only run this if the build context is extras/docker/fromsource
# this can be detected by the presence of a Dockerfile in the root from the
# previous COPY instruction
#
# Do a full build, including checkout of the master branch
RUN true \
    && [[ ! -e $GOPATH/src/github.com/heketi/heketi/Dockerfile ]] \
    || \
      ( true \
        && rm -rf $GOPATH/src/github.com/heketi/heketi \
        && dnf -y install glide golang git make mercurial \
        && mkdir -p $GOPATH/src/github.com/heketi \
        && cd $GOPATH/src/github.com/heketi \
        && git clone -b $HEKETI_BRANCH $HEKETI_REPO \
        && cd $GOPATH/src/github.com/heketi/heketi \
        && glide install -v \
        && make \
        && cp heketi /usr/bin/heketi \
        && cp client/cli/go/heketi-cli /usr/bin/heketi-cli \
        && mkdir /etc/heketi \
	&& cp extras/docker/fromsource/heketi.json /etc/heketi/heketi.json \
	&& cp extras/docker/fromsource/heketi-start.sh /usr/bin/heketi-start.sh \
      ) \
    && true

# only run this in case the previous RUN instruction did not build and
# installed /usr/bin/heketi
#
# install dependencies and build
# in case vendor/ is copied, no need to run 'glide install'
RUN true \
    && [[ -e /usr/bin/heketi ]] \
    || \
      ( true \
        && dnf -y install glide golang git make mercurial \
        && cd $GOPATH/src/github.com/heketi/heketi \
        && ( [[ -e vendor ]] || glide install -v ) \
        && make \
        && cp heketi /usr/bin/heketi \
        && cp client/cli/go/heketi-cli /usr/bin/heketi-cli \
        && mkdir /etc/heketi \
	&& cp extras/docker/fromsource/heketi.json /etc/heketi/heketi.json \
	&& cp extras/docker/fromsource/heketi-start.sh /usr/bin/heketi-start.sh \
      ) \
    && true


# the final container can be based on the latest Fedora version
FROM fedora:latest

MAINTAINER Heketi Developers <heketi-devel@gluster.org>

LABEL version="1.3.1"
LABEL description="Development build"

# install latest updates
RUN true \
    && dnf -y update \
    && dnf clean all \
    && true

# copy the compiled binaries from the build container
COPY --from=build /usr/bin/heketi /usr/bin/heketi
COPY --from=build /usr/bin/heketi-cli /usr/bin/heketi-cli
COPY --from=build /etc/heketi/heketi.json /etc/heketi/heketi.json
COPY --from=build /usr/bin/heketi-start.sh /usr/bin/heketi-start.sh

VOLUME /etc/heketi

RUN mkdir /var/lib/heketi
VOLUME /var/lib/heketi

# expose port, set user and set entrypoint with config option
ENTRYPOINT ["/usr/bin/heketi-start.sh"]
EXPOSE 8080
