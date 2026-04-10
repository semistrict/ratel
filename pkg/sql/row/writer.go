// Copyright 2015 The Cockroach Authors.
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

package row

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/roachpb"
	"github.com/semistrict/ratel/pkg/sql/catalog"
	"github.com/semistrict/ratel/pkg/sql/catalog/descpb"
	"github.com/semistrict/ratel/pkg/sql/rowenc"
	"github.com/semistrict/ratel/pkg/sql/rowenc/valueside"
	"github.com/semistrict/ratel/pkg/sql/sem/tree"
	"github.com/semistrict/ratel/pkg/util"
)

// This file contains common functions for the three writers, Inserter, Deleter
// and Updater.

// ColIDtoRowIndexFromCols groups a slice of ColumnDescriptors by their ID
// field, returning a map from ID to the index of the column in the input slice.
// It assumes there are no duplicate descriptors in the input.
func ColIDtoRowIndexFromCols(cols []catalog.Column) catalog.TableColMap {
	var colIDtoRowIndex catalog.TableColMap
	for i := range cols {
		colIDtoRowIndex.Set(cols[i].GetID(), i)
	}
	return colIDtoRowIndex
}

// ColMapping returns a map from ordinals in the fromCols list to ordinals in
// the toCols list. More precisely, for 0 <= i < fromCols:
//
//	result[i] = j such that fromCols[i].ID == toCols[j].ID, or
//	             -1 if the column is not part of toCols.
func ColMapping(fromCols, toCols []catalog.Column) []int {
	// colMap is a map from ColumnID to ordinal into fromCols.
	var colMap util.FastIntMap
	for i := range fromCols {
		colMap.Set(int(fromCols[i].GetID()), i)
	}

	result := make([]int, len(fromCols))
	for i := range result {
		// -1 value indicates that this column is not being returned.
		result[i] = -1
	}

	// Set the appropriate index values for the returning columns.
	for toOrd := range toCols {
		if fromOrd, ok := colMap.Get(int(toCols[toOrd].GetID())); ok {
			result[fromOrd] = toOrd
		}
	}

	return result
}

// prepareInsertOrUpdateBatch constructs a KV batch that inserts or
// updates a row in KV.
//   - batch is the KV batch where commands should be appended.
//   - putFn is the functions that can append Put/CPut commands to the batch.
//     (must be adapted depending on whether 'overwrite' is set)
//   - helper is the rowHelper that knows about the table being modified.
//   - primaryIndexKey is the PK prefix for the current row.
//   - fetchedCols is the list of schema columns that have been fetched
//     in preparation for this update.
//   - values is the SQL-level row values that are being written.
//   - marshaledValues contains the pre-encoded KV-level row values.
//     marshaledValues is only used for single-KV row writes. Pre-encoding must
//     occur prior to calling this function to check whether the encoding is
//     _possible_ (i.e. values fit in the column types, etc).
//   - valColIDMapping/marshaledColIDMapping is the mapping from column
//     IDs into positions of the slices values or marshaledValues.
//   - kvKey and kvValues must be heap-allocated scratch buffers to write
//     roachpb.Key and roachpb.Value values.
//   - rawValueBuf must be a scratch byte array. This must be reinitialized
//     to an empty slice on each call but can be preserved at its current
//     capacity to avoid allocations. The function returns the slice.
//   - overwrite must be set to true for UPDATE and UPSERT.
//   - traceKV is to be set to log the KV operations added to the batch.
func prepareInsertOrUpdateBatch(
	ctx context.Context,
	batch putter,
	helper *rowHelper,
	primaryIndexKey []byte,
	fetchedCols []catalog.Column,
	values []tree.Datum,
	valColIDMapping catalog.TableColMap,
	marshaledValues []roachpb.Value,
	marshaledColIDMapping catalog.TableColMap,
	subordinateEntries []rowenc.IndexEntry,
	kvKey *roachpb.Key,
	kvValue *roachpb.Value,
	rawValueBuf []byte,
	putFn func(ctx context.Context, b putter, key *roachpb.Key, value *roachpb.Value, traceKV bool),
	overwrite, traceKV bool,
) ([]byte, error) {
	rowGroup := &helper.TableDesc.GetRowGroups()[0]
	*kvKey = keys.MakeFamilyKey(primaryIndexKey, 0)
	rawValueBuf = rawValueBuf[:0]

	var lastColID descpb.ColumnID
	for _, colID := range helper.sortedPrimaryRowGroup() {
		idx, ok := valColIDMapping.Get(colID)
		if !ok || values[idx] == tree.DNull {
			continue
		}
		if skip, err := helper.skipColumnNotInPrimaryIndexValue(colID, values[idx]); err != nil {
			return nil, err
		} else if skip {
			continue
		}
		// Non-empty array columns are encoded as subordinate keys rather than
		// inline in the row-group tuple. NULL and empty arrays stay inline to
		// preserve their distinction.
		if helper.isArrayColumn(colID) {
			if dArr, isDArr := values[idx].(*tree.DArray); isDArr && dArr != nil && dArr.Len() > 0 {
				continue
			}
		}
		// JSON values are always stored via recursive subordinate keys.
		if helper.isJSONColumn(colID) {
			continue
		}
		col := fetchedCols[idx]
		if lastColID > col.GetID() {
			return nil, errors.AssertionFailedf("cannot write column id %d after %d", col.GetID(), lastColID)
		}
		colIDDelta := valueside.MakeColumnIDDelta(lastColID, col.GetID())
		lastColID = col.GetID()
		var err error
		rawValueBuf, err = valueside.Encode(rawValueBuf, colIDDelta, values[idx], nil)
		if err != nil {
			return nil, err
		}
	}

	kvValue.SetTuple(rawValueBuf)
	if err := helper.checkRowSizeWithSubordinates(ctx, kvKey, kvValue, rowGroup.ID, subordinateEntries); err != nil {
		return nil, err
	}
	putFn(ctx, batch, kvKey, kvValue, traceKV)

	*kvKey = nil
	*kvValue = roachpb.Value{}

	return rawValueBuf, nil
}
