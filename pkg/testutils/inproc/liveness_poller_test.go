// Copyright 2026 The Cockroach Authors
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
	"testing"
	"testing/synctest"
	"time"

	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/liveness"
	"github.com/stretchr/testify/require"
)

// TestSyncLivenessPollerAdvancesFakeTime verifies that the liveness poller
// uses time.Sleep (not select-on-timer), which allows synctest to advance
// fake time through the poll interval. After advancing 10s (two poll cycles
// at 5s each), every node should have liveness records for all peers.
func TestSyncLivenessPollerAdvancesFakeTime(t *testing.T) {
	runSyncTest(t, func(t *testing.T) {
		c := startSyncCluster(t, 3)
		defer stopSyncCluster(c)

		// Advance fake time past two poll intervals (5s each) to ensure
		// liveness records are distributed.
		time.Sleep(12 * time.Second)
		synctest.Wait()

		// Each node's NodeLiveness cache should know about all 3 nodes.
		for i := 0; i < 3; i++ {
			nl := c.Server(i).NodeLiveness().(*liveness.NodeLiveness)
			for j := 0; j < 3; j++ {
				nodeID := c.Server(j).NodeID()
				live, err := nl.IsLive(nodeID)
				require.NoError(t, err, "node %d checking liveness of n%d", i+1, nodeID)
				require.True(t, live, "node %d sees n%d as not live", i+1, nodeID)
			}
		}
	})
}

// TestSyncLivenessPollerDetectsExpiry verifies that when a node stops
// heartbeating, the liveness poller on surviving nodes eventually sees
// the record as expired (after the liveness threshold elapses).
func TestSyncLivenessPollerDetectsExpiry(t *testing.T) {
	runSyncTest(t, func(t *testing.T) {
		c := startSyncCluster(t, 3)
		defer stopSyncCluster(c)

		// Let the cluster stabilize.
		time.Sleep(12 * time.Second)
		synctest.Wait()

		stoppedNodeID := c.Server(2).NodeID()
		c.StopNode(2)

		// Advance past the liveness threshold (default 9s) plus a few poll
		// intervals to ensure the poller picks up the stale record.
		nl := c.Server(0).NodeLiveness().(*liveness.NodeLiveness)
		time.Sleep(30 * time.Second)
		synctest.Wait()

		live, err := nl.IsLive(stoppedNodeID)
		require.NoError(t, err, "checking liveness of stopped node")
		require.False(t, live,
			"stopped node should not be live after threshold+polls elapsed")

		// Surviving nodes should still be live.
		for i := 0; i < 2; i++ {
			nodeID := c.Server(i).NodeID()
			live, err := nl.IsLive(nodeID)
			require.NoError(t, err)
			require.True(t, live,
				"surviving node n%d should still be live", nodeID)
		}
	})
}
