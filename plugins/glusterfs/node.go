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
	"github.com/lpabon/heketi/requests"
	"github.com/lpabon/heketi/utils"
)

type Node struct {
	node *requests.NodeInfoResp
}

func (m *GlusterFSPlugin) NodeAdd(v *requests.NodeAddRequest) (*requests.NodeInfoResp, error) {

	m.rwlock.Lock()
	defer m.rwlock.Unlock()

	var err error

	info := &requests.NodeInfoResp{}
	info.Name = v.Name
	info.Zone = v.Zone

	// in kb
	info.Storage.Total, err = m.vgSize(v.Name, v.Lvm.VolumeGroup)
	if err != nil {
		return nil, err
	}

	info.Id, err = utils.GenUUID()
	if err != nil {
		return nil, err
	}

	// Create struct to save on the DB
	node := &Node{
		node: info,
	}

	// Save to the db
	m.db.nodes[info.Id] = node

	// Save db to persistent storage
	m.db.Commit()

	return node.node, nil
}

func (m *GlusterFSPlugin) NodeList() (*requests.NodeListResponse, error) {

	m.rwlock.RLock()
	defer m.rwlock.RUnlock()

	list := &requests.NodeListResponse{}
	list.Nodes = make([]requests.NodeInfoResp, 0)

	for _, info := range m.db.nodes {
		list.Nodes = append(list.Nodes, *info.node)
	}

	return list, nil
}

func (m *GlusterFSPlugin) NodeRemove(id string) error {

	m.rwlock.Lock()
	defer m.rwlock.Unlock()

	if _, ok := m.db.nodes[id]; ok {
		delete(m.db.nodes, id)
		return nil
	} else {
		return errors.New("Id not found")
	}

	// Save db to persistent storage
	m.db.Commit()

	return nil

}

func (m *GlusterFSPlugin) NodeInfo(id string) (*requests.NodeInfoResp, error) {

	m.rwlock.RLock()
	defer m.rwlock.RUnlock()

	if node, ok := m.db.nodes[id]; ok {
		return node.node, nil
	} else {
		return nil, errors.New("Id not found")
	}

}
