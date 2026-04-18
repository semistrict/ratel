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

	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/testutils/inproc"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

// TestAddVoters verifies that explicit Raft replication via AddVoters
// works with in-memory networking: a value written on node 0 is
// readable from all nodes after replication.
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
