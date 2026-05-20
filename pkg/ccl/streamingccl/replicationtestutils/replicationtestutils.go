// Copyright 2026 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package replicationtestutils

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/rowenc"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/stretchr/testify/require"
)

// EncodeKV encodes a single primary-key value for tests shared with CCL code.
func EncodeKV(
	t testing.TB, codec keys.SQLCodec, tableDesc catalog.TableDescriptor, pkValue string,
) roachpb.KeyValue {
	t.Helper()
	return EncodeKVs(t, codec, tableDesc, pkValue)[0]
}

// EncodeKVs encodes primary-key values for tests shared with CCL code.
func EncodeKVs(
	t testing.TB, codec keys.SQLCodec, tableDesc catalog.TableDescriptor, pkValues ...interface{},
) []roachpb.KeyValue {
	t.Helper()
	primaryIndex := tableDesc.GetPrimaryIndex()

	colMap := catalog.TableColMap{}
	values := make([]tree.Datum, primaryIndex.NumKeyColumns())
	for i := 0; i < primaryIndex.NumKeyColumns(); i++ {
		colID := primaryIndex.GetKeyColumnID(i)
		colMap.Set(colID, i)
		col, err := catalog.MustFindColumnByID(tableDesc, colID)
		require.NoError(t, err)
		switch pkValue := pkValues[i].(type) {
		case tree.Datum:
			values[i] = pkValue
		case string:
			values[i], _, err = tree.ParseAndRequireString(col.GetType(), pkValue, nil)
			require.NoError(t, err)
		default:
			t.Fatalf("unsupported primary key value type %T", pkValue)
		}
	}
	entries, err := rowenc.EncodePrimaryIndex(codec, tableDesc, primaryIndex, colMap, values, false /* includeEmpty */)
	require.NoError(t, err)

	kvs := make([]roachpb.KeyValue, len(entries))
	for i := range entries {
		kvs[i] = roachpb.KeyValue{Key: entries[i].Key, Value: entries[i].Value}
	}
	return kvs
}
