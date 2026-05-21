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
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/desctestutils"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/fetchpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/tabledesc"
	"github.com/cockroachdb/cockroach/pkg/sql/rowenc"
	"github.com/cockroachdb/cockroach/pkg/sql/rowenc/valueside"
	"github.com/cockroachdb/cockroach/pkg/sql/rowinfra"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/encoding"
	"github.com/cockroachdb/cockroach/pkg/util/intsets"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/protoutil"
	"github.com/stretchr/testify/require"
)

func makeArraySubordinateFetcher(t *testing.T, materialize bool, left tree.Datum) *Fetcher {
	t.Helper()

	var colIdxMap catalog.TableColMap
	colIdxMap.Set(2, 0)

	rf := &Fetcher{
		table: tableInfo{
			spec: fetchpb.IndexFetchSpec{
				FetchedColumns: []fetchpb.IndexFetchSpec_Column{{
					ColumnID: 2,
					Name:     "vals",
					Type:     types.IntArray,
				}},
			},
			colIdxMap:          colIdxMap,
			neededValueCols:    1,
			row:                make(rowenc.EncDatumRow, 1),
			decodedRow:         make(tree.Datums, 1),
			keyVals:            make([]rowenc.EncDatum, 0),
			extraVals:          make([]rowenc.EncDatum, 0),
			indexColIdx:        []int{-1},
			timestampOutputIdx: noOutputColumn,
			oidOutputIdx:       noOutputColumn,
		},
		args: FetcherInitArgs{Alloc: &tree.DatumAlloc{}},
	}
	rf.ConfigureArrayEqualsAnyFilter(eval.NewTestingEvalContext(cluster.MakeTestingClusterSettings()), 0, left, materialize)
	return rf
}

func makeSingleColumnFetcher(t *testing.T, typ *types.T, colID descpb.ColumnID, name string) *Fetcher {
	t.Helper()

	var colIdxMap catalog.TableColMap
	colIdxMap.Set(colID, 0)
	var neededValueColsByIdx intsets.Fast
	neededValueColsByIdx.Add(0)

	return &Fetcher{
		table: tableInfo{
			spec: fetchpb.IndexFetchSpec{
				FetchedColumns: []fetchpb.IndexFetchSpec_Column{{
					ColumnID: colID,
					Name:     name,
					Type:     typ,
				}},
			},
			colIdxMap:            colIdxMap,
			neededValueColsByIdx: neededValueColsByIdx,
			neededValueCols:      1,
			row:                  make(rowenc.EncDatumRow, 1),
			decodedRow:           make(tree.Datums, 1),
			keyVals:              make([]rowenc.EncDatum, 0),
			extraVals:            make([]rowenc.EncDatum, 0),
			indexColIdx:          []int{-1},
			timestampOutputIdx:   noOutputColumn,
			oidOutputIdx:         noOutputColumn,
		},
		args: FetcherInitArgs{Alloc: &tree.DatumAlloc{}},
	}
}

func makeSubordinateKV(t *testing.T, colID descpb.ColumnID, elemIdx int, d tree.Datum) (roachpb.KeyValue, []byte) {
	t.Helper()

	remaining := encoding.EncodeUvarintAscending(nil, uint64(colID))
	remaining = encoding.EncodeUvarintAscending(remaining, uint64(elemIdx))

	var value roachpb.Value
	if d == tree.DNull {
		value.SetTuple(nil)
	} else {
		var err error
		value, err = valueside.MarshalLegacy(types.Int, d)
		require.NoError(t, err)
	}
	return roachpb.KeyValue{Value: value}, remaining
}

func TestSubordinateArrayBuilderMaterializeOrder(t *testing.T) {
	defer leaktest.AfterTest(t)()

	builder := newSubordinateArrayBuilder(types.Int)
	builder.Set(2, tree.NewDInt(30))
	builder.Set(0, tree.NewDInt(10))
	builder.Set(1, tree.NewDInt(20))

	arr, err := builder.Materialize()
	require.NoError(t, err)
	require.Equal(t, "ARRAY[10,20,30]", arr.String())
}

func TestSubordinateArrayBuilderMaterializeMissingElement(t *testing.T) {
	defer leaktest.AfterTest(t)()

	builder := newSubordinateArrayBuilder(types.Int)
	builder.Set(1, tree.NewDInt(20))

	_, err := builder.Materialize()
	require.Error(t, err)
	require.Regexp(t, "missing subordinate array element 0", err.Error())
}

func TestFinishArrayEqualsAnyFilterInlineArrayStates(t *testing.T) {
	defer leaktest.AfterTest(t)()

	makeFetcher := func(d tree.Datum) *Fetcher {
		rf := &Fetcher{
			table: tableInfo{
				spec: fetchpb.IndexFetchSpec{
					FetchedColumns: []fetchpb.IndexFetchSpec_Column{{Type: types.IntArray}},
				},
				row: rowenc.EncDatumRow{{Datum: d}},
			},
			args: FetcherInitArgs{Alloc: &tree.DatumAlloc{}},
			arrayEqualsAnyFilter: &arrayEqualsAnyFilterState{
				evalCtx: eval.NewTestingEvalContext(cluster.MakeTestingClusterSettings()),
				colIdx:  0,
				left:    tree.NewDInt(20),
			},
		}
		return rf
	}

	rf := makeFetcher(tree.NewDArray(types.Int))
	require.NoError(t, rf.finishArrayEqualsAnyFilter())
	require.False(t, rf.RowPassesArrayEqualsAnyFilter())

	rf = makeFetcher(tree.DNull)
	require.NoError(t, rf.finishArrayEqualsAnyFilter())
	require.False(t, rf.RowPassesArrayEqualsAnyFilter())
	require.True(t, rf.arrayEqualsAnyFilter.sawNull)
}

func TestFinishArrayEqualsAnyFilterRejectsNonEmptyInlineArray(t *testing.T) {
	defer leaktest.AfterTest(t)()

	arr := tree.NewDArray(types.Int)
	require.NoError(t, arr.Append(tree.NewDInt(10)))
	rf := &Fetcher{
		table: tableInfo{
			spec: fetchpb.IndexFetchSpec{
				FetchedColumns: []fetchpb.IndexFetchSpec_Column{{Name: "vals", Type: types.IntArray}},
			},
			row: rowenc.EncDatumRow{{Datum: arr}},
		},
		args: FetcherInitArgs{Alloc: &tree.DatumAlloc{}},
		arrayEqualsAnyFilter: &arrayEqualsAnyFilterState{
			evalCtx: eval.NewTestingEvalContext(cluster.MakeTestingClusterSettings()),
			colIdx:  0,
			left:    tree.NewDInt(20),
		},
	}

	err := rf.finishArrayEqualsAnyFilter()
	require.Error(t, err)
	require.Regexp(t, "non-empty inline array encountered", err.Error())
}

func TestFinishArrayEqualsAnyFilterUnsetColumn(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := &Fetcher{
		table: tableInfo{
			spec: fetchpb.IndexFetchSpec{
				FetchedColumns: []fetchpb.IndexFetchSpec_Column{{Type: types.IntArray}},
			},
			row: make(rowenc.EncDatumRow, 1),
		},
		args: FetcherInitArgs{Alloc: &tree.DatumAlloc{}},
		arrayEqualsAnyFilter: &arrayEqualsAnyFilterState{
			evalCtx: eval.NewTestingEvalContext(cluster.MakeTestingClusterSettings()),
			colIdx:  0,
			left:    tree.NewDInt(20),
		},
	}

	require.NoError(t, rf.finishArrayEqualsAnyFilter())
	require.False(t, rf.RowPassesArrayEqualsAnyFilter())
}

func TestConfigureArrayEqualsAnyFilterAndReset(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := &Fetcher{
		table: tableInfo{
			row: rowenc.EncDatumRow{{Datum: tree.NewDInt(1)}},
		},
		subordinateArrays: map[int]*subordinateArrayBuilder{
			0: newSubordinateArrayBuilder(types.Int),
		},
		lastRowPassesArrayEqualsAnyFilter: false,
	}
	rf.ConfigureArrayEqualsAnyFilter(eval.NewTestingEvalContext(cluster.MakeTestingClusterSettings()), 3, tree.NewDInt(20), false)
	require.NotNil(t, rf.arrayEqualsAnyFilter)
	require.Equal(t, 3, rf.arrayEqualsAnyFilter.colIdx)
	require.True(t, rf.RowPassesArrayEqualsAnyFilter())

	tableBefore := rf.table
	rf.Reset()
	require.Equal(t, tableBefore, rf.table)
	require.Nil(t, rf.arrayEqualsAnyFilter)
	require.Nil(t, rf.subordinateArrays)
}

func TestProcessSubordinateKVMaterializesAndFilters(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeArraySubordinateFetcher(t, true, tree.NewDInt(20))

	kv, remaining := makeSubordinateKV(t, 2, 1, tree.NewDInt(20))
	prettyKey, prettyValue, err := rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)
	require.Equal(t, "/tbl/1/0", prettyKey)
	require.Empty(t, prettyValue)

	kv, remaining = makeSubordinateKV(t, 2, 0, tree.NewDInt(10))
	_, _, err = rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)

	require.NoError(t, rf.finalizeRow())
	require.True(t, rf.RowPassesArrayEqualsAnyFilter())
	require.False(t, rf.table.row[0].IsUnset())
	require.NoError(t, rf.table.row[0].EnsureDecoded(types.IntArray, rf.args.Alloc))
	require.Equal(t, "ARRAY[10,20]", rf.table.row[0].Datum.String())
}

func TestProcessSubordinateKVPredicateOnlyLeavesRowUnset(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeArraySubordinateFetcher(t, false, tree.NewDInt(20))

	kv, remaining := makeSubordinateKV(t, 2, 1, tree.NewDInt(20))
	prettyKey, prettyValue, err := rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)
	require.Equal(t, "/tbl/1/0", prettyKey)
	require.Empty(t, prettyValue)
	require.Nil(t, rf.subordinateArrays)

	kv, remaining = makeSubordinateKV(t, 2, 0, tree.DNull)
	_, _, err = rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)

	require.NoError(t, rf.finalizeRow())
	require.True(t, rf.RowPassesArrayEqualsAnyFilter())
	require.True(t, rf.table.row[0].IsUnset())
	require.True(t, rf.arrayEqualsAnyFilter.sawNull)
	require.True(t, rf.arrayEqualsAnyFilter.sawSubordinate)
	require.Equal(t, 0, rf.valueColsFound)
}

func TestProcessSubordinateKVSkipsUnknownColumn(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeArraySubordinateFetcher(t, true, tree.NewDInt(20))
	kv, remaining := makeSubordinateKV(t, 99, 0, tree.NewDInt(10))

	prettyKey, prettyValue, err := rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)
	require.Equal(t, "/tbl/1/0", prettyKey)
	require.Empty(t, prettyValue)
	require.Nil(t, rf.subordinateArrays)
}

func TestProcessValueSingleDecodesDatum(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeSingleColumnFetcher(t, types.Int, 7, "v")
	kvValue, err := valueside.MarshalLegacy(types.Int, tree.NewDInt(42))
	require.NoError(t, err)

	prettyKey, prettyValue, err := rf.processValueSingle(
		context.Background(), &rf.table, 7, roachpb.KeyValue{Value: kvValue}, "/tbl/1/0",
	)
	require.NoError(t, err)
	require.Equal(t, "/tbl/1/0", prettyKey)
	require.Empty(t, prettyValue)
	require.NoError(t, rf.table.row[0].EnsureDecoded(types.Int, rf.args.Alloc))
	require.Equal(t, "42", rf.table.row[0].Datum.String())
}

func TestProcessValueSingleSkipsUnknownColumn(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeSingleColumnFetcher(t, types.Int, 7, "v")
	kvValue, err := valueside.MarshalLegacy(types.Int, tree.NewDInt(42))
	require.NoError(t, err)

	prettyKey, prettyValue, err := rf.processValueSingle(
		context.Background(), &rf.table, 99, roachpb.KeyValue{Value: kvValue}, "/tbl/1/0",
	)
	require.NoError(t, err)
	require.Equal(t, "/tbl/1/0", prettyKey)
	require.Empty(t, prettyValue)
	require.True(t, rf.table.row[0].IsUnset())
}

func TestProcessValueSingleHandlesEmptyValue(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeSingleColumnFetcher(t, types.Int, 7, "v")

	prettyKey, prettyValue, err := rf.processValueSingle(
		context.Background(), &rf.table, 7, roachpb.KeyValue{Value: roachpb.Value{}}, "/tbl/1/0",
	)
	require.NoError(t, err)
	require.Equal(t, "/tbl/1/0", prettyKey)
	require.Empty(t, prettyValue)
	require.True(t, rf.table.row[0].IsUnset())
}

func TestProcessValueSingleRejectsArrayEncoding(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeSingleColumnFetcher(t, types.IntArray, 7, "vals")
	kvValue, err := valueside.MarshalLegacy(types.Int, tree.NewDInt(42))
	require.NoError(t, err)

	_, _, err = rf.processValueSingle(
		context.Background(), &rf.table, 7, roachpb.KeyValue{Value: kvValue}, "/tbl/1/0",
	)
	require.Error(t, err)
	require.Regexp(t, "incompatible CockroachDB version", err.Error())
}

func TestProcessValueBytesSkipsUnknownAndDecodes(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeSingleColumnFetcher(t, types.Int, 7, "v")

	var buf []byte
	buf, err := valueside.Encode(buf, valueside.MakeColumnIDDelta(0, 5), tree.NewDInt(11), nil)
	require.NoError(t, err)
	buf, err = valueside.Encode(buf, valueside.MakeColumnIDDelta(5, 7), tree.NewDInt(42), nil)
	require.NoError(t, err)

	prettyKey, prettyValue, err := rf.processValueBytes(
		context.Background(), &rf.table, roachpb.KeyValue{}, buf, "/tbl/1/0",
	)
	require.NoError(t, err)
	require.Equal(t, "/tbl/1/0", prettyKey)
	require.Empty(t, prettyValue)
	require.Equal(t, 1, rf.valueColsFound)
	require.NoError(t, rf.table.row[0].EnsureDecoded(types.Int, rf.args.Alloc))
	require.Equal(t, "42", rf.table.row[0].Datum.String())
}

func TestProcessValueSingleTraceKV(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeSingleColumnFetcher(t, types.Int, 7, "v")
	rf.args.TraceKV = true
	kvValue, err := valueside.MarshalLegacy(types.Int, tree.NewDInt(42))
	require.NoError(t, err)

	prettyKey, prettyValue, err := rf.processValueSingle(
		context.Background(), &rf.table, 7, roachpb.KeyValue{Value: kvValue}, "/tbl/1/0",
	)
	require.NoError(t, err)
	require.Equal(t, "/tbl/1/0/v", prettyKey)
	require.Equal(t, "42", prettyValue)
}

func TestProcessValueBytesTraceKV(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeSingleColumnFetcher(t, types.Int, 7, "v")
	rf.args.TraceKV = true

	var buf []byte
	buf, err := valueside.Encode(buf, valueside.MakeColumnIDDelta(0, 7), tree.NewDInt(42), nil)
	require.NoError(t, err)

	prettyKey, prettyValue, err := rf.processValueBytes(
		context.Background(), &rf.table, roachpb.KeyValue{}, buf, "/tbl/1/0",
	)
	require.NoError(t, err)
	require.Equal(t, "/tbl/1/0/v", prettyKey)
	require.Equal(t, "/42", prettyValue)
}

func TestProcessSubordinateKVTraceKVMaterialized(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeArraySubordinateFetcher(t, true, tree.NewDInt(20))
	rf.args.TraceKV = true
	kv, remaining := makeSubordinateKV(t, 2, 1, tree.NewDInt(20))

	prettyKey, prettyValue, err := rf.processSubordinateKV(
		context.Background(), &rf.table, kv, remaining, "/tbl/1/0",
	)
	require.NoError(t, err)
	require.Equal(t, "/tbl/1/0/vals[1]", prettyKey)
	require.Equal(t, "20", prettyValue)
}

func TestProcessSubordinateKVTraceKVPredicateOnly(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeArraySubordinateFetcher(t, false, tree.NewDInt(20))
	rf.args.TraceKV = true
	kv, remaining := makeSubordinateKV(t, 2, 1, tree.NewDInt(20))

	prettyKey, prettyValue, err := rf.processSubordinateKV(
		context.Background(), &rf.table, kv, remaining, "/tbl/1/0",
	)
	require.NoError(t, err)
	require.Equal(t, "/tbl/1/0/vals[*]", prettyKey)
	require.Equal(t, "20", prettyValue)
}

func TestRowMetadataAccessors(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := &Fetcher{}
	rf.table.rowLastModified.WallTime = 123
	rf.table.rowLastModified.Logical = 7
	rf.table.rowIsDeleted = true

	require.Equal(t, rf.table.rowLastModified, rf.RowLastModified())
	require.True(t, rf.RowIsDeleted())
}

func TestFinalizeRowFillsNullsAndSystemColumns(t *testing.T) {
	defer leaktest.AfterTest(t)()

	var colIdxMap catalog.TableColMap
	colIdxMap.Set(7, 0)
	colIdxMap.Set(8, 1)
	var neededValueColsByIdx intsets.Fast
	neededValueColsByIdx.AddRange(0, 1)

	tsCol := descpb.ColumnID(10)
	oidCol := descpb.ColumnID(11)
	colIdxMap.Set(tsCol, 2)
	colIdxMap.Set(oidCol, 3)

	rf := &Fetcher{
		table: tableInfo{
			spec: fetchpb.IndexFetchSpec{
				TableName: "t",
				FetchedColumns: []fetchpb.IndexFetchSpec_Column{
					{ColumnID: 7, Name: "v", Type: types.Int},
					{ColumnID: 8, Name: "note", Type: types.String},
					{ColumnID: tsCol, Name: "crdb_internal_mvcc_timestamp", Type: types.Decimal},
					{ColumnID: oidCol, Name: "tableoid", Type: types.Oid},
				},
			},
			colIdxMap:            colIdxMap,
			neededValueColsByIdx: neededValueColsByIdx,
			neededValueCols:      2,
			row:                  make(rowenc.EncDatumRow, 4),
			decodedRow:           make(tree.Datums, 4),
			timestampOutputIdx:   2,
			oidOutputIdx:         3,
			tableOid:             tree.NewDOid(123),
		},
		args: FetcherInitArgs{Alloc: &tree.DatumAlloc{}},
	}
	rf.table.row[0] = rowenc.DatumToEncDatum(types.Int, tree.NewDInt(42))
	rf.valueColsFound = 1
	rf.table.rowLastModified.WallTime = 123

	require.NoError(t, rf.finalizeRow())
	require.NoError(t, rf.table.row[1].EnsureDecoded(types.String, rf.args.Alloc))
	require.Equal(t, tree.DNull, rf.table.row[1].Datum)
	require.NoError(t, rf.table.row[2].EnsureDecoded(types.Decimal, rf.args.Alloc))
	require.NotEqual(t, tree.DNull, rf.table.row[2].Datum)
	require.NoError(t, rf.table.row[3].EnsureDecoded(types.Oid, rf.args.Alloc))
	require.Equal(t, rf.table.tableOid, rf.table.row[3].Datum)
}

func TestFinalizeRowAllowsDeletedRowMissingNonNullableValue(t *testing.T) {
	defer leaktest.AfterTest(t)()

	var colIdxMap catalog.TableColMap
	colIdxMap.Set(7, 0)
	var neededValueColsByIdx intsets.Fast
	neededValueColsByIdx.Add(0)

	rf := &Fetcher{
		table: tableInfo{
			spec: fetchpb.IndexFetchSpec{
				TableName: "t",
				IndexName: "primary",
				FetchedColumns: []fetchpb.IndexFetchSpec_Column{{
					ColumnID:      7,
					Name:          "v",
					Type:          types.Int,
					IsNonNullable: true,
				}},
			},
			colIdxMap:            colIdxMap,
			neededValueColsByIdx: neededValueColsByIdx,
			neededValueCols:      1,
			row:                  make(rowenc.EncDatumRow, 1),
			decodedRow:           make(tree.Datums, 1),
			rowIsDeleted:         true,
		},
		args: FetcherInitArgs{Alloc: &tree.DatumAlloc{}},
	}

	require.NoError(t, rf.finalizeRow())
	require.NoError(t, rf.table.row[0].EnsureDecoded(types.Int, rf.args.Alloc))
	require.Equal(t, tree.DNull, rf.table.row[0].Datum)
}

func TestFinalizeRowSkipsPredicateOnlyArrayColumn(t *testing.T) {
	defer leaktest.AfterTest(t)()

	var colIdxMap catalog.TableColMap
	colIdxMap.Set(7, 0)
	var neededValueColsByIdx intsets.Fast
	neededValueColsByIdx.Add(0)

	rf := &Fetcher{
		table: tableInfo{
			spec: fetchpb.IndexFetchSpec{
				FetchedColumns: []fetchpb.IndexFetchSpec_Column{{
					ColumnID: 7,
					Name:     "vals",
					Type:     types.IntArray,
				}},
			},
			colIdxMap:            colIdxMap,
			neededValueColsByIdx: neededValueColsByIdx,
			neededValueCols:      1,
			row:                  make(rowenc.EncDatumRow, 1),
			decodedRow:           make(tree.Datums, 1),
		},
		args: FetcherInitArgs{Alloc: &tree.DatumAlloc{}},
		arrayEqualsAnyFilter: &arrayEqualsAnyFilterState{
			evalCtx:        eval.NewTestingEvalContext(cluster.MakeTestingClusterSettings()),
			colIdx:         0,
			left:           tree.NewDInt(5),
			materialize:    false,
			matched:        true,
			sawSubordinate: true,
		},
	}

	require.NoError(t, rf.finalizeRow())
	require.True(t, rf.table.row[0].IsUnset())
	require.True(t, rf.RowPassesArrayEqualsAnyFilter())
}

func TestNextRowIntoAndNextRowDecodedInto(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()

	s, sqlDB, kvDB := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)

	sqlRunner := sqlutils.MakeSQLRunner(sqlDB)
	sqlRunner.Exec(t, `CREATE DATABASE d`)
	sqlRunner.Exec(t, `CREATE TABLE d.copy_paths (k INT PRIMARY KEY, v INT, note STRING)`)
	sqlRunner.Exec(t, `INSERT INTO d.copy_paths VALUES (1, 10, 'a'), (2, 20, 'b')`)

	tableDesc := desctestutils.TestingGetPublicTableDescriptor(
		kvDB, keys.SystemSQLCodec, "d", "copy_paths",
	)
	args := initFetcherArgs{
		tableDesc: tableDesc,
		indexIdx:  0,
		columns:   []int{0, 2},
	}
	spans := roachpb.Spans{tableDesc.IndexSpan(keys.SystemSQLCodec, tableDesc.GetPrimaryIndexID())}

	rf := initFetcher(t, kv.NewTxn(ctx, kvDB, 0), args, false /* reverseScan */, &tree.DatumAlloc{}, nil /* memMon */)
	require.NoError(t, rf.StartScan(
		ctx,
		spans,
		nil,
		rowinfra.NoBytesLimit,
		rowinfra.NoRowLimit,
	))

	var encMap catalog.TableColMap
	encMap.Set(tableDesc.PublicColumns()[0].GetID(), 1)
	encMap.Set(tableDesc.PublicColumns()[2].GetID(), 0)
	dst := rowenc.EncDatumRow{
		{Datum: tree.DNull},
		{Datum: tree.DNull},
		{Datum: tree.NewDString("keep")},
	}

	ok, err := rf.NextRowInto(ctx, dst, encMap)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, dst[0].EnsureDecoded(types.String, rf.args.Alloc))
	require.NoError(t, dst[1].EnsureDecoded(types.Int, rf.args.Alloc))
	require.Equal(t, "a", string(tree.MustBeDString(dst[0].Datum)))
	require.EqualValues(t, 1, tree.MustBeDInt(dst[1].Datum))
	require.NoError(t, dst[2].EnsureDecoded(types.String, rf.args.Alloc))
	require.Equal(t, "keep", string(tree.MustBeDString(dst[2].Datum)))
	require.Equal(t, rf.table.rowLastModified, rf.RowLastModified())

	ok, err = rf.NextRowInto(ctx, dst, encMap)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, dst[0].EnsureDecoded(types.String, rf.args.Alloc))
	require.NoError(t, dst[1].EnsureDecoded(types.Int, rf.args.Alloc))
	require.Equal(t, "b", string(tree.MustBeDString(dst[0].Datum)))
	require.EqualValues(t, 2, tree.MustBeDInt(dst[1].Datum))

	ok, err = rf.NextRowInto(ctx, dst, encMap)
	require.NoError(t, err)
	require.False(t, ok)

	rf = initFetcher(t, kv.NewTxn(ctx, kvDB, 0), args, false /* reverseScan */, &tree.DatumAlloc{}, nil /* memMon */)
	spans = roachpb.Spans{tableDesc.IndexSpan(keys.SystemSQLCodec, tableDesc.GetPrimaryIndexID())}
	require.NoError(t, rf.StartScan(
		ctx,
		spans,
		nil,
		rowinfra.NoBytesLimit,
		rowinfra.NoRowLimit,
	))

	var decodedMap catalog.TableColMap
	decodedMap.Set(tableDesc.PublicColumns()[0].GetID(), 0)
	decodedMap.Set(tableDesc.PublicColumns()[2].GetID(), 2)
	decodedDst := tree.Datums{tree.NewDInt(99), tree.NewDString("keep"), tree.DNull}

	ok, err = rf.NextRowDecodedInto(ctx, decodedDst, decodedMap)
	require.NoError(t, err)
	require.True(t, ok)
	require.EqualValues(t, 1, tree.MustBeDInt(decodedDst[0]))
	require.Equal(t, "keep", string(tree.MustBeDString(decodedDst[1])))
	require.Equal(t, "a", string(tree.MustBeDString(decodedDst[2])))

	ok, err = rf.NextRowDecodedInto(ctx, decodedDst, decodedMap)
	require.NoError(t, err)
	require.True(t, ok)
	require.EqualValues(t, 2, tree.MustBeDInt(decodedDst[0]))
	require.Equal(t, "keep", string(tree.MustBeDString(decodedDst[1])))
	require.Equal(t, "b", string(tree.MustBeDString(decodedDst[2])))

	ok, err = rf.NextRowDecodedInto(ctx, decodedDst, decodedMap)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestFetcherIgnoresProjectedArrayColumnButStillGroupsSubordinateKeys(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()

	s, sqlDB, kvDB := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)

	sqlRunner := sqlutils.MakeSQLRunner(sqlDB)
	sqlRunner.Exec(t, `CREATE DATABASE d`)
	sqlRunner.Exec(t, `CREATE TABLE d.t (k INT PRIMARY KEY, a INT[])`)
	sqlRunner.Exec(t, `INSERT INTO d.t VALUES (1, ARRAY[10, 20]), (2, ARRAY[30])`)

	tableDesc := desctestutils.TestingGetPublicTableDescriptor(
		kvDB, keys.SystemSQLCodec, "d", "t",
	)
	rf := initFetcher(t, kv.NewTxn(ctx, kvDB, 0), initFetcherArgs{
		tableDesc: tableDesc,
		indexIdx:  0,
		columns:   []int{0},
	}, false /* reverseScan */, &tree.DatumAlloc{}, nil /* memMon */)

	require.NoError(t, rf.StartScan(
		ctx,
		roachpb.Spans{tableDesc.IndexSpan(keys.SystemSQLCodec, tableDesc.GetPrimaryIndexID())},
		nil,
		rowinfra.NoBytesLimit,
		rowinfra.NoRowLimit,
	))

	var got []string
	for {
		row, err := rf.NextRowDecoded(ctx)
		require.NoError(t, err)
		if row == nil {
			break
		}
		require.Len(t, row, 1)
		got = append(got, row[0].String())
	}

	require.Equal(t, []string{"1", "2"}, got)
}

func TestDeleteRemovesSubordinateKeysForDroppedArrayColumns(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()

	s, sqlDB, kvDB := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)

	sqlRunner := sqlutils.MakeSQLRunner(sqlDB)
	sqlRunner.Exec(t, `CREATE DATABASE d`)
	sqlRunner.Exec(t, `CREATE TABLE d.t (k INT PRIMARY KEY, a INT[])`)
	sqlRunner.Exec(t, `INSERT INTO d.t VALUES (1, ARRAY[10, 20])`)

	originalDesc := desctestutils.TestingGetPublicTableDescriptor(
		kvDB, keys.SystemSQLCodec, "d", "t",
	)
	descProto := protoutil.Clone(originalDesc.TableDesc()).(*descpb.TableDescriptor)
	descProto.Columns = descProto.Columns[:1]
	descProto.RowGroups = []descpb.RowGroupDescriptor{{
		ID:              0,
		Name:            "primary",
		ColumnNames:     []string{"k"},
		ColumnIDs:       []descpb.ColumnID{1},
		DefaultColumnID: 1,
	}}
	descProto.NextRowGroupID = 1
	descProto.PrimaryIndex.StoreColumnIDs = nil
	descProto.PrimaryIndex.StoreColumnNames = nil
	currentDesc := tabledesc.NewBuilder(descProto).BuildImmutableTable()

	st := cluster.MakeTestingClusterSettings()
	rd := MakeDeleter(keys.SystemSQLCodec, currentDesc, nil /* requestedCols */, &st.SV, false /* internal */, nil /* metrics */)

	b := &kv.Batch{}
	require.NoError(t, rd.DeleteRow(
		ctx, b, []tree.Datum{tree.NewDInt(1)}, PartialIndexUpdateHelper{}, false, /* traceKV */
	))
	require.NoError(t, kvDB.Run(ctx, b))

	span := originalDesc.IndexSpan(keys.SystemSQLCodec, originalDesc.GetPrimaryIndexID())
	kvs, err := kvDB.Scan(ctx, span.Key, span.EndKey, 0)
	require.NoError(t, err)
	require.Len(t, kvs, 0)
}
