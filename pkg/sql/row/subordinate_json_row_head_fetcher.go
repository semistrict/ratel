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

package row

import (
	"context"
	"sort"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/kv"
	"github.com/semistrict/ratel/pkg/kv/kvserver/concurrency/lock"
	"github.com/semistrict/ratel/pkg/roachpb"
	"github.com/semistrict/ratel/pkg/sql/catalog/descpb"
)

// SubordinateJSONRowLookupSpec identifies the subordinate JSON subtree prefixes
// that should be fetched for each row when broad scans are reduced to a row
// head plus targeted side lookups.
type SubordinateJSONRowLookupSpec struct {
	ColID         descpb.ColumnID
	SelectedPaths [][]keys.SubordinatePathSegment
}

type subordinateJSONRowHeadSpanBuilder struct {
	rowKey roachpb.Key
	spans  roachpb.Spans
	seen   map[string]struct{}
}

func newSubordinateJSONRowHeadSpanBuilder(rowKey roachpb.Key) *subordinateJSONRowHeadSpanBuilder {
	return &subordinateJSONRowHeadSpanBuilder{
		rowKey: append(roachpb.Key(nil), rowKey...),
		seen:   make(map[string]struct{}),
	}
}

func (b *subordinateJSONRowHeadSpanBuilder) addSpan(span roachpb.Span) {
	key := string(span.Key) + "\x00" + string(span.EndKey)
	if _, ok := b.seen[key]; ok {
		return
	}
	b.seen[key] = struct{}{}
	b.spans = append(b.spans, span)
}

func (b *subordinateJSONRowHeadSpanBuilder) addExactKey(key roachpb.Key) {
	cp := append(roachpb.Key(nil), key...)
	b.addSpan(roachpb.Span{Key: cp, EndKey: cp.Next()})
}

func (b *subordinateJSONRowHeadSpanBuilder) addPathPrefix(prefix roachpb.Key) {
	cp := append(roachpb.Key(nil), prefix...)
	b.addSpan(roachpb.Span{Key: cp, EndKey: cp.PrefixEnd()})
}

func subordinateJSONStoredLookupPath(
	path []keys.SubordinatePathSegment,
) []keys.SubordinatePathSegment {
	stored := make([]keys.SubordinatePathSegment, 0, len(path)+1)
	stored = append(stored, keys.SubordinatePathSegment{Kind: keys.SubordinatePathHeader})
	stored = append(stored, path...)
	return stored
}

func (b *subordinateJSONRowHeadSpanBuilder) addSelectedPath(
	colID descpb.ColumnID, path []keys.SubordinatePathSegment,
) {
	storedPath := subordinateJSONStoredLookupPath(path)
	b.addExactKey(roachpb.Key(keys.MakeSubordinatePathKey(
		b.rowKey, uint32(colID), []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}},
	)))
	for i := 2; i < len(storedPath); i++ {
		b.addExactKey(roachpb.Key(keys.MakeSubordinatePathKey(
			b.rowKey, uint32(colID), storedPath[:i],
		)))
	}
	if len(path) == 0 {
		return
	}
	b.addPathPrefix(roachpb.Key(keys.MakeSubordinatePathPrefix(
		b.rowKey, uint32(colID), storedPath,
	)))
}

func (b *subordinateJSONRowHeadSpanBuilder) finish() roachpb.Spans {
	sort.Slice(b.spans, func(i, j int) bool {
		if cmp := b.spans[i].Key.Compare(b.spans[j].Key); cmp != 0 {
			return cmp < 0
		}
		return b.spans[i].EndKey.Compare(b.spans[j].EndKey) < 0
	})
	return b.spans
}

type subordinateJSONRowHeadBatchFetcher struct {
	sendFn         sendFunc
	spans          roachpb.Spans
	lockStrength   lock.Strength
	lockWaitPolicy lock.WaitPolicy
	lockTimeout    time.Duration
	lookups        []SubordinateJSONRowLookupSpec
}

// NewSubordinateJSONRowHeadKVFetcher constructs a KV fetcher that turns a
// broad scan into a sequence of row-head scans plus exact subordinate JSON
// side lookups for selected static path prefixes.
func NewSubordinateJSONRowHeadKVFetcher(
	txn *kv.Txn,
	spans roachpb.Spans,
	reverse bool,
	lockStrength descpb.ScanLockingStrength,
	lockWaitPolicy descpb.ScanLockingWaitPolicy,
	lockTimeout time.Duration,
	lookups []SubordinateJSONRowLookupSpec,
) (*KVFetcher, error) {
	if reverse {
		return nil, errors.AssertionFailedf("reverse subordinate JSON row-head fetcher is unsupported")
	}
	cp := append(roachpb.Spans(nil), spans...)
	f := &subordinateJSONRowHeadBatchFetcher{
		sendFn:         makeKVBatchFetcherDefaultSendFunc(txn),
		spans:          cp,
		lockStrength:   getKeyLockingStrength(lockStrength),
		lockWaitPolicy: GetWaitPolicy(lockWaitPolicy),
		lockTimeout:    lockTimeout,
		lookups:        append([]SubordinateJSONRowLookupSpec(nil), lookups...),
	}
	return newKVFetcher(f), nil
}

func (f *subordinateJSONRowHeadBatchFetcher) nextBatch(
	ctx context.Context,
) (ok bool, kvs []roachpb.KeyValue, batchResponse []byte, err error) {
	for len(f.spans) > 0 {
		headKV, rowKey, spanDone, err := f.fetchNextRowHead(ctx, f.spans[0])
		if err != nil {
			return false, nil, nil, err
		}
		if spanDone {
			f.spans[0] = roachpb.Span{}
			f.spans = f.spans[1:]
			continue
		}
		kvs = append(kvs, headKV)
		lookupKVs, err := f.fetchRowLookups(ctx, rowKey)
		if err != nil {
			return false, nil, nil, err
		}
		kvs = append(kvs, lookupKVs...)
		return true, kvs, nil, nil
	}
	return false, nil, nil, nil
}

func (f *subordinateJSONRowHeadBatchFetcher) close(context.Context) {}

func (f *subordinateJSONRowHeadBatchFetcher) fetchNextRowHead(
	ctx context.Context, span roachpb.Span,
) (kv roachpb.KeyValue, rowKey roachpb.Key, spanDone bool, err error) {
	var ba roachpb.BatchRequest
	ba.Header.WaitPolicy = f.lockWaitPolicy
	ba.Header.LockTimeout = f.lockTimeout
	ba.Header.MaxSpanRequestKeys = 1

	var req roachpb.ScanRequest
	req.SetSpan(span)
	req.ScanFormat = roachpb.KEY_VALUES
	req.KeyLocking = f.lockStrength
	union := roachpb.RequestUnion{}
	union.MustSetInner(&req)
	ba.Requests = []roachpb.RequestUnion{union}

	br, err := f.sendFn(ctx, ba)
	if err != nil {
		return kv, nil, false, err
	}
	resp := br.Responses[0].GetInner().(*roachpb.ScanResponse)
	if len(resp.Rows) == 0 {
		return kv, nil, true, nil
	}
	if len(resp.Rows) != 1 {
		return kv, nil, false, errors.AssertionFailedf("expected one row-head KV, got %d", len(resp.Rows))
	}
	kv = resp.Rows[0]
	prefixLen, err := keys.GetRowPrefixLength(kv.Key)
	if err != nil {
		return kv, nil, false, err
	}
	if prefixLen <= 0 || prefixLen >= len(kv.Key) {
		return kv, nil, false, errors.AssertionFailedf("invalid row-head key %q", kv.Key)
	}
	rowKey = roachpb.Key(keys.MakeFamilyKey(append([]byte(nil), kv.Key[:prefixLen]...), 0))

	nextKey := append(roachpb.Key(nil), rowKey...)
	nextKey = nextKey.PrefixEnd()
	if nextKey.Compare(span.EndKey) >= 0 {
		f.spans[0] = roachpb.Span{}
		f.spans = f.spans[1:]
	} else {
		f.spans[0] = roachpb.Span{Key: nextKey, EndKey: span.EndKey}
	}
	return kv, rowKey, false, nil
}

func (f *subordinateJSONRowHeadBatchFetcher) fetchRowLookups(
	ctx context.Context, rowKey roachpb.Key,
) ([]roachpb.KeyValue, error) {
	builder := newSubordinateJSONRowHeadSpanBuilder(rowKey)
	for i := range f.lookups {
		lookup := &f.lookups[i]
		for _, path := range lookup.SelectedPaths {
			builder.addSelectedPath(lookup.ColID, path)
		}
	}
	spans := builder.finish()
	if len(spans) == 0 {
		return nil, nil
	}

	var ba roachpb.BatchRequest
	ba.Header.WaitPolicy = f.lockWaitPolicy
	ba.Header.LockTimeout = f.lockTimeout
	ba.Requests = make([]roachpb.RequestUnion, 0, len(spans))
	for i := range spans {
		span := spans[i]
		if span.EndKey.Equal(span.Key.Next()) {
			var req roachpb.GetRequest
			req.Key = span.Key
			req.KeyLocking = f.lockStrength
			union := roachpb.RequestUnion{}
			union.MustSetInner(&req)
			ba.Requests = append(ba.Requests, union)
			continue
		}
		var req roachpb.ScanRequest
		req.SetSpan(span)
		req.ScanFormat = roachpb.KEY_VALUES
		req.KeyLocking = f.lockStrength
		union := roachpb.RequestUnion{}
		union.MustSetInner(&req)
		ba.Requests = append(ba.Requests, union)
	}

	br, err := f.sendFn(ctx, ba)
	if err != nil {
		return nil, err
	}

	var kvs []roachpb.KeyValue
	for i := range br.Responses {
		switch t := br.Responses[i].GetInner().(type) {
		case *roachpb.GetResponse:
			if t.Value != nil {
				kvs = append(kvs, roachpb.KeyValue{Key: spans[i].Key, Value: *t.Value})
			}
		case *roachpb.ScanResponse:
			kvs = append(kvs, t.Rows...)
		default:
			return nil, errors.AssertionFailedf("unexpected subordinate lookup response %T", t)
		}
	}
	return kvs, nil
}
