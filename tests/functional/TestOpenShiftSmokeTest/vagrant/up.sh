#!/bin/sh

vagrant up --no-provision $@ \
    && vagrant provision \
    && vagrant halt \
    && vagrant up
