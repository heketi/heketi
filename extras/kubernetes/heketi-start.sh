#!/bin/sh

HEKETI_DB_PATH=${HEKETI_DB_PATH:-/var/lib/heketi/heketi.db}

if [ -f /backupdb/heketi.db.gz ] ; then
    gunzip -c /backupdb/heketi.db.gz > "${HEKETI_DB_PATH}"
    if [ $? -ne 0 ] ; then
        echo "Unable to copy database"
        exit 1
    fi
    echo "Copied backup db to ${HEKETI_DB_PATH}"
elif [ -f /backupdb/heketi.db ] ; then
    cp /backupdb/heketi.db "${HEKETI_DB_PATH}"
    if [ $? -ne 0 ] ; then
        echo "Unable to copy database"
        exit 1
    fi
    echo "Copied backup db to ${HEKETI_DB_PATH}"
fi

/usr/bin/heketi --config=/etc/heketi/heketi.json
