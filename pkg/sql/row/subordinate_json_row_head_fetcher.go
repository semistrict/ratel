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
	"github.com/semistrict/ratel/pkg/sql/rowenc"
)

// SubordinateJSONRowLookupSpec identifies the subordinate JSON subtree prefixes
// that should be fetched for each row when broad scans are reduced to a row
// head plus targeted side lookups.
type SubordinateJSONRowLookupSpec struct {
	ColID         descpb.ColumnID
	SelectedPaths [][]keys.SubordinatePathSegment
	ExistsKeys    []string
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

func (b *subordinateJSONRowHeadSpanBuilder) addExistsKeys(colID descpb.ColumnID, keysToCheck []string) {
	b.addExactKey(roachpb.Key(keys.MakeSubordinatePathKey(
		b.rowKey, uint32(colID), []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}},
	)))
	for _, key := range keysToCheck {
		b.addExactKey(roachpb.Key(keys.MakeSubordinatePathKey(
			b.rowKey,
			uint32(colID),
			[]keys.SubordinatePathSegment{
				{Kind: keys.SubordinatePathHeader},
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: key},
			},
		)))
	}
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
	reverse        bool
	lockStrength   lock.Strength
	lockWaitPolicy lock.WaitPolicy
	lockTimeout    time.Duration
	lookups        []SubordinateJSONRowLookupSpec
}

// TestingSubordinateJSONRowHeadFetcherCreated, when non-nil, is invoked after a
// subordinate JSON row-head fetcher is constructed. Tests can use it to assert
// that a query took the optimized broad-scan path.
var TestingSubordinateJSONRowHeadFetcherCreated func([]SubordinateJSONRowLookupSpec)

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
	cp := append(roachpb.Spans(nil), spans...)
	f := &subordinateJSONRowHeadBatchFetcher{
		sendFn:         makeKVBatchFetcherDefaultSendFunc(txn),
		spans:          cp,
		reverse:        reverse,
		lockStrength:   getKeyLockingStrength(lockStrength),
		lockWaitPolicy: GetWaitPolicy(lockWaitPolicy),
		lockTimeout:    lockTimeout,
		lookups:        append([]SubordinateJSONRowLookupSpec(nil), lookups...),
	}
	if TestingSubordinateJSONRowHeadFetcherCreated != nil {
		TestingSubordinateJSONRowHeadFetcherCreated(append([]SubordinateJSONRowLookupSpec(nil), f.lookups...))
	}
	return newKVFetcher(f), nil
}

func (f *subordinateJSONRowHeadBatchFetcher) nextBatch(
	ctx context.Context,
) (ok bool, kvs []roachpb.KeyValue, batchResponse []byte, err error) {
	for len(f.spans) > 0 {
		rowKey, spanDone, err := f.fetchNextRowKey(ctx, f.spans[0])
		if err != nil {
			return false, nil, nil, err
		}
		if spanDone {
			f.spans[0] = roachpb.Span{}
			f.spans = f.spans[1:]
			continue
		}
		headKV, err := f.fetchRowHeadKV(ctx, rowKey)
		if err != nil {
			return false, nil, nil, err
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

func (f *subordinateJSONRowHeadBatchFetcher) fetchNextRowKey(
	ctx context.Context, span roachpb.Span,
) (rowKey roachpb.Key, spanDone bool, err error) {
	var ba roachpb.BatchRequest
	ba.Header.WaitPolicy = f.lockWaitPolicy
	ba.Header.LockTimeout = f.lockTimeout
	ba.Header.MaxSpanRequestKeys = 1

	union := roachpb.RequestUnion{}
	if f.reverse {
		var req roachpb.ReverseScanRequest
		req.SetSpan(span)
		req.ScanFormat = roachpb.KEY_VALUES
		req.KeyLocking = f.lockStrength
		union.MustSetInner(&req)
	} else {
		var req roachpb.ScanRequest
		req.SetSpan(span)
		req.ScanFormat = roachpb.KEY_VALUES
		req.KeyLocking = f.lockStrength
		union.MustSetInner(&req)
	}
	ba.Requests = []roachpb.RequestUnion{union}

	br, err := f.sendFn(ctx, ba)
	if err != nil {
		return nil, false, err
	}
	var rows []roachpb.KeyValue
	switch t := br.Responses[0].GetInner().(type) {
	case *roachpb.ScanResponse:
		rows = t.Rows
	case *roachpb.ReverseScanResponse:
		rows = t.Rows
	default:
		return nil, false, errors.AssertionFailedf("unexpected row-head scan response %T", t)
	}
	if len(rows) == 0 {
		return nil, true, nil
	}
	if len(rows) != 1 {
		return nil, false, errors.AssertionFailedf("expected one row-marker KV, got %d", len(rows))
	}
	kv := rows[0]
	prefixLen, err := keys.GetRowPrefixLength(kv.Key)
	if err != nil {
		return nil, false, err
	}
	if prefixLen <= 0 || prefixLen >= len(kv.Key) {
		return nil, false, errors.AssertionFailedf("invalid row-marker key %q", kv.Key)
	}
	rowKey = roachpb.Key(keys.MakeFamilyKey(append([]byte(nil), kv.Key[:prefixLen]...), 0))

	if f.reverse {
		if rowKey.Compare(span.Key) <= 0 {
			f.spans[0] = roachpb.Span{}
			f.spans = f.spans[1:]
		} else {
			f.spans[0] = roachpb.Span{Key: span.Key, EndKey: append(roachpb.Key(nil), rowKey...)}
		}
	} else {
		nextKey := append(roachpb.Key(nil), rowKey...)
		nextKey = nextKey.PrefixEnd()
		if nextKey.Compare(span.EndKey) >= 0 {
			f.spans[0] = roachpb.Span{}
			f.spans = f.spans[1:]
		} else {
			f.spans[0] = roachpb.Span{Key: nextKey, EndKey: span.EndKey}
		}
	}
	return rowKey, false, nil
}

func (f *subordinateJSONRowHeadBatchFetcher) fetchRowHeadKV(
	ctx context.Context, rowKey roachpb.Key,
) (roachpb.KeyValue, error) {
	var ba roachpb.BatchRequest
	ba.Header.WaitPolicy = f.lockWaitPolicy
	ba.Header.LockTimeout = f.lockTimeout

	var req roachpb.GetRequest
	req.Key = rowKey
	req.KeyLocking = f.lockStrength
	union := roachpb.RequestUnion{}
	union.MustSetInner(&req)
	ba.Requests = []roachpb.RequestUnion{union}

	br, err := f.sendFn(ctx, ba)
	if err != nil {
		return roachpb.KeyValue{}, err
	}
	resp := br.Responses[0].GetInner().(*roachpb.GetResponse)
	if resp.Value == nil {
		return roachpb.KeyValue{}, errors.AssertionFailedf("missing row-head value for key %q", rowKey)
	}
	return roachpb.KeyValue{Key: append(roachpb.Key(nil), rowKey...), Value: *resp.Value}, nil
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
		if len(lookup.ExistsKeys) > 0 {
			builder.addExistsKeys(lookup.ColID, lookup.ExistsKeys)
		}
	}
	spans := builder.finish()
	if len(spans) == 0 {
		return nil, nil
	}
	kvs, err := f.fetchLookupSpans(ctx, spans)
	if err != nil {
		return nil, err
	}
	extraSpans := f.buildExistsArrayElementSpans(rowKey, kvs)
	if len(extraSpans) == 0 {
		return kvs, nil
	}
	extraKVs, err := f.fetchLookupSpans(ctx, extraSpans)
	if err != nil {
		return nil, err
	}
	kvs = append(kvs, extraKVs...)
	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].Key.Compare(kvs[j].Key) < 0
	})
	return kvs, nil
}

func (f *subordinateJSONRowHeadBatchFetcher) buildExistsArrayElementSpans(
	rowKey roachpb.Key, kvs []roachpb.KeyValue,
) roachpb.Spans {
	var spans roachpb.Spans
	for i := range f.lookups {
		lookup := &f.lookups[i]
		if len(lookup.ExistsKeys) == 0 {
			continue
		}
		nodeKind, childCount, ok, err := lookupRootSubordinateJSONNode(kvs, rowKey, lookup.ColID)
		if err != nil || !ok {
			continue
		}
		if nodeKind != rowenc.SubordinateJSONArray || childCount <= 0 {
			continue
		}
		for elemIdx := 0; elemIdx < childCount; elemIdx++ {
			key := roachpb.Key(keys.MakeSubordinatePathKey(
				rowKey,
				uint32(lookup.ColID),
				[]keys.SubordinatePathSegment{
					{Kind: keys.SubordinatePathHeader},
					{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: uint32(elemIdx)},
				},
			))
			spans = append(spans, roachpb.Span{Key: key, EndKey: key.Next()})
		}
	}
	return spans
}

func lookupRootSubordinateJSONNode(
	kvs []roachpb.KeyValue, rowKey roachpb.Key, colID descpb.ColumnID,
) (nodeKind rowenc.SubordinateJSONNodeKind, childCount int, ok bool, err error) {
	rootKey := roachpb.Key(keys.MakeSubordinatePathKey(
		rowKey, uint32(colID), []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}},
	))
	for i := range kvs {
		if !kvs[i].Key.Equal(rootKey) {
			continue
		}
		nodeKind, childCount, _, err = rowenc.PeekSubordinateJSONValueMetadata(kvs[i].Value)
		if err != nil {
			return 0, 0, false, err
		}
		return nodeKind, childCount, true, nil
	}
	return 0, 0, false, nil
}

func (f *subordinateJSONRowHeadBatchFetcher) fetchLookupSpans(
	ctx context.Context, spans roachpb.Spans,
) ([]roachpb.KeyValue, error) {
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
