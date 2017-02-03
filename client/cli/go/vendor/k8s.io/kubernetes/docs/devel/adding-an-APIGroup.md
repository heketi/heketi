Adding an API Group
===============

This document includes the steps to add an API group. You may also want to take
a look at PR [#16621](https://github.com/kubernetes/kubernetes/pull/16621) and
PR [#13146](https://github.com/kubernetes/kubernetes/pull/13146), which add API
groups.

Please also read about [API conventions](api-conventions.md) and
[API changes](api_changes.md) before adding an API group.

### Your core group package:

We plan on improving the way the types are factored in the future; see
[#16062](https://github.com/kubernetes/kubernetes/pull/16062) for the directions
in which this might evolve.

1. Create a folder in pkg/apis to hold your group. Create types.go in
pkg/apis/`<group>`/ and pkg/apis/`<group>`/`<version>`/ to define API objects
in your group;

2. Create pkg/apis/`<group>`/{register.go, `<version>`/register.go} to register
this group's API objects to the encoding/decoding scheme (e.g.,
[pkg/apis/authentication/register.go](../../pkg/apis/authentication/register.go) and
[pkg/apis/authentication/v1beta1/register.go](../../pkg/apis/authentication/v1beta1/register.go);

3. Add a pkg/apis/`<group>`/install/install.go, which is responsible for adding
the group to the `latest` package, so that other packages can access the group's
meta through `latest.Group`. You probably only need to change the name of group
and version in the [example](../../pkg/apis/authentication/install/install.go)). You
need to import this `install` package in {pkg/master,
pkg/client/unversioned}/import_known_versions.go, if you want to make your group
accessible to other packages in the kube-apiserver binary, binaries that uses
the client package.

Step 2 and 3 are mechanical, we plan on autogenerate these using the
cmd/libs/go2idl/ tool.

### Scripts changes and auto-generated code:

1. Generate conversions and deep-copies:

    1. Add your "group/" or "group/version" into
       cmd/libs/go2idl/conversion-gen/main.go;
    2. Make sure your pkg/apis/`<group>`/`<version>` directory has a doc.go file
       with the comment `// +k8s:deepcopy-gen=package,register`, to catch the
       attention of our generation tools.
    3. Make sure your `pkg/apis/<group>/<version>` directory has a doc.go file
       with the comment `// +k8s:conversion-gen=<internal-pkg>`, to catch the
       attention of our generation tools.  For most APIs the only target you
       need is `k8s.io/kubernetes/pkg/apis/<group>` (your internal API).
    3. Make sure your `pkg/apis/<group>` and `pkg/apis/<group>/<version>` directories
       have a doc.go file with the comment `+groupName=<group>.k8s.io`, to correctly
       generate the DNS-suffixed group name.
    5. Run hack/update-all.sh.

2. Generate files for Ugorji codec:

    1. Touch types.generated.go in pkg/apis/`<group>`{/, `<version>`};
    2. Run hack/update-codecgen.sh.

3. Generate protobuf objects:

    1. Add your group to `cmd/libs/go2idl/go-to-protobuf/protobuf/cmd.go` to
       `New()` in the `Packages` field
    2. Run hack/update-generated-protobuf.sh

### Client (optional):

We are overhauling pkg/client, so this section might be outdated; see
[#15730](https://github.com/kubernetes/kubernetes/pull/15730) for how the client
package might evolve. Currently, to add your group to the client package, you
need to:

1. Create pkg/client/unversioned/`<group>`.go, define a group client interface
and implement the client. You can take pkg/client/unversioned/extensions.go as a
reference.

2. Add the group client interface to the `Interface` in
pkg/client/unversioned/client.go and add method to fetch the interface. Again,
you can take how we add the Extensions group there as an example.

3. If you need to support the group in kubectl, you'll also need to modify
pkg/kubectl/cmd/util/factory.go.

### Make the group/version selectable in unit tests (optional):

1. Add your group in pkg/api/testapi/testapi.go, then you can access the group
in tests through testapi.`<group>`;

2. Add your "group/version" to `KUBE_TEST_API_VERSIONS` in
   hack/make-rules/test.sh and hack/make-rules/test-integration.sh

TODO: Add a troubleshooting section.



<!-- BEGIN MUNGE: GENERATED_ANALYTICS -->
[![Analytics](https://kubernetes-site.appspot.com/UA-36037335-10/GitHub/docs/devel/adding-an-APIGroup.md?pixel)]()
<!-- END MUNGE: GENERATED_ANALYTICS -->
