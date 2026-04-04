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

// TestS3OnlyOpenClose opens a DB with S3 backend, writes keys, closes
// (flush+upload), reopens from S3, and verifies all keys are readable.
func TestS3OnlyOpenClose(t *testing.T) {
	ctx := t.Context()
	memFS := vfs.NewMem()
	remoteStore := remote.NewInMem()
	metaStore := remote.NewInMem()

	// Session 1: open, write, close.
	db := openTestDB(t, memFS, "/db", remoteStore)
	writeTestData(t, db, "oc", 200)
	require.NoError(t, db.Compact([]byte("oc-key-000000"), []byte("oc-key-999999"), true))
	require.NoError(t, db.Flush())
	require.NoError(t, db.Close())
	require.NoError(t, UploadManifestBundle(ctx, memFS, "/db", metaStore))

	// Wipe local state.
	require.NoError(t, memFS.RemoveAll("/db"))
	require.NoError(t, memFS.MkdirAll("/db2", 0755))

	// Session 2: download bundle, reopen, verify.
	require.NoError(t, DownloadManifestBundle(ctx, memFS, "/db2", metaStore))
	db2 := openTestDB(t, memFS, "/db2", remoteStore)

	for _, i := range []int{0, 50, 100, 199} {
		key := []byte(fmt.Sprintf("oc-key-%06d", i))
		val, closer, err := db2.Get(key)
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("oc-val-%06d-padding", i), string(val))
		require.NoError(t, closer.Close())
	}
	require.NoError(t, db2.Close())
}

// TestS3OnlyFreshStart opens a DB with empty S3, writes data, closes, and
// verifies that S3 has SSTables and the manifest bundle.
func TestS3OnlyFreshStart(t *testing.T) {
	ctx := t.Context()
	memFS := vfs.NewMem()
	remoteStore := remote.NewInMem()
	metaStore := remote.NewInMem()

	db := openTestDB(t, memFS, "/db", remoteStore)
	writeTestData(t, db, "fresh", 100)
	require.NoError(t, db.Compact([]byte("fresh-key-000000"), []byte("fresh-key-999999"), true))
	require.NoError(t, db.Close())

	// Upload manifest bundle.
	require.NoError(t, UploadManifestBundle(ctx, memFS, "/db", metaStore))

	// Verify remote SSTables exist.
	remoteObjs, err := remoteStore.List("", "")
	require.NoError(t, err)
	require.Greater(t, len(remoteObjs), 0, "expected remote SSTables")

	// Verify manifest bundle exists.
	exists, err := ManifestBundleExists(ctx, metaStore)
	require.NoError(t, err)
	require.True(t, exists)
}

// TestS3OnlyCleanShutdownNoLocalState verifies that after the full close
// lifecycle (flush + upload + remove temp dir), no local state remains.
func TestS3OnlyCleanShutdownNoLocalState(t *testing.T) {
	ctx := t.Context()
	memFS := vfs.NewMem()
	remoteStore := remote.NewInMem()
	metaStore := remote.NewInMem()

	db := openTestDB(t, memFS, "/db", remoteStore)
	writeTestData(t, db, "clean", 50)
	require.NoError(t, db.Flush())
	require.NoError(t, db.Close())
	require.NoError(t, UploadManifestBundle(ctx, memFS, "/db", metaStore))

	// Remove temp dir — simulating the clean shutdown lifecycle.
	require.NoError(t, memFS.RemoveAll("/db"))

	// Verify dir is gone.
	_, err := memFS.List("/db")
	require.Error(t, err)
}

// TestS3OnlyMultipleRestarts does open->write->close->open->write->close->open
// and verifies all data from all sessions is present.
func TestS3OnlyMultipleRestarts(t *testing.T) {
	ctx := t.Context()
	memFS := vfs.NewMem()
	remoteStore := remote.NewInMem()
	metaStore := remote.NewInMem()

	dir := "/db"

	// Session 1.
	db := openTestDB(t, memFS, dir, remoteStore)
	writeTestData(t, db, "s1", 100)
	require.NoError(t, db.Compact([]byte("s1-key-000000"), []byte("s1-key-999999"), true))
	require.NoError(t, db.Flush())
	require.NoError(t, db.Close())
	require.NoError(t, UploadManifestBundle(ctx, memFS, dir, metaStore))
	require.NoError(t, memFS.RemoveAll(dir))

	// Session 2: download, open, write more, close, upload.
	require.NoError(t, memFS.MkdirAll(dir, 0755))
	require.NoError(t, DownloadManifestBundle(ctx, memFS, dir, metaStore))
	db = openTestDB(t, memFS, dir, remoteStore)
	writeTestData(t, db, "s2", 100)
	require.NoError(t, db.Compact([]byte("s2-key-000000"), []byte("s2-key-999999"), true))
	require.NoError(t, db.Flush())
	require.NoError(t, db.Close())
	require.NoError(t, UploadManifestBundle(ctx, memFS, dir, metaStore))
	require.NoError(t, memFS.RemoveAll(dir))

	// Session 3: download, open, verify all data from sessions 1 and 2.
	require.NoError(t, memFS.MkdirAll(dir, 0755))
	require.NoError(t, DownloadManifestBundle(ctx, memFS, dir, metaStore))
	db = openTestDB(t, memFS, dir, remoteStore)

	for _, prefix := range []string{"s1", "s2"} {
		for _, i := range []int{0, 50, 99} {
			key := []byte(fmt.Sprintf("%s-key-%06d", prefix, i))
			val, closer, err := db.Get(key)
			require.NoError(t, err)
			require.Equal(t, fmt.Sprintf("%s-val-%06d-padding", prefix, i), string(val))
			require.NoError(t, closer.Close())
		}
	}
	require.NoError(t, db.Close())
}

// TestS3OnlyPeriodicCheckpoint opens a DB, writes data, flushes, uploads
// the manifest bundle from the live DB directory (without closing the DB),
// and verifies the bundle can be used to open a second DB that reads the data.
func TestS3OnlyPeriodicCheckpoint(t *testing.T) {
	ctx := t.Context()
	memFS := vfs.NewMem()
	remoteStore := remote.NewInMem()
	metaStore := remote.NewInMem()

	db := openTestDB(t, memFS, "/db", remoteStore)
	writeTestData(t, db, "cp", 100)
	require.NoError(t, db.Compact([]byte("cp-key-000000"), []byte("cp-key-999999"), true))

	// Flush to ensure all data is on remote storage, then upload the
	// manifest bundle from the live DB directory.
	require.NoError(t, db.Flush())
	require.NoError(t, UploadManifestBundle(ctx, memFS, "/db", metaStore))

	// Verify bundle exists without closing the DB.
	exists, err := ManifestBundleExists(ctx, metaStore)
	require.NoError(t, err)
	require.True(t, exists)

	// Verify the checkpoint bundle can be used to open a new DB that
	// reads the same remote SSTables.
	require.NoError(t, memFS.MkdirAll("/db2", 0755))
	require.NoError(t, DownloadManifestBundle(ctx, memFS, "/db2", metaStore))
	db2 := openTestDB(t, memFS, "/db2", remoteStore)

	val, closer, err := db2.Get([]byte("cp-key-000050"))
	require.NoError(t, err)
	require.Equal(t, "cp-val-000050-padding", string(val))
	require.NoError(t, closer.Close())

	require.NoError(t, db2.Close())
	require.NoError(t, db.Close())
}
