# Overview
The main purpose of this container is to be used for testing
and verification of the unstable master builds.

# How to use for testing

## Downloading
First you will need to download the latest development container:

    # docker pull heketi/heketi:dev
    
> NOTE: Must likely you will always need to do a new pull before staring your tests since the container changes so often.

## Setup
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

Now you can see the container running.  Here is an example:

```
$ sudo docker ps
CONTAINER ID        IMAGE               COMMAND                  CREATED             STATUS              PORTS                    NAMES
6e3ed5c59f87        heketidev           "/usr/bin/heketi -con"   32 minutes ago      Up 32 minutes       0.0.0.0:8080->8080/tcp   goofy_kowalevski
```

Now we can check the logs

```


# Build
If you need to build it:

    # docker build --rm --tag <username>/heketi:dev .

