#!/bin/sh

GOFILES="$(go list ./... | grep -v vendor)"
# no special options, exec to go test w/ all pkgs
if [[ ${HEKETI_TEST_EXITFIRST} != "yes" ]]; then
	exec go test ${GOFILES}
fi

# our options are set so we need to handle each go package one
# at at time
failed=0
for gofile in ${GOFILES}; do
	go test "${gofile}"
	[ $? -ne 0 ] && ((failed+=1))
	if [[ ${failed} -ne 0 && ${HEKETI_TEST_EXITFIRST} = "yes" ]]; then
		exit ${failed}
	fi
done
exit ${failed}
