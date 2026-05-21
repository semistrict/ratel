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

package colfetcher

import (
	"context"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/col/coldata"
	"github.com/cockroachdb/cockroach/pkg/col/coldataext"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/fetchpb"
	"github.com/cockroachdb/cockroach/pkg/sql/rowenc"
	"github.com/cockroachdb/cockroach/pkg/sql/rowenc/valueside"
	"github.com/cockroachdb/cockroach/pkg/sql/rowinfra"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/util/encoding"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

func makeArrayCFetcher() *cFetcher {
	return makeSingleColumnCFetcher(types.IntArray, 2, "vals")
}

func makeSingleColumnCFetcher(typ *types.T, colID descpb.ColumnID, name string) *cFetcher {
	evalCtx := eval.NewTestingEvalContext(cluster.MakeTestingClusterSettings())
	factory := coldataext.NewExtendedColumnFactory(evalCtx)
	batch := coldata.NewMemBatchWithCapacity([]*types.T{typ}, 1, factory)
	batch.SetLength(1)

	var colIdxMap catalog.TableColMap
	colIdxMap.Set(colID, 0)

	cf := &cFetcher{
		table: &cTableInfo{
			cFetcherTableArgs: &cFetcherTableArgs{
				spec: fetchpb.IndexFetchSpec{
					FetchedColumns: []fetchpb.IndexFetchSpec_Column{{
						ColumnID: colID,
						Name:     name,
						Type:     typ,
					}},
				},
				ColIdxMap: colIdxMap,
				typs:      []*types.T{typ},
			},
		},
	}
	cf.machine.batch = batch
	cf.machine.colvecs.SetBatch(batch)
	cf.machine.remainingValueColsByIdx.Add(0)
	return cf
}

func makeSubordinateValue(
	t *testing.T, colID descpb.ColumnID, elemIdx int, d tree.Datum,
) (roachpb.Value, []byte) {
	t.Helper()

	remaining := encoding.EncodeUvarintAscending(nil, uint64(colID))
	remaining = encoding.EncodeUvarintAscending(remaining, uint64(elemIdx))

	if d == tree.DNull {
		var value roachpb.Value
		value.SetTuple(nil)
		return value, remaining
	}

	value, err := valueside.MarshalLegacy(types.Int, d)
	require.NoError(t, err)
	return value, remaining
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

func TestProcessSubordinateValueAndFinalize(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeArrayCFetcher()

	cf.machine.nextKV.Value, _ = makeSubordinateValue(t, 2, 1, tree.NewDInt(20))
	prettyKey, prettyValue, err := cf.processSubordinateValue(
		context.Background(), cf.table, encoding.EncodeUvarintAscending(encoding.EncodeUvarintAscending(nil, 2), 1), "/tbl/1/0",
	)
	require.NoError(t, err)
	require.Equal(t, "/tbl/1/0", prettyKey)
	require.Empty(t, prettyValue)

	cf.machine.nextKV.Value, _ = makeSubordinateValue(t, 2, 0, tree.NewDInt(10))
	_, _, err = cf.processSubordinateValue(
		context.Background(), cf.table, encoding.EncodeUvarintAscending(encoding.EncodeUvarintAscending(nil, 2), 0), "/tbl/1/0",
	)
	require.NoError(t, err)

	require.NoError(t, cf.finalizeSubordinateArrays())
	arr := cf.machine.colvecs.Vecs[0].Datum().Get(0).(*tree.DArray)
	require.Equal(t, "ARRAY[10,20]", arr.String())
	require.False(t, cf.machine.colvecs.Nulls[0].NullAt(0))
	require.False(t, cf.machine.remainingValueColsByIdx.Contains(0))
	require.Empty(t, cf.subordinateArrays)
}

func TestProcessSubordinateValueHandlesNullElement(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeArrayCFetcher()

	cf.machine.nextKV.Value, _ = makeSubordinateValue(t, 2, 0, tree.DNull)
	_, _, err := cf.processSubordinateValue(
		context.Background(), cf.table, encoding.EncodeUvarintAscending(encoding.EncodeUvarintAscending(nil, 2), 0), "/tbl/1/0",
	)
	require.NoError(t, err)

	cf.machine.nextKV.Value, _ = makeSubordinateValue(t, 2, 1, tree.NewDInt(20))
	_, _, err = cf.processSubordinateValue(
		context.Background(), cf.table, encoding.EncodeUvarintAscending(encoding.EncodeUvarintAscending(nil, 2), 1), "/tbl/1/0",
	)
	require.NoError(t, err)

	require.NoError(t, cf.finalizeSubordinateArrays())
	arr := cf.machine.colvecs.Vecs[0].Datum().Get(0).(*tree.DArray)
	require.Equal(t, "ARRAY[NULL,20]", arr.String())
}

func TestProcessSubordinateValueSkipsUnknownColumn(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeArrayCFetcher()
	cf.machine.nextKV.Value, _ = makeSubordinateValue(t, 99, 0, tree.NewDInt(10))
	remaining := encoding.EncodeUvarintAscending(nil, 99)
	remaining = encoding.EncodeUvarintAscending(remaining, 0)

	prettyKey, prettyValue, err := cf.processSubordinateValue(
		context.Background(), cf.table, remaining, "/tbl/1/0",
	)
	require.NoError(t, err)
	require.Equal(t, "/tbl/1/0", prettyKey)
	require.Empty(t, prettyValue)
	require.Nil(t, cf.subordinateArrays)
	require.True(t, cf.machine.remainingValueColsByIdx.Contains(0))
}

func TestFinalizeSubordinateArraysErrorsOnGap(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeArrayCFetcher()
	cf.subordinateArrays = map[int]*subordinateArrayBuilder{
		0: newSubordinateArrayBuilder(types.Int),
	}
	cf.subordinateArrays[0].Set(1, tree.NewDInt(20))

	err := cf.finalizeSubordinateArrays()
	require.Error(t, err)
	require.Regexp(t, "missing subordinate array element 0", err.Error())
}

func TestMakeSubordinateValueNullSentinel(t *testing.T) {
	defer leaktest.AfterTest(t)()

	value, _ := makeSubordinateValue(t, 2, 0, tree.DNull)
	require.True(t, rowenc.IsSubordinateNull(value))
}

func TestProcessValueSingleDecodesAndFormats(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeSingleColumnCFetcher(types.Int, 7, "v")
	cf.traceKV = true
	cf.machine.nextKV.Value, _ = valueside.MarshalLegacy(types.Int, tree.NewDInt(42))

	prettyKey, prettyValue, err := cf.processValueSingle(context.Background(), cf.table, 7, "/tbl/1/0")
	require.NoError(t, err)
	require.Equal(t, "/tbl/1/0/v", prettyKey)
	require.Equal(t, "42", prettyValue)
	require.Equal(t, "42", cf.getDatumAt(0, 0).String())
	require.False(t, cf.machine.remainingValueColsByIdx.Contains(0))
}

func TestProcessValueSingleRejectsArrayEncoding(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeArrayCFetcher()
	cf.machine.nextKV.Value, _ = valueside.MarshalLegacy(types.Int, tree.NewDInt(42))

	_, _, err := cf.processValueSingle(context.Background(), cf.table, 2, "/tbl/1/0")
	require.Error(t, err)
	require.Regexp(t, "incompatible data layout", err.Error())
}

func TestProcessValueSingleSkipsUnknownColumn(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeSingleColumnCFetcher(types.Int, 7, "v")
	cf.machine.nextKV.Value, _ = valueside.MarshalLegacy(types.Int, tree.NewDInt(42))

	prettyKey, prettyValue, err := cf.processValueSingle(context.Background(), cf.table, 99, "/tbl/1/0")
	require.NoError(t, err)
	require.Equal(t, "/tbl/1/0", prettyKey)
	require.Empty(t, prettyValue)
	require.True(t, cf.machine.remainingValueColsByIdx.Contains(0))
}

func TestProcessValueSingleHandlesEmptyValue(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeSingleColumnCFetcher(types.Int, 7, "v")
	cf.machine.nextKV.Value = roachpb.Value{}

	prettyKey, prettyValue, err := cf.processValueSingle(context.Background(), cf.table, 7, "/tbl/1/0")
	require.NoError(t, err)
	require.Equal(t, "/tbl/1/0", prettyKey)
	require.Empty(t, prettyValue)
	require.True(t, cf.machine.remainingValueColsByIdx.Contains(0))
}

func TestWriteDecodedCols(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeSingleColumnCFetcher(types.Int, 7, "v")
	cf.machine.colvecs.Vecs[0].Int64()[0] = 42

	var buf strings.Builder
	cf.writeDecodedCols(&buf, []int{0, -1}, '/')
	require.Equal(t, "42/?", buf.String())
}

func TestSetFetcherAndEstimatedRowCount(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeArrayCFetcher()
	cf.machine.lastRowPrefix = roachpb.Key("stale")

	cf.setEstimatedRowCount(17)
	require.Equal(t, uint64(17), cf.estimatedRowCount)

	cf.machine.limitHint = int(rowinfra.RowLimit(9))
	cf.machine.lastRowPrefix = nil
	cf.machine.state[0] = stateResetBatch
	cf.machine.state[1] = stateInitFetch
	require.Nil(t, cf.machine.lastRowPrefix)
	require.Equal(t, 9, cf.machine.limitHint)
	require.Equal(t, stateResetBatch, cf.machine.state[0])
	require.Equal(t, stateInitFetch, cf.machine.state[1])
}
