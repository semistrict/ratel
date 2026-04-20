// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package inproc_test

import (
	"context"
	"testing"
	"testing/synctest"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/testutils/inproc"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

// TestAddVoters verifies that explicit Raft replication via AddVoters
// works with in-memory networking.
func TestAddVoters(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	c := inproc.StartCluster(t, 3)
	defer c.Stop()

	ctx := context.Background()
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
}

// TestSyncAddVoters verifies that explicit Raft replication via
// AddVoters works inside a synctest bubble with fake time.
func TestSyncAddVoters(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

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
// in-memory networking: a cluster with ReplicationAuto automatically
// replicates ranges and data is readable from any node.
func TestAutoReplication(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

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
		require.Equal(t, []byte("replicated"), val.ValueBytes(),
			"node %d should be able to read replicated data", i)
	}
}

// TestSyncAutoReplication is the same as TestAutoReplication but runs
// inside a synctest bubble with fake time, verifying that Raft
// consensus and replication work deterministically.
func TestSyncAutoReplication(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

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
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

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

		c.RestartNode(t, 2)

		val, err := db.Get(ctx, roachpb.Key("raft-restart"))
		require.NoError(t, err)
		require.Equal(t, []byte("v1"), val.ValueBytes())

		val, err = db.Get(ctx, roachpb.Key("while-down"))
		require.NoError(t, err)
		require.Equal(t, []byte("v2"), val.ValueBytes())
	})
}
