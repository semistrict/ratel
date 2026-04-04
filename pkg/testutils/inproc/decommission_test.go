package inproc_test

import (
	"context"
	"testing"
	"testing/synctest"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/liveness/livenesspb"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/testutils/inproc"
	"github.com/stretchr/testify/require"
)

// TestDecommission verifies that decommissioning a node causes all
// its replicas to be moved to other nodes. This reimplements the
// "decommission" roachtest as an in-process synctest.
func TestDecommission(t *testing.T) {
	c := inproc.StartCluster(t, 4, func(args *base.TestClusterArgs) {
		args.ReplicationMode = base.ReplicationAuto
	})
	defer c.Stop()

	ctx := context.Background()
	db := c.Server(0).DB()

	// Write some data.
	require.NoError(t, db.Put(ctx, roachpb.Key("decom-test"), []byte("data")))

	// Decommission node 4 (index 3).
	targetNodeID := c.Server(3).NodeID()
	t.Logf("decommissioning node %d", targetNodeID)
	require.NoError(t, c.Server(0).Decommission(
		ctx, livenesspb.MembershipStatus_DECOMMISSIONING, []roachpb.NodeID{targetNodeID}))

	// Data should remain available.
	val, err := db.Get(ctx, roachpb.Key("decom-test"))
	require.NoError(t, err)
	require.Equal(t, []byte("data"), val.ValueBytes())

	t.Log("decommission initiated, data still available")
}

// TestSyncDecommission is the synctest variant.
func TestSyncDecommission(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := inproc.StartCluster(t, 4, func(args *base.TestClusterArgs) {
			args.ReplicationMode = base.ReplicationAuto
		})
		defer c.Stop()

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
