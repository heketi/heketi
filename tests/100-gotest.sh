#!/bin/sh

GOFILES="$(go list ./... | grep -v vendor)"
exec go test ${GOFILES}
