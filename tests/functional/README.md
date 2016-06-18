# Automated Functional Test

## Requirements


### Packages

```
# dnf -y install libvirt qemu-kvm \
   ansible vagrant vagrant-libvirt go git make 
```

### User

The user running the tests must have password-less sudo access

## Setup

```
$ mkdir go
$ cd go
$ export GOPATH=$PWD
$ export PATH=$PATH:$GOPATH/bin
$ mkdir -p src/github.com/heketi
$ cd src/github.com/heketi
$ git clone https://github.com/heketi/heketi.git
$ go get github.com/robfig/glock 
$ glock sync github.com/heketi/heketi
```

## Running

```
$ cd $GOPATH/src/github.com/heketi/heketi/tests/functional
$ ./run.sh
```

## Adding new tests

Create a new directory under tests/functional matching the style of
the current ones.  Create a shell script called `run.sh` in that directory
which will run that test.
