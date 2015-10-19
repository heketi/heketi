# set author and base
FROM centos:centos7
MAINTAINER Sid Carter <nospam@sidcarter.com>

LABEL version="0.3"
LABEL description="Centos 7 docker image for heketi"

# let's setup all the necessary environment variables
ENV BUILD_HOME=/build
ENV GOPATH=$BUILD_HOME/golang
ENV PATH=$GOPATH/bin:$PATH
ENV HEKETIC_CONF_DIR=/etc/heketi

# install dependencies, build and cleanup
RUN mkdir $BUILD_HOME $GOPATH $HEKETI_CONF_DIR && \
    yum -y install epel-release && \
    yum -y install golang git && \
    yum -y install make && \
    yum -y clean all && \
    cd $BUILD_HOME && \
    git clone https://github.com/heketi/heketi.git && \
    go get github.com/heketi/heketi && \
    go get github.com/robfig/glock && \
    glock sync github.com/heketi/heketi && \
    cd heketi && make && \
    cp $GOPATH/bin/heketi /usr/bin/heketi && \
    cd && rm -rf $BUILD_HOME && \
    yum -y remove git golang make epel-release && \
    yum -y autoremove && \
    yum -y clean all

ADD ./heketi.json /etc/heketi/heketi.json

VOLUME /etc/heketi

# script to create directories and change ownership
ADD ./post_setup.sh /tmp/post_setup.sh
RUN chmod +x /tmp/post_setup.sh
RUN /tmp/post_setup.sh

VOLUME /var/lib/heketi

# expose pose and set entypoint with config option
ENTRYPOINT ["/usr/bin/heketi"]
EXPOSE 8080

CMD ["-config=/etc/heketi/heketi.json"]
