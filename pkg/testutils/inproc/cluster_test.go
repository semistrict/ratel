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

	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/testutils/inproc"
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
	t.Skip("background goroutines leak timers out of the synctest bubble; " +
		"requires porting the Disable* server TestingKnobs from ratel")
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	synctest.Test(t, func(t *testing.T) {
		c := inproc.StartCluster(t, 3)
		defer c.Stop()

		db := c.Server(0).DB()

		require.NoError(t, db.Put(t.Context(), roachpb.Key("hello"), []byte("world")))

		val, err := db.Get(t.Context(), roachpb.Key("hello"))
		require.NoError(t, err)
		require.Equal(t, []byte("world"), val.ValueBytes())
	})
}
