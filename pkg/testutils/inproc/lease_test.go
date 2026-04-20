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
	"testing/synctest"

	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/testutils/inproc"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

// TestLeaseTransferAfterAddVoters replicates to all 3 nodes, stops
// the leaseholder, and verifies a surviving node can still read.
func TestLeaseTransferAfterAddVoters(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

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
	t.Log("read succeeded")

	c.RestartNode(t, lhIdx)
}

// TestLeaseTransferAfterAddVotersSynctest is the synctest variant.
// Currently hangs: after stopping the leaseholder, surviving nodes
// deadlock trying to elect a new leader. Investigation ongoing.
func TestLeaseTransferAfterAddVotersSynctest(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	t.Skip("WIP: deadlocks under synctest; see commit 09751515290")

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

		c.RestartNode(t, lhIdx)
	})
}
