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

package rowenc_test

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/tabledesc"
	. "github.com/cockroachdb/cockroach/pkg/sql/rowenc"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/stretchr/testify/require"
)

func buildPrimaryIndexKey(
	t *testing.T,
	codec keys.SQLCodec,
	tableDesc catalog.TableDescriptor,
	colMap catalog.TableColMap,
	values []tree.Datum,
) []byte {
	t.Helper()
	keyPrefix := MakeIndexKeyPrefix(codec, tableDesc.GetID(), tableDesc.GetPrimaryIndexID())
	indexKey, containsNull, err := EncodeIndexKey(
		tableDesc, tableDesc.GetPrimaryIndex(), colMap, values, keyPrefix,
	)
	require.NoError(t, err)
	require.False(t, containsNull)
	return indexKey
}

func TestEncodeSubordinateKeysBasic(t *testing.T) {
	tableDesc, colMap := makeTableDescWithArray()
	codec := keys.SystemSQLCodec

	arr := tree.NewDArray(types.Int)
	for _, v := range []int64{10, 20, 30} {
		require.NoError(t, arr.Append(tree.NewDInt(tree.DInt(v))))
	}
	values := []tree.Datum{tree.NewDInt(1), arr}

	pkKey := buildPrimaryIndexKey(t, codec, tableDesc, colMap, values)
	entries, err := EncodeSubordinateKeys(tableDesc, pkKey, colMap, values)
	require.NoError(t, err)
	require.Equal(t, 3, len(entries))

	for i, entry := range entries {
		rowPrefix, colID, elemIdx, err := keys.DecodeSubordinateKey(entry.Key)
		require.NoError(t, err)
		require.Equal(t, uint32(2), colID)
		require.Equal(t, uint32(i), elemIdx)
		require.Equal(t, roachpb.Key(pkKey), roachpb.Key(rowPrefix))

		got, err := entry.Value.GetInt()
		require.NoError(t, err)
		require.Equal(t, []int64{10, 20, 30}[i], got)
	}
}

func TestEncodeSubordinateKeysNullArray(t *testing.T) {
	tableDesc, colMap := makeTableDescWithArray()
	codec := keys.SystemSQLCodec

	values := []tree.Datum{tree.NewDInt(1), tree.DNull}
	pkKey := buildPrimaryIndexKey(t, codec, tableDesc, colMap, values)

	entries, err := EncodeSubordinateKeys(tableDesc, pkKey, colMap, values)
	require.NoError(t, err)
	require.Equal(t, 0, len(entries))
}

func TestEncodeSubordinateKeysEmptyArray(t *testing.T) {
	tableDesc, colMap := makeTableDescWithArray()
	codec := keys.SystemSQLCodec

	arr := tree.NewDArray(types.Int)
	values := []tree.Datum{tree.NewDInt(1), arr}
	pkKey := buildPrimaryIndexKey(t, codec, tableDesc, colMap, values)

	entries, err := EncodeSubordinateKeys(tableDesc, pkKey, colMap, values)
	require.NoError(t, err)
	require.Equal(t, 0, len(entries))
}

func TestEncodeSubordinateKeysMixedColumns(t *testing.T) {
	// Table: pk INT, name STRING, tags STRING[]
	columns := []descpb.ColumnDescriptor{
		{ID: 1, Name: "pk", Type: types.Int},
		{ID: 2, Name: "name", Type: types.String},
		{ID: 3, Name: "tags", Type: types.StringArray},
	}
	var colMap catalog.TableColMap
	colMap.Set(1, 0)
	colMap.Set(2, 1)
	colMap.Set(3, 2)

	td := descpb.TableDescriptor{
		ID:      42,
		Columns: columns,
		PrimaryIndex: descpb.IndexDescriptor{
			ID:                  1,
			KeyColumnIDs:        []descpb.ColumnID{1},
			KeyColumnDirections: []descpb.IndexDescriptor_Direction{descpb.IndexDescriptor_ASC},
		},
		Families: []descpb.ColumnFamilyDescriptor{{
			Name:            "primary",
			ID:              0,
			ColumnNames:     []string{"pk", "name", "tags"},
			ColumnIDs:       []descpb.ColumnID{1, 2, 3},
			DefaultColumnID: 1,
		}},
	}
	tableDesc := tabledesc.NewBuilder(&td).BuildImmutableTable()
	codec := keys.SystemSQLCodec

	arr := tree.NewDArray(types.String)
	for _, s := range []string{"foo", "bar"} {
		require.NoError(t, arr.Append(tree.NewDString(s)))
	}
	values := []tree.Datum{tree.NewDInt(1), tree.NewDString("hello"), arr}
	pkKey := buildPrimaryIndexKey(t, codec, tableDesc, colMap, values)

	entries, err := EncodeSubordinateKeys(tableDesc, pkKey, colMap, values)
	require.NoError(t, err)

	// Only the array column (id=3) produces subordinate entries.
	require.Equal(t, 2, len(entries))
	for i, entry := range entries {
		_, colID, elemIdx, err := keys.DecodeSubordinateKey(entry.Key)
		require.NoError(t, err)
		require.Equal(t, uint32(3), colID)
		require.Equal(t, uint32(i), elemIdx)
	}
}

func TestSubordinateKeysForColumn(t *testing.T) {
	codec := keys.SystemSQLCodec
	tableDesc, colMap := makeTableDescWithArray()
	values := []tree.Datum{tree.NewDInt(1), tree.DNull}
	pkKey := buildPrimaryIndexKey(t, codec, tableDesc, colMap, values)

	delKeys := SubordinateKeysForColumn(pkKey, 2, 3)
	require.Equal(t, 3, len(delKeys))

	for i, k := range delKeys {
		_, colID, elemIdx, err := keys.DecodeSubordinateKey(k)
		require.NoError(t, err)
		require.Equal(t, uint32(2), colID)
		require.Equal(t, uint32(i), elemIdx)
	}
}
