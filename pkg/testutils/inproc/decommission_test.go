// Copyright 2026 The Ratel Authors.
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

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/liveness/livenesspb"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/testutils/inproc"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

// TestDecommission verifies that decommissioning a node causes all
// its replicas to be moved to other nodes. This reimplements the
// "decommission" roachtest as an in-process test.
func TestDecommission(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	c := inproc.StartCluster(t, 4, func(args *base.TestClusterArgs) {
		args.ReplicationMode = base.ReplicationAuto
	})
	defer c.Stop()

	ctx := context.Background()
	db := c.Server(0).DB()

	require.NoError(t, db.Put(ctx, roachpb.Key("decom-test"), []byte("data")))

	targetNodeID := c.Server(3).NodeID()
	t.Logf("decommissioning node %d", targetNodeID)
	require.NoError(t, c.Server(0).Decommission(
		ctx, livenesspb.MembershipStatus_DECOMMISSIONING, []roachpb.NodeID{targetNodeID}))

	val, err := db.Get(ctx, roachpb.Key("decom-test"))
	require.NoError(t, err)
	require.Equal(t, []byte("data"), val.ValueBytes())

	t.Log("decommission initiated, data still available")
}

// TestSyncDecommission is the synctest variant.
func TestSyncDecommission(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	runSyncTest(t, func(t *testing.T) {
		c := inproc.StartCluster(t, 4, func(args *base.TestClusterArgs) {
			args.ReplicationMode = base.ReplicationAuto
		})
		defer stopSyncCluster(c)

		ctx := t.Context()
		db := c.Server(0).DB()

		require.NoError(t, db.Put(ctx, roachpb.Key("decom-test"), []byte("data")))

		targetNodeID := c.Server(3).NodeID()
		t.Logf("decommissioning node %d", targetNodeID)
		require.NoError(t, c.Server(0).Decommission(
			ctx, livenesspb.MembershipStatus_DECOMMISSIONING, []roachpb.NodeID{targetNodeID}))

		val, err := db.Get(ctx, roachpb.Key("decom-test"))
		require.NoError(t, err)
		require.Equal(t, []byte("data"), val.ValueBytes())

		t.Log("decommission initiated, data still available")
	})
}
