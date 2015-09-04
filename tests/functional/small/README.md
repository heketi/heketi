# Small Functional Test
This functional test can be used on a system with at least 8GB of RAM.

## Requirements

* Vagrant
* Andible
* Hypervisor: VirtualBox or Libvirt/KVM

## Setup 

* Go to `tests/functional/small/vagrant` and type:
    * If using VirtualBox

```
$ vagrant up
```

    * If using Libvirt


```
$ vagrant up --provider=libvirt
```

## Running the Tests

* Go to the top of the source tree build and run a new Heketi server:

```
$ rm heketi.db
$ make
$ ./heketi -config=tests/functional/small/config/heketi.json | tee log

```

* Once it is ready, then start running the tests

```
$ cd tests/functional/small/tests
$ go test -tags ftsmall
```

Output will be shows by the logs on the heketi server.