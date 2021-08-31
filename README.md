
# Heketi
Heketi provides a RESTful management interface which can be used to manage the life cycle of GlusterFS volumes.  With Heketi, cloud services like OpenStack Manila, Kubernetes, and OpenShift can dynamically provision GlusterFS volumes with any of the supported durability types.  Heketi will automatically determine the location for bricks across the cluster, making sure to place bricks and its replicas across different failure domains.  Heketi also supports any number of GlusterFS clusters, allowing cloud services to provide network file storage without being limited to a single GlusterFS cluster.



# Maintenance Status

⚠️ IMPORTANT - Please read this section carefully if you are currently using or plan to use Heketi or want to contribute to the project. ⚠️

As of Heketi release v10.2 the project is considered to be in maintenance only status. This means that only major bugs, performance regressions, or security defects are being considered for inclusion by the project team. Please also expect slow replies to issues.

It has been over a year since we entered "soft" maintenance mode, plus a call to other interested parties to help with Heketi development. With little additional activity since then it no longer makes sense for us to spend as much time on this project, even at the level we had in 2020.

Additionally, we would like to note that the Heketi maintenance team does not maintain the gluster volume integration found in Kubernetes that makes use of Heketi. Issues beyond the Heketi server, cli tool, and client API are best addressed elsewhere.

Thank you for your understanding.


# Downloads

Heketi source code can be obtained via the
[project's releases page](https://github.com/heketi/heketi/releases)
or by cloning this repository.

# Documentation

Heketi's official documentation is located in the
[docs/ directory](https://github.com/heketi/heketi/tree/master/docs/)
within the repo.

# Demo
Please visit [Vagrant-Heketi](https://github.com/heketi/vagrant-heketi) to try out the demo.

# Community

* Mailing list: [Join our mailing list](http://lists.gluster.org/mailman/listinfo/heketi-devel)

# Talks

* DevNation 2016

[![image](https://img.youtube.com/vi/gmEUnOmDziQ/3.jpg)](https://youtu.be/gmEUnOmDziQ)
[Slides](http://bit.ly/29avBJX)

* Devconf.cz 2016:

[![image](https://img.youtube.com/vi/jpkG4wciy4U/3.jpg)](https://www.youtube.com/watch?v=jpkG4wciy4U) [Slides](https://github.com/lpabon/go-slides)

