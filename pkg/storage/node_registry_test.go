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
	"time"

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
		NodeID:      1,
		RatelNodeID: "ratel-1",
		StoreID:     10,
		Addr:        "10.0.0.1:26257",
		SQLAddr:     "10.0.0.1:26257",
		HTTPAddr:    "10.0.0.1:8080",
	}
	require.NoError(t, RegisterNode(ctx, store, reg1))

	// Register node 2.
	reg2 := NodeRegistration{
		NodeID:      2,
		RatelNodeID: "ratel-2",
		StoreID:     20,
		Addr:        "10.0.0.2:26257",
		SQLAddr:     "10.0.0.2:26257",
		HTTPAddr:    "10.0.0.2:8080",
	}
	require.NoError(t, RegisterNode(ctx, store, reg2))

	// List should return both nodes in order.
	nodes, err = ListNodes(ctx, store)
	require.NoError(t, err)
	require.Len(t, nodes, 2)
	require.Equal(t, reg1, nodes[0])
	require.Equal(t, reg2, nodes[1])

	// Read single node by ratel node ID.
	got, err := ReadNodeRegistration(ctx, store, "ratel-1")
	require.NoError(t, err)
	require.Equal(t, reg1, got)

	// Remove node 1 by ratel node ID.
	require.NoError(t, RemoveNode(ctx, store, "ratel-1"))
	nodes, err = ListNodes(ctx, store)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, reg2, nodes[0])

	// Reading removed node should error.
	_, err = ReadNodeRegistration(ctx, store, "ratel-1")
	require.Error(t, err)
}

func TestNodeRegistrationExists(t *testing.T) {
	ctx := t.Context()
	store := remote.NewInMem()
	defer func() { _ = store.Close() }()

	// Non-existent node returns false.
	_, exists, err := NodeRegistrationExists(ctx, store, "ratel-99")
	require.NoError(t, err)
	require.False(t, exists)

	// Register and check.
	reg := NodeRegistration{
		NodeID:      1,
		RatelNodeID: "ratel-99",
		StoreID:     42,
		Addr:        "10.0.0.1:26257",
		SQLAddr:     "10.0.0.1:26257",
		HTTPAddr:    "10.0.0.1:8080",
	}
	require.NoError(t, RegisterNode(ctx, store, reg))

	got, exists, err := NodeRegistrationExists(ctx, store, "ratel-99")
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, reg, got)
}

func TestHeartbeatNode(t *testing.T) {
	ctx := t.Context()
	store := remote.NewInMem()
	defer func() { _ = store.Close() }()

	reg := NodeRegistration{
		NodeID:      1,
		RatelNodeID: "ratel-hb",
		Addr:        "10.0.0.1:26257",
		SQLAddr:     "10.0.0.1:26257",
		HTTPAddr:    "10.0.0.1:8080",
	}
	require.NoError(t, RegisterNode(ctx, store, reg))

	// Heartbeat should set LastHeartbeat.
	require.NoError(t, HeartbeatNode(ctx, store, reg))

	got, err := ReadNodeRegistration(ctx, store, "ratel-hb")
	require.NoError(t, err)
	require.NotNil(t, got.LastHeartbeat)
	require.WithinDuration(t, time.Now().UTC(), *got.LastHeartbeat, 5*time.Second)
}
