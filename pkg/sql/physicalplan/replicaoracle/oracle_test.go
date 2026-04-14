// Copyright 2019 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package replicaoracle

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/semistrict/ratel/pkg/roachpb"
	"github.com/semistrict/ratel/pkg/util"
	"github.com/semistrict/ratel/pkg/util/leaktest"
)

// TestRandomOracle defeats TestUnused for RandomChoice.
func TestRandomOracle(t *testing.T) {
	_ = NewOracle(RandomChoice, Config{})
}

func TestClosest(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()
	ns := makeNodeStore(t)
	nd, _ := ns.GetNodeDescriptor(1)
	o := NewOracle(ClosestChoice, Config{
		NodeDescs: ns,
		NodeDesc:  *nd,
	})
	o.(*closestOracle).latencyFunc = func(s string) (time.Duration, bool) {
		if strings.HasSuffix(s, "2") {
			return time.Nanosecond, true
		}
		return time.Millisecond, true
	}
	info, err := o.ChoosePreferredReplica(
		ctx,
		nil, /* txn */
		&roachpb.RangeDescriptor{
			InternalReplicas: []roachpb.ReplicaDescriptor{
				{NodeID: 4, StoreID: 4},
				{NodeID: 2, StoreID: 2},
				{NodeID: 3, StoreID: 3},
			},
		},
		nil, /* leaseHolder */
		roachpb.LAG_BY_CLUSTER_SETTING,
		QueryState{},
	)
	if err != nil {
		t.Fatalf("Failed to choose closest replica: %v", err)
	}
	if info.NodeID != 2 {
		t.Fatalf("Failed to choose node 2, got %v", info.NodeID)
	}
}

// testNodeDescStore implements kvcoord.NodeDescStore for tests.
type testNodeDescStore struct {
	nodes map[roachpb.NodeID]*roachpb.NodeDescriptor
}

func (s *testNodeDescStore) GetNodeDescriptor(id roachpb.NodeID) (*roachpb.NodeDescriptor, error) {
	if d, ok := s.nodes[id]; ok {
		return d, nil
	}
	return nil, errors.Errorf("node %d not found", id)
}

func makeNodeStore(t *testing.T) *testNodeDescStore {
	ns := &testNodeDescStore{nodes: make(map[roachpb.NodeID]*roachpb.NodeDescriptor)}
	for i := roachpb.NodeID(1); i <= 3; i++ {
		ns.nodes[i] = newNodeDesc(i)
	}
	return ns
}

func newNodeDesc(nodeID roachpb.NodeID) *roachpb.NodeDescriptor {
	return &roachpb.NodeDescriptor{
		NodeID:  nodeID,
		Address: util.MakeUnresolvedAddr("tcp", fmt.Sprintf("invalid.invalid:%d", nodeID)),
	}
}
