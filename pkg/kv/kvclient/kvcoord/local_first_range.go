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

package kvcoord

import (
	"github.com/cockroachdb/errors"
	"github.com/semistrict/ratel/pkg/roachpb"
	"github.com/semistrict/ratel/pkg/util/syncutil"
)

// LocalFirstRangeProvider is an in-memory FirstRangeProvider that replaces
// gossip. It gets populated by the range 1 leaseholder via Set() and serves
// the descriptor to the local DistSender.
type LocalFirstRangeProvider struct {
	mu struct {
		syncutil.Mutex
		desc      *roachpb.RangeDescriptor
		callbacks []func(*roachpb.RangeDescriptor)
	}
}

var _ FirstRangeProvider = (*LocalFirstRangeProvider)(nil)

// NewLocalFirstRangeProvider creates a new LocalFirstRangeProvider.
func NewLocalFirstRangeProvider() *LocalFirstRangeProvider {
	return &LocalFirstRangeProvider{}
}

// GetFirstRangeDescriptor implements FirstRangeProvider.
func (p *LocalFirstRangeProvider) GetFirstRangeDescriptor() (*roachpb.RangeDescriptor, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.mu.desc == nil {
		return nil, errors.New("first range descriptor not yet available")
	}
	return p.mu.desc, nil
}

// OnFirstRangeChanged implements FirstRangeProvider.
func (p *LocalFirstRangeProvider) OnFirstRangeChanged(cb func(*roachpb.RangeDescriptor)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mu.callbacks = append(p.mu.callbacks, cb)
	// If we already have a descriptor, fire the callback immediately.
	if p.mu.desc != nil {
		cb(p.mu.desc)
	}
}

// Set updates the first range descriptor and notifies all registered
// callbacks. Called by the range 1 leaseholder.
func (p *LocalFirstRangeProvider) Set(desc *roachpb.RangeDescriptor) {
	p.mu.Lock()
	p.mu.desc = desc
	callbacks := make([]func(*roachpb.RangeDescriptor), len(p.mu.callbacks))
	copy(callbacks, p.mu.callbacks)
	p.mu.Unlock()

	for _, cb := range callbacks {
		cb(desc)
	}
}
