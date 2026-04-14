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
	"context"

	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/pebble/objstorage/remote"
	"github.com/semistrict/ratel/pkg/roachpb"
	"github.com/semistrict/ratel/pkg/util/log"
	"github.com/semistrict/ratel/pkg/util/protoutil"
	"github.com/semistrict/ratel/pkg/util/syncutil"
)

const firstRangeObjectName = "first-range-descriptor"

// S3FirstRangeProvider implements FirstRangeProvider using S3 (remote.Storage)
// for cold-start bootstrap. The range 1 leaseholder writes the first range
// descriptor to S3; joining nodes read it to bootstrap DistSender.
type S3FirstRangeProvider struct {
	store remote.Storage

	mu struct {
		syncutil.Mutex
		desc      *roachpb.RangeDescriptor
		callbacks []func(*roachpb.RangeDescriptor)
	}
}

var _ FirstRangeProvider = (*S3FirstRangeProvider)(nil)

// NewS3FirstRangeProvider creates a FirstRangeProvider backed by S3.
func NewS3FirstRangeProvider(store remote.Storage) *S3FirstRangeProvider {
	return &S3FirstRangeProvider{store: store}
}

// GetFirstRangeDescriptor implements FirstRangeProvider. It returns the cached
// descriptor if available, otherwise reads from S3.
func (p *S3FirstRangeProvider) GetFirstRangeDescriptor() (*roachpb.RangeDescriptor, error) {
	p.mu.Lock()
	if p.mu.desc != nil {
		desc := p.mu.desc
		p.mu.Unlock()
		return desc, nil
	}
	p.mu.Unlock()

	// Read from S3.
	desc, err := p.readFromS3()
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	p.mu.desc = desc
	p.mu.Unlock()
	return desc, nil
}

// OnFirstRangeChanged implements FirstRangeProvider.
func (p *S3FirstRangeProvider) OnFirstRangeChanged(cb func(*roachpb.RangeDescriptor)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mu.callbacks = append(p.mu.callbacks, cb)
}

// Set updates the cached first range descriptor and notifies all registered
// callbacks. Called by the range 1 leaseholder after writing to S3.
func (p *S3FirstRangeProvider) Set(desc *roachpb.RangeDescriptor) {
	p.mu.Lock()
	p.mu.desc = desc
	callbacks := make([]func(*roachpb.RangeDescriptor), len(p.mu.callbacks))
	copy(callbacks, p.mu.callbacks)
	p.mu.Unlock()

	for _, cb := range callbacks {
		cb(desc)
	}
}

// WriteToS3 writes the first range descriptor to S3. Called by the range 1
// leaseholder.
func (p *S3FirstRangeProvider) WriteToS3(ctx context.Context, desc *roachpb.RangeDescriptor) error {
	data, err := protoutil.Marshal(desc)
	if err != nil {
		return errors.Wrap(err, "marshaling first range descriptor")
	}
	w, err := p.store.CreateObject(firstRangeObjectName)
	if err != nil {
		return errors.Wrap(err, "creating first range descriptor object")
	}
	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return errors.Wrap(err, "writing first range descriptor")
	}
	if err := w.Close(); err != nil {
		return errors.Wrap(err, "closing first range descriptor object")
	}
	p.Set(desc)
	log.Infof(ctx, "wrote first range descriptor to S3")
	return nil
}

func (p *S3FirstRangeProvider) readFromS3() (*roachpb.RangeDescriptor, error) {
	ctx := context.Background()
	reader, size, err := p.store.ReadObject(ctx, firstRangeObjectName)
	if err != nil {
		return nil, errors.Wrap(err, "the first range descriptor is not available in S3")
	}
	buf := make([]byte, size)
	if err := reader.ReadAt(ctx, buf, 0); err != nil {
		_ = reader.Close()
		return nil, errors.Wrap(err, "reading first range descriptor from S3")
	}
	_ = reader.Close()

	desc := &roachpb.RangeDescriptor{}
	if err := protoutil.Unmarshal(buf, desc); err != nil {
		return nil, errors.Wrap(err, "unmarshaling first range descriptor from S3")
	}
	return desc, nil
}
