[![Stories in Ready](https://badge.waffle.io/heketi/heketi.png?label=in%20progress&title=In%20Progress)](https://waffle.io/heketi/heketi)
[![Build Status](https://travis-ci.org/heketi/heketi.svg?branch=master)](https://travis-ci.org/heketi/heketi)
[![Coverage Status](https://coveralls.io/repos/heketi/heketi/badge.svg)](https://coveralls.io/r/heketi/heketi)
[![Join the chat at https://gitter.im/heketi/heketi](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/heketi/heketi?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

# Heketi
Heketi provides a RESTful management interface which can be used to manage the life cycle of GlusterFS volumes.  The goal of Heketi is to provide a simple way to create, list, and delete GlusterFS volumes in any number of GlusterFS clusters.  Heketi will intelligently manage the allocation, creation, and deletion of bricks throughout the disks across hundreds or thousands of clusters.  To acomplish this, Heketi must first be told about the topology of the clusters.

# Workflow
When a request is received, Heketi will first allocate appropriate storage in a cluster, making sure to place brick replicas across failure domains.  It will then format, then mount the storage to create bricks for the volume requested.  Once all bricks have been automatically created, Heketi will finally satisfy the request by creating, then starting the newly created GlusterFS volume.

# Downloads
Please go to the [Releases](https://github.com/heketi/heketi/releases) page for the latest release

# Documentation
Please visit the [WIKI](http://github.com/heketi/heketi/wiki) for project documentation and demo information

# Demo
Please visit [Vagrant-Heketi](https://github.com/heketi/vagrant-heketi) to try out the demo.

# Licensing
Heketi is licensed under the Apache License, Version 2.0.  See [LICENSE](https://github.com/heketi/heketi/blob/master/LICENSE) for the full license text.
