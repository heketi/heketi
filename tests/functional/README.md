# Automated Functional Test

## Requirements
You will need a system with at least 16G of ram.  If you only have 8G, you can at least run the TestSmokeTest.

### Packages

```
# dnf -y install docker libvirt qemu-kvm \
   ansible vagrant vagrant-libvirt go git make 
```

Make sure docker and libvirt deamons are running

### User

The user running the tests must have password-less sudo access

## Setup

```
$ mkdir go
$ cd go
$ export GOPATH=$PWD
$ export PATH=$PATH:$GOPATH/bin
$ curl https://glide.sh/get | sh
$ mkdir -p src/github.com/heketi
$ cd src/github.com/heketi
$ git clone https://github.com/heketi/heketi.git
```

## Running

Run the entire suite:

```
$ cd $GOPATH/src/github.com/heketi/heketi/tests/functional
$ ./run.sh
```

To run a specific functional test, go into that functionl test's directory and type:

```
$ ./run.sh
```

Some of the test setup code assumes that root privileges are needed.
If the user is not already root the test setup code will run sudo, if
you know this is not needed on your system you can disable this by
setting `HEKETI_TEST_USE_SUDO=no` in your environment.

## Adding new tests

Create a new directory under tests/functional matching the style of
the current ones.  Create a shell script called `run.sh` in that directory
which will run that test.
