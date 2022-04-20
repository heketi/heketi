#!/bin/bash

TAGS="functional dbexamples"
find tests/functional -name '*.go' -print0 | \
	xargs -0 dirname | sort -u | \
	sed -e 's,^/*,github.com/heketi/heketi/v10/,' | \
	xargs -L1 go test -c -o /dev/null -tags "$TAGS"
