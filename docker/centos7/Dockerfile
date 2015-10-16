# set author and base
FROM centos:centos7
MAINTAINER Sid Carter <nospam@sidcarter.com>

LABEL version="0.1"
LABEL description="Centos 7 docker image for hekekti"

# install dependencies for heketi
RUN yum -y install epel-release
RUN yum -y clean all

RUN yum -y install golang git
RUN yum -y install make
RUN yum -y clean all

# let's checkout heketi repo next
ENV BUILD_HOME=/build
RUN mkdir $BUILD_HOME
RUN cd $BUILD_HOME
RUN git clone https://github.com/heketi/heketi.git

# prep go and dependencies
ENV GOPATH=$BUILD_HOME/golang
RUN mkdir $GOPATH
ENV PATH=$GOPATH/bin:$PATH
RUN go get github.com/heketi/heketi
RUN go get github.com/robfig/glock
RUN glock sync github.com/heketi/heketi

RUN cd heketi && make

RUN cp $GOPATH/bin/heketi /usr/bin/heketi
ADD ../../etc/heketi.json /etc/heketi.json

# script to create directories and change ownership
ADD ./post_setup.sh /tmp/post_setup.sh
RUN chmod +x /tmp/post_setup.sh
RUN /tmp/post_setup.sh

# cleanup
RUN rm -rf $BUILD_HOME

# start heketi
ENTRYPOINT ["/usr/bin/heketi"]

EXPOSE 8080
CMD ["-config=/etc/heketi.json"]
