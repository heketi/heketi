# Overview
The main purpose of this container is to be used for testing
and verification of the unstable master builds.

# How to use for testing
You will need to create a directory which has a directory containing configuraiton and any private key if necessary, and an empty directory used for storing the database.  Directory and files must be read/write by user with id 1000 and if an ssh private key is used, it must also have a mod of 0600.

Here is an example:

    $ mkdir -p heketi/config
    $ mkdir -p heketi/db
    $ cp heketi.json heketi/config
    $ cp myprivate_key heketi/config
    $ chmod 600 heketi/config/myprivate_key
    $ chown 1000:1000 -R heketi

To run:

    # docker run -d -p 8080:8080 \
                 -v $PWD/heketi/config:/etc/heketi \
                 -v $PWD/heketi/db:/var/lib/heketi \
                 heketi/heketi:dev

# Build
If you need to build it:

    # docker build --rm --tag <username>/heketi:dev .

