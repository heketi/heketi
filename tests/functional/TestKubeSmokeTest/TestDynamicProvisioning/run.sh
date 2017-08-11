#!/bin/sh

TOP=../../../..
CURRENT_DIR=`pwd`
RESOURCES_DIR=$CURRENT_DIR/resources
FUNCTIONAL_DIR=${CURRENT_DIR}/../..
CLIENTDIR=$TOP/client/cli/go

source ${FUNCTIONAL_DIR}/lib.sh

### MAIN ###

# Check
volumes=$(heketi-cli --json volume list | jq '.[] | length')
[ $? -eq 0 ] || fail "Unable to get volume list"
if [ $volumes -ne 0 ] ; then
    fail "There is already a volume available. Zero expected"
fi

# Create
kubectl create -f pvc.yaml
wait_for_pvc_bound "default" "pvc-test"

volumes=$(heketi-cli --json volume list | jq '.[] | length')
if [ $volumes -ne 1 ] ; then
    fail "One volume expected, found $volumes"
fi

# Check size
echo "Checking volume information is correct"
id=$(heketi-cli --json volume list | jq -r '.volumes[0]')
size=$(heketi-cli --json volume info $id | jq -r '.size')
pvc_request_size=$(kubectl get pvc pvc-test -o json | jq -r ".spec.resources.requests.storage" | sed -e "s#Gi##")
if [ $size -ne $pvc_request_size ] ; then
    fail "Size requested by pvc of ${pvc_request_size}GiB does not equal size of volume ${size}GiB"
fi

# Delete
kubectl delete -f pvc.yaml
wait_for_pvc_deleted "default" "pvc-test"

volumes=$(heketi-cli --json volume list | jq '.[] | length')
if [ $volumes -ne 0 ] ; then
    fail "Expected volume to be deleted, but it is still managed by Heketi"
fi

