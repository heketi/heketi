#!/bin/sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${0}")" && pwd)"
cd "${SCRIPT_DIR}"

set +e
set -x
for hn in storage0 storage1 storage2 storage3; do
    echo '+++' $hn
    vagrant ssh $hn -- rpm -qa | grep gluster
    vagrant ssh $hn -- systemctl status gluster-blockd
    vagrant ssh $hn -- sudo grep . /var/log/gluster-block/gluster-block-cli.log /var/log/gluster-block/gluster-block-configshell.log /var/log/gluster-block/gluster-block-gfapi.log /var/log/gluster-block/gluster-blockd.log
done

exit 0
