// Copyright 2026 The Ratel Authors
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

// Package storedescstore provides a rangefeed-backed cache of store descriptors
// stored in the system keyspace, replacing gossip for store discovery.
package storedescstore

import (
	"bytes"
	"context"

	"github.com/cockroachdb/errors"
	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/kv"
	"github.com/semistrict/ratel/pkg/kv/kvclient/rangefeed"
	"github.com/semistrict/ratel/pkg/roachpb"
	"github.com/semistrict/ratel/pkg/util/encoding"
	"github.com/semistrict/ratel/pkg/util/hlc"
	"github.com/semistrict/ratel/pkg/util/log"
	"github.com/semistrict/ratel/pkg/util/stop"
	"github.com/semistrict/ratel/pkg/util/syncutil"
)

// UpdateCallback is called when a store descriptor is updated in the cache.
type UpdateCallback func(desc roachpb.StoreDescriptor)

// Store is a rangefeed-backed cache of store descriptors. It provides
// callback-on-update for consumers like StorePool.
type Store struct {
	db              *kv.DB
	clock           *hlc.Clock
	f               *rangefeed.Factory
	stopper         *stop.Stopper
	initialScanDone chan struct{}

	mu struct {
		syncutil.RWMutex
		stores     map[roachpb.StoreID]*roachpb.StoreDescriptor
		callbacks  []UpdateCallback
		started    bool
		startError error
	}
}

// New creates a new store descriptor store.
func New(
	db *kv.DB,
	clock *hlc.Clock,
	f *rangefeed.Factory,
	stopper *stop.Stopper,
) *Store {
	s := &Store{
		db:              db,
		clock:           clock,
		f:               f,
		stopper:         stopper,
		initialScanDone: make(chan struct{}),
	}
	s.mu.stores = make(map[roachpb.StoreID]*roachpb.StoreDescriptor)
	return s
}

// RegisterCallback registers a callback that fires on every store descriptor
// update, including redundant ones (needed by StorePool as a staleness clock).
func (s *Store) RegisterCallback(cb UpdateCallback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mu.callbacks = append(s.mu.callbacks, cb)
}

// Start initializes the rangefeed and blocks until the initial scan completes
// or fails. Store descriptors are available once Start returns nil.
func (s *Store) Start(ctx context.Context) error {
	rf := s.maybeStartRangeFeed(ctx)
	if rf != nil {
		s.stopper.AddCloser(rf)
	}
	select {
	case <-s.initialScanDone:
	case <-ctx.Done():
		return ctx.Err()
	case <-s.stopper.ShouldQuiesce():
		return errors.New("stopper quiescing during store descriptor store startup")
	}
	return s.checkStarted()
}

func (s *Store) maybeStartRangeFeed(ctx context.Context) *rangefeed.RangeFeed {
	s.mu.Lock()
	if s.mu.started {
		s.mu.Unlock()
		return nil
	}
	s.mu.started = true
	s.mu.Unlock()

	updateFn := func(ctx context.Context, kv *roachpb.RangeFeedValue) {
		storeID, err := decodeStoreDescriptorKey(kv.Key)
		if err != nil {
			log.Warningf(ctx, "failed to decode store descriptor key %v: %v", kv.Key, err)
			return
		}
		if len(kv.Value.RawBytes) == 0 {
			s.mu.Lock()
			delete(s.mu.stores, storeID)
			s.mu.Unlock()
			return
		}
		var desc roachpb.StoreDescriptor
		if err := kv.Value.GetProto(&desc); err != nil {
			log.Warningf(ctx, "failed to decode store descriptor for s%d: %v", storeID, err)
			return
		}
		s.mu.Lock()
		s.mu.stores[storeID] = &desc
		callbacks := make([]UpdateCallback, len(s.mu.callbacks))
		copy(callbacks, s.mu.callbacks)
		s.mu.Unlock()

		for _, cb := range callbacks {
			cb(desc)
		}
	}

	initialScanDoneFn := func(_ context.Context) {
		close(s.initialScanDone)
	}
	initialScanErrFn := func(_ context.Context, err error) (shouldFail bool) {
		s.mu.Lock()
		s.mu.startError = err
		s.mu.Unlock()
		close(s.initialScanDone)
		return true
	}

	rf, err := s.f.RangeFeed(ctx,
		"store-descriptors",
		[]roachpb.Span{keys.StoreDescriptorSpan},
		s.clock.Now(),
		updateFn,
		rangefeed.WithInitialScan(initialScanDoneFn),
		rangefeed.WithOnInitialScanError(initialScanErrFn),
	)
	if err != nil {
		s.mu.Lock()
		s.mu.startError = err
		s.mu.Unlock()
		close(s.initialScanDone)
		return nil
	}
	return rf
}

// GetStoreDescriptor looks up a store descriptor by store ID.
func (s *Store) GetStoreDescriptor(storeID roachpb.StoreID) (*roachpb.StoreDescriptor, error) {
	s.mu.RLock()
	desc, ok := s.mu.stores[storeID]
	s.mu.RUnlock()
	if !ok {
		return nil, errors.Errorf("unable to look up descriptor for s%d", storeID)
	}
	return desc, nil
}

// GetAllStoreDescriptors returns a snapshot of all cached store descriptors.
func (s *Store) GetAllStoreDescriptors() []*roachpb.StoreDescriptor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	descs := make([]*roachpb.StoreDescriptor, 0, len(s.mu.stores))
	for _, desc := range s.mu.stores {
		descs = append(descs, desc)
	}
	return descs
}

// Upsert writes a store descriptor to KV.
func (s *Store) Upsert(ctx context.Context, desc *roachpb.StoreDescriptor) error {
	key := keys.StoreDescriptorKey(desc.StoreID)
	return s.db.Put(ctx, key, desc)
}

// InitialScanDone returns a channel that is closed when the initial rangefeed
// scan completes (or fails).
func (s *Store) InitialScanDone() <-chan struct{} {
	return s.initialScanDone
}

func (s *Store) checkStarted() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.mu.started {
		return errors.New("store descriptor store not started")
	}
	return s.mu.startError
}

// decodeStoreDescriptorKey extracts the StoreID from a store descriptor key.
func decodeStoreDescriptorKey(key roachpb.Key) (roachpb.StoreID, error) {
	if !bytes.HasPrefix(key, keys.StoreDescriptorPrefix) {
		return 0, errors.Errorf("key %s does not have store descriptor prefix", key)
	}
	remainder := key[len(keys.StoreDescriptorPrefix):]
	_, storeID, err := encoding.DecodeUvarintAscending(remainder)
	if err != nil {
		return 0, errors.Wrap(err, "decoding store ID from descriptor key")
	}
	return roachpb.StoreID(storeID), nil
}
