#!/bin/bash
#
# HEKETI_TOPOLOGY_FILE can be passed as an environment variable with the
# filename of the initial topology.json. In case the heketi.db does not exist
# yet, this file will be used to populate the database.

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

# if the heketi.db does not exist and HEKETI_TOPOLOGY_FILE is set, start the
# heketi service in the background and load the topology. Once done, move the
# heketi service back to the foreground again.
if [[ "$(stat -c %s ${HEKETI_PATH}/heketi.db)" == 0 && -n "${HEKETI_TOPOLOGY_FILE}" ]]; then
    # start hketi in the background
    /usr/bin/heketi --config=/etc/heketi/heketi.json &

    # wait until heketi replies
    while ! curl http://localhost:8080/hello; do
        sleep 0.5
    done

    # load the topology
    if [[ -n "${HEKETI_ADMIN_KEY}" ]]; then
        HEKETI_SECRET_ARG="--secret='${HEKETI_ADMIN_KEY}'"
    fi
    heketi-cli --user=admin "${HEKETI_SECRET_ARG}" topology load --json="${HEKETI_TOPOLOGY_FILE}"
    if [[ $? -ne 0 ]]; then
        # something failed, need to exit with an error
        kill %1
        echo "failed to load topology from ${HEKETI_TOPOLOGY_FILE}"
        exit 1
    fi

    # bring heketi back to the foreground
    fg %1
else
    # just start in the foreground
    exec /usr/bin/heketi --config=/etc/heketi/heketi.json
fi
