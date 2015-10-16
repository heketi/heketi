#!/bin/bash

useradd heketi -m -k /etc/skel
mkdir /var/lib/heketi
chown -R heketi:heketi /var/lib/heketi
