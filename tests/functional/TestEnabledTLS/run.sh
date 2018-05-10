#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "${0}")" && pwd)"
HEKETI_DIR="$(cd "$SCRIPT_DIR" && cd ../../.. && pwd)"
HEKETI_SERVER="./heketi-server"

cd "$SCRIPT_DIR" || exit 1

(cd "$HEKETI_DIR" && make server) || exit 1

cp "$HEKETI_DIR/heketi" "$HEKETI_SERVER"

openssl req \
    -newkey rsa:2048 \
    -x509 \
    -nodes \
    -keyout heketi.key \
    -new \
    -out heketi.crt \
    -subj /CN=localhost \
    -extensions alt_names \
    -config ssl.conf \
    -days 3650

if ! command -v virtualenv &>/dev/null; then
    echo "WARNING: virtualenv not installed... skipping test" >&2
    exit 0
fi

rm -rf .env
virtualenv .env
. .env/bin/activate
pip install -r "$HEKETI_DIR/client/api/python/requirements.txt"
echo '----> Running test_tls.py'
exec python test_tls.py -v "$@"
