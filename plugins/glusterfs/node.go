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
	"github.com/lpabon/heketi/requests"
	"github.com/lpabon/heketi/utils"
	"strconv"
	"strings"
)

type Node struct {
	node *requests.NodeInfoResp
}

func (m *GlusterFSPlugin) vgSize(host string, vg string) (uint64, error) {

	commands := []string{
		fmt.Sprintf("sudo vgdisplay -c %v", vg),
	}

	b, err := m.sshexec.ConnectAndExec(host+":22", commands, nil)
	if err != nil {
		return 0, err
	}
	for k, v := range b {
		fmt.Printf("[%v] ==\n%v\n", k, v)
	}

	vginfo := strings.Split(b[0], ":")
	if len(vginfo) < 12 {
		return 0, errors.New("vgdisplay returned an invalid string")
	}

	return strconv.ParseUint(vginfo[11], 10, 64)

}

func (m *GlusterFSPlugin) NodeAdd(v *requests.NodeAddRequest) (*requests.NodeInfoResp, error) {

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

	node := &Node{
		node: info,
	}

	m.db.nodes[info.Id] = node

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

func (m *GlusterFSPlugin) NodeRemove(id string) error {

	if _, ok := m.db.nodes[id]; ok {
		delete(m.db.nodes, id)
		return nil
	} else {
		return errors.New("Id not found")
	}

}

func (m *GlusterFSPlugin) NodeInfo(id string) (*requests.NodeInfoResp, error) {

	if node, ok := m.db.nodes[id]; ok {
		info := &requests.NodeInfoResp{}
		*info = *node.node
		return info, nil
	} else {
		return nil, errors.New("Id not found")
	}

}
