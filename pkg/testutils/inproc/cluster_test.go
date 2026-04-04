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
	"testing"
	"testing/synctest"

	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/testutils/inproc"
	"github.com/stretchr/testify/require"
)

// TestInprocSmoke verifies that a 3-node cluster starts with
// in-memory networking and that basic KV operations work.
// This test runs WITHOUT synctest to validate the networking layer.
func TestInprocSmoke(t *testing.T) {
	c := inproc.StartCluster(t, 3)
	defer c.Stop()

	ctx := context.Background()
	db := c.Server(0).DB()

	// Write a key on node 0.
	err := db.Put(ctx, roachpb.Key("hello"), []byte("world"))
	require.NoError(t, err)

	// Read it back.
	val, err := db.Get(ctx, roachpb.Key("hello"))
	require.NoError(t, err)
	require.Equal(t, []byte("world"), val.ValueBytes())
}

// TestSyncTestSmoke verifies that a 3-node cluster can start inside
// a synctest bubble with in-memory networking and storage.
func TestSyncTestSmoke(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := inproc.StartCluster(t, 3)
		defer c.Stop()

		db := c.Server(0).DB()

		// Write a key on node 0.
		err := db.Put(t.Context(), roachpb.Key("hello"), []byte("world"))
		require.NoError(t, err)

		// Read it back.
		val, err := db.Get(t.Context(), roachpb.Key("hello"))
		require.NoError(t, err)
		require.Equal(t, []byte("world"), val.ValueBytes())
	})
}
