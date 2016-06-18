#!/bin/sh

export ANSIBLE_HOST_KEY_CHECKING=False 
ansible-playbook -i ../../vagrant/.vagrant/provisioners/ansible/inventory/vagrant_ansible_inventory  deploy.yml
