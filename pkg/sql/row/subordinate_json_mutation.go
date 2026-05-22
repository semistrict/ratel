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
	"strconv"

	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgcode"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgerror"
	"github.com/cockroachdb/cockroach/pkg/sql/rowenc"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	jsonutil "github.com/cockroachdb/cockroach/pkg/util/json"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/errors"
)

type SubordinateJSONMutationKind uint8

const (
	SubordinateJSONMutationConcat SubordinateJSONMutationKind = iota + 1
	SubordinateJSONMutationDeleteKey
	SubordinateJSONMutationDeleteLastArrayElement
	SubordinateJSONMutationSetPath
)

type SubordinateJSONMutationOp struct {
	ColID         descpb.ColumnID
	Kind          SubordinateJSONMutationKind
	Key           string
	Path          []string
	Value         jsonutil.JSON
	CreateMissing bool
}

var errSubordinateJSONMutationFallback = errors.New("subordinate JSON mutation requires generic fallback")

// TestingSubordinateJSONMutationResult describes how a direct subordinate JSON
// update executed. Tests can use it to assert that SQL mutations stayed on the
// local delta path instead of falling back to full-value rewrite.
type TestingSubordinateJSONMutationResult struct {
	TableID                  descpb.ID
	MutationKind             SubordinateJSONMutationKind
	ColID                    descpb.ColumnID
	LocalApplied             bool
	FellBackToGenericRewrite bool
	ApproximateMutationBytes int64
}

// TestingSubordinateJSONMutationApplied, when non-nil, is invoked after
// UpdateSubordinateJSONRow decides whether the local mutation path was used.
var TestingSubordinateJSONMutationApplied func(TestingSubordinateJSONMutationResult)

func (ru *Updater) UpdateSubordinateJSONRow(
	ctx context.Context,
	txn *kv.Txn,
	batch *kv.Batch,
	oldValues []tree.Datum,
	mutation SubordinateJSONMutationOp,
	traceKV bool,
) error {
	if len(oldValues) != len(ru.FetchCols) {
		return errors.AssertionFailedf(
			"got %d values but expected %d", len(oldValues), len(ru.FetchCols),
		)
	}
	if ru.primaryKeyColChange || ru.DeleteHelper != nil || len(ru.Helper.Indexes) != 0 {
		return errors.AssertionFailedf("direct subordinate JSON update requires primary-index-only writes")
	}

	colIdx, ok := ru.FetchColIDtoRowIndex.Get(mutation.ColID)
	if !ok || ru.FetchCols[colIdx].GetType().Family() != types.JsonFamily {
		return errors.AssertionFailedf("direct subordinate JSON update requires fetched JSON column %d", mutation.ColID)
	}

	primaryIndexKey, err := ru.Helper.encodePrimaryIndex(ru.FetchColIDtoRowIndex, oldValues)
	if err != nil {
		return err
	}
	rowKey := keys.MakeFamilyKey(primaryIndexKey, 0)
	columnPrefix := roachpb.Key(keys.MakeSubordinatePathPrefix(rowKey, uint32(mutation.ColID), nil))
	headerPath := []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}
	headerKey := roachpb.Key(keys.MakeSubordinatePathKey(rowKey, uint32(mutation.ColID), headerPath))

	localApplied, err := ru.tryApplyLocalSubordinateJSONMutation(
		ctx, txn, batch, rowKey, headerKey, mutation, traceKV,
	)
	if err != nil {
		return err
	}
	if localApplied {
		if TestingSubordinateJSONMutationApplied != nil {
			TestingSubordinateJSONMutationApplied(TestingSubordinateJSONMutationResult{
				TableID:                  ru.Helper.TableDesc.GetID(),
				MutationKind:             mutation.Kind,
				ColID:                    mutation.ColID,
				LocalApplied:             true,
				ApproximateMutationBytes: int64(batch.ApproximateMutationBytes()),
			})
		}
		return nil
	}

	current, err := ru.readSubordinateJSONValue(ctx, txn, columnPrefix)
	if err != nil {
		return err
	}
	next, err := applyGenericSubordinateJSONMutation(current, mutation)
	if err != nil {
		return err
	}
	ru.deleteSubordinatePrefix(ctx, batch, columnPrefix, traceKV)
	var entries []rowenc.IndexEntry
	entries, err = rowenc.EncodeJSONSubordinateEntriesAtPath(entries, rowKey, uint32(mutation.ColID), headerPath, next)
	if err != nil {
		return err
	}
	for i := range entries {
		if traceKV {
			log.VEventf(ctx, 2, "Put %s -> %v", entries[i].Key, entries[i].Value.PrettyPrint())
		}
		batch.Put(entries[i].Key, &entries[i].Value)
	}
	if TestingSubordinateJSONMutationApplied != nil {
		TestingSubordinateJSONMutationApplied(TestingSubordinateJSONMutationResult{
			TableID:                  ru.Helper.TableDesc.GetID(),
			MutationKind:             mutation.Kind,
			ColID:                    mutation.ColID,
			LocalApplied:             false,
			FellBackToGenericRewrite: true,
			ApproximateMutationBytes: int64(batch.ApproximateMutationBytes()),
		})
	}
	return nil
}

func (ru *Updater) ClearSubordinateJSONColumn(
	ctx context.Context, batch *kv.Batch, oldValues []tree.Datum, colID descpb.ColumnID, traceKV bool,
) error {
	if len(oldValues) != len(ru.FetchCols) {
		return errors.AssertionFailedf(
			"got %d values but expected %d", len(oldValues), len(ru.FetchCols),
		)
	}
	colIdx, ok := ru.FetchColIDtoRowIndex.Get(colID)
	if !ok || ru.FetchCols[colIdx].GetType().Family() != types.JsonFamily {
		return errors.AssertionFailedf("direct subordinate JSON clear requires fetched JSON column %d", colID)
	}
	primaryIndexKey, err := ru.Helper.encodePrimaryIndex(ru.FetchColIDtoRowIndex, oldValues)
	if err != nil {
		return err
	}
	rowKey := keys.MakeFamilyKey(primaryIndexKey, 0)
	columnPrefix := roachpb.Key(keys.MakeSubordinatePathPrefix(rowKey, uint32(colID), nil))
	ru.deleteSubordinatePrefix(ctx, batch, columnPrefix, traceKV)
	return nil
}

func applyGenericSubordinateJSONMutation(
	current jsonutil.JSON, mutation SubordinateJSONMutationOp,
) (jsonutil.JSON, error) {
	switch mutation.Kind {
	case SubordinateJSONMutationConcat:
		return current.Concat(mutation.Value)
	case SubordinateJSONMutationDeleteKey:
		next, _, err := current.RemoveString(mutation.Key)
		return next, err
	case SubordinateJSONMutationDeleteLastArrayElement:
		switch current.Type() {
		case jsonutil.ArrayJSONType:
			next, _, err := current.RemoveIndex(current.Len() - 1)
			return next, err
		case jsonutil.ObjectJSONType:
			return nil, pgerror.New(pgcode.InvalidParameterValue, "cannot get array length of a non-array")
		default:
			return nil, pgerror.New(pgcode.InvalidParameterValue, "cannot get array length of a scalar")
		}
	case SubordinateJSONMutationSetPath:
		return jsonutil.DeepSet(current, mutation.Path, mutation.Value, mutation.CreateMissing)
	default:
		return nil, errors.AssertionFailedf("unknown subordinate JSON mutation kind %d", mutation.Kind)
	}
}

func (ru *Updater) tryApplyLocalSubordinateJSONMutation(
	ctx context.Context,
	txn *kv.Txn,
	batch *kv.Batch,
	rowKey roachpb.Key,
	headerKey roachpb.Key,
	mutation SubordinateJSONMutationOp,
	traceKV bool,
) (bool, error) {
	headerKV, err := txn.Get(ctx, headerKey)
	if err != nil {
		return false, err
	}
	if headerKV.Value == nil {
		return false, nil
	}
	rootKind, rootCount, _, err := rowenc.DecodeSubordinateJSONValueWithCardinality(*headerKV.Value)
	if err != nil {
		return false, err
	}

	switch mutation.Kind {
	case SubordinateJSONMutationConcat:
		switch {
		case rootKind == rowenc.SubordinateJSONObject && mutation.Value.Type() == jsonutil.ObjectJSONType:
			return true, ru.applyLocalRootObjectConcat(ctx, txn, batch, rowKey, mutation.ColID, rootCount, mutation.Value, traceKV)
		case rootKind == rowenc.SubordinateJSONArray && mutation.Value.Type() == jsonutil.ArrayJSONType:
			return true, ru.applyLocalRootArrayAppend(ctx, batch, rowKey, mutation.ColID, rootCount, mutation.Value, traceKV)
		default:
			return false, nil
		}
	case SubordinateJSONMutationDeleteKey:
		if rootKind != rowenc.SubordinateJSONObject {
			return false, nil
		}
		return true, ru.applyLocalRootObjectDeleteKey(ctx, txn, batch, rowKey, mutation.ColID, rootCount, mutation.Key, traceKV)
	case SubordinateJSONMutationDeleteLastArrayElement:
		if rootKind != rowenc.SubordinateJSONArray {
			return false, nil
		}
		return true, ru.applyLocalRootArrayDeleteLast(ctx, batch, rowKey, mutation.ColID, rootCount, traceKV)
	case SubordinateJSONMutationSetPath:
		switch {
		case rootKind == rowenc.SubordinateJSONObject && len(mutation.Path) == 1:
			return true, ru.applyLocalRootObjectSetKey(
				ctx, txn, batch, rowKey, mutation.ColID, rootCount, mutation.Path[0], mutation.Value, mutation.CreateMissing, traceKV,
			)
		case rootKind == rowenc.SubordinateJSONArray && len(mutation.Path) == 2:
			idx, err := strconv.Atoi(mutation.Path[0])
			if err != nil || idx < 0 {
				return false, nil
			}
			err = ru.applyLocalRootArrayObjectKeySet(
				ctx, txn, batch, rowKey, mutation.ColID, rootCount, idx, mutation.Path[1], mutation.Value, mutation.CreateMissing, traceKV,
			)
			if errors.Is(err, errSubordinateJSONMutationFallback) {
				return false, nil
			}
			return true, err
		default:
			return false, nil
		}
	default:
		return false, errors.AssertionFailedf("unknown subordinate JSON mutation kind %d", mutation.Kind)
	}
}

func (ru *Updater) applyLocalRootObjectConcat(
	ctx context.Context,
	txn *kv.Txn,
	batch *kv.Batch,
	rowKey roachpb.Key,
	colID descpb.ColumnID,
	rootCount int,
	right jsonutil.JSON,
	traceKV bool,
) error {
	iter, err := right.ObjectIter()
	if err != nil {
		return err
	}
	newCount := rootCount
	for iter.Next() {
		key := iter.Key()
		childPath := []keys.SubordinatePathSegment{
			{Kind: keys.SubordinatePathHeader},
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: key},
		}
		childKey := roachpb.Key(keys.MakeSubordinatePathKey(rowKey, uint32(colID), childPath))
		childKV, err := txn.Get(ctx, childKey)
		if err != nil {
			return err
		}
		if childKV.Value != nil {
			ru.deleteSubordinateExactSubtree(ctx, batch, childKey, traceKV)
		} else {
			newCount++
		}
		if err := ru.putSubordinateJSONSubtree(ctx, batch, rowKey, colID, childPath, iter.Value(), traceKV); err != nil {
			return err
		}
	}
	if newCount != rootCount {
		return ru.putSubordinateJSONHeader(ctx, batch, rowKey, colID, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, rowenc.SubordinateJSONObject, newCount, traceKV)
	}
	return nil
}

func (ru *Updater) applyLocalRootObjectDeleteKey(
	ctx context.Context,
	txn *kv.Txn,
	batch *kv.Batch,
	rowKey roachpb.Key,
	colID descpb.ColumnID,
	rootCount int,
	key string,
	traceKV bool,
) error {
	childPath := []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathHeader},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: key},
	}
	childKey := roachpb.Key(keys.MakeSubordinatePathKey(rowKey, uint32(colID), childPath))
	childKV, err := txn.Get(ctx, childKey)
	if err != nil {
		return err
	}
	if childKV.Value == nil {
		return nil
	}
	ru.deleteSubordinateExactSubtree(ctx, batch, childKey, traceKV)
	return ru.putSubordinateJSONHeader(ctx, batch, rowKey, colID, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, rowenc.SubordinateJSONObject, rootCount-1, traceKV)
}

func (ru *Updater) applyLocalRootObjectSetKey(
	ctx context.Context,
	txn *kv.Txn,
	batch *kv.Batch,
	rowKey roachpb.Key,
	colID descpb.ColumnID,
	rootCount int,
	key string,
	value jsonutil.JSON,
	createMissing bool,
	traceKV bool,
) error {
	childPath := []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathHeader},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: key},
	}
	childKey := roachpb.Key(keys.MakeSubordinatePathKey(rowKey, uint32(colID), childPath))
	childKV, err := txn.Get(ctx, childKey)
	if err != nil {
		return err
	}
	exists := childKV.Value != nil
	if !exists && !createMissing {
		return nil
	}
	newCount := rootCount
	if exists {
		ru.deleteSubordinateExactSubtree(ctx, batch, childKey, traceKV)
	} else {
		newCount++
	}
	if err := ru.putSubordinateJSONSubtree(ctx, batch, rowKey, colID, childPath, value, traceKV); err != nil {
		return err
	}
	if newCount != rootCount {
		return ru.putSubordinateJSONHeader(ctx, batch, rowKey, colID, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, rowenc.SubordinateJSONObject, newCount, traceKV)
	}
	return nil
}

func (ru *Updater) applyLocalRootArrayAppend(
	ctx context.Context,
	batch *kv.Batch,
	rowKey roachpb.Key,
	colID descpb.ColumnID,
	rootCount int,
	right jsonutil.JSON,
	traceKV bool,
) error {
	for i := 0; i < right.Len(); i++ {
		child, err := right.FetchValIdx(i)
		if err != nil {
			return err
		}
		if child == nil {
			return errors.AssertionFailedf("missing JSON array element %d", i)
		}
		path := []keys.SubordinatePathSegment{
			{Kind: keys.SubordinatePathHeader},
			{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: uint32(rootCount + i)},
		}
		if err := ru.putSubordinateJSONSubtree(ctx, batch, rowKey, colID, path, child, traceKV); err != nil {
			return err
		}
	}
	return ru.putSubordinateJSONHeader(ctx, batch, rowKey, colID, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, rowenc.SubordinateJSONArray, rootCount+right.Len(), traceKV)
}

func (ru *Updater) applyLocalRootArrayDeleteLast(
	ctx context.Context,
	batch *kv.Batch,
	rowKey roachpb.Key,
	colID descpb.ColumnID,
	rootCount int,
	traceKV bool,
) error {
	if rootCount == 0 {
		return nil
	}
	childKey := roachpb.Key(keys.MakeSubordinatePathKey(rowKey, uint32(colID), []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathHeader},
		{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: uint32(rootCount - 1)},
	}))
	ru.deleteSubordinateExactSubtree(ctx, batch, childKey, traceKV)
	return ru.putSubordinateJSONHeader(ctx, batch, rowKey, colID, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, rowenc.SubordinateJSONArray, rootCount-1, traceKV)
}

func (ru *Updater) applyLocalRootArrayObjectKeySet(
	ctx context.Context,
	txn *kv.Txn,
	batch *kv.Batch,
	rowKey roachpb.Key,
	colID descpb.ColumnID,
	rootCount int,
	idx int,
	key string,
	value jsonutil.JSON,
	createMissing bool,
	traceKV bool,
) error {
	if idx >= rootCount {
		if !createMissing {
			return nil
		}
		return nil
	}
	elemPath := []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathHeader},
		{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: uint32(idx)},
	}
	elemKey := roachpb.Key(keys.MakeSubordinatePathKey(rowKey, uint32(colID), elemPath))
	elemKV, err := txn.Get(ctx, elemKey)
	if err != nil {
		return err
	}
	if elemKV.Value == nil {
		if !createMissing {
			return nil
		}
		return nil
	}
	elemKind, elemCount, _, err := rowenc.DecodeSubordinateJSONValueWithCardinality(*elemKV.Value)
	if err != nil {
		return err
	}
	if elemKind != rowenc.SubordinateJSONObject {
		return errSubordinateJSONMutationFallback
	}
	childPath := append(append([]keys.SubordinatePathSegment(nil), elemPath...),
		keys.SubordinatePathSegment{Kind: keys.SubordinatePathObjectKey, ObjectKey: key},
	)
	childKey := roachpb.Key(keys.MakeSubordinatePathKey(rowKey, uint32(colID), childPath))
	childKV, err := txn.Get(ctx, childKey)
	if err != nil {
		return err
	}
	exists := childKV.Value != nil
	if !exists && !createMissing {
		return nil
	}
	newCount := elemCount
	if exists {
		ru.deleteSubordinateExactSubtree(ctx, batch, childKey, traceKV)
	} else {
		newCount++
	}
	if err := ru.putSubordinateJSONSubtree(ctx, batch, rowKey, colID, childPath, value, traceKV); err != nil {
		return err
	}
	if newCount != elemCount {
		return ru.putSubordinateJSONHeader(ctx, batch, rowKey, colID, elemPath, rowenc.SubordinateJSONObject, newCount, traceKV)
	}
	return nil
}

func (ru *Updater) readSubordinateJSONValue(
	ctx context.Context, txn *kv.Txn, columnPrefix roachpb.Key,
) (jsonutil.JSON, error) {
	kvs, err := txn.Scan(ctx, columnPrefix, columnPrefix.PrefixEnd(), 0)
	if err != nil {
		return nil, err
	}
	var builder SubordinateJSONBuilder
	for _, kv := range kvs {
		if kv.Value == nil {
			continue
		}
		_, _, path, err := keys.DecodeSubordinatePathKey(kv.Key)
		if err != nil {
			return nil, err
		}
		kind, scalar, err := rowenc.DecodeSubordinateJSONValue(*kv.Value)
		if err != nil {
			return nil, err
		}
		builderKind, err := SubordinateJSONNodeKindFromEncoded(kind)
		if err != nil {
			return nil, err
		}
		if err := builder.Set(path, builderKind, scalar); err != nil {
			return nil, err
		}
	}
	d, err := builder.Materialize()
	if err != nil {
		return nil, err
	}
	return d.JSON, nil
}

func (ru *Updater) putSubordinateJSONSubtree(
	ctx context.Context,
	batch *kv.Batch,
	rowKey roachpb.Key,
	colID descpb.ColumnID,
	path []keys.SubordinatePathSegment,
	j jsonutil.JSON,
	traceKV bool,
) error {
	var entries []rowenc.IndexEntry
	var err error
	entries, err = rowenc.EncodeJSONSubordinateEntriesAtPath(entries, rowKey, uint32(colID), path, j)
	if err != nil {
		return err
	}
	for i := range entries {
		if traceKV {
			log.VEventf(ctx, 2, "Put %s -> %v", entries[i].Key, entries[i].Value.PrettyPrint())
		}
		batch.Put(entries[i].Key, &entries[i].Value)
	}
	return nil
}

func (ru *Updater) putSubordinateJSONHeader(
	ctx context.Context,
	batch *kv.Batch,
	rowKey roachpb.Key,
	colID descpb.ColumnID,
	path []keys.SubordinatePathSegment,
	kind rowenc.SubordinateJSONNodeKind,
	childCount int,
	traceKV bool,
) error {
	val, err := rowenc.EncodeSubordinateJSONContainerValue(kind, childCount)
	if err != nil {
		return err
	}
	key := roachpb.Key(keys.MakeSubordinatePathKey(rowKey, uint32(colID), path))
	if traceKV {
		log.VEventf(ctx, 2, "Put %s -> %v", key, val.PrettyPrint())
	}
	batch.Put(key, &val)
	return nil
}

func (ru *Updater) deleteSubordinateExactSubtree(
	ctx context.Context, batch *kv.Batch, exactKey roachpb.Key, traceKV bool,
) {
	rowPrefix, colID, path, err := keys.DecodeSubordinatePathKey(exactKey)
	if err != nil {
		panic(err)
	}
	rowKey := keys.MakeFamilyKey(rowPrefix, 0)
	prefix := roachpb.Key(keys.MakeSubordinatePathPrefix(rowKey, colID, path))
	end := prefix.PrefixEnd()
	if traceKV {
		log.VEventf(ctx, 2, "DelRange %s - %s", prefix, end)
	}
	batch.DelRange(prefix, end, false /* returnKeys */)
}

func (ru *Updater) deleteSubordinatePrefix(
	ctx context.Context, batch *kv.Batch, prefix roachpb.Key, traceKV bool,
) {
	end := prefix.PrefixEnd()
	if traceKV {
		log.VEventf(ctx, 2, "DelRange %s - %s", prefix, end)
	}
	batch.DelRange(prefix, end, false /* returnKeys */)
}
