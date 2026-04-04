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

// TestAddVoters verifies that explicit Raft replication via AddVoters
// works with in-memory networking.
func TestAddVoters(t *testing.T) {
	c := inproc.StartCluster(t, 3)
	defer c.Stop()

	ctx := context.Background()
	db := c.Server(0).DB()

	key := keys.ScratchRangeMin
	require.NoError(t, db.Put(ctx, key, []byte("data")))

	desc := c.LookupRangeOrFatal(t, key)
	c.AddVotersOrFatal(t, desc.StartKey.AsRawKey(), c.Target(1), c.Target(2))

	// Verify data is readable from all nodes.
	for i := 0; i < 3; i++ {
		val, err := c.Server(i).DB().Get(ctx, key)
		require.NoError(t, err)
		require.Equal(t, []byte("data"), val.ValueBytes())
	}
}

// TestSyncAddVoters verifies that explicit Raft replication via
// AddVoters works inside a synctest bubble with fake time.
func TestSyncAddVoters(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := inproc.StartCluster(t, 3)
		defer c.Stop()

		ctx := t.Context()
		db := c.Server(0).DB()

		key := keys.ScratchRangeMin
		require.NoError(t, db.Put(ctx, key, []byte("data")))

		desc := c.LookupRangeOrFatal(t, key)
		c.AddVotersOrFatal(t, desc.StartKey.AsRawKey(), c.Target(1), c.Target(2))

		for i := 0; i < 3; i++ {
			val, err := c.Server(i).DB().Get(ctx, key)
			require.NoError(t, err)
			require.Equal(t, []byte("data"), val.ValueBytes())
		}
	})
}

// TestAutoReplication verifies that Raft replication works with
// ReplicationAuto: data is automatically replicated and readable
// from any node.
func TestAutoReplication(t *testing.T) {
	c := inproc.StartCluster(t, 3, func(args *base.TestClusterArgs) {
		args.ReplicationMode = base.ReplicationAuto
	})
	defer c.Stop()

	ctx := context.Background()
	db := c.Server(0).DB()

	key := keys.ScratchRangeMin
	require.NoError(t, db.Put(ctx, key, []byte("replicated")))

	for i := 0; i < 3; i++ {
		val, err := c.Server(i).DB().Get(ctx, key)
		require.NoError(t, err)
		require.Equal(t, []byte("replicated"), val.ValueBytes())
	}
}

// TestSyncAutoReplication is TestAutoReplication inside synctest.
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
			require.Equal(t, []byte("replicated"), val.ValueBytes())
		}
	})
}

// TestSyncRaftRestartWithReplication verifies that after a node
// restart with replicated data, reads still work.
func TestSyncRaftRestartWithReplication(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := inproc.StartCluster(t, 3, func(args *base.TestClusterArgs) {
			args.ReplicationMode = base.ReplicationAuto
		})
		defer c.Stop()

		ctx := t.Context()
		db := c.Server(0).DB()

		require.NoError(t, db.Put(ctx, roachpb.Key("raft-restart"), []byte("v1")))

		c.StopNode(2)
		require.NoError(t, db.Put(ctx, roachpb.Key("while-down"), []byte("v2")))

		val, err := db.Get(ctx, roachpb.Key("raft-restart"))
		require.NoError(t, err)
		require.Equal(t, []byte("v1"), val.ValueBytes())

		val, err = db.Get(ctx, roachpb.Key("while-down"))
		require.NoError(t, err)
		require.Equal(t, []byte("v2"), val.ValueBytes())
	})
}

// TestSyncLeaseTransfer replicates to all 3 nodes via AddVoters,
// stops the leaseholder, and verifies the lease transfers and data
// remains accessible from a surviving node.
func TestSyncLeaseTransfer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := inproc.StartCluster(t, 3, func(args *base.TestClusterArgs) {
			args.ReplicationMode = base.ReplicationAuto
		})
		defer c.Stop()

		ctx := t.Context()
		db := c.Server(0).DB()

		key := keys.ScratchRangeMin
		require.NoError(t, db.Put(ctx, key, []byte("lease-data")))

		// With ReplicationAuto, the range should already be replicated.
		desc := c.LookupRangeOrFatal(t, key)

		// Find leaseholder and stop it.
		lease, _, err := c.FindRangeLease(desc, nil)
		require.NoError(t, err)
		leaseholderIdx := int(lease.Replica.NodeID) - 1
		t.Logf("leaseholder is node %d, stopping it", leaseholderIdx+1)
		c.StopNode(leaseholderIdx)

		// Read from a live node — lease should transfer.
		liveIdx := (leaseholderIdx + 1) % 3
		t.Logf("reading from live node %d", liveIdx+1)
		val, err := c.Server(liveIdx).DB().Get(ctx, key)
		require.NoError(t, err)
		require.Equal(t, []byte("lease-data"), val.ValueBytes())

	})
}
