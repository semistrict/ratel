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
	"time"

	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/testutils/inproc"
	"github.com/cockroachdb/cockroach/pkg/testutils/skip"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

// TestInprocSmoke verifies that a 3-node cluster starts with
// in-memory networking and that basic KV operations work.
// This test runs WITHOUT synctest to validate the networking layer.
func TestInprocSmoke(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	c := inproc.StartCluster(t, 3)
	defer c.Stop()

	ctx := context.Background()
	db := c.Server(0).DB()

	require.NoError(t, db.Put(ctx, roachpb.Key("hello"), []byte("world")))

	val, err := db.Get(ctx, roachpb.Key("hello"))
	require.NoError(t, err)
	require.Equal(t, []byte("world"), val.ValueBytes())
}

// TestSyncInprocSmoke is TestInprocSmoke inside a synctest bubble.
func TestSyncInprocSmoke(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	runSyncTest(t, func(t *testing.T) {
		c := inproc.StartCluster(t, 3)
		defer stopSyncCluster(c)

		db := c.Server(0).DB()

		err := db.Put(t.Context(), roachpb.Key("hello"), []byte("world"))
		require.NoError(t, err)

		val, err := db.Get(t.Context(), roachpb.Key("hello"))
		require.NoError(t, err)
		require.Equal(t, []byte("world"), val.ValueBytes())
	})
}

// TestSyncTestSmoke verifies that a 3-node cluster can start inside a
// synctest bubble with in-memory networking and storage, and that basic
// KV operations work with virtualized time.
//
// Currently skipped: a bare 3-node cluster on v23.1.28 spawns many
// background goroutines (store queues, gossip, rangefeed updater,
// replication reporter, diagnostics, etc.) that survive past
// Cluster.Stop() and reset timers after the synctest bubble has exited,
// producing "reset of synctest timer from outside bubble" panics. The
// ratel fork addressed this with a family of Disable* testing knobs
// (DisablePeriodicGossips, DisableRangefeedUpdater, DisableStoreRebalancer,
// DisableScanner, DisableRunnableCountCallbacks, DisableReplicationReporter,
// DisableProtectedTSProvider, DisableEnvironmentSample, etc.). Porting
// those is a follow-up to this change.
func TestSyncTestSmoke(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	runSyncTest(t, func(t *testing.T) {
		c := inproc.StartCluster(t, 3)
		defer stopSyncCluster(c)

		db := c.Server(0).DB()

		require.NoError(t, db.Put(t.Context(), roachpb.Key("hello"), []byte("world")))

		val, err := db.Get(t.Context(), roachpb.Key("hello"))
		require.NoError(t, err)
		require.Equal(t, []byte("world"), val.ValueBytes())
	})
}

// TestSyncFakeTime verifies that synctest's fake clock controls
// CockroachDB's HLC.
func TestSyncFakeTime(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	runSyncTest(t, func(t *testing.T) {
		c := inproc.StartCluster(t, 1)
		defer stopSyncCluster(c)

		clock := c.Server(0).Clock()

		// synctest starts at midnight UTC 2000-01-01.
		ts1 := clock.Now()
		require.Equal(t, 2000, ts1.GoTime().Year())

		time.Sleep(time.Hour)

		ts2 := clock.Now()
		elapsed := ts2.GoTime().Sub(ts1.GoTime())
		require.GreaterOrEqual(t, elapsed, time.Hour)
	})
}

// TestSyncRestart reimplements the "restart/down-for-2m" roachtest:
// stop a node, verify the cluster still serves traffic, restart the
// node, verify it recovers. Inside synctest, the downtime is instant.
func TestSyncRestart(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	runSyncTest(t, func(t *testing.T) {
		c := inproc.StartCluster(t, 3)
		defer stopSyncCluster(c)

		ctx := t.Context()
		db := c.Server(0).DB()

		require.NoError(t, db.Put(ctx, roachpb.Key("before-stop"), []byte("v1")))

		c.StopNode(2)

		require.NoError(t, db.Put(ctx, roachpb.Key("during-stop"), []byte("v2")))
		val, err := db.Get(ctx, roachpb.Key("during-stop"))
		require.NoError(t, err)
		require.Equal(t, []byte("v2"), val.ValueBytes())

		c.RestartNode(t, 2)

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
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	runSyncTest(t, func(t *testing.T) {
		c := inproc.StartCluster(t, 3)
		defer stopSyncCluster(c)

		ctx := t.Context()
		db := c.Server(0).DB()

		require.NoError(t, db.Put(ctx, roachpb.Key("pre-partition"), []byte("v1")))

		c.PartitionNode(2)

		require.NoError(t, db.Put(ctx, roachpb.Key("during-partition"), []byte("v2")))
		val, err := db.Get(ctx, roachpb.Key("during-partition"))
		require.NoError(t, err)
		require.Equal(t, []byte("v2"), val.ValueBytes())

		c.HealPartition(2)

		val, err = db.Get(ctx, roachpb.Key("pre-partition"))
		require.NoError(t, err)
		require.Equal(t, []byte("v1"), val.ValueBytes())
	})
}

// TestSyncClockJump reimplements the "clock-jump" roachtest: verify
// that the HLC tracks time advancement correctly under synctest's
// full control over time progression.
func TestSyncClockJump(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	skip.UnderRace(t, "synctest+race hangs or crashes in this clock-jump case; non-race still checks intended behavior")

	runSyncTest(t, func(t *testing.T) {
		c := inproc.StartCluster(t, 1)
		defer stopSyncCluster(c)

		ctx := t.Context()
		db := c.Server(0).DB()
		clock := c.Server(0).Clock()

		require.NoError(t, db.Put(ctx, roachpb.Key("t0"), []byte("before")))
		t0 := clock.Now()

		time.Sleep(10 * time.Minute)

		require.NoError(t, db.Put(ctx, roachpb.Key("t1"), []byte("after")))
		t1 := clock.Now()

		require.GreaterOrEqual(t, t1.WallTime-t0.WallTime, int64(10*time.Minute))

		v0, err := db.Get(ctx, roachpb.Key("t0"))
		require.NoError(t, err)
		require.Equal(t, []byte("before"), v0.ValueBytes())

		v1, err := db.Get(ctx, roachpb.Key("t1"))
		require.NoError(t, err)
		require.Equal(t, []byte("after"), v1.ValueBytes())
	})
}
