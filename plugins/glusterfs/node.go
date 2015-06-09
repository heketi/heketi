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
)

type Node struct {
	node *requests.NodeInfoResp
}

func (m *GlusterFSPlugin) NodeAdd(v *requests.NodeAddRequest) (*requests.NodeInfoResp, error) {
	m.db.current_id++

	info := &requests.NodeInfoResp{}
	info.Name = v.Name
	info.Zone = v.Zone
	info.Id = m.db.current_id

	node := &Node{
		node: info,
	}

	m.db.nodes[m.db.current_id] = node

	return m.NodeInfo(info.Id)
}

func (m *GlusterFSPlugin) NodeList() (*requests.NodeListResponse, error) {

	list := &requests.NodeListResponse{}
	list.Nodes = make([]requests.NodeInfoResp, 0)

	for id, _ := range m.db.nodes {
		info, err := m.NodeInfo(id)
		if err != nil {
			return nil, err
		}
		list.Nodes = append(list.Nodes, *info)
	}

	return list, nil
}

func (m *GlusterFSPlugin) NodeRemove(id uint64) error {

	if _, ok := m.db.nodes[id]; ok {
		delete(m.db.nodes, id)
		return nil
	} else {
		return errors.New("Id not found")
	}

}

func (m *GlusterFSPlugin) NodeInfo(id uint64) (*requests.NodeInfoResp, error) {

	if node, ok := m.db.nodes[id]; ok {
		info := &requests.NodeInfoResp{}
		*info = *node.node
		return info, nil
	} else {
		return nil, errors.New("Id not found")
	}

}
