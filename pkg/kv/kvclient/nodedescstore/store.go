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

// Package nodedescstore provides a rangefeed-backed cache of node descriptors
// stored in the system keyspace, replacing gossip for node discovery.
package nodedescstore

import (
	"bytes"
	"context"
	"net"

	"github.com/cockroachdb/errors"
	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/kv"
	"github.com/semistrict/ratel/pkg/kv/kvclient/rangefeed"
	"github.com/semistrict/ratel/pkg/roachpb"
	"github.com/semistrict/ratel/pkg/rpc/nodedialer"
	"github.com/semistrict/ratel/pkg/util"
	"github.com/semistrict/ratel/pkg/util/encoding"
	"github.com/semistrict/ratel/pkg/util/hlc"
	"github.com/semistrict/ratel/pkg/util/log"
	"github.com/semistrict/ratel/pkg/util/stop"
	"github.com/semistrict/ratel/pkg/util/syncutil"
)

// Store is a rangefeed-backed cache of node descriptors stored in the
// system keyspace. It implements kvcoord.NodeDescStore and provides an
// AddressResolver for the node dialer.
type Store struct {
	db              *kv.DB
	clock           *hlc.Clock
	f               *rangefeed.Factory
	stopper         *stop.Stopper
	initialScanDone chan struct{}

	mu struct {
		syncutil.RWMutex
		nodes      map[roachpb.NodeID]*roachpb.NodeDescriptor
		started    bool
		startError error
	}
}

// Store implements kvcoord.NodeDescStore (verified at the wiring
// site in pkg/server/server.go).

// New creates a new node descriptor store.
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
	s.mu.nodes = make(map[roachpb.NodeID]*roachpb.NodeDescriptor)
	return s
}

// Start initializes the rangefeed and blocks until the initial scan completes
// or fails. The store is usable immediately via SetLocal() for the local
// node's descriptor. Remote node descriptors become available once Start
// returns nil.
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
		return errors.New("stopper quiescing during node descriptor store startup")
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
		nodeID, err := decodeNodeDescriptorKey(kv.Key)
		if err != nil {
			log.Warningf(ctx, "failed to decode node descriptor key %v: %v", kv.Key, err)
			return
		}
		if len(kv.Value.RawBytes) == 0 {
			// Deletion.
			s.mu.Lock()
			delete(s.mu.nodes, nodeID)
			s.mu.Unlock()
			return
		}
		var desc roachpb.NodeDescriptor
		if err := kv.Value.GetProto(&desc); err != nil {
			log.Warningf(ctx, "failed to decode node descriptor for n%d: %v", nodeID, err)
			return
		}
		s.mu.Lock()
		s.mu.nodes[nodeID] = &desc
		s.mu.Unlock()
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
		"node-descriptors",
		[]roachpb.Span{keys.NodeDescriptorSpan},
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

// GetNodeDescriptor implements kvcoord.NodeDescStore.
func (s *Store) GetNodeDescriptor(nodeID roachpb.NodeID) (*roachpb.NodeDescriptor, error) {
	s.mu.RLock()
	desc, ok := s.mu.nodes[nodeID]
	s.mu.RUnlock()
	if !ok {
		return nil, errors.Errorf("unable to look up descriptor for n%d", nodeID)
	}
	return desc, nil
}

// GetNodeCount returns the number of node descriptors in the cache.
func (s *Store) GetNodeCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.mu.nodes)
}

// GetAllNodeDescriptors returns a snapshot of all cached node descriptors.
func (s *Store) GetAllNodeDescriptors() []*roachpb.NodeDescriptor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	descs := make([]*roachpb.NodeDescriptor, 0, len(s.mu.nodes))
	for _, desc := range s.mu.nodes {
		descs = append(descs, desc)
	}
	return descs
}

// AddressResolver returns a nodedialer.AddressResolver backed by this store.
func (s *Store) AddressResolver() nodedialer.AddressResolver {
	return func(nodeID roachpb.NodeID) (net.Addr, error) {
		desc, err := s.GetNodeDescriptor(nodeID)
		if err != nil {
			return nil, err
		}
		return desc.Address.Resolve()
	}
}

// GetNodeIDAddress returns the network address for the given node.
func (s *Store) GetNodeIDAddress(nodeID roachpb.NodeID) (*util.UnresolvedAddr, error) {
	desc, err := s.GetNodeDescriptor(nodeID)
	if err != nil {
		return nil, err
	}
	return &desc.Address, nil
}

// GetNodeIDSQLAddress returns the SQL address for the given node.
func (s *Store) GetNodeIDSQLAddress(nodeID roachpb.NodeID) (*util.UnresolvedAddr, error) {
	desc, err := s.GetNodeDescriptor(nodeID)
	if err != nil {
		return nil, err
	}
	return desc.CheckedSQLAddress(), nil
}

// GetNodeIDHTTPAddress returns the HTTP address for the given node.
func (s *Store) GetNodeIDHTTPAddress(nodeID roachpb.NodeID) (*util.UnresolvedAddr, error) {
	desc, err := s.GetNodeDescriptor(nodeID)
	if err != nil {
		return nil, err
	}
	return &desc.HTTPAddress, nil
}

// Upsert writes a node descriptor to KV and updates the in-memory cache.
// Each node calls this for its own descriptor, similar to liveness
// heartbeats.
func (s *Store) Upsert(ctx context.Context, desc *roachpb.NodeDescriptor) error {
	s.SetLocal(desc)
	key := keys.NodeDescriptorKey(desc.NodeID)
	return s.db.Put(ctx, key, desc)
}

// SetLocal registers a node descriptor in the in-memory cache without
// writing to KV. Use during startup to make the local node's address
// resolvable before the rangefeed or KV write completes.
func (s *Store) SetLocal(desc *roachpb.NodeDescriptor) {
	s.mu.Lock()
	s.mu.nodes[desc.NodeID] = desc
	s.mu.Unlock()
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
		return errors.New("node descriptor store not started")
	}
	return s.mu.startError
}

// decodeNodeDescriptorKey extracts the NodeID from a node descriptor key.
func decodeNodeDescriptorKey(key roachpb.Key) (roachpb.NodeID, error) {
	if !bytes.HasPrefix(key, keys.NodeDescriptorPrefix) {
		return 0, errors.Errorf("key %s does not have node descriptor prefix", key)
	}
	remainder := key[len(keys.NodeDescriptorPrefix):]
	_, nodeID, err := encoding.DecodeUvarintAscending(remainder)
	if err != nil {
		return 0, errors.Wrap(err, "decoding node ID from descriptor key")
	}
	return roachpb.NodeID(nodeID), nil
}
