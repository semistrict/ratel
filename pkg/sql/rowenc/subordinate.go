// Copyright 2026 The Ratel Authors.
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

package rowenc

import (
	"github.com/cockroachdb/errors"
	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/roachpb"
	"github.com/semistrict/ratel/pkg/sql/catalog"
	"github.com/semistrict/ratel/pkg/sql/catalog/descpb"
	"github.com/semistrict/ratel/pkg/sql/rowenc/valueside"
	"github.com/semistrict/ratel/pkg/sql/sem/tree"
	"github.com/semistrict/ratel/pkg/sql/types"
	"github.com/semistrict/ratel/pkg/util/encoding"
	jsonutil "github.com/semistrict/ratel/pkg/util/json"
)

// IsSubordinateNull returns true if the given value represents a NULL array
// element in subordinate key encoding. NULL elements are encoded as a
// TUPLE-tagged value with no data, which MarshalLegacy never produces
// for scalar types.
func IsSubordinateNull(v roachpb.Value) bool {
	return v.GetTag() == roachpb.ValueType_TUPLE && len(v.RawBytes) == 5
}

// subordinateNullValue returns a roachpb.Value representing a NULL array
// element. Uses TUPLE tag with empty data as a sentinel.
func subordinateNullValue() roachpb.Value {
	var v roachpb.Value
	v.SetTuple(nil)
	return v
}

type SubordinateJSONNodeKind byte

const (
	SubordinateJSONScalar SubordinateJSONNodeKind = iota + 1
	SubordinateJSONObject
	SubordinateJSONArray
)

func EncodeSubordinateJSONValue(j jsonutil.JSON) (roachpb.Value, error) {
	var raw []byte
	switch j.Type() {
	case jsonutil.ObjectJSONType:
		raw = []byte{byte(SubordinateJSONObject)}
		raw = encoding.EncodeUvarintAscending(raw, uint64(j.Len()))
	case jsonutil.ArrayJSONType:
		raw = []byte{byte(SubordinateJSONArray)}
		raw = encoding.EncodeUvarintAscending(raw, uint64(j.Len()))
	default:
		raw = []byte{byte(SubordinateJSONScalar)}
		var err error
		raw, err = jsonutil.EncodeJSON(raw, j)
		if err != nil {
			return roachpb.Value{}, err
		}
	}
	var v roachpb.Value
	v.SetBytes(raw)
	return v, nil
}

func PeekSubordinateJSONValueMetadata(
	v roachpb.Value,
) (SubordinateJSONNodeKind, int, []byte, error) {
	raw, err := v.GetBytes()
	if err != nil {
		return 0, 0, nil, err
	}
	if len(raw) == 0 {
		return 0, 0, nil, errors.AssertionFailedf("empty subordinate JSON value")
	}
	kind := SubordinateJSONNodeKind(raw[0])
	switch kind {
	case SubordinateJSONObject, SubordinateJSONArray:
		remaining, count, err := encoding.DecodeUvarintAscending(raw[1:])
		if err != nil {
			return 0, 0, nil, err
		}
		if len(remaining) != 0 {
			return 0, 0, nil, errors.AssertionFailedf("trailing data in subordinate JSON container")
		}
		return kind, int(count), nil, nil
	case SubordinateJSONScalar:
		return kind, 0, raw[1:], nil
	default:
		return 0, 0, nil, errors.AssertionFailedf("unknown subordinate JSON node kind %d", kind)
	}
}

func DecodeSubordinateJSONScalarBytes(raw []byte) (jsonutil.JSON, error) {
	remaining, j, err := jsonutil.DecodeJSON(raw)
	if err != nil {
		return nil, err
	}
	if len(remaining) != 0 {
		return nil, errors.AssertionFailedf("trailing data in subordinate JSON scalar")
	}
	return j, nil
}

func DecodeSubordinateJSONValueWithCardinality(
	v roachpb.Value,
) (SubordinateJSONNodeKind, int, jsonutil.JSON, error) {
	kind, childCount, scalarRaw, err := PeekSubordinateJSONValueMetadata(v)
	if err != nil {
		return 0, 0, nil, err
	}
	switch kind {
	case SubordinateJSONObject, SubordinateJSONArray:
		return kind, childCount, nil, nil
	case SubordinateJSONScalar:
		j, err := DecodeSubordinateJSONScalarBytes(scalarRaw)
		if err != nil {
			return 0, 0, nil, err
		}
		return kind, 0, j, nil
	default:
		return 0, 0, nil, errors.AssertionFailedf("unknown subordinate JSON node kind %d", kind)
	}
}

func DecodeSubordinateJSONValue(v roachpb.Value) (SubordinateJSONNodeKind, jsonutil.JSON, error) {
	kind, _, j, err := DecodeSubordinateJSONValueWithCardinality(v)
	return kind, j, err
}

func appendJSONSubordinateEntries(
	entries []IndexEntry,
	sentinelKey []byte,
	colID uint32,
	path []keys.SubordinatePathSegment,
	j jsonutil.JSON,
) ([]IndexEntry, error) {
	subKey := keys.MakeSubordinatePathKey(sentinelKey, colID, path)
	val, err := EncodeSubordinateJSONValue(j)
	if err != nil {
		return nil, err
	}
	entries = append(entries, IndexEntry{Key: subKey, Value: val, RowGroup: 0})

	switch j.Type() {
	case jsonutil.ObjectJSONType:
		iter, err := j.ObjectIter()
		if err != nil {
			return nil, err
		}
		for iter.Next() {
			entries, err = appendJSONSubordinateEntries(entries, sentinelKey, colID,
				append(path[:len(path):len(path)], keys.SubordinatePathSegment{
					Kind:      keys.SubordinatePathObjectKey,
					ObjectKey: iter.Key(),
				}), iter.Value())
			if err != nil {
				return nil, err
			}
		}
	case jsonutil.ArrayJSONType:
		for i := 0; i < j.Len(); i++ {
			child, err := j.FetchValIdx(i)
			if err != nil {
				return nil, err
			}
			if child == nil {
				return nil, errors.AssertionFailedf("missing JSON array element %d", i)
			}
			entries, err = appendJSONSubordinateEntries(entries, sentinelKey, colID,
				append(path[:len(path):len(path)], keys.SubordinatePathSegment{
					Kind:     keys.SubordinatePathArrayIndex,
					ArrayIdx: uint32(i),
				}), child)
			if err != nil {
				return nil, err
			}
		}
	}
	return entries, nil
}

// EncodeSubordinateKeys returns subordinate key entries for all columns that
// use subordinate storage in the given row.
//
// primaryIndexKey is the PK prefix (before family encoding). The function
// computes the family-0 sentinel from it.
//
// This function does not modify EncodePrimaryIndex's output — it produces
// a separate set of entries that the caller is responsible for writing to or
// deleting from the KV batch.
func EncodeSubordinateKeys(
	tableDesc catalog.TableDescriptor,
	primaryIndexKey []byte,
	colMap catalog.TableColMap,
	values []tree.Datum,
) ([]IndexEntry, error) {
	sentinelKey := keys.MakeFamilyKey(primaryIndexKey, 0)
	var entries []IndexEntry

	for _, col := range tableDesc.PublicColumns() {
		idx, ok := colMap.Get(col.GetID())
		if !ok {
			continue
		}
		datum := values[idx]
		switch col.GetType().Family() {
		case types.ArrayFamily:
			dArr, ok := datum.(*tree.DArray)
			if !ok || dArr == nil || dArr.Len() == 0 {
				continue
			}
			elemType := col.GetType().ArrayContents()
			for i, elem := range dArr.Array {
				subKey := keys.MakeSubordinateKey(sentinelKey, uint32(col.GetID()), uint32(i))
				var val roachpb.Value
				if elem == tree.DNull {
					// NULL elements use a TUPLE-tagged value with no data
					// as a sentinel. MarshalLegacy never produces TUPLE for
					// scalar types, so this is unambiguous.
					val = subordinateNullValue()
				} else {
					var err error
					val, err = valueside.MarshalLegacy(elemType, elem)
					if err != nil {
						return nil, err
					}
				}
				entries = append(entries, IndexEntry{Key: subKey, Value: val, RowGroup: 0})
			}
		case types.JsonFamily:
			if datum == tree.DNull {
				continue
			}
			dJSON, ok := datum.(*tree.DJSON)
			if !ok || dJSON == nil {
				continue
			}
			var err error
			entries, err = appendJSONSubordinateEntries(entries, sentinelKey, uint32(col.GetID()), []keys.SubordinatePathSegment{{
				Kind: keys.SubordinatePathHeader,
			}}, dJSON.JSON)
			if err != nil {
				return nil, err
			}
		}
	}

	return entries, nil
}

// SubordinateKeysForColumn returns the subordinate keys that would exist for a
// single array column with the given number of elements. This is used to
// compute which keys to delete when an array shrinks or is removed entirely.
func SubordinateKeysForColumn(
	primaryIndexKey []byte, colID descpb.ColumnID, numElements int,
) []roachpb.Key {
	sentinelKey := keys.MakeFamilyKey(primaryIndexKey, 0)
	result := make([]roachpb.Key, numElements)
	for i := 0; i < numElements; i++ {
		result[i] = keys.MakeSubordinateKey(sentinelKey, uint32(colID), uint32(i))
	}
	return result
}

// SubordinateRowRange returns the range containing every subordinate key for
// the row identified by primaryIndexKey.
func SubordinateRowRange(primaryIndexKey []byte) (start, end roachpb.Key) {
	sentinelKey := keys.MakeFamilyKey(primaryIndexKey, 0)
	start = append(roachpb.Key(nil), sentinelKey...)
	start = encoding.EncodeUvarintAscending(start, 0)
	end = keys.MakeFamilyKey(primaryIndexKey[:len(primaryIndexKey):len(primaryIndexKey)], 1)
	return start, end
}
