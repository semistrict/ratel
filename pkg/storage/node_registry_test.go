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

	nodes, err := ListNodes(ctx, store)
	require.NoError(t, err)
	require.Empty(t, nodes)

	reg1 := NodeRegistration{
		NodeID:      1,
		RatelNodeID: "ratel-1",
		Addr:        "10.0.0.1:26257",
		SQLAddr:     "10.0.0.1:26257",
		HTTPAddr:    "10.0.0.1:5273",
	}
	require.NoError(t, RegisterNode(ctx, store, reg1))

	reg2 := NodeRegistration{
		NodeID:      2,
		RatelNodeID: "ratel-2",
		Addr:        "10.0.0.2:26257",
		SQLAddr:     "10.0.0.2:26257",
		HTTPAddr:    "10.0.0.2:5273",
	}
	require.NoError(t, RegisterNode(ctx, store, reg2))

	nodes, err = ListNodes(ctx, store)
	require.NoError(t, err)
	require.Len(t, nodes, 2)
	require.Equal(t, reg1, nodes[0])
	require.Equal(t, reg2, nodes[1])

	got, err := ReadNodeRegistration(ctx, store, "ratel-1")
	require.NoError(t, err)
	require.Equal(t, reg1, got)

	require.NoError(t, RemoveNode(ctx, store, "ratel-1"))
	nodes, err = ListNodes(ctx, store)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, reg2, nodes[0])

	_, err = ReadNodeRegistration(ctx, store, "ratel-1")
	require.Error(t, err)
}

func TestNodeRegistrationExists(t *testing.T) {
	ctx := t.Context()
	store := remote.NewInMem()
	defer func() { _ = store.Close() }()

	_, ok, err := NodeRegistrationExists(ctx, store, "ratel-missing")
	require.NoError(t, err)
	require.False(t, ok)

	reg := NodeRegistration{
		NodeID:      1,
		RatelNodeID: "ratel-a",
		Addr:        "10.0.0.1:26257",
	}
	require.NoError(t, RegisterNode(ctx, store, reg))

	got, ok, err := NodeRegistrationExists(ctx, store, "ratel-a")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, reg, got)
}
