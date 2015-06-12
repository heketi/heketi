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
	"fmt"
	"github.com/lpabon/godbc"
	"github.com/lpabon/heketi/utils"
)

type Brick struct {
	Id     string
	Path   string
	NodeId string
	Online bool
	Size   uint64
}

func NewBrick(size uint64) *Brick {
	id, _ := utils.GenUUID()
	return &Brick{
		Id:   id,
		Size: size,
	}
}

func (b *Brick) Create() error {
	godbc.Require(b.NodeId != "")

	// SSH into node and create brick
	b.Online = true
	b.Path = fmt.Sprintf("/fake/node/path/%v", b.Id)
	return nil
}

func (b *Brick) Destroy() error {
	godbc.Require(b.NodeId != "")
	godbc.Require(b.Path != "")

	// SSH into node and destroy the brick,
	b.Path = ""
	b.Online = false
	return nil
}
