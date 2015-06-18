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

package mock

import (
	"errors"
	"github.com/heketi/heketi/requests"
	"github.com/heketi/heketi/utils"
)

type Node struct {
	node *requests.NodeInfoResp
}

func (m *MockPlugin) NodeAddDevice(id string, req *requests.DeviceAddRequest) error {

	if node, ok := m.db.nodes[id]; ok {

		for device := range req.Devices {
			dev := &requests.DeviceResponse{}
			dev.Name = req.Devices[device].Name
			dev.Weight = req.Devices[device].Weight
			dev.Id = utils.GenUUID()

			node.node.Devices[dev.Id] = dev
		}

	} else {
		return errors.New("Node not found")
	}

	return nil
}

func (m *MockPlugin) NodeAdd(v *requests.NodeAddRequest) (*requests.NodeInfoResp, error) {

	var err error

	info := &requests.NodeInfoResp{}
	info.Name = v.Name
	info.Zone = v.Zone
	info.Id = utils.GenUUID()
	info.Devices = make(map[string]*requests.DeviceResponse)
	if err != nil {
		return nil, err
	}

	node := &Node{
		node: info,
	}

	m.db.nodes[info.Id] = node

	return m.NodeInfo(info.Id)
}

func (m *MockPlugin) NodeList() (*requests.NodeListResponse, error) {

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

func (m *MockPlugin) NodeRemove(id string) error {

	if _, ok := m.db.nodes[id]; ok {
		delete(m.db.nodes, id)
		return nil
	} else {
		return errors.New("Id not found")
	}

}

func (m *MockPlugin) NodeInfo(id string) (*requests.NodeInfoResp, error) {

	if node, ok := m.db.nodes[id]; ok {
		info := &requests.NodeInfoResp{}
		*info = *node.node
		return info, nil
	} else {
		return nil, errors.New("Id not found")
	}

}
