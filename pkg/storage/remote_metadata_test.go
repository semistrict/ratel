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

// openTestDB creates a Pebble DB with shared storage on the given FS and
// remote storage. It writes some data, flushes, and compacts so that SSTables
// land on remote storage.
func openTestDB(t *testing.T, fs vfs.FS, dir string, remoteStore remote.Storage) *pebble.DB {
	t.Helper()
	opts := &pebble.Options{FS: fs}
	opts.Experimental.RemoteStorage = remote.MakeSimpleFactory(map[remote.Locator]remote.Storage{
		"": remoteStore,
	})
	opts.Experimental.CreateOnShared = remote.CreateOnSharedAll
	opts.Experimental.CreateOnSharedLocator = ""
	opts.FormatMajorVersion = pebble.FormatVirtualSSTables

	db, err := pebble.Open(dir, opts)
	require.NoError(t, err)
	require.NoError(t, db.SetCreatorID(1))
	return db
}

// writeTestData writes N key-value pairs to the DB, flushes, and compacts.
func writeTestData(t *testing.T, db *pebble.DB, prefix string, n int) {
	t.Helper()
	batch := db.NewBatch()
	for i := 0; i < n; i++ {
		key := []byte(fmt.Sprintf("%s-key-%06d", prefix, i))
		val := []byte(fmt.Sprintf("%s-val-%06d-padding", prefix, i))
		require.NoError(t, batch.Set(key, val, nil))
	}
	require.NoError(t, batch.Commit(pebble.Sync))
	require.NoError(t, db.Flush())
}

// TestManifestBundleRoundTrip creates a Pebble DB in a temp dir, writes data,
// closes it, uploads the manifest bundle to in-mem storage, wipes the temp dir,
// downloads the bundle to a new temp dir, reopens, and verifies data.
func TestManifestBundleRoundTrip(t *testing.T) {
	ctx := t.Context()
	memFS := vfs.NewMem()
	remoteStore := remote.NewInMem()
	metaStore := remote.NewInMem()

	// Create and populate a DB.
	db := openTestDB(t, memFS, "/db", remoteStore)
	writeTestData(t, db, "round", 100)
	require.NoError(t, db.Compact([]byte("round-key-000000"), []byte("round-key-999999"), true))

	// Verify a key is readable before close.
	val, closer, err := db.Get([]byte("round-key-000050"))
	require.NoError(t, err)
	require.Equal(t, "round-val-000050-padding", string(val))
	require.NoError(t, closer.Close())

	require.NoError(t, db.Close())

	// Upload the manifest bundle.
	require.NoError(t, UploadManifestBundle(ctx, memFS, "/db", metaStore))

	// Wipe the local directory.
	require.NoError(t, memFS.RemoveAll("/db"))
	require.NoError(t, memFS.MkdirAll("/db2", 0755))

	// Download the manifest bundle to a new directory.
	require.NoError(t, DownloadManifestBundle(ctx, memFS, "/db2", metaStore))

	// Reopen the DB from the new directory with the same remote storage.
	db2 := openTestDB(t, memFS, "/db2", remoteStore)

	// Verify data is still readable.
	val, closer, err = db2.Get([]byte("round-key-000050"))
	require.NoError(t, err)
	require.Equal(t, "round-val-000050-padding", string(val))
	require.NoError(t, closer.Close())

	require.NoError(t, db2.Close())
}

// TestManifestBundleExists verifies that ManifestBundleExists returns true
// after uploading a bundle, and false on empty storage.
func TestManifestBundleExists(t *testing.T) {
	ctx := t.Context()
	memFS := vfs.NewMem()
	remoteStore := remote.NewInMem()
	metaStore := remote.NewInMem()

	// Empty storage should return false.
	exists, err := ManifestBundleExists(ctx, metaStore)
	require.NoError(t, err)
	require.False(t, exists)

	// Create a DB, close it, upload the bundle.
	db := openTestDB(t, memFS, "/db", remoteStore)
	writeTestData(t, db, "exists", 10)
	require.NoError(t, db.Close())
	require.NoError(t, UploadManifestBundle(ctx, memFS, "/db", metaStore))

	// Now it should exist.
	exists, err = ManifestBundleExists(ctx, metaStore)
	require.NoError(t, err)
	require.True(t, exists)
}

// TestManifestBundleOverwrite uploads a bundle twice and verifies the second
// upload overwrites the first (i.e. the data from the second DB is what we
// get back).
func TestManifestBundleOverwrite(t *testing.T) {
	ctx := t.Context()
	memFS := vfs.NewMem()
	remoteStore := remote.NewInMem()
	metaStore := remote.NewInMem()

	// First DB session.
	db := openTestDB(t, memFS, "/db1", remoteStore)
	writeTestData(t, db, "first", 50)
	require.NoError(t, db.Compact([]byte("first-key-000000"), []byte("first-key-999999"), true))
	require.NoError(t, db.Close())
	require.NoError(t, UploadManifestBundle(ctx, memFS, "/db1", metaStore))

	// Second DB session — reopen from the first bundle, write more data.
	require.NoError(t, memFS.MkdirAll("/db2", 0755))
	require.NoError(t, DownloadManifestBundle(ctx, memFS, "/db2", metaStore))
	db2 := openTestDB(t, memFS, "/db2", remoteStore)
	writeTestData(t, db2, "second", 50)
	require.NoError(t, db2.Compact([]byte("second-key-000000"), []byte("second-key-999999"), true))
	require.NoError(t, db2.Close())

	// Overwrite the bundle with the second session's state.
	require.NoError(t, UploadManifestBundle(ctx, memFS, "/db2", metaStore))

	// Download into a fresh dir and verify both sessions' data is present.
	require.NoError(t, memFS.MkdirAll("/db3", 0755))
	require.NoError(t, DownloadManifestBundle(ctx, memFS, "/db3", metaStore))
	db3 := openTestDB(t, memFS, "/db3", remoteStore)

	val, closer, err := db3.Get([]byte("first-key-000025"))
	require.NoError(t, err)
	require.Equal(t, "first-val-000025-padding", string(val))
	require.NoError(t, closer.Close())

	val, closer, err = db3.Get([]byte("second-key-000025"))
	require.NoError(t, err)
	require.Equal(t, "second-val-000025-padding", string(val))
	require.NoError(t, closer.Close())

	require.NoError(t, db3.Close())
}
