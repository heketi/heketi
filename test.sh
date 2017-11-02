#!/bin/bash

# main test runner for heketi
# Executes all executable scripts under tests dir
# in sorted order.

FAILURES=()

vecho () {
	if [[ "${verbose}" = "yes" ]] ; then
		echo "$*"
	fi
}

run_test() {
	cmd="${1}"
	vecho "-- Running: ${tname} --"
	"${cmd}"
	sts=$?
	if [[ ${sts} -ne 0 ]]; then
		vecho "failed ${cmd} [${sts}]"
		FAILURES+=(${cmd})
		if [[ "${exitfirst}" = "yes" ]]; then
			exit 1
		fi
	fi
}

summary() {
	if [[ ${#FAILURES[@]} -gt 0 ]]; then
		echo "ERROR: failing tests:"
		for i in ${!FAILURES[@]}; do
			echo "  ${FAILURES[i]}"
		done
		exit 1
	else
		echo "all tests passed"
		exit 0
	fi
}

trap summary EXIT

CLI="$(getopt -o xvh --long exitfirst,verbose,help -n $0 -- "$@")"
eval set -- "${CLI}"
while true ; do
	case "$1" in
		-x|--exitfirst)
			exitfirst=yes
			shift
		;;
		-v|--verbose)
			verbose=yes
			shift
		;;
		-h|--help)
			echo "$0 [options]"
			echo "  Options:"
			echo "    -v|--verbose      Print verbose output"
			echo "    -x|--exitfirst    Exit on first test failure"
			echo "    -h|--help         Display help"
			exit 0
		;;
		--)
			shift
			break
		;;
		*)
			echo "unknown option" >&2
			exit 2
		;;
	esac
done

SCRIPT_DIR="$(cd "$(dirname "${0}")" && pwd)"

# environment vars exported for test scripts
# (this way test scripts dont need cli parsing, we do it here)
export HEKETI_TEST_EXITFIRST=${exitfirst}
export HEKETI_TEST_SCRIPT_DIR="${SCRIPT_DIR}"

cd "${SCRIPT_DIR}"
for tname in $(ls tests | sort) ; do
	tpath="./tests/${tname}"
	if [[ ${tpath} =~ .*\.sh$ && -f ${tpath} && -x ${tpath} ]]; then
		run_test "${tpath}"
	fi
done
