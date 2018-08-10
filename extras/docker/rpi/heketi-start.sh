#!/bin/bash

: "${HEKETI_PATH:=/var/lib/heketi}"
: "${BACKUPDB_PATH:=/backupdb}"

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
    # This code uses a non-blocking flock in a loop rather than a blocking
    # lock with timeout due to issues with current gluster and flock
    # ( see rhbz#1613260 )
    for _ in $(seq 1 60); do
        flock --nonblock "${HEKETI_PATH}/heketi.db" true
        flock_status=$?
        if [[ $flock_status -eq 0 ]]; then
            break
        fi
        sleep 1
    done
    if [[ $flock_status -ne 0 ]]; then
        echo "Database file is read-only" | tee -a "${HEKETI_PATH}/container.log"
    fi
else
    mkdir -p "${HEKETI_PATH}"
    if [[ $? -ne 0 ]]; then
        echo "Failed to create ${HEKETI_PATH}"
        exit 1
    fi
fi

if [[ -d "${BACKUPDB_PATH}" ]]; then
    if [[ -f "${BACKUPDB_PATH}/heketi.db.gz" ]] ; then
        gunzip -c "${BACKUPDB_PATH}/heketi.db.gz" > "${BACKUPDB_PATH}/heketi.db"
        if [[ $? -ne 0 ]]; then
            echo "Unable to extract backup database" | tee -a "${HEKETI_PATH}/container.log"
            exit 1
        fi
    fi
    if [[ -f "${BACKUPDB_PATH}/heketi.db" ]] ; then
        cp "${BACKUPDB_PATH}/heketi.db" "${HEKETI_PATH}/heketi.db"
        if [[ $? -ne 0 ]]; then
            echo "Unable to copy backup database" | tee -a "${HEKETI_PATH}/container.log"
            exit 1
        fi
        echo "Copied backup db to ${HEKETI_PATH}/heketi.db"
    fi
fi

/usr/bin/heketi --config=/etc/heketi/heketi.json
