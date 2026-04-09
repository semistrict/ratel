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
	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/roachpb"
	"github.com/semistrict/ratel/pkg/sql/catalog"
	"github.com/semistrict/ratel/pkg/sql/catalog/descpb"
	"github.com/semistrict/ratel/pkg/sql/rowenc/valueside"
	"github.com/semistrict/ratel/pkg/sql/sem/tree"
	"github.com/semistrict/ratel/pkg/sql/types"
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

// EncodeSubordinateKeys returns subordinate key entries for all array columns
// in the given row. Each non-null, non-empty array column produces one entry
// per element, keyed under the row's sentinel key.
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
		if col.GetType().Family() != types.ArrayFamily {
			continue
		}
		idx, ok := colMap.Get(col.GetID())
		if !ok {
			continue
		}
		datum := values[idx]
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
