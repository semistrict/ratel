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

// Package rateltest provides test utilities for multi-node ratel clusters
// backed by in-memory shared storage.
package rateltest

import (
	gosql "database/sql"
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/testutils/testcluster"
	"github.com/cockroachdb/pebble/objstorage/remote"
)

// SharedStorage holds the in-memory remote.Storage instances that back a test
// cluster's shared SSTable and metadata storage. Each node gets its own
// metadata store (since Pebble MANIFEST is per-node) but shares the SSTable
// store.
type SharedStorage struct {
	// SSTables is the shared SSTable store used by all nodes.
	SSTables remote.Storage
	// Metadata holds per-node metadata stores, keyed by node index (0-based).
	Metadata []remote.Storage
}

// NewSharedStorage creates a SharedStorage for n nodes.
func NewSharedStorage(n int) *SharedStorage {
	ss := &SharedStorage{
		SSTables: remote.NewInMem(),
		Metadata: make([]remote.Storage, n),
	}
	for i := range ss.Metadata {
		ss.Metadata[i] = remote.NewInMem()
	}
	return ss
}

// Close closes all storage instances.
func (ss *SharedStorage) Close() {
	_ = ss.SSTables.Close()
	for _, m := range ss.Metadata {
		_ = m.Close()
	}
}

// factoryFor returns a StorageFactory that always returns the shared SSTable
// storage. This satisfies remote.StorageFactory.
func (ss *SharedStorage) factoryFor() remote.StorageFactory {
	return &fixedFactory{store: ss.SSTables}
}

type fixedFactory struct {
	store remote.Storage
}

func (f *fixedFactory) CreateStorage(locator remote.Locator) (remote.Storage, error) {
	return f.store, nil
}

// ClusterArgs returns TestClusterArgs configured with shared in-memory remote
// storage for n nodes.
func ClusterArgs(ss *SharedStorage) base.TestClusterArgs {
	perNode := make(map[int]base.TestServerArgs, len(ss.Metadata))
	for i := range ss.Metadata {
		perNode[i] = base.TestServerArgs{
			StoreSpecs: []base.StoreSpec{{
				InMemory: true,
				Size: base.SizeSpec{
					InBytes: 512 << 20,
				},
				RemoteStorageFactory:  ss.factoryFor(),
				RemoteMetadataStorage: ss.Metadata[i],
			}},
		}
	}
	return base.TestClusterArgs{
		ServerArgsPerNode: perNode,
	}
}

// StartCluster starts an n-node TestCluster with shared in-memory remote
// storage. Returns the cluster and SharedStorage (caller must stop/close both).
func StartCluster(t testing.TB, nodes int) (*testcluster.TestCluster, *SharedStorage) {
	t.Helper()
	ss := NewSharedStorage(nodes)
	tc := testcluster.StartTestCluster(t, nodes, ClusterArgs(ss))
	return tc, ss
}

// ExecSQL executes a SQL statement on the given node and returns the result.
func ExecSQL(t testing.TB, db *gosql.DB, query string, args ...interface{}) gosql.Result {
	t.Helper()
	res, err := db.Exec(query, args...)
	if err != nil {
		t.Fatalf("ExecSQL(%q): %v", query, err)
	}
	return res
}

// QueryRowSQL executes a query expected to return a single row on the given
// node and scans the result into dest.
func QueryRowSQL(t testing.TB, db *gosql.DB, query string, dest ...interface{}) {
	t.Helper()
	if err := db.QueryRow(query).Scan(dest...); err != nil {
		t.Fatalf("QueryRowSQL(%q): %v", query, err)
	}
}

// QueryCountSQL is a convenience that runs "SELECT count(*) FROM ..." and
// returns the count.
func QueryCountSQL(t testing.TB, db *gosql.DB, table string) int {
	t.Helper()
	var count int
	QueryRowSQL(t, db, fmt.Sprintf("SELECT count(*) FROM %s", table), &count)
	return count
}
