//
// Copyright (c) 2014 The heketi Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package glusterfs

import (
	"errors"
	"fmt"
	"github.com/heketi/heketi/requests"
	"github.com/heketi/heketi/utils/ssh"
	"github.com/lpabon/godbc"
)

func (m *GlusterFSPlugin) peerDetach(name string) error {

	// Just for now, it will work wih https://github.com/lpabon/vagrant-gfsm
	sshexec := ssh.NewSshExecWithKeyFile("vagrant", "insecure_private_key")
	godbc.Check(sshexec != nil)

	// create the commands
	commands := []string{
		fmt.Sprintf("sudo gluster peer detach %v", name),
	}

	_, err := sshexec.ConnectAndExec(m.peerHost+":22", commands, nil)
	if err != nil {
		return err
	}

	return nil
}

func (m *GlusterFSPlugin) peerProbe(name string) error {

	// Just for now, it will work wih https://github.com/lpabon/vagrant-gfsm
	sshexec := ssh.NewSshExecWithKeyFile("vagrant", "insecure_private_key")
	godbc.Check(sshexec != nil)

	// create the commands
	commands := []string{
		fmt.Sprintf("sudo gluster peer probe %v", name),
	}

	_, err := sshexec.ConnectAndExec(m.peerHost+":22", commands, nil)
	if err != nil {
		return err
	}

	return nil
}

func (m *GlusterFSPlugin) NodeAddDevice(id string, req *requests.DeviceAddRequest) error {

	err := m.db.Writer(func() error {
		if node, ok := m.db.nodes[id]; ok {

			for device := range req.Devices {

				// :TODO: This should be done in parallel
				err := node.DeviceAdd(&req.Devices[device])
				if err != nil {
					return err
				}
			}

		} else {
			return errors.New("Node not found")
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Create a new ring
	m.ring.CreateRing()

	return nil
}

func (m *GlusterFSPlugin) NodeAdd(v *requests.NodeAddRequest) (*requests.NodeInfoResp, error) {

	node := NewNodeEntry(v, m.db)

	// Add to the cluster
	if m.peerHost == "" {
		m.peerHost = node.Info.Name
	} else {
		err := m.peerProbe(node.Info.Name)
		if err != nil {
			return nil, err
		}
	}

	err := m.db.Writer(func() error {
		// Save to the db
		m.db.nodes[node.Info.Id] = node

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &node.Info, nil
}

func (m *GlusterFSPlugin) NodeList() (*requests.NodeListResponse, error) {

	list := &requests.NodeListResponse{}
	list.Nodes = make([]requests.NodeInfoResp, 0)

	err := m.db.Reader(func() error {
		for _, info := range m.db.nodes {
			list.Nodes = append(list.Nodes, info.Info)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return list, nil
}

func (m *GlusterFSPlugin) NodeRemove(id string) error {

	err := m.db.Writer(func() error {
		if _, ok := m.db.nodes[id]; ok {
			// :TODO: Need to unattach!!!

			// :TODO: What happens when we remove a node that has
			// brick in use?

			delete(m.db.nodes, id)
		} else {
			return errors.New("Id not found")
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Create a new ring
	m.ring.CreateRing()

	return nil

}

func (m *GlusterFSPlugin) NodeInfo(id string) (*requests.NodeInfoResp, error) {

	var node_copy *NodeEntry

	err := m.db.Reader(func() error {
		if node, ok := m.db.nodes[id]; ok {
			node_copy = node.Copy()
			return nil
		} else {
			return errors.New("Id not found")
		}
	})

	if err != nil {
		return nil, err
	}

	return &node_copy.Info, nil

}
