[![Stories in Ready](https://badge.waffle.io/heketi/heketi.png?label=in%20progress&title=In%20Progress)](https://waffle.io/heketi/heketi)
[![Build Status](https://travis-ci.org/heketi/heketi.svg?branch=master)](https://travis-ci.org/heketi/heketi)
[![Coverage Status](https://coveralls.io/repos/heketi/heketi/badge.svg)](https://coveralls.io/r/heketi/heketi)
[![Join the chat at https://gitter.im/heketi/heketi](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/heketi/heketi?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

# Heketi

Heketi provides a RESTful management interface which can be used to manage the life cycle of GlusterFS volumes.  The goal of Heketi is to provide a simple way to create, list, and delete GlusterFS volumes in multiple storage clusters.  Heketi intelligently will manage the allocation, creation, and deletion of bricks throughout the disks in the cluster.

# Status
Heketi is currently under heavy development to provide the new approved [API](https://github.com/heketi/heketi/wiki/API).  The [prototype](https://github.com/heketi/heketi/tree/prototype) can be used to experience what Heketi will provide.  The API for the prototype can be found [here](https://github.com/heketi/heketi/wiki/API/c4be1ddcfd17e72117ebc584d646eec2987fcb58).  You can also try out the demo below which is based on the prototype version. 

# Documentation
Please visit the [WIKI](http://github.com/heketi/heketi/wiki) for project documentation and demo information

# Try out the Prototype version
Please visit [Vagrant-Heketi](https://github.com/heketi/vagrant-heketi) to try out the demo using the [prototype](https://github.com/heketi/heketi/tree/prototype) version of Heketi.

# Licensing
Heketi is licensed under the Apache License, Version 2.0.  See [LICENSE](https://github.com/heketi/heketi/blob/master/LICENSE) for the full license text.
