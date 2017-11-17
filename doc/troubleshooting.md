Troubleshooting Guide
========================================

## Deployment

## Setup

1. Error "Unable to open topology file":
    * You use old syntax of single '-' as prefix for json option. Solution: Use new syntax of double hyphens.
1. Cannot create volumes smaller than 4GB:
    * This is due to the limits set in Heketi.  If you want to change the limits, please update the config file using the [advanced settings](admin/server.md#advanced-options).  If you are using Heketi as a container, then you must create a new container based on Heketi which just updates the config file `/etc/heketi/heketi.json`.

## Management

TODO
