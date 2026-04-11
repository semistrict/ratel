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
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package planassert

import (
	"context"
	"sync"
	"testing"

	"github.com/semistrict/ratel/pkg/roachpb"
	"github.com/stretchr/testify/require"
)

// KVKeyCounter counts actual KV keys scanned by read requests against a target
// span while it is armed.
type KVKeyCounter struct {
	mu struct {
		sync.Mutex
		armed      bool
		targetSpan roachpb.Span
		scanned    int64
	}
}

// ResponseFilter exposes the counter as a StoreTestingKnobs response filter.
func (c *KVKeyCounter) ResponseFilter(
	_ context.Context, ba roachpb.BatchRequest, br *roachpb.BatchResponse,
) *roachpb.Error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.mu.armed || br == nil {
		return nil
	}
	for i := range ba.Requests {
		req := ba.Requests[i].GetInner()
		if req == nil {
			continue
		}
		switch req.Method() {
		case roachpb.Get, roachpb.Scan, roachpb.ReverseScan:
		default:
			continue
		}
		if !req.Header().Span().Overlaps(c.mu.targetSpan) {
			continue
		}
		if i >= len(br.Responses) {
			continue
		}
		resp := br.Responses[i].GetInner()
		if resp == nil {
			continue
		}
		c.mu.scanned += resp.Header().NumKeys
	}
	return nil
}

// Measure arms the counter for the given target span while fn runs, then
// returns the total number of KV keys scanned.
func (c *KVKeyCounter) Measure(targetSpan roachpb.Span, fn func()) int64 {
	c.mu.Lock()
	c.mu.armed = true
	c.mu.targetSpan = targetSpan
	c.mu.scanned = 0
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.mu.armed = false
		c.mu.targetSpan = roachpb.Span{}
		c.mu.Unlock()
	}()

	fn()

	c.mu.Lock()
	defer c.mu.Unlock()
	return c.mu.scanned
}

// UsesAtMostScannedKeys asserts that a KV key count stays bounded.
func UsesAtMostScannedKeys(t testing.TB, scanned int64, max int64) {
	t.Helper()
	require.LessOrEqual(t, scanned, max)
}
