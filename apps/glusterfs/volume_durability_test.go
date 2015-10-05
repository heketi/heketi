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
	"github.com/heketi/tests"
	"testing"
)

func TestNoneDurabilityDefaults(t *testing.T) {
	r := &NoneDurability{}
	tests.Assert(t, r.Replica == 0)

	r.SetDurability()
	tests.Assert(t, r.Replica == 1)
}

func TestDisperseDurabilityDefaults(t *testing.T) {
	r := &DisperseDurability{}
	tests.Assert(t, r.Data == 0)
	tests.Assert(t, r.Redundancy == 0)

	r.SetDurability()
	tests.Assert(t, r.Data == DEFAULT_EC_DATA)
	tests.Assert(t, r.Redundancy == DEFAULT_EC_REDUNDANCY)
}

func TestReplicaDurabilityDefaults(t *testing.T) {
	r := &ReplicaDurability{}
	tests.Assert(t, r.Replica == 0)

	r.SetDurability()
	tests.Assert(t, r.Replica == DEFAULT_REPLICA)
}

func TestNoneDurabilitySetExecutorRequest(t *testing.T) {
	r := &NoneDurability{}
	r.SetDurability()

	v := &executors.VolumeRequest{}
	r.SetExecutorVolumeRequest(v)
	tests.Assert(t, v.Replica == 1)
	tests.Assert(t, v.Type == executors.DurabilityNone)
}

func TestDisperseDurabilitySetExecutorRequest(t *testing.T) {
	r := &DisperseDurability{}
	r.SetDurability()

	v := &executors.VolumeRequest{}
	r.SetExecutorVolumeRequest(v)
	tests.Assert(t, v.Data == r.Data)
	tests.Assert(t, v.Redundancy == r.Redundancy)
	tests.Assert(t, v.Type == executors.DurabilityDispersion)
}

func TestReplicaDurabilitySetExecutorRequest(t *testing.T) {
	r := &ReplicaDurability{}
	r.SetDurability()

	v := &executors.VolumeRequest{}
	r.SetExecutorVolumeRequest(v)
	tests.Assert(t, v.Replica == r.Replica)
	tests.Assert(t, v.Type == executors.DurabilityReplica)
}

func TestNoneDurability(t *testing.T) {
	r := &NoneDurability{}
	r.SetDurability()

	gen := r.BrickSizeGenerator(100 * GB)

	// Gen 1
	sets, brick_size, err := gen()
	tests.Assert(t, err == nil)
	tests.Assert(t, sets == 2)
	tests.Assert(t, brick_size == 50*GB)
	tests.Assert(t, 1 == r.BricksInSet())

	// Gen 2
	sets, brick_size, err = gen()
	tests.Assert(t, err == nil)
	tests.Assert(t, sets == 4)
	tests.Assert(t, brick_size == 25*GB)
	tests.Assert(t, 1 == r.BricksInSet())

	// Gen 3
	sets, brick_size, err = gen()
	tests.Assert(t, err == nil)
	tests.Assert(t, sets == 8)
	tests.Assert(t, brick_size == 12800*1024)
	tests.Assert(t, 1 == r.BricksInSet())

	// Gen 4
	sets, brick_size, err = gen()
	tests.Assert(t, err == nil)
	tests.Assert(t, sets == 16)
	tests.Assert(t, brick_size == 6400*1024)
	tests.Assert(t, 1 == r.BricksInSet())

	// Gen 5
	sets, brick_size, err = gen()
	tests.Assert(t, err == ErrMininumBrickSize)
	tests.Assert(t, sets == 0)
	tests.Assert(t, brick_size == 0)
	tests.Assert(t, 1 == r.BricksInSet())
}

func TestDisperseDurability(t *testing.T) {

	r := &DisperseDurability{
		Data:       8,
		Redundancy: 3,
	}

	gen := r.BrickSizeGenerator(200 * GB)

	// Gen 1
	sets, brick_size, err := gen()
	tests.Assert(t, err == nil)
	tests.Assert(t, sets == 2)
	tests.Assert(t, brick_size == uint64(100*GB/8))
	tests.Assert(t, 8+3 == r.BricksInSet())

	// Gen 2
	sets, brick_size, err = gen()
	tests.Assert(t, err == nil)
	tests.Assert(t, sets == 4)
	tests.Assert(t, brick_size == uint64(50*GB/8))
	tests.Assert(t, 8+3 == r.BricksInSet())

	// Gen 3
	sets, brick_size, err = gen()
	tests.Assert(t, err == ErrMininumBrickSize)
	tests.Assert(t, 8+3 == r.BricksInSet())
}

func TestDisperseDurabilityLargeBrickGenerator(t *testing.T) {
	r := &DisperseDurability{
		Data:       8,
		Redundancy: 3,
	}
	gen := r.BrickSizeGenerator(800 * TB)

	// Gen 1
	sets, brick_size, err := gen()
	tests.Assert(t, err == nil)
	tests.Assert(t, sets == 32)
	tests.Assert(t, brick_size == 3200*GB)
	tests.Assert(t, 8+3 == r.BricksInSet())
}

func TestReplicaDurabilityGenerator(t *testing.T) {
	r := &ReplicaDurability{
		Replica: 2,
	}
	gen := r.BrickSizeGenerator(100 * GB)

	// Gen 1
	sets, brick_size, err := gen()
	tests.Assert(t, err == nil)
	tests.Assert(t, sets == 2)
	tests.Assert(t, brick_size == 50*GB)
	tests.Assert(t, 2 == r.BricksInSet())

	// Gen 2
	sets, brick_size, err = gen()
	tests.Assert(t, err == nil)
	tests.Assert(t, sets == 4)
	tests.Assert(t, brick_size == 25*GB)
	tests.Assert(t, 2 == r.BricksInSet())

	// Gen 3
	sets, brick_size, err = gen()
	tests.Assert(t, err == nil)
	tests.Assert(t, sets == 8)
	tests.Assert(t, brick_size == 12800*1024)
	tests.Assert(t, 2 == r.BricksInSet())

	// Gen 4
	sets, brick_size, err = gen()
	tests.Assert(t, err == nil)
	tests.Assert(t, sets == 16)
	tests.Assert(t, brick_size == 6400*1024)
	tests.Assert(t, 2 == r.BricksInSet())

	// Gen 5
	sets, brick_size, err = gen()
	tests.Assert(t, err == ErrMininumBrickSize)
	tests.Assert(t, sets == 0)
	tests.Assert(t, brick_size == 0)
	tests.Assert(t, 2 == r.BricksInSet())
}

func TestReplicaDurabilityLargeBrickGenerator(t *testing.T) {
	r := &ReplicaDurability{
		Replica: 2,
	}
	gen := r.BrickSizeGenerator(100 * TB)

	// Gen 1
	sets, brick_size, err := gen()
	tests.Assert(t, err == nil)
	tests.Assert(t, sets == 32)
	tests.Assert(t, brick_size == 3200*GB)
	tests.Assert(t, 2 == r.BricksInSet())
}
