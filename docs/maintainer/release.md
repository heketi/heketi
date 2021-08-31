# Heketi Release management
This page describes the Heketi project administrators release a version of Heketi.

#### Create new branch
* Master will be tagged with a value of `v{number}.0.0`. Make sure it is an annotated tag.
* A branch from this tag on master will be created called: `release/{number}`.

#### Update Docker file
The Dockerfile in the branch must be updated so that the Docker Hub build system creates the correct container from the appropriate branch:
* Update the Heketi branch value of `HEKETI_BRANCH` in the Docker file `extras/docker/fromsource/Dockerfile` in the new branch.

#### Updating Docker Hub
* Go to [Docker Hub](https://hub.docker.com/), and click on the the `heketi` team.
* Go to `heketi/heketi`
* Click on `Build Settings`
* Hit the `+` sign to add a new branch:
    * Name: `release/{number}`
    * Dockerfile: `extras/docker/fromsource`
    * Tag: `{number}`
* If the team is confident to release the new version, they will update the `latest` tag so that the _Name_ is set to the new branch name of `release/{number}`.

#### Creating a Github Release
* A new release will be created and updated with all the new clients.
* Clients can be created by using the following command: `make release`
* Each of these clients will be in the directory `dist`
* Each of these clients will be added and uploaded to the Github release.

#### Updating the community
* An email will be sent to the mailing list with the new release and an update of the changes in that version.