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
	gosql "database/sql"
	"testing"
	"testing/synctest"
	"time"

	"github.com/semistrict/ratel/pkg/roachpb"
	"github.com/semistrict/ratel/pkg/testutils/inproc"
	"github.com/semistrict/ratel/pkg/testutils/skip"
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

// TestSyncInprocSmoke is TestInprocSmoke inside a synctest bubble.
func TestSyncInprocSmoke(t *testing.T) {
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
// node, verify it recovers.
func TestSyncRestart(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := inproc.StartCluster(t, 3)
		defer c.Stop()

		ctx := t.Context()
		db := c.Server(0).DB()

		require.NoError(t, db.Put(ctx, roachpb.Key("before-stop"), []byte("v1")))

		c.StopNode(2)

		// Cluster should still serve reads/writes (2 of 3 nodes up).
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
	synctest.Test(t, func(t *testing.T) {
		c := inproc.StartCluster(t, 3)
		defer c.Stop()

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

// TestSyncNetworkPartitionClosesExistingConnections verifies that partitioning
// a node tears down already-established SQL connections to that node, instead
// of only blocking future dials.
func TestSyncNetworkPartitionClosesExistingConnections(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := inproc.StartCluster(t, 3)
		defer c.Stop()

		ctx := t.Context()
		db := c.ServerConn(2)
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)

		conn, err := db.Conn(ctx)
		require.NoError(t, err)
		defer conn.Close()

		var before int
		require.NoError(t, conn.QueryRowContext(ctx, `SELECT 1`).Scan(&before))
		require.Equal(t, 1, before)

		c.PartitionNode(2)

		var during int
		err = conn.QueryRowContext(ctx, `SELECT 1`).Scan(&during)
		require.Error(t, err)
		require.NotErrorIs(t, err, gosql.ErrNoRows)

		c.HealPartition(2)

		fresh := c.ServerConn(2)
		fresh.SetMaxOpenConns(1)
		fresh.SetMaxIdleConns(1)

		var after int
		require.NoError(t, fresh.QueryRowContext(ctx, `SELECT 1`).Scan(&after))
		require.Equal(t, 1, after)
	})
}

// TestSyncFakeTime verifies that synctest's fake clock controls
// CockroachDB's HLC.
func TestSyncFakeTime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := inproc.StartCluster(t, 1)
		defer c.Stop()

		clock := c.Server(0).Clock()

		// synctest starts at midnight UTC 2000-01-01.
		ts1 := clock.Now()
		require.Equal(t, 2000, ts1.GoTime().Year())

		// Advance fake time by 1 hour.
		time.Sleep(time.Hour)

		ts2 := clock.Now()
		elapsed := ts2.GoTime().Sub(ts1.GoTime())
		require.GreaterOrEqual(t, elapsed, time.Hour)
	})
}

// TestSyncClockJump reimplements the "clock-jump" roachtest: verify
// that the HLC tracks time advancement correctly under synctest's
// full control over time progression.
func TestSyncClockJump(t *testing.T) {
	skip.UnderRace(t, "synctest+race hangs or crashes in this clock-jump case; non-race still checks intended behavior")
	synctest.Test(t, func(t *testing.T) {
		c := inproc.StartCluster(t, 1)
		defer c.Stop()

		ctx := t.Context()
		db := c.Server(0).DB()
		clock := c.Server(0).Clock()

		require.NoError(t, db.Put(ctx, roachpb.Key("t0"), []byte("before")))
		t0 := clock.Now()

		// Jump forward 10 minutes.
		time.Sleep(10 * time.Minute)

		require.NoError(t, db.Put(ctx, roachpb.Key("t1"), []byte("after")))
		t1 := clock.Now()

		// HLC advanced by at least 10 minutes.
		require.GreaterOrEqual(t, t1.WallTime-t0.WallTime, int64(10*time.Minute))

		v0, err := db.Get(ctx, roachpb.Key("t0"))
		require.NoError(t, err)
		require.Equal(t, []byte("before"), v0.ValueBytes())

		v1, err := db.Get(ctx, roachpb.Key("t1"))
		require.NoError(t, err)
		require.Equal(t, []byte("after"), v1.ValueBytes())
	})
}
