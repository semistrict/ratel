// Copyright 2024 The Cockroach Authors.
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

package storage

import (
	"testing"

	"github.com/cockroachdb/pebble/objstorage/remote"
	"github.com/stretchr/testify/require"
)

func TestNodeRegistry(t *testing.T) {
	ctx := t.Context()
	store := remote.NewInMem()
	defer func() { _ = store.Close() }()

	// Initially empty.
	nodes, err := ListNodes(ctx, store)
	require.NoError(t, err)
	require.Empty(t, nodes)

	// Register node 1.
	reg1 := NodeRegistration{
		NodeID:   1,
		Addr:     "10.0.0.1:26257",
		SQLAddr:  "10.0.0.1:26257",
		HTTPAddr: "10.0.0.1:8080",
	}
	require.NoError(t, RegisterNode(ctx, store, reg1))

	// Register node 2.
	reg2 := NodeRegistration{
		NodeID:   2,
		Addr:     "10.0.0.2:26257",
		SQLAddr:  "10.0.0.2:26257",
		HTTPAddr: "10.0.0.2:8080",
	}
	require.NoError(t, RegisterNode(ctx, store, reg2))

	// List should return both nodes in order.
	nodes, err = ListNodes(ctx, store)
	require.NoError(t, err)
	require.Len(t, nodes, 2)
	require.Equal(t, reg1, nodes[0])
	require.Equal(t, reg2, nodes[1])

	// Read single node.
	got, err := ReadNodeRegistration(ctx, store, 1)
	require.NoError(t, err)
	require.Equal(t, reg1, got)

	// Remove node 1.
	require.NoError(t, RemoveNode(ctx, store, 1))
	nodes, err = ListNodes(ctx, store)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, reg2, nodes[0])

	// Reading removed node should error.
	_, err = ReadNodeRegistration(ctx, store, 1)
	require.Error(t, err)
}
