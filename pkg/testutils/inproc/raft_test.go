// Copyright 2025 The Cockroach Authors.
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

package inproc_test

import (
	"context"
	"testing"
	"testing/synctest"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/testutils/inproc"
	"github.com/stretchr/testify/require"
)

// TestAutoReplication verifies that Raft replication works with
// in-memory networking: a cluster with ReplicationAuto automatically
// replicates ranges and data is readable from any node.
func TestAutoReplication(t *testing.T) {
	c := inproc.StartCluster(t, 3, func(args *base.TestClusterArgs) {
		args.ReplicationMode = base.ReplicationAuto
	})
	defer c.Stop()

	ctx := context.Background()
	db := c.Server(0).DB()

	key := keys.ScratchRangeMin
	require.NoError(t, db.Put(ctx, key, []byte("replicated")))

	// Read from each node to verify replication.
	for i := 0; i < 3; i++ {
		val, err := c.Server(i).DB().Get(ctx, key)
		require.NoError(t, err)
		require.Equal(t, []byte("replicated"), val.ValueBytes(),
			"node %d should be able to read replicated data", i)
	}
}

// TestSyncAutoReplication is the same as TestAutoReplication but runs
// inside a synctest bubble with fake time, verifying that Raft
// consensus and replication work deterministically.
func TestSyncAutoReplication(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := inproc.StartCluster(t, 3, func(args *base.TestClusterArgs) {
			args.ReplicationMode = base.ReplicationAuto
		})
		defer c.Stop()

		ctx := t.Context()
		db := c.Server(0).DB()

		key := keys.ScratchRangeMin
		require.NoError(t, db.Put(ctx, key, []byte("replicated")))

		for i := 0; i < 3; i++ {
			val, err := c.Server(i).DB().Get(ctx, key)
			require.NoError(t, err)
			require.Equal(t, []byte("replicated"), val.ValueBytes(),
				"node %d should be able to read replicated data", i)
		}
	})
}

// TestSyncRaftRestartWithReplication verifies that after a node
// restart, Raft recovers and data remains accessible. Uses
// ReplicationAuto so ranges are replicated to all nodes.
func TestSyncRaftRestartWithReplication(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := inproc.StartCluster(t, 3, func(args *base.TestClusterArgs) {
			args.ReplicationMode = base.ReplicationAuto
		})
		defer c.Stop()

		ctx := t.Context()
		db := c.Server(0).DB()

		// Write data that will be replicated.
		require.NoError(t, db.Put(ctx, roachpb.Key("raft-restart"), []byte("v1")))

		// Stop node 2.
		c.StopNode(2)

		// Write more data while node 2 is down.
		require.NoError(t, db.Put(ctx, roachpb.Key("while-down"), []byte("v2")))

		// Restart node 2.
		c.RestartNode(t, 2)

		// Verify data from before and during downtime is readable.
		val, err := db.Get(ctx, roachpb.Key("raft-restart"))
		require.NoError(t, err)
		require.Equal(t, []byte("v1"), val.ValueBytes())

		val, err = db.Get(ctx, roachpb.Key("while-down"))
		require.NoError(t, err)
		require.Equal(t, []byte("v2"), val.ValueBytes())
	})
}
