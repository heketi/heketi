//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"fmt"
)

// nodeHosts is a mapping from the node ID to the hosts's
// management name.
type nodeHosts map[string]string

type tryOnHosts struct {
	Hosts nodeHosts
	done  func(error) bool
}

func newTryOnHosts(hosts nodeHosts) *tryOnHosts {
	return &tryOnHosts{Hosts: hosts}
}

func (c *tryOnHosts) run(f func(host string) error) error {
	// if a custom done is not provided only stop
	// if err == nil
	done := c.done
	if done == nil {
		done = func(err error) bool {
			return err == nil
		}
	}

	nodeUp := currentNodeHealthStatus()
	for nodeId, host := range c.Hosts {
		if up, found := nodeUp[nodeId]; found && !up {
			// if the node is in the cache and we know it was not
			// recently healthy, skip it
			logger.Debug("skipping node. %v (%v) is presumed unhealthy",
				nodeId, host)
			continue
		}
		logger.Debug("running function on node %v (%v)", nodeId, host)
		err := f(host)
		if done(err) {
			return err
		}
		logger.Warning("error running on node %v (%v): %v", nodeId, host, err)
	}
	return fmt.Errorf("no hosts available (%v total)", len(c.Hosts))
}
