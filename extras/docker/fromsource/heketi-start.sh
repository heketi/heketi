#!/bin/sh

if [ -f /backupdb/heketi.db ] ; then
    cp /backupdb/heketi.db /var/lib/heketi/heketi.db
    if [ $? -ne 0 ] ; then
        echo "Unable to copy database"
        exit 1
    fi
    echo "Copied backup db to /var/lib/heketi/heketi.db"
fi
#ssh dirtyhack

if [ "$HEKETI_EXECUTOR" == "ssh" ]; then
    /usr/sbin/sshd -D &
    chmod 600 /etc/heketi/heketi_key*
fi
/usr/bin/heketi --config=/etc/heketi/heketi.json
