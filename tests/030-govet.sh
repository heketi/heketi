#!/bin/bash

GOPACKAGES="$(go list ./... | grep -v vendor)"
exec go vet ${GOPACKAGES}
