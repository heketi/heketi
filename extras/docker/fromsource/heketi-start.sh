#!/bin/sh

: ${HEKETI_PATH:=/var/lib/heketi}

echo "Setting up heketi database"

if [[ -d "${HEKETI_PATH}" ]]; then
    # Test that our volume is writable.
    touch "${HEKETI_PATH}/test" && rm "${HEKETI_PATH}/test"
    if [ $? -ne 0 ]; then
        echo "${HEKETI_PATH} is read-only"
        exit 1
    fi

    if [[ ! -f "${HEKETI_PATH}/heketi.db" ]]; then
        echo "No database file found" | tee -a "${HEKETI_PATH}/container.log"
        out=$(mount | grep "${HEKETI_PATH}" | grep heketidbstorage)
        if [[ $? -eq 0 ]]; then
            echo "Database volume found: ${out}" | tee -a "${HEKETI_PATH}/container.log"
            echo "Database file is expected, waiting..." | tee -a "${HEKETI_PATH}/container.log"
            check=0
            while [[ ! -f "${HEKETI_PATH}/heketi.db" ]]; do
                sleep 5
                if [[ ${check} -eq 5 ]]; then
                   echo "Database file did not appear, exiting." | tee -a "${HEKETI_PATH}/container.log"
                   exit 1
                fi
                ((check+=1))
            done
        fi
    fi

    stat "${HEKETI_PATH}/heketi.db" | tee -a "${HEKETI_PATH}/container.log"
    # Workaround for scenario where a lock on the heketi.db has not been
    # released.
    flock -w 60 "${HEKETI_PATH}/heketi.db" true
    if [[ $? -ne 0 ]]; then
        echo "Database file is read-only" | tee -a "${HEKETI_PATH}/container.log"
    fi
else
    mkdir -p "${HEKETI_PATH}"
    if [[ $? -ne 0 ]]; then
        echo "Failed to create ${HEKETI_PATH}"
        exit 1
    fi
fi

if [ -f /backupdb/heketi.db.gz ] ; then
    gunzip -c /backupdb/heketi.db.gz > /var/lib/heketi/heketi.db
    if [ $? -ne 0 ] ; then
        echo "Unable to copy database"
        exit 1
    fi
    echo "Copied backup db to /var/lib/heketi/heketi.db"
elif [ -f /backupdb/heketi.db ] ; then
    cp /backupdb/heketi.db /var/lib/heketi/heketi.db
    if [ $? -ne 0 ] ; then
        echo "Unable to copy database"
        exit 1
    fi
    echo "Copied backup db to /var/lib/heketi/heketi.db"
fi

/usr/bin/heketi --config=/etc/heketi/heketi.json
