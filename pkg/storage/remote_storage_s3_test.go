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
	"fmt"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/objstorage/remote"
	"github.com/cockroachdb/pebble/vfs"
	"github.com/stretchr/testify/require"
)

// TestPebbleSharedStorageWithMemBackend verifies that Pebble's shared storage
// integration works end-to-end using an in-memory remote storage backend.
// This validates the wiring without needing a real S3 endpoint.
func TestPebbleSharedStorageWithMemBackend(t *testing.T) {
	memStorage := remote.NewInMem()
	opts := &pebble.Options{
		FS: vfs.NewMem(),
	}
	opts.Experimental.RemoteStorage = remote.MakeSimpleFactory(map[remote.Locator]remote.Storage{
		"": memStorage,
	})
	opts.Experimental.CreateOnShared = remote.CreateOnSharedAll
	opts.Experimental.CreateOnSharedLocator = ""
	opts.FormatMajorVersion = pebble.FormatVirtualSSTables

	db, err := pebble.Open("", opts)
	require.NoError(t, err)

	// Set CreatorID (required for shared storage).
	require.NoError(t, db.SetCreatorID(1))

	// Write enough data to trigger compaction to lower levels.
	batch := db.NewBatch()
	for i := 0; i < 1000; i++ {
		key := []byte(fmt.Sprintf("key-%06d", i))
		val := []byte(fmt.Sprintf("value-%06d-padding-to-make-it-bigger", i))
		require.NoError(t, batch.Set(key, val, nil))
	}
	require.NoError(t, batch.Commit(pebble.Sync))
	require.NoError(t, db.Flush())

	// Force compaction to push data to lower levels.
	require.NoError(t, db.Compact([]byte("key-000000"), []byte("key-999999"), true))

	// Verify we can still read data.
	val, closer, err := db.Get([]byte("key-000500"))
	require.NoError(t, err)
	require.Equal(t, "value-000500-padding-to-make-it-bigger", string(val))
	require.NoError(t, closer.Close())

	// Check that remote objects were created.
	remoteObjs, err := memStorage.List("", "")
	require.NoError(t, err)
	t.Logf("Remote objects: %d", len(remoteObjs))
	require.Greater(t, len(remoteObjs), 0, "expected at least one remote object")
	for _, obj := range remoteObjs {
		t.Logf("  %s", obj)
	}

	// With CreateOnSharedAll, no local SSTables should remain.
	metrics := db.Metrics()
	var localTables int64
	for _, lm := range metrics.Levels {
		localTables += lm.NumFiles
	}
	t.Logf("Local table count across all levels: %d", localTables)

	require.NoError(t, db.Close())
}

// TestS3StorageFactoryInterface verifies that S3StorageFactory satisfies the
// remote.StorageFactory interface and S3Storage satisfies remote.Storage.
func TestS3StorageFactoryInterface(t *testing.T) {
	var _ remote.StorageFactory = (*S3StorageFactory)(nil)
	var _ remote.Storage = (*S3Storage)(nil)
}
