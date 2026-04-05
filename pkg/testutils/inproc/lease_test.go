package inproc_test

import (
	"context"
	"testing"
	"testing/synctest"

	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/roachpb"
	"github.com/semistrict/ratel/pkg/testutils/inproc"
	"github.com/stretchr/testify/require"
)

// TestLeaseTransferAfterAddVoters replicates to all 3 nodes, stops
// the leaseholder, and verifies a surviving node can still read.
func TestLeaseTransferAfterAddVoters(t *testing.T) {
	c := inproc.StartCluster(t, 3)
	defer c.Stop()

	ctx := context.Background()
	db := c.Server(0).DB()

	key := keys.ScratchRangeMin
	require.NoError(t, db.Put(ctx, key, []byte("data")))

	// Replicate system ranges (liveness, meta) so the cluster survives
	// losing any single node.
	for _, sysKey := range []roachpb.Key{
		keys.NodeLivenessPrefix,
		keys.Meta1Prefix,
	} {
		sysDesc := c.LookupRangeOrFatal(t, sysKey)
		c.AddVotersOrFatal(t, sysDesc.StartKey.AsRawKey(), c.Target(1), c.Target(2))
	}

	desc := c.LookupRangeOrFatal(t, key)
	c.AddVotersOrFatal(t, desc.StartKey.AsRawKey(), c.Target(1), c.Target(2))
	t.Log("AddVoters succeeded")

	// Find and stop leaseholder.
	lease, _, err := c.FindRangeLease(desc, nil)
	require.NoError(t, err)
	lhIdx := int(lease.Replica.NodeID) - 1
	t.Logf("leaseholder is n%d, stopping", lhIdx+1)
	c.StopNode(lhIdx)

	// Read from a surviving node.
	liveIdx := (lhIdx + 1) % 3
	t.Logf("reading from n%d", liveIdx+1)
	val, err := c.Server(liveIdx).DB().Get(ctx, key)
	require.NoError(t, err)
	require.Equal(t, []byte("data"), val.ValueBytes())
	t.Log("read succeeded")
}

// TestLeaseTransferAfterAddVotersSynctest is the synctest variant.
func TestLeaseTransferAfterAddVotersSynctest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := inproc.StartCluster(t, 3)
		defer c.Stop()

		ctx := t.Context()
		db := c.Server(0).DB()

		key := keys.ScratchRangeMin
		require.NoError(t, db.Put(ctx, key, []byte("data")))

		for _, sysKey := range []roachpb.Key{
			keys.NodeLivenessPrefix,
			keys.Meta1Prefix,
		} {
			sysDesc := c.LookupRangeOrFatal(t, sysKey)
			c.AddVotersOrFatal(t, sysDesc.StartKey.AsRawKey(), c.Target(1), c.Target(2))
		}

		desc := c.LookupRangeOrFatal(t, key)
		c.AddVotersOrFatal(t, desc.StartKey.AsRawKey(), c.Target(1), c.Target(2))

		lease, _, err := c.FindRangeLease(desc, nil)
		require.NoError(t, err)
		lhIdx := int(lease.Replica.NodeID) - 1
		t.Logf("leaseholder is n%d, stopping", lhIdx+1)
		c.StopNode(lhIdx)

		liveIdx := (lhIdx + 1) % 3
		t.Logf("reading from n%d", liveIdx+1)
		val, err := c.Server(liveIdx).DB().Get(ctx, key)
		require.NoError(t, err)
		require.Equal(t, []byte("data"), val.ValueBytes())
	})
}
