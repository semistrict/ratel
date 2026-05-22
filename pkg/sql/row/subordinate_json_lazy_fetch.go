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

	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/rowenc"
	jsonutil "github.com/cockroachdb/cockroach/pkg/util/json"
	"github.com/cockroachdb/errors"
)

// TestingSubordinateJSONLazyRootDecodeHook, when non-nil, is invoked whenever a
// lazy root subordinate JSON array falls back to decoding the full array value.
var TestingSubordinateJSONLazyRootDecodeHook func()

// TestingSubordinateJSONLazyRootIndexFetchHook, when non-nil, is invoked
// whenever a lazy root subordinate JSON array falls back to per-index lookup.
var TestingSubordinateJSONLazyRootIndexFetchHook func()

// TestingSubordinateJSONLazyDirectChildMissHook, when non-nil, is invoked
// whenever a lazy subordinate object/array cannot satisfy a direct-child lookup
// from its on-demand cache and must fall back to a KV path lookup.
var TestingSubordinateJSONLazyDirectChildMissHook func()

const subordinateJSONArrayIteratorPageSize = 128

func appendSubordinatePath(path []keys.SubordinatePathSegment, seg keys.SubordinatePathSegment) []keys.SubordinatePathSegment {
	out := make([]keys.SubordinatePathSegment, len(path)+1)
	copy(out, path)
	out[len(path)] = seg
	return out
}

func subordinatePathKey(rowKey roachpb.Key, colID descpb.ColumnID, path []keys.SubordinatePathSegment) roachpb.Key {
	return roachpb.Key(keys.MakeSubordinatePathKey(rowKey, uint32(colID), path))
}

func makeLazySubordinateJSONNode(
	txn *kv.Txn,
	rowKey roachpb.Key,
	colID descpb.ColumnID,
	path []keys.SubordinatePathSegment,
	kind rowenc.SubordinateJSONNodeKind,
	childCount int,
	scalarRaw []byte,
) (jsonutil.JSON, error) {
	return makeLazySubordinateJSONNodeWithCachedDirectChildren(
		txn, rowKey, colID, path, kind, childCount, scalarRaw, nil /* directObjectChildren */, nil, /* directArrayChildren */
	)
}

func makeLazySubordinateJSONNodeWithCachedDirectChildren(
	txn *kv.Txn,
	rowKey roachpb.Key,
	colID descpb.ColumnID,
	path []keys.SubordinatePathSegment,
	kind rowenc.SubordinateJSONNodeKind,
	childCount int,
	scalarRaw []byte,
	directObjectChildren map[string]jsonutil.JSON,
	directArrayChildren map[int]jsonutil.JSON,
) (jsonutil.JSON, error) {
	switch kind {
	case rowenc.SubordinateJSONScalar:
		return rowenc.DecodeSubordinateJSONScalarBytes(scalarRaw)
	case rowenc.SubordinateJSONObject:
		rowKeyCopy := append(roachpb.Key(nil), rowKey...)
		pathCopy := append([]keys.SubordinatePathSegment(nil), path...)
		return jsonutil.NewLazyObject(
			childCount,
			func(ctx context.Context, key string) (jsonutil.JSON, error) {
				if directObjectChildren != nil {
					if child, ok := directObjectChildren[key]; ok {
						return child, nil
					}
				}
				if TestingSubordinateJSONLazyDirectChildMissHook != nil {
					TestingSubordinateJSONLazyDirectChildMissHook()
				}
				return fetchSubordinateJSONNodeAtPath(ctx, txn, rowKeyCopy, colID, appendSubordinatePath(pathCopy, keys.SubordinatePathSegment{
					Kind:      keys.SubordinatePathObjectKey,
					ObjectKey: key,
				}))
			},
			func(ctx context.Context) (jsonutil.JSON, error) {
				return fetchSubordinateJSONValueAtPath(ctx, txn, rowKeyCopy, colID, pathCopy)
			},
		), nil
	case rowenc.SubordinateJSONArray:
		rowKeyCopy := append(roachpb.Key(nil), rowKey...)
		pathCopy := append([]keys.SubordinatePathSegment(nil), path...)
		return jsonutil.NewLazyArray(
			childCount,
			func(ctx context.Context, idx int) (jsonutil.JSON, error) {
				if directArrayChildren != nil {
					if child, ok := directArrayChildren[idx]; ok {
						return child, nil
					}
				}
				if TestingSubordinateJSONLazyDirectChildMissHook != nil {
					TestingSubordinateJSONLazyDirectChildMissHook()
				}
				return fetchSubordinateJSONNodeAtPath(ctx, txn, rowKeyCopy, colID, appendSubordinatePath(pathCopy, keys.SubordinatePathSegment{
					Kind:     keys.SubordinatePathArrayIndex,
					ArrayIdx: uint32(idx),
				}))
			},
			func(ctx context.Context) (jsonutil.JSON, error) {
				return fetchSubordinateJSONValueAtPath(ctx, txn, rowKeyCopy, colID, pathCopy)
			},
		), nil
	default:
		return nil, errors.AssertionFailedf("unknown subordinate JSON node kind %d", kind)
	}
}

func fetchSubordinateJSONNodeAtPath(
	ctx context.Context,
	txn *kv.Txn,
	rowKey roachpb.Key,
	colID descpb.ColumnID,
	path []keys.SubordinatePathSegment,
) (jsonutil.JSON, error) {
	kvValue, err := txn.Get(ctx, subordinatePathKey(rowKey, colID, path))
	if err != nil {
		return nil, err
	}
	if !kvValue.Exists() {
		return nil, nil
	}
	kind, childCount, scalarRaw, err := rowenc.PeekSubordinateJSONValueMetadata(*kvValue.Value)
	if err != nil {
		return nil, err
	}
	return makeLazySubordinateJSONNode(txn, rowKey, colID, path, kind, childCount, scalarRaw)
}

func fetchSubordinateJSONValueAtPath(
	ctx context.Context,
	txn *kv.Txn,
	rowKey roachpb.Key,
	colID descpb.ColumnID,
	path []keys.SubordinatePathSegment,
) (jsonutil.JSON, error) {
	prefix := roachpb.Key(keys.MakeSubordinatePathPrefix(rowKey, uint32(colID), path))
	kvs, err := txn.Scan(ctx, prefix, prefix.PrefixEnd(), 0)
	if err != nil {
		return nil, err
	}
	if len(kvs) == 0 {
		return nil, nil
	}

	var builder SubordinateJSONBuilder
	for _, kv := range kvs {
		_, _, fullPath, err := keys.DecodeSubordinatePathKey(kv.Key)
		if err != nil {
			return nil, err
		}
		if len(fullPath) < len(path) {
			return nil, errors.AssertionFailedf("subordinate JSON path %v shorter than prefix %v", fullPath, path)
		}
		relPath := append([]keys.SubordinatePathSegment(nil), fullPath[len(path):]...)
		kind, scalar, err := rowenc.DecodeSubordinateJSONValue(*kv.Value)
		if err != nil {
			return nil, err
		}
		builderKind, err := SubordinateJSONNodeKindFromEncoded(kind)
		if err != nil {
			return nil, err
		}
		if err := builder.Set(relPath, builderKind, scalar); err != nil {
			return nil, err
		}
	}
	d, err := builder.Materialize()
	if err != nil {
		return nil, err
	}
	return d.JSON, nil
}

// MakeLazyRootJSONArray constructs a JSON array datum that keeps the root array
// lazy for direct array-element expansion, while still supporting full JSON
// fallback operations on demand.
func MakeLazyRootJSONArray(
	txn *kv.Txn, rowKey roachpb.Key, colID descpb.ColumnID, length int,
) jsonutil.JSON {
	rowKeyCopy := append(roachpb.Key(nil), rowKey...)
	return jsonutil.NewLazyArrayWithIterator(
		length,
		func(ctx context.Context, idx int) (jsonutil.JSON, error) {
			if TestingSubordinateJSONLazyRootIndexFetchHook != nil {
				TestingSubordinateJSONLazyRootIndexFetchHook()
			}
			return fetchSubordinateJSONNodeAtPath(ctx, txn, rowKeyCopy, colID, []keys.SubordinatePathSegment{
				{Kind: keys.SubordinatePathHeader},
				{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: uint32(idx)},
			})
		},
		func(ctx context.Context) (jsonutil.JSON, error) {
			if TestingSubordinateJSONLazyRootDecodeHook != nil {
				TestingSubordinateJSONLazyRootDecodeHook()
			}
			return fetchSubordinateJSONValueAtPath(ctx, txn, rowKeyCopy, colID, nil)
		},
		func() jsonutil.ArrayValueIterator {
			return &subordinateJSONArrayIterator{
				txn:    txn,
				rowKey: append(roachpb.Key(nil), rowKeyCopy...),
				colID:  colID,
			}
		},
	)
}

type subordinateJSONArrayIterator struct {
	txn    *kv.Txn
	rowKey roachpb.Key
	colID  descpb.ColumnID

	begin roachpb.Key
	end   roachpb.Key
	page  []kv.KeyValue
	idx   int
	done  bool

	currentPath       []keys.SubordinatePathSegment
	currentKind       rowenc.SubordinateJSONNodeKind
	currentChildCount int
	currentScalarRaw  []byte
	currentObjKids    map[string]jsonutil.JSON
	currentArrKids    map[int]jsonutil.JSON
	node              *jsonutil.MutableLazyNode

	pendingPath       []keys.SubordinatePathSegment
	pendingKind       rowenc.SubordinateJSONNodeKind
	pendingChildCount int
	pendingScalarRaw  []byte
}

func (it *subordinateJSONArrayIterator) initRange() {
	if it.begin != nil {
		return
	}
	prefix := roachpb.Key(keys.MakeSubordinatePathPrefix(it.rowKey, uint32(it.colID), []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}))
	it.begin = prefix
	it.end = prefix.PrefixEnd()
}

func (it *subordinateJSONArrayIterator) NextValue(ctx context.Context) (jsonutil.JSON, bool, error) {
	it.initRange()
	if it.node == nil {
		it.node = jsonutil.NewMutableLazyNode(it)
	}
	if len(it.pendingPath) != 0 {
		it.currentPath = append(it.currentPath[:0], it.pendingPath...)
		it.currentKind = it.pendingKind
		it.currentChildCount = it.pendingChildCount
		it.currentScalarRaw = append(it.currentScalarRaw[:0], it.pendingScalarRaw...)
		it.currentObjKids = nil
		it.currentArrKids = nil
		it.clearPending()
		return it.finishCurrentNode(), true, nil
	}
	for {
		kvValue, ok, err := it.nextKV(ctx)
		if err != nil {
			return nil, false, err
		}
		if !ok {
			return nil, false, nil
		}

		_, _, path, err := keys.DecodeSubordinatePathKey(kvValue.Key)
		if err != nil {
			return nil, false, err
		}
		if !isRootArrayElementHeaderPath(path) {
			continue
		}
		kind, childCount, scalarRaw, err := rowenc.PeekSubordinateJSONValueMetadata(*kvValue.Value)
		if err != nil {
			return nil, false, err
		}
		it.setCurrentHeader(path, kind, childCount, scalarRaw)
		return it.finishCurrentNode(), true, nil
	}
}

func (it *subordinateJSONArrayIterator) Close(context.Context) {}

func (it *subordinateJSONArrayIterator) finishCurrentNode() jsonutil.JSON {
	switch it.currentKind {
	case rowenc.SubordinateJSONScalar:
		scalar, err := rowenc.DecodeSubordinateJSONScalarBytes(it.currentScalarRaw)
		if err != nil {
			panic(err)
		}
		it.node.ResetScalar(scalar)
	case rowenc.SubordinateJSONObject:
		it.node.ResetObject(it.currentChildCount)
	case rowenc.SubordinateJSONArray:
		it.node.ResetArray(it.currentChildCount)
	default:
		panic(errors.AssertionFailedf("unknown subordinate JSON node kind %d", it.currentKind))
	}
	return it.node
}

func (it *subordinateJSONArrayIterator) FetchValKeyContext(
	ctx context.Context, key string,
) (jsonutil.JSON, error) {
	if it.currentKind != rowenc.SubordinateJSONObject {
		return nil, nil
	}
	if it.currentObjKids != nil {
		if child, ok := it.currentObjKids[key]; ok {
			return child, nil
		}
	}
	if TestingSubordinateJSONLazyDirectChildMissHook != nil {
		TestingSubordinateJSONLazyDirectChildMissHook()
	}
	child, err := it.findCurrentObjectChild(ctx, key)
	if err != nil || child == nil {
		child, err = fetchSubordinateJSONNodeAtPath(ctx, it.txn, it.rowKey, it.colID, appendSubordinatePath(it.currentPath, keys.SubordinatePathSegment{
			Kind:      keys.SubordinatePathObjectKey,
			ObjectKey: key,
		}))
		if err != nil || child == nil {
			return child, err
		}
	}
	if it.currentObjKids == nil {
		it.currentObjKids = make(map[string]jsonutil.JSON)
	}
	it.currentObjKids[key] = child
	return child, nil
}

func (it *subordinateJSONArrayIterator) FetchValIdxContext(
	ctx context.Context, idx int,
) (jsonutil.JSON, error) {
	if it.currentKind != rowenc.SubordinateJSONArray {
		return nil, nil
	}
	if it.currentArrKids != nil {
		if child, ok := it.currentArrKids[idx]; ok {
			return child, nil
		}
	}
	if TestingSubordinateJSONLazyDirectChildMissHook != nil {
		TestingSubordinateJSONLazyDirectChildMissHook()
	}
	child, err := it.findCurrentArrayChild(ctx, idx)
	if err != nil || child == nil {
		child, err = fetchSubordinateJSONNodeAtPath(ctx, it.txn, it.rowKey, it.colID, appendSubordinatePath(it.currentPath, keys.SubordinatePathSegment{
			Kind:     keys.SubordinatePathArrayIndex,
			ArrayIdx: uint32(idx),
		}))
		if err != nil || child == nil {
			return child, err
		}
	}
	if it.currentArrKids == nil {
		it.currentArrKids = make(map[int]jsonutil.JSON)
	}
	it.currentArrKids[idx] = child
	return child, nil
}

func (it *subordinateJSONArrayIterator) DecodeContext(ctx context.Context) (jsonutil.JSON, error) {
	switch it.currentKind {
	case rowenc.SubordinateJSONScalar:
		return rowenc.DecodeSubordinateJSONScalarBytes(it.currentScalarRaw)
	case rowenc.SubordinateJSONObject, rowenc.SubordinateJSONArray:
		return fetchSubordinateJSONValueAtPath(ctx, it.txn, it.rowKey, it.colID, it.currentPath)
	default:
		return nil, errors.AssertionFailedf("unknown subordinate JSON node kind %d", it.currentKind)
	}
}

func (it *subordinateJSONArrayIterator) nextKV(ctx context.Context) (kv.KeyValue, bool, error) {
	for it.idx >= len(it.page) {
		if it.done {
			return kv.KeyValue{}, false, nil
		}
		rows, err := it.txn.Scan(ctx, it.begin, it.end, subordinateJSONArrayIteratorPageSize)
		if err != nil {
			return kv.KeyValue{}, false, err
		}
		if len(rows) == 0 {
			it.done = true
			return kv.KeyValue{}, false, nil
		}
		it.page = rows
		it.idx = 0
		it.begin = rows[len(rows)-1].Key.Next()
		if len(rows) < subordinateJSONArrayIteratorPageSize {
			it.done = true
		}
	}
	kvValue := it.page[it.idx]
	it.idx++
	return kvValue, true, nil
}

func (it *subordinateJSONArrayIterator) setCurrentHeader(
	path []keys.SubordinatePathSegment,
	kind rowenc.SubordinateJSONNodeKind,
	childCount int,
	scalarRaw []byte,
) {
	it.currentPath = append(it.currentPath[:0], path...)
	it.currentKind = kind
	it.currentChildCount = childCount
	it.currentScalarRaw = append(it.currentScalarRaw[:0], scalarRaw...)
	it.currentObjKids = nil
	it.currentArrKids = nil
}

func (it *subordinateJSONArrayIterator) setPendingHeader(
	path []keys.SubordinatePathSegment,
	kind rowenc.SubordinateJSONNodeKind,
	childCount int,
	scalarRaw []byte,
) {
	it.pendingPath = append(it.pendingPath[:0], path...)
	it.pendingKind = kind
	it.pendingChildCount = childCount
	it.pendingScalarRaw = append(it.pendingScalarRaw[:0], scalarRaw...)
}

func (it *subordinateJSONArrayIterator) clearPending() {
	it.pendingPath = it.pendingPath[:0]
	it.pendingScalarRaw = it.pendingScalarRaw[:0]
	it.pendingKind = 0
	it.pendingChildCount = 0
}

func isRootArrayElementHeaderPath(path []keys.SubordinatePathSegment) bool {
	return len(path) == 2 && path[0].Kind == keys.SubordinatePathHeader && path[1].Kind == keys.SubordinatePathArrayIndex
}

func (it *subordinateJSONArrayIterator) findCurrentObjectChild(
	ctx context.Context, key string,
) (jsonutil.JSON, error) {
	if len(it.pendingPath) != 0 {
		return nil, nil
	}
	for {
		kvValue, ok, err := it.nextKV(ctx)
		if err != nil || !ok {
			return nil, err
		}
		_, _, path, err := keys.DecodeSubordinatePathKey(kvValue.Key)
		if err != nil {
			return nil, err
		}
		if isRootArrayElementHeaderPath(path) {
			kind, childCount, scalarRaw, err := rowenc.PeekSubordinateJSONValueMetadata(*kvValue.Value)
			if err != nil {
				return nil, err
			}
			it.setPendingHeader(path, kind, childCount, scalarRaw)
			return nil, nil
		}
		if len(path) != len(it.currentPath)+1 {
			continue
		}
		last := path[len(path)-1]
		if last.Kind != keys.SubordinatePathObjectKey || last.ObjectKey != key {
			continue
		}
		kind, childCount, scalarRaw, err := rowenc.PeekSubordinateJSONValueMetadata(*kvValue.Value)
		if err != nil {
			return nil, err
		}
		return makeLazySubordinateJSONNode(it.txn, it.rowKey, it.colID, path, kind, childCount, scalarRaw)
	}
}

func (it *subordinateJSONArrayIterator) findCurrentArrayChild(
	ctx context.Context, idx int,
) (jsonutil.JSON, error) {
	if len(it.pendingPath) != 0 {
		return nil, nil
	}
	for {
		kvValue, ok, err := it.nextKV(ctx)
		if err != nil || !ok {
			return nil, err
		}
		_, _, path, err := keys.DecodeSubordinatePathKey(kvValue.Key)
		if err != nil {
			return nil, err
		}
		if isRootArrayElementHeaderPath(path) {
			kind, childCount, scalarRaw, err := rowenc.PeekSubordinateJSONValueMetadata(*kvValue.Value)
			if err != nil {
				return nil, err
			}
			it.setPendingHeader(path, kind, childCount, scalarRaw)
			return nil, nil
		}
		if len(path) != len(it.currentPath)+1 {
			continue
		}
		last := path[len(path)-1]
		if last.Kind != keys.SubordinatePathArrayIndex || int(last.ArrayIdx) != idx {
			continue
		}
		kind, childCount, scalarRaw, err := rowenc.PeekSubordinateJSONValueMetadata(*kvValue.Value)
		if err != nil {
			return nil, err
		}
		return makeLazySubordinateJSONNode(it.txn, it.rowKey, it.colID, path, kind, childCount, scalarRaw)
	}
}
