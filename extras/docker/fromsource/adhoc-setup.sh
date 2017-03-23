#!/bin/bash

#To run heketi in OCP and manage external Gluster we need sshd
yum -y install openssh-server openssh-clients; yum clean all;
echo 'root:screencast' | chpasswd
sed -i 's/PermitRootLogin prohibit-password/PermitRootLogin yes/' /etc/ssh/sshd_config
# SSH login fix. Otherwise user is kicked off after login
sed 's@session\s*required\s*pam_loginuid.so@session optional pam_loginuid.so@g' -i /etc/pam.d/sshd
export NOTVISIBLE="in users profile"
echo "export VISIBLE=now" >> /etc/profile