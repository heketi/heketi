#!/bin/sh

vagrant up --provider=libvirt 
vagrant provision
vagrant halt
vagrant up --provider=libvirt
vagrant provision
