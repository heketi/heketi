//
// Copyright (c) 2015 The heketi Authors
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
	"github.com/heketi/heketi/executors"
)

func (r *ReplicaDurability) SetDurability() {
	if r.Replica == 0 {
		r.Replica = DEFAULT_REPLICA
	}
}

func (r *ReplicaDurability) BrickSizeGenerator(size uint64) func() (int, uint64, error) {

	sets := 1
	return func() (int, uint64, error) {

		var brick_size uint64

		for {
			sets *= 2
			brick_size = size / uint64(sets)

			if brick_size < BrickMinSize {
				return 0, 0, ErrMininumBrickSize
			} else if brick_size <= BrickMaxSize {
				break
			}
		}

		return sets, brick_size, nil
	}
}

func (r *ReplicaDurability) BricksInSet() int {
	return r.Replica
}

func (r *ReplicaDurability) SetExecutorVolumeRequest(v *executors.VolumeRequest) {
	v.Type = executors.DurabilityReplica
	v.Replica = r.Replica
}
