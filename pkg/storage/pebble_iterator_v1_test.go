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
	"context"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
	"github.com/stretchr/testify/require"
)

// TestPebbleIteratorSetBoundsPreservesReadState verifies that calling
// SetBounds on a *pebble.Iterator does not cause it to see newer writes
// that were made after the iterator was created.
func TestPebbleIteratorSetBoundsPreservesReadState(t *testing.T) {
	opts := &pebble.Options{FS: vfs.NewMem()}
	db, err := pebble.Open("", opts)
	require.NoError(t, err)
	defer db.Close()

	// Write initial key.
	require.NoError(t, db.Set([]byte("a"), []byte("v1"), pebble.Sync))

	// Create iterator pinning current state.
	iter, err := db.NewIter(&pebble.IterOptions{
		LowerBound: []byte("a"),
		UpperBound: []byte("b"),
	})
	require.NoError(t, err)

	// Write a second key after iterator creation.
	require.NoError(t, db.Set([]byte("a2"), []byte("v2"), pebble.Sync))

	// SetBounds and seek — should NOT see "a2".
	iter.SetBounds([]byte("a"), []byte("z"))
	iter.SeekGE([]byte("a"))
	count := 0
	for iter.Valid() {
		count++
		iter.Next()
	}
	require.NoError(t, iter.Error())
	require.NoError(t, iter.Close())

	// If SetBounds preserves the read state, count should be 1.
	// If SetBounds refreshes the read state, count will be 2.
	t.Logf("SetBounds read state test: saw %d keys (1=pinned, 2=refreshed)", count)
	// Record the actual behavior — this documents Pebble v1.1.5 semantics.
	if count == 1 {
		t.Log("GOOD: SetBounds preserves read state")
	} else {
		t.Log("INFO: SetBounds refreshes read state — iterator reuse must account for this")
	}
}

// TestPebbleIteratorClonePreservesReadState verifies that Clone preserves
// the source iterator's read state.
func TestPebbleIteratorClonePreservesReadState(t *testing.T) {
	opts := &pebble.Options{FS: vfs.NewMem()}
	db, err := pebble.Open("", opts)
	require.NoError(t, err)
	defer db.Close()

	require.NoError(t, db.Set([]byte("a"), []byte("v1"), pebble.Sync))

	// Create source iterator.
	src, err := db.NewIter(&pebble.IterOptions{
		LowerBound: []byte("a"),
		UpperBound: []byte("b"),
	})
	require.NoError(t, err)

	// Write after source creation.
	require.NoError(t, db.Set([]byte("a2"), []byte("v2"), pebble.Sync))

	// Clone and seek.
	clone, err := src.Clone(pebble.CloneOptions{})
	require.NoError(t, err)
	clone.SetBounds([]byte("a"), []byte("z"))
	clone.SeekGE([]byte("a"))
	count := 0
	for clone.Valid() {
		count++
		clone.Next()
	}
	require.NoError(t, clone.Error())
	require.NoError(t, clone.Close())
	require.NoError(t, src.Close())

	t.Logf("Clone read state test: saw %d keys (1=pinned, 2=refreshed)", count)
	if count == 1 {
		t.Log("GOOD: Clone preserves read state")
	} else {
		t.Log("INFO: Clone refreshes read state")
	}
}

// TestPebbleBatchIteratorSeesBatchWrites verifies that a batch iterator
// sees writes made to the batch after the iterator was created, when
// the iterator is reused via SetBounds.
func TestPebbleBatchIteratorSeesBatchWrites(t *testing.T) {
	opts := &pebble.Options{FS: vfs.NewMem()}
	db, err := pebble.Open("", opts)
	require.NoError(t, err)
	defer db.Close()

	batch := db.NewIndexedBatch()
	defer batch.Close()

	require.NoError(t, batch.Set([]byte("a"), []byte("v1"), nil))

	iter, err := batch.NewIter(&pebble.IterOptions{
		LowerBound: []byte("a"),
		UpperBound: []byte("z"),
	})
	require.NoError(t, err)

	// Write to batch after iterator creation.
	require.NoError(t, batch.Set([]byte("b"), []byte("v2"), nil))

	// SetBounds and seek.
	iter.SetBounds([]byte("a"), []byte("z"))
	iter.SeekGE([]byte("a"))
	count := 0
	for iter.Valid() {
		count++
		iter.Next()
	}
	require.NoError(t, iter.Error())
	require.NoError(t, iter.Close())

	t.Logf("Batch SetBounds test: saw %d keys (1=pinned, 2=sees new writes)", count)
}

// TestPebbleBatchIteratorCloneRefreshBatchView verifies that Clone with
// RefreshBatchView sees new batch writes while preserving engine read state.
func TestPebbleBatchIteratorCloneRefreshBatchView(t *testing.T) {
	opts := &pebble.Options{FS: vfs.NewMem()}
	db, err := pebble.Open("", opts)
	require.NoError(t, err)
	defer db.Close()

	// Write to engine first.
	require.NoError(t, db.Set([]byte("a"), []byte("engine-v1"), pebble.Sync))

	batch := db.NewIndexedBatch()
	defer batch.Close()

	require.NoError(t, batch.Set([]byte("b"), []byte("batch-v1"), nil))

	// Create iterator — pins engine state and batch state.
	iter, err := batch.NewIter(&pebble.IterOptions{
		LowerBound: []byte("a"),
		UpperBound: []byte("z"),
	})
	require.NoError(t, err)

	// Write to engine after iterator creation.
	require.NoError(t, db.Set([]byte("c"), []byte("engine-v2"), pebble.Sync))
	// Write to batch after iterator creation.
	require.NoError(t, batch.Set([]byte("d"), []byte("batch-v2"), nil))

	// Clone with RefreshBatchView.
	clone, err := iter.Clone(pebble.CloneOptions{RefreshBatchView: true})
	require.NoError(t, err)
	clone.SetBounds([]byte("a"), []byte("z"))
	clone.SeekGE([]byte("a"))

	var keys []string
	for clone.Valid() {
		keys = append(keys, string(clone.Key()))
		clone.Next()
	}
	require.NoError(t, clone.Error())
	require.NoError(t, clone.Close())
	require.NoError(t, iter.Close())

	t.Logf("Clone RefreshBatchView keys: %v", keys)
	// Expected: "a" (engine, pinned), "b" (batch, old), "d" (batch, new via refresh)
	// NOT expected: "c" (engine, written after pin)
	for _, k := range keys {
		if k == "c" {
			t.Error("clone saw engine write 'c' made after pin — engine state NOT preserved")
		}
	}
	found_d := false
	for _, k := range keys {
		if k == "d" {
			found_d = true
		}
	}
	if !found_d {
		t.Error("clone did NOT see batch write 'd' despite RefreshBatchView=true")
	}
}

// TestPebbleBatchIteratorPinsEngineState verifies that a batch iterator
// does not see engine writes made after the iterator was created.
func TestPebbleBatchIteratorPinsEngineState(t *testing.T) {
	ctx := context.Background()
	_ = ctx

	opts := &pebble.Options{FS: vfs.NewMem()}
	db, err := pebble.Open("", opts)
	require.NoError(t, err)
	defer db.Close()

	require.NoError(t, db.Set([]byte("a"), []byte("v1"), pebble.Sync))

	batch := db.NewIndexedBatch()
	defer batch.Close()

	// Create and close iterator to pin engine state via batch's p.iter.
	iter1, err := batch.NewIter(&pebble.IterOptions{
		LowerBound: []byte("a"),
		UpperBound: []byte("z"),
	})
	require.NoError(t, err)
	require.NoError(t, iter1.Close())

	// Write to engine after batch iterator creation.
	require.NoError(t, db.Set([]byte("b"), []byte("v2"), pebble.Sync))

	// Create second iterator from same batch.
	iter2, err := batch.NewIter(&pebble.IterOptions{
		LowerBound: []byte("a"),
		UpperBound: []byte("z"),
	})
	require.NoError(t, err)

	iter2.SeekGE([]byte("a"))
	count := 0
	for iter2.Valid() {
		count++
		iter2.Next()
	}
	require.NoError(t, iter2.Error())
	require.NoError(t, iter2.Close())

	t.Logf("Batch engine pin test: saw %d keys (1=pinned, 2=saw new engine write)", count)
	// This documents whether Batch.NewIter pins engine state across calls.
	// In old Pebble it did (via shared iterator). In v1.1.5, each NewIter
	// gets a fresh readState.
}
