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

	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/testutils/inproc"
	"github.com/stretchr/testify/require"
)

// TestInprocSmoke verifies that a 3-node cluster starts with
// in-memory networking and that basic KV operations work.
func TestInprocSmoke(t *testing.T) {
	c := inproc.StartCluster(t, 3)
	defer c.Stop()

	ctx := context.Background()
	db := c.Server(0).DB()

	err := db.Put(ctx, roachpb.Key("hello"), []byte("world"))
	require.NoError(t, err)

	val, err := db.Get(ctx, roachpb.Key("hello"))
	require.NoError(t, err)
	require.Equal(t, []byte("world"), val.ValueBytes())
}

// TestSyncTestSmoke verifies that a 3-node cluster can start inside
// a synctest bubble with in-memory networking and storage.
func TestSyncTestSmoke(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := inproc.StartCluster(t, 3)
		defer c.Stop()

		db := c.Server(0).DB()

		err := db.Put(t.Context(), roachpb.Key("hello"), []byte("world"))
		require.NoError(t, err)

		val, err := db.Get(t.Context(), roachpb.Key("hello"))
		require.NoError(t, err)
		require.Equal(t, []byte("world"), val.ValueBytes())
	})
}

// TestSyncRestart reimplements the "restart/down-for-2m" roachtest:
// stop a node, verify the cluster still serves traffic, restart the
// node, verify it recovers. Inside synctest, the downtime is instant.
func TestSyncRestart(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := inproc.StartCluster(t, 3)
		defer c.Stop()

		ctx := t.Context()
		db := c.Server(0).DB()

		// Write initial data.
		require.NoError(t, db.Put(ctx, roachpb.Key("before-stop"), []byte("v1")))

		// Stop node 2.
		c.StopNode(2)

		// Cluster should still serve reads/writes (2 of 3 nodes up).
		require.NoError(t, db.Put(ctx, roachpb.Key("during-stop"), []byte("v2")))
		val, err := db.Get(ctx, roachpb.Key("during-stop"))
		require.NoError(t, err)
		require.Equal(t, []byte("v2"), val.ValueBytes())

		// Restart node 2.
		c.RestartNode(t, 2)

		// Verify the restarted node's data is accessible.
		val, err = db.Get(ctx, roachpb.Key("before-stop"))
		require.NoError(t, err)
		require.Equal(t, []byte("v1"), val.ValueBytes())

		val, err = db.Get(ctx, roachpb.Key("during-stop"))
		require.NoError(t, err)
		require.Equal(t, []byte("v2"), val.ValueBytes())
	})
}

// TestSyncNetworkPartition verifies that a 3-node cluster survives a
// network partition: block a node, verify reads/writes on remaining
// nodes, then heal the partition.
func TestSyncNetworkPartition(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := inproc.StartCluster(t, 3)
		defer c.Stop()

		ctx := t.Context()
		db := c.Server(0).DB()

		// Write before partition.
		require.NoError(t, db.Put(ctx, roachpb.Key("pre-partition"), []byte("v1")))

		// Partition node 2 — new connections to it will fail.
		c.PartitionNode(2)

		// Cluster should still serve reads/writes (nodes 0 and 1 are up
		// and can form quorum for RF=1 ranges on those nodes).
		require.NoError(t, db.Put(ctx, roachpb.Key("during-partition"), []byte("v2")))
		val, err := db.Get(ctx, roachpb.Key("during-partition"))
		require.NoError(t, err)
		require.Equal(t, []byte("v2"), val.ValueBytes())

		// Heal the partition.
		c.HealPartition(2)

		// Data written during partition should be readable.
		val, err = db.Get(ctx, roachpb.Key("pre-partition"))
		require.NoError(t, err)
		require.Equal(t, []byte("v1"), val.ValueBytes())
	})
}
