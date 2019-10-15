* Docker hub integration with GitHub

DockerHub requires OAuth authentication to GitHub which has administrative
rights to read repo details and write/edit repo webhooks settings. It requires
such wide permissions because it wants to inspect the branches,show them in the
UI and auto create webhooks for the branches that are enabled in the DockerHub
interface.

There is no option to manually specify the branch names and tags that we are
interested in building. For most user container repositories this isn't a big
deal but for container repos owned by orgs, this becomes a problem. If a
maintainer of Heketi connects DockerHub Heketi container repo to GitHub using
their OAuth, they will expose all the private repos they have in Github to
everyone else in the Heketi DockerHub org.

Solution:
1. Create a service account. This is a GitHub account that is created that for
the build requirements.  There is no distinction in GitHub for service accounts,
it is just another account. The name of this account is `heketimachine`. This
account has been linked to raghavendra-talur's email ID.
2. Create a `build` team in the GitHub Heketi org and provide read/write access
to repos.
3. Add the service user `heketimachine` to the `build` team.
4. Log in to Docker Hub, authenticate with OAuth but this time with the service
account `heketimachine` on GitHub, not your GitHub ID.
5. Now you should be able to configure container builds.
