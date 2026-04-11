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
	"strconv"
	"strings"
	"testing"

	"github.com/semistrict/ratel/pkg/col/coldata"
	"github.com/semistrict/ratel/pkg/col/coldataext"
	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/roachpb"
	"github.com/semistrict/ratel/pkg/settings/cluster"
	"github.com/semistrict/ratel/pkg/sql/catalog"
	"github.com/semistrict/ratel/pkg/sql/catalog/descpb"
	"github.com/semistrict/ratel/pkg/sql/opt/exec"
	"github.com/semistrict/ratel/pkg/sql/row"
	"github.com/semistrict/ratel/pkg/sql/rowenc"
	"github.com/semistrict/ratel/pkg/sql/rowenc/valueside"
	"github.com/semistrict/ratel/pkg/sql/rowinfra"
	"github.com/semistrict/ratel/pkg/sql/sem/tree"
	"github.com/semistrict/ratel/pkg/sql/types"
	jsonutil "github.com/semistrict/ratel/pkg/util/json"
	"github.com/semistrict/ratel/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

const jsonPathStepKeyOrIndexPrefix = "p:"

func makeArrayCFetcher() *cFetcher {
	return makeSingleColumnCFetcher(types.IntArray, 2, "vals")
}

func makeJSONCFetcher() *cFetcher {
	return makeSingleColumnCFetcher(types.Jsonb, 3, "doc")
}

func makeSingleColumnCFetcher(typ *types.T, colID descpb.ColumnID, name string) *cFetcher {
	return makeSingleColumnCFetcherWithOutputs(typ, colID, name)
}

func makeSingleColumnCFetcherWithOutputs(
	typ *types.T, colID descpb.ColumnID, name string, outputTyps ...*types.T,
) *cFetcher {
	evalCtx := tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings())
	factory := coldataext.NewExtendedColumnFactory(evalCtx)
	allTyps := append([]*types.T{typ}, outputTyps...)
	batch := coldata.NewMemBatchWithCapacity(allTyps, 1, factory)
	batch.SetLength(1)

	var colMap catalog.TableColMap
	colMap.Set(colID, 0)

	cf := &cFetcher{
		table: &cTableInfo{
			cFetcherTableArgs: &cFetcherTableArgs{
				spec: descpb.IndexFetchSpec{
					FetchedColumns: []descpb.IndexFetchSpec_Column{{
						ColumnID: colID,
						Name:     name,
						Type:     typ,
					}},
				},
				ColIdxMap: colMap,
				typs:      allTyps,
			},
			orderedColIdxMap: &colIdxMap{
				vals: []descpb.ColumnID{colID},
				ords: []int{0},
			},
		},
	}
	cf.machine.batch = batch
	cf.machine.colvecs.SetBatch(batch)
	cf.machine.remainingValueColsByIdx.Add(0)
	return cf
}

func makeTwoColumnCFetcher(
	firstType *types.T,
	firstID descpb.ColumnID,
	firstName string,
	secondType *types.T,
	secondID descpb.ColumnID,
	secondName string,
) *cFetcher {
	evalCtx := tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings())
	factory := coldataext.NewExtendedColumnFactory(evalCtx)
	allTyps := []*types.T{firstType, secondType}
	batch := coldata.NewMemBatchWithCapacity(allTyps, 1, factory)
	batch.SetLength(1)

	var colMap catalog.TableColMap
	colMap.Set(firstID, 0)
	colMap.Set(secondID, 1)

	cf := &cFetcher{
		table: &cTableInfo{
			cFetcherTableArgs: &cFetcherTableArgs{
				spec: descpb.IndexFetchSpec{
					FetchedColumns: []descpb.IndexFetchSpec_Column{
						{ColumnID: firstID, Name: firstName, Type: firstType},
						{ColumnID: secondID, Name: secondName, Type: secondType},
					},
				},
				ColIdxMap: colMap,
				typs:      allTyps,
			},
			orderedColIdxMap: &colIdxMap{
				vals: []descpb.ColumnID{firstID, secondID},
				ords: []int{0, 1},
			},
		},
	}
	cf.machine.batch = batch
	cf.machine.colvecs.SetBatch(batch)
	cf.machine.remainingValueColsByIdx.Add(0)
	cf.machine.remainingValueColsByIdx.Add(1)
	cf.table.neededValueColsByIdx.Add(0)
	cf.table.neededValueColsByIdx.Add(1)
	return cf
}

func makeSubordinateValue(
	t *testing.T, colID descpb.ColumnID, elemIdx int, d tree.Datum,
) (roachpb.Value, []byte) {
	t.Helper()

	if d == tree.DNull {
		var value roachpb.Value
		value.SetTuple(nil)
		return value, nil
	}

	value, err := valueside.MarshalLegacy(types.Int, d)
	require.NoError(t, err)
	return value, nil
}

func makeSubordinateJSONValue(
	t *testing.T, colID descpb.ColumnID, path []keys.SubordinatePathSegment, s string,
) (roachpb.Value, []byte) {
	t.Helper()

	j, err := jsonutil.ParseJSON(s)
	require.NoError(t, err)
	value, err := rowenc.EncodeSubordinateJSONValue(j)
	require.NoError(t, err)
	return value, nil
}

func encodeJSONQueryPath(steps ...string) []string {
	encoded := make([]string, len(steps))
	for i, step := range steps {
		encoded[i] = jsonPathStepKeyOrIndexPrefix + strconv.Quote(step)
	}
	return encoded
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

	rowKey := keys.MakeFamilyKey(keys.SystemSQLCodec.IndexPrefix(1, 1), 0)
	cf.machine.nextKV.Key = keys.MakeSubordinateKey(rowKey, 2, 1)
	cf.machine.nextKV.Value, _ = makeSubordinateValue(t, 2, 1, tree.NewDInt(20))
	prettyKey, prettyValue, err := cf.processSubordinateValue(
		context.Background(), cf.table, nil, "/tbl/1/0",
	)
	require.NoError(t, err)
	require.Equal(t, "/tbl/1/0", prettyKey)
	require.Empty(t, prettyValue)

	cf.machine.nextKV.Key = keys.MakeSubordinateKey(rowKey, 2, 0)
	cf.machine.nextKV.Value, _ = makeSubordinateValue(t, 2, 0, tree.NewDInt(10))
	_, _, err = cf.processSubordinateValue(
		context.Background(), cf.table, nil, "/tbl/1/0",
	)
	require.NoError(t, err)

	require.NoError(t, cf.finalizeSubordinateValues())
	arr := cf.machine.colvecs.Vecs[0].Datum().Get(0).(*tree.DArray)
	require.Equal(t, "ARRAY[10,20]", arr.String())
	require.False(t, cf.machine.colvecs.Nulls[0].NullAt(0))
	require.False(t, cf.machine.remainingValueColsByIdx.Contains(0))
	require.Empty(t, cf.subordinateArrays)
}

func TestProcessSubordinateValueHandlesNullElement(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeArrayCFetcher()

	rowKey := keys.MakeFamilyKey(keys.SystemSQLCodec.IndexPrefix(1, 1), 0)
	cf.machine.nextKV.Key = keys.MakeSubordinateKey(rowKey, 2, 0)
	cf.machine.nextKV.Value, _ = makeSubordinateValue(t, 2, 0, tree.DNull)
	_, _, err := cf.processSubordinateValue(
		context.Background(), cf.table, nil, "/tbl/1/0",
	)
	require.NoError(t, err)

	cf.machine.nextKV.Key = keys.MakeSubordinateKey(rowKey, 2, 1)
	cf.machine.nextKV.Value, _ = makeSubordinateValue(t, 2, 1, tree.NewDInt(20))
	_, _, err = cf.processSubordinateValue(
		context.Background(), cf.table, nil, "/tbl/1/0",
	)
	require.NoError(t, err)

	require.NoError(t, cf.finalizeSubordinateValues())
	arr := cf.machine.colvecs.Vecs[0].Datum().Get(0).(*tree.DArray)
	require.Equal(t, "ARRAY[NULL,20]", arr.String())
}

func TestProcessSubordinateValueSkipsUnknownColumn(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeArrayCFetcher()
	rowKey := keys.MakeFamilyKey(keys.SystemSQLCodec.IndexPrefix(1, 1), 0)
	cf.machine.nextKV.Key = keys.MakeSubordinateKey(rowKey, 99, 0)
	cf.machine.nextKV.Value, _ = makeSubordinateValue(t, 99, 0, tree.NewDInt(10))

	prettyKey, prettyValue, err := cf.processSubordinateValue(
		context.Background(), cf.table, nil, "/tbl/1/0",
	)
	require.NoError(t, err)
	require.Equal(t, "/tbl/1/0", prettyKey)
	require.Empty(t, prettyValue)
	require.Nil(t, cf.subordinateArrays)
	require.True(t, cf.machine.remainingValueColsByIdx.Contains(0))
}

func TestSubordinateJSONBuilderMaterialize(t *testing.T) {
	defer leaktest.AfterTest(t)()

	builder := &subordinateJSONBuilder{}
	require.NoError(t, builder.Set([]keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, subordinateJSONNodeObject, nil))
	scalar, err := jsonutil.ParseJSON(`1`)
	require.NoError(t, err)
	require.NoError(t, builder.Set([]keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathHeader},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
	}, subordinateJSONNodeScalar, scalar))
	arrNode, err := jsonutil.ParseJSON(`[]`)
	require.NoError(t, err)
	require.NoError(t, builder.Set([]keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathHeader},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
	}, subordinateJSONNodeArray, arrNode))
	elem, err := jsonutil.ParseJSON(`"x"`)
	require.NoError(t, err)
	require.NoError(t, builder.Set([]keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathHeader},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
		{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0},
	}, subordinateJSONNodeScalar, elem))

	j, err := builder.Materialize()
	require.NoError(t, err)
	require.JSONEq(t, `{"a":1,"b":["x"]}`, j.JSON.String())
}

func TestProcessSubordinateJSONValueAndFinalize(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeJSONCFetcher()
	rowKey := keys.MakeFamilyKey(keys.SystemSQLCodec.IndexPrefix(1, 1), 0)

	cf.machine.nextKV.Key = keys.MakeSubordinatePathKey(rowKey, 3, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}})
	cf.machine.nextKV.Value, _ = makeSubordinateJSONValue(t, 3, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `{}`)
	_, _, err := cf.processSubordinateValue(context.Background(), cf.table, nil, "/tbl/1/0")
	require.NoError(t, err)

	cf.machine.nextKV.Key = keys.MakeSubordinatePathKey(rowKey, 3, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
	})
	cf.machine.nextKV.Value, _ = makeSubordinateJSONValue(t, 3, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
	}, `1`)
	_, _, err = cf.processSubordinateValue(context.Background(), cf.table, nil, "/tbl/1/0")
	require.NoError(t, err)

	cf.machine.nextKV.Key = keys.MakeSubordinatePathKey(rowKey, 3, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathHeader},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
	})
	cf.machine.nextKV.Value, _ = makeSubordinateJSONValue(t, 3, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathHeader},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
	}, `[]`)
	_, _, err = cf.processSubordinateValue(context.Background(), cf.table, nil, "/tbl/1/0")
	require.NoError(t, err)

	cf.machine.nextKV.Key = keys.MakeSubordinatePathKey(rowKey, 3, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathHeader},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
		{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0},
	})
	cf.machine.nextKV.Value, _ = makeSubordinateJSONValue(t, 3, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathHeader},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
		{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0},
	}, `true`)
	_, _, err = cf.processSubordinateValue(context.Background(), cf.table, nil, "/tbl/1/0")
	require.NoError(t, err)

	require.NoError(t, cf.finalizeSubordinateValues())
	doc := cf.machine.colvecs.Vecs[0].JSON().Get(0)
	require.JSONEq(t, `{"a":1,"b":[true]}`, doc.String())
	require.False(t, cf.machine.colvecs.Nulls[0].NullAt(0))
	require.False(t, cf.machine.remainingValueColsByIdx.Contains(0))
	require.Empty(t, cf.subordinateJSONBuilders)
}

func TestJSONExistsFilterNoMaterialize(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeJSONCFetcher()
	require.NoError(t, cf.ConfigureJSONExistsFilter(0, row.JSONAccessExists, "a", nil, false))
	cf.resetJSONProgramsForRow()

	rowKey := keys.MakeFamilyKey(keys.SystemSQLCodec.IndexPrefix(1, 1), 0)
	cf.machine.nextKV.Key = keys.MakeSubordinatePathKey(rowKey, 3, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}})
	cf.machine.nextKV.Value, _ = makeSubordinateJSONValue(t, 3, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `"a"`)
	_, _, err := cf.processSubordinateValue(context.Background(), cf.table, nil, "/tbl/1/0")
	require.NoError(t, err)

	require.NoError(t, cf.finishJSONExistsFilter())
	require.True(t, cf.lastRowPassesJSONExistsFilter)
	require.True(t, cf.jsonExistsFilter.shared.haveCached)
	require.Equal(t, tree.DBoolTrue, cf.jsonExistsFilter.shared.cachedResult)
	require.Nil(t, cf.subordinateJSONBuilders)
}

func TestConfigureJSONAccessProgramsSharesPathStateWithJSONPathCompareFilter(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeJSONCFetcher()
	encodedPath := []string{
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("a"),
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("b"),
	}
	require.NoError(t, cf.ConfigureJSONPathCompareFilter(
		tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings()),
		0,
		row.JSONAccessFetchTextPath,
		encodedPath,
		exec.JSONPathFilterEq,
		tree.NewDString("20"),
		false, /* materialize */
	))
	require.NoError(t, cf.ConfigureJSONAccessPrograms([]row.JSONAccessSpec{{
		ColIdx:      0,
		Kind:        row.JSONAccessFetchJSONPath,
		Path:        encodedPath,
		Materialize: false,
	}}, 0))

	require.Len(t, cf.jsonSharedAccessPrograms, 1)
	require.NotNil(t, cf.jsonPathCompareFilter)
	require.Len(t, cf.jsonAccessPrograms, 1)
	require.Same(t, cf.jsonPathCompareFilter.shared, cf.jsonAccessPrograms[0].shared)
}

func TestSubordinateJSONRowHeadLookupSpecs(t *testing.T) {
	defer leaktest.AfterTest(t)()

	t.Run("static object prefix before negative index", func(t *testing.T) {
		cf := makeJSONCFetcher()
		cf.hasSubordinateColumns = true
		cf.table.spec.MaxKeysPerRow = 1
		require.NoError(t, cf.ConfigureJSONPathCompareFilter(
			tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings()),
			0,
			row.JSONAccessFetchTextPath,
			[]string{
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("needle"),
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("-1"),
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("tiny"),
			},
			exec.JSONPathFilterEq,
			tree.NewDString("x"),
			false, /* materialize */
		))

		lookups, ok, err := cf.subordinateJSONRowHeadLookupSpecs()
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, []row.SubordinateJSONRowLookupSpec{{
			ColID: 3,
			SelectedPaths: [][]keys.SubordinatePathSegment{{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "needle"},
			}},
		}}, lookups)
	})

	t.Run("allows reverse scans with static prefix", func(t *testing.T) {
		cf := makeJSONCFetcher()
		cf.hasSubordinateColumns = true
		cf.reverse = true
		cf.table.spec.MaxKeysPerRow = 1
		require.NoError(t, cf.ConfigureJSONPathCompareFilter(
			tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings()),
			0,
			row.JSONAccessFetchTextPath,
			[]string{
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("needle"),
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("-1"),
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("tiny"),
			},
			exec.JSONPathFilterEq,
			tree.NewDString("x"),
			false, /* materialize */
		))

		lookups, ok, err := cf.subordinateJSONRowHeadLookupSpecs()
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, []row.SubordinateJSONRowLookupSpec{{
			ColID: 3,
			SelectedPaths: [][]keys.SubordinatePathSegment{{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "needle"},
			}},
		}}, lookups)
	})

	t.Run("rejects ambiguous numeric key-or-index prefix", func(t *testing.T) {
		cf := makeJSONCFetcher()
		cf.hasSubordinateColumns = true
		cf.table.spec.MaxKeysPerRow = 1
		require.NoError(t, cf.ConfigureJSONPathCompareFilter(
			tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings()),
			0,
			row.JSONAccessFetchTextPath,
			[]string{
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("0"),
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("tiny"),
			},
			exec.JSONPathFilterEq,
			tree.NewDString("x"),
			false, /* materialize */
		))

		lookups, ok, err := cf.subordinateJSONRowHeadLookupSpecs()
		require.NoError(t, err)
		require.False(t, ok)
		require.Nil(t, lookups)
	})

	t.Run("includes exists keys without selected paths", func(t *testing.T) {
		cf := makeJSONCFetcher()
		cf.hasSubordinateColumns = true
		cf.table.spec.MaxKeysPerRow = 1
		require.NoError(t, cf.ConfigureJSONExistsFilter(0, row.JSONAccessExistsAny, "", []string{"b", "a"}, false))

		lookups, ok, err := cf.subordinateJSONRowHeadLookupSpecs()
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, []row.SubordinateJSONRowLookupSpec{{
			ColID:         3,
			SelectedPaths: nil,
			ExistsKeys:    []string{"a", "b"},
		}}, lookups)
	})

	t.Run("rejects projected source json even when max keys per row is one", func(t *testing.T) {
		cf := makeJSONCFetcher()
		cf.hasSubordinateColumns = true
		cf.table.spec.MaxKeysPerRow = 1
		require.NoError(t, cf.ConfigureJSONExistsFilter(0, row.JSONAccessExists, "a", nil, true /* materialize */))

		lookups, ok, err := cf.subordinateJSONRowHeadLookupSpecs()
		require.NoError(t, err)
		require.False(t, ok)
		require.Nil(t, lookups)
	})

	t.Run("allows row head lookup with no json programs when only non-subordinate values are needed", func(t *testing.T) {
		cf := makeSingleColumnCFetcher(types.String, 4, "pad")
		cf.hasSubordinateColumns = true
		cf.table.spec.MaxKeysPerRow = 0

		lookups, ok, err := cf.subordinateJSONRowHeadLookupSpecs()
		require.NoError(t, err)
		require.True(t, ok)
		require.Nil(t, lookups)
	})

	t.Run("allows max keys per row when all needed value cols are looked-up json", func(t *testing.T) {
		cf := makeJSONCFetcher()
		cf.hasSubordinateColumns = true
		cf.table.spec.MaxKeysPerRow = 2
		require.NoError(t, cf.ConfigureJSONPathCompareFilter(
			tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings()),
			0,
			row.JSONAccessFetchTextPath,
			[]string{
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("needle"),
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("-1"),
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("tiny"),
			},
			exec.JSONPathFilterEq,
			tree.NewDString("x"),
			false, /* materialize */
		))

		lookups, ok, err := cf.subordinateJSONRowHeadLookupSpecs()
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, []row.SubordinateJSONRowLookupSpec{{
			ColID: 3,
			SelectedPaths: [][]keys.SubordinatePathSegment{{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "needle"},
			}},
		}}, lookups)
	})

	t.Run("rejects max keys per row when non-json value column is also needed", func(t *testing.T) {
		cf := makeTwoColumnCFetcher(types.Jsonb, 3, "doc", types.String, 4, "pad")
		cf.hasSubordinateColumns = true
		cf.table.spec.MaxKeysPerRow = 2
		require.NoError(t, cf.ConfigureJSONPathCompareFilter(
			tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings()),
			0,
			row.JSONAccessFetchTextPath,
			[]string{
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("needle"),
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("-1"),
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("tiny"),
			},
			exec.JSONPathFilterEq,
			tree.NewDString("x"),
			false, /* materialize */
		))

		lookups, ok, err := cf.subordinateJSONRowHeadLookupSpecs()
		require.NoError(t, err)
		require.False(t, ok)
		require.Nil(t, lookups)
	})

	t.Run("rejects max keys per row when source json column must be materialized", func(t *testing.T) {
		cf := makeJSONCFetcher()
		cf.hasSubordinateColumns = true
		cf.table.spec.MaxKeysPerRow = 2
		require.NoError(t, cf.ConfigureJSONPathCompareFilter(
			tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings()),
			0,
			row.JSONAccessFetchTextPath,
			[]string{
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("needle"),
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("-1"),
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("tiny"),
			},
			exec.JSONPathFilterEq,
			tree.NewDString("x"),
			true, /* materialize */
		))

		lookups, ok, err := cf.subordinateJSONRowHeadLookupSpecs()
		require.NoError(t, err)
		require.False(t, ok)
		require.Nil(t, lookups)
	})
}

func BenchmarkSubordinateJSONRowHeadLookupSpecs(b *testing.B) {
	encodedPath := []string{
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("needle"),
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("-1"),
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("tiny"),
	}
	evalCtx := tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings())

	b.Run("max_keys_per_row_1", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cf := makeJSONCFetcher()
			cf.hasSubordinateColumns = true
			cf.table.spec.MaxKeysPerRow = 1
			require.NoError(b, cf.ConfigureJSONPathCompareFilter(
				evalCtx, 0, row.JSONAccessFetchTextPath, encodedPath, exec.JSONPathFilterEq, tree.NewDString("x"), false,
			))
			lookups, ok, err := cf.subordinateJSONRowHeadLookupSpecs()
			require.NoError(b, err)
			require.True(b, ok)
			require.Len(b, lookups, 1)
		}
	})

	b.Run("max_keys_per_row_2_json_only", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cf := makeJSONCFetcher()
			cf.hasSubordinateColumns = true
			cf.table.spec.MaxKeysPerRow = 2
			require.NoError(b, cf.ConfigureJSONPathCompareFilter(
				evalCtx, 0, row.JSONAccessFetchTextPath, encodedPath, exec.JSONPathFilterEq, tree.NewDString("x"), false,
			))
			lookups, ok, err := cf.subordinateJSONRowHeadLookupSpecs()
			require.NoError(b, err)
			require.True(b, ok)
			require.Len(b, lookups, 1)
		}
	})
}

func TestConfigureJSONContainsFilterSharesSelectedPathStateWithJSONAccessPrograms(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeJSONCFetcher()
	encodedPath := []string{
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("a"),
	}
	right, err := jsonutil.ParseJSON(`{"b":[30]}`)
	require.NoError(t, err)
	require.NoError(t, cf.ConfigureJSONContainsFilter(0, encodedPath, false, right, false))
	require.NoError(t, cf.ConfigureJSONAccessPrograms([]row.JSONAccessSpec{{
		ColIdx:      0,
		Kind:        row.JSONAccessFetchJSONPath,
		Path:        encodedPath,
		Materialize: false,
	}}, 0))

	require.Len(t, cf.jsonSharedSelectedPaths, 1)
	require.Len(t, cf.jsonContainsFilters, 1)
	require.NotNil(t, cf.jsonContainsFilters[0].selected)
	require.Len(t, cf.jsonAccessPrograms, 1)
	require.Same(t, cf.jsonContainsFilters[0].selected, cf.jsonAccessPrograms[0].shared.selected)
	require.Len(t, cf.jsonContainsFilters[0].selected.contains, 1)
	require.Len(t, cf.jsonContainsFilters[0].selected.access, 1)
}

func observeSelectedJSONPathStateForCFetcherTests(t *testing.T, selected *cSharedJSONSelectedPathState) {
	t.Helper()

	require.NoError(t, selected.observe(
		[]keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}},
		rowenc.SubordinateJSONObject,
		1,
		nil,
	))
	scalar, err := jsonutil.ParseJSON(`"x"`)
	require.NoError(t, err)
	require.NoError(t, selected.observe(
		[]keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}},
		rowenc.SubordinateJSONScalar,
		0,
		scalar,
	))
}

func TestSharedJSONAccessProgramStateCachesResultDatums(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeJSONCFetcher()
	encodedPath := []string{jsonPathStepKeyOrIndexPrefix + strconv.Quote("a")}
	require.NoError(t, cf.ConfigureJSONAccessPrograms([]row.JSONAccessSpec{
		{
			ColIdx:      0,
			Kind:        row.JSONAccessFetchJSONPath,
			Path:        encodedPath,
			Materialize: false,
		},
		{
			ColIdx:      0,
			Kind:        row.JSONAccessFetchTextPath,
			Path:        encodedPath,
			Materialize: false,
		},
	}, 1 /* outputStartIdx */))

	require.Len(t, cf.jsonSharedAccessPrograms, 1)
	shared := cf.jsonSharedAccessPrograms[0]
	require.NotNil(t, shared.selected)

	observeSelectedJSONPathStateForCFetcherTests(t, shared.selected)

	json1, err := shared.resultDatum(row.JSONAccessFetchJSONPath)
	require.NoError(t, err)
	json2, err := shared.resultDatum(row.JSONAccessFetchJSONPath)
	require.NoError(t, err)
	require.Same(t, json1, json2)
	require.False(t, shared.haveCached)

	text1, err := shared.resultDatum(row.JSONAccessFetchTextPath)
	require.NoError(t, err)
	text2, err := shared.resultDatum(row.JSONAccessFetchTextPath)
	require.NoError(t, err)
	require.Same(t, text1, text2)
	require.False(t, shared.haveCached)

	shared.reset()
	require.False(t, shared.haveCached)
	observeSelectedJSONPathStateForCFetcherTests(t, shared.selected)
	text3, err := shared.resultDatum(row.JSONAccessFetchTextPath)
	require.NoError(t, err)
	require.Equal(t, text1, text3)
}

func TestSharedJSONSelectedPathStateCachesMaterializedJSONAcrossKinds(t *testing.T) {
	defer leaktest.AfterTest(t)()

	selected := &cSharedJSONSelectedPathState{
		selector: row.NewJSONSelectedPathState([]string{jsonPathStepKeyOrIndexPrefix + strconv.Quote("a")}),
		access:   []*cSharedJSONAccessProgramState{{}},
	}
	observeSelectedJSONPathStateForCFetcherTests(t, selected)

	jsonDatum, err := selected.resultDatum(row.JSONAccessFetchJSONPath)
	require.NoError(t, err)
	require.NotEqual(t, tree.DNull, jsonDatum)

	selected.builder = nil

	textDatum, err := selected.resultDatum(row.JSONAccessFetchTextPath)
	require.NoError(t, err)
	require.Equal(t, tree.NewDString("x"), textDatum)
}

func TestJSONExistsAllFilterNoMaterialize(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeJSONCFetcher()
	require.NoError(t, cf.ConfigureJSONExistsFilter(
		0, row.JSONAccessExistsAll, "", []string{"a", "b"}, false,
	))
	cf.resetJSONProgramsForRow()

	rowKey := keys.MakeFamilyKey(keys.SystemSQLCodec.IndexPrefix(1, 1), 0)
	for _, tc := range []struct {
		path []keys.SubordinatePathSegment
		json string
	}{
		{
			path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}},
			json: `{}`,
		},
		{
			path: []keys.SubordinatePathSegment{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
			},
			json: `1`,
		},
		{
			path: []keys.SubordinatePathSegment{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
			},
			json: `2`,
		},
	} {
		cf.machine.nextKV.Key = keys.MakeSubordinatePathKey(rowKey, 3, tc.path)
		cf.machine.nextKV.Value, _ = makeSubordinateJSONValue(t, 3, tc.path, tc.json)
		_, _, err := cf.processSubordinateValue(context.Background(), cf.table, nil, "/tbl/1/0")
		require.NoError(t, err)
	}

	require.NoError(t, cf.finishJSONExistsFilter())
	require.True(t, cf.lastRowPassesJSONExistsFilter)
	require.Nil(t, cf.subordinateJSONBuilders)
}

func TestJSONPathCompareFilterNoMaterialize(t *testing.T) {
	defer leaktest.AfterTest(t)()

	evalCtx := tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings())
	cf := makeJSONCFetcher()
	require.NoError(t, cf.ConfigureJSONPathCompareFilter(
		evalCtx, 0, row.JSONAccessFetchTextPath, encodeJSONQueryPath("a", "-1"), exec.JSONPathFilterEq, tree.NewDString("20"), false,
	))
	cf.resetJSONProgramsForRow()

	rowKey := keys.MakeFamilyKey(keys.SystemSQLCodec.IndexPrefix(1, 1), 0)
	cf.machine.nextKV.Key = keys.MakeSubordinatePathKey(rowKey, 3, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}})
	cf.machine.nextKV.Value, _ = makeSubordinateJSONValue(t, 3, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `{}`)
	_, _, err := cf.processSubordinateValue(context.Background(), cf.table, nil, "/tbl/1/0")
	require.NoError(t, err)

	for _, tc := range []struct {
		path []keys.SubordinatePathSegment
		json string
	}{
		{
			path: []keys.SubordinatePathSegment{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
			},
			json: `[10,20]`,
		},
		{
			path: []keys.SubordinatePathSegment{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
				{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1},
			},
			json: `20`,
		},
	} {
		cf.machine.nextKV.Key = keys.MakeSubordinatePathKey(rowKey, 3, tc.path)
		cf.machine.nextKV.Value, _ = makeSubordinateJSONValue(t, 3, tc.path, tc.json)
		_, _, err = cf.processSubordinateValue(context.Background(), cf.table, nil, "/tbl/1/0")
		require.NoError(t, err)
	}

	require.NoError(t, cf.finishJSONPathCompareFilter())
	require.True(t, cf.lastRowPassesJSONPathCompareFilter)
	require.False(t, cf.jsonPathCompareFilter.shared.haveCached)
	d, err := cf.jsonPathCompareFilter.shared.resultDatum(row.JSONAccessFetchTextPath)
	require.NoError(t, err)
	require.Equal(t, tree.NewDString("20"), d)
	require.Nil(t, cf.subordinateJSONBuilders)
}

func TestJSONPathCompareIsNullFilterNoMaterialize(t *testing.T) {
	defer leaktest.AfterTest(t)()

	evalCtx := tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings())
	cf := makeJSONCFetcher()
	require.NoError(t, cf.ConfigureJSONPathCompareFilter(
		evalCtx, 0, row.JSONAccessFetchTextPath, encodeJSONQueryPath("a", "b", "1"),
		exec.JSONPathFilterIsNull, tree.DNull, false,
	))
	cf.resetJSONProgramsForRow()

	rowKey := keys.MakeFamilyKey(keys.SystemSQLCodec.IndexPrefix(1, 1), 0)
	for _, tc := range []struct {
		path []keys.SubordinatePathSegment
		json string
	}{
		{
			path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}},
			json: `{}`,
		},
		{
			path: []keys.SubordinatePathSegment{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
			},
			json: `{}`,
		},
		{
			path: []keys.SubordinatePathSegment{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
			},
			json: `[10, null]`,
		},
		{
			path: []keys.SubordinatePathSegment{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
				{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0},
			},
			json: `10`,
		},
		{
			path: []keys.SubordinatePathSegment{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
				{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1},
			},
			json: `null`,
		},
	} {
		cf.machine.nextKV.Key = keys.MakeSubordinatePathKey(rowKey, 3, tc.path)
		cf.machine.nextKV.Value, _ = makeSubordinateJSONValue(t, 3, tc.path, tc.json)
		_, _, err := cf.processSubordinateValue(context.Background(), cf.table, nil, "/tbl/1/0")
		require.NoError(t, err)
	}

	require.NoError(t, cf.finishJSONPathCompareFilter())
	require.True(t, cf.lastRowPassesJSONPathCompareFilter)
	require.Nil(t, cf.subordinateJSONBuilders)
}

func TestJSONContainsFilterNoMaterialize(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeJSONCFetcher()
	right, err := jsonutil.ParseJSON(`{"b":[20]}`)
	require.NoError(t, err)
	require.NoError(t, cf.ConfigureJSONContainsFilter(
		0, encodeJSONQueryPath("a"), false, right, false,
	))
	cf.resetJSONProgramsForRow()

	rowKey := keys.MakeFamilyKey(keys.SystemSQLCodec.IndexPrefix(1, 1), 0)
	for _, tc := range []struct {
		path []keys.SubordinatePathSegment
		json string
	}{
		{
			path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}},
			json: `{}`,
		},
		{
			path: []keys.SubordinatePathSegment{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
			},
			json: `{"b":[10,20]}`,
		},
		{
			path: []keys.SubordinatePathSegment{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
			},
			json: `[10,20]`,
		},
		{
			path: []keys.SubordinatePathSegment{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
				{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0},
			},
			json: `10`,
		},
		{
			path: []keys.SubordinatePathSegment{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
				{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1},
			},
			json: `20`,
		},
	} {
		cf.machine.nextKV.Key = keys.MakeSubordinatePathKey(rowKey, 3, tc.path)
		cf.machine.nextKV.Value, _ = makeSubordinateJSONValue(t, 3, tc.path, tc.json)
		_, _, err := cf.processSubordinateValue(context.Background(), cf.table, nil, "/tbl/1/0")
		require.NoError(t, err)
	}

	require.NoError(t, cf.finishJSONContainsFilter())
	require.True(t, cf.lastRowPassesJSONContainsFilter)
	require.Nil(t, cf.subordinateJSONBuilders)
}

func TestJSONContainedByFilterNoMaterialize(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeJSONCFetcher()
	right, err := jsonutil.ParseJSON(`{"b":[10,20],"extra":true}`)
	require.NoError(t, err)
	require.NoError(t, cf.ConfigureJSONContainsFilter(
		0, encodeJSONQueryPath("a"), true, right, false,
	))
	cf.resetJSONProgramsForRow()

	rowKey := keys.MakeFamilyKey(keys.SystemSQLCodec.IndexPrefix(1, 1), 0)
	for _, tc := range []struct {
		path []keys.SubordinatePathSegment
		json string
	}{
		{
			path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}},
			json: `{}`,
		},
		{
			path: []keys.SubordinatePathSegment{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
			},
			json: `{"b":[10,20]}`,
		},
		{
			path: []keys.SubordinatePathSegment{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
			},
			json: `[10,20]`,
		},
		{
			path: []keys.SubordinatePathSegment{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
				{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0},
			},
			json: `10`,
		},
		{
			path: []keys.SubordinatePathSegment{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
				{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1},
			},
			json: `20`,
		},
	} {
		cf.machine.nextKV.Key = keys.MakeSubordinatePathKey(rowKey, 3, tc.path)
		cf.machine.nextKV.Value, _ = makeSubordinateJSONValue(t, 3, tc.path, tc.json)
		_, _, err := cf.processSubordinateValue(context.Background(), cf.table, nil, "/tbl/1/0")
		require.NoError(t, err)
	}

	require.NoError(t, cf.finishJSONContainsFilter())
	require.True(t, cf.lastRowPassesJSONContainsFilter)
	require.Nil(t, cf.subordinateJSONBuilders)
}

func TestJSONContainsArrayOfObjectsFilterNoMaterialize(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeJSONCFetcher()
	right, err := jsonutil.ParseJSON(`[{"x":1},{"y":2}]`)
	require.NoError(t, err)
	require.NoError(t, cf.ConfigureJSONContainsFilter(
		0, encodeJSONQueryPath("a"), false, right, false,
	))
	cf.resetJSONProgramsForRow()

	rowKey := keys.MakeFamilyKey(keys.SystemSQLCodec.IndexPrefix(1, 1), 0)
	for _, tc := range []struct {
		path []keys.SubordinatePathSegment
		json string
	}{
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, json: `{}`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, json: `[{"x":1,"extra":9},{"y":2}]`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}, {Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0}}, json: `{"x":1,"extra":9}`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}, {Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0}, {Kind: keys.SubordinatePathObjectKey, ObjectKey: "extra"}}, json: `9`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}, {Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0}, {Kind: keys.SubordinatePathObjectKey, ObjectKey: "x"}}, json: `1`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}, {Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1}}, json: `{"y":2}`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}, {Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1}, {Kind: keys.SubordinatePathObjectKey, ObjectKey: "y"}}, json: `2`},
	} {
		cf.machine.nextKV.Key = keys.MakeSubordinatePathKey(rowKey, 3, tc.path)
		cf.machine.nextKV.Value, _ = makeSubordinateJSONValue(t, 3, tc.path, tc.json)
		_, _, err := cf.processSubordinateValue(context.Background(), cf.table, nil, "/tbl/1/0")
		require.NoError(t, err)
	}

	require.NoError(t, cf.finishJSONContainsFilter())
	require.True(t, cf.lastRowPassesJSONContainsFilter)
	require.Nil(t, cf.subordinateJSONBuilders)
}

func TestJSONContainedByArrayOfObjectsFilterNoMaterialize(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeJSONCFetcher()
	right, err := jsonutil.ParseJSON(`[{"x":1,"z":0},{"y":2}]`)
	require.NoError(t, err)
	require.NoError(t, cf.ConfigureJSONContainsFilter(
		0, encodeJSONQueryPath("a"), true, right, false,
	))
	cf.resetJSONProgramsForRow()

	rowKey := keys.MakeFamilyKey(keys.SystemSQLCodec.IndexPrefix(1, 1), 0)
	for _, tc := range []struct {
		path []keys.SubordinatePathSegment
		json string
	}{
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, json: `{}`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, json: `[{"x":1},{"y":2}]`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}, {Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0}}, json: `{"x":1}`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}, {Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0}, {Kind: keys.SubordinatePathObjectKey, ObjectKey: "x"}}, json: `1`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}, {Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1}}, json: `{"y":2}`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}, {Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1}, {Kind: keys.SubordinatePathObjectKey, ObjectKey: "y"}}, json: `2`},
	} {
		cf.machine.nextKV.Key = keys.MakeSubordinatePathKey(rowKey, 3, tc.path)
		cf.machine.nextKV.Value, _ = makeSubordinateJSONValue(t, 3, tc.path, tc.json)
		_, _, err := cf.processSubordinateValue(context.Background(), cf.table, nil, "/tbl/1/0")
		require.NoError(t, err)
	}

	require.NoError(t, cf.finishJSONContainsFilter())
	require.True(t, cf.lastRowPassesJSONContainsFilter)
	require.Nil(t, cf.subordinateJSONBuilders)
}

func TestJSONContainsNullAndEmptyFilterNoMaterialize(t *testing.T) {
	defer leaktest.AfterTest(t)()

	t.Run("json-null", func(t *testing.T) {
		cf := makeJSONCFetcher()
		right, err := jsonutil.ParseJSON(`null`)
		require.NoError(t, err)
		require.NoError(t, cf.ConfigureJSONContainsFilter(0, nil, false, right, false))
		cf.resetJSONProgramsForRow()

		rowKey := keys.MakeFamilyKey(keys.SystemSQLCodec.IndexPrefix(1, 1), 0)
		cf.machine.nextKV.Key = keys.MakeSubordinatePathKey(rowKey, 3, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}})
		cf.machine.nextKV.Value, _ = makeSubordinateJSONValue(t, 3, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `null`)
		_, _, err = cf.processSubordinateValue(context.Background(), cf.table, nil, "/tbl/1/0")
		require.NoError(t, err)

		require.NoError(t, cf.finishJSONContainsFilter())
		require.True(t, cf.lastRowPassesJSONContainsFilter)
		require.Nil(t, cf.subordinateJSONBuilders)
	})

	t.Run("empty-object-contained-by", func(t *testing.T) {
		cf := makeJSONCFetcher()
		right, err := jsonutil.ParseJSON(`{}`)
		require.NoError(t, err)
		require.NoError(t, cf.ConfigureJSONContainsFilter(0, encodeJSONQueryPath("a"), true, right, false))
		cf.resetJSONProgramsForRow()

		rowKey := keys.MakeFamilyKey(keys.SystemSQLCodec.IndexPrefix(1, 1), 0)
		for _, tc := range []struct {
			path []keys.SubordinatePathSegment
			json string
		}{
			{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, json: `{}`},
			{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, json: `{}`},
		} {
			cf.machine.nextKV.Key = keys.MakeSubordinatePathKey(rowKey, 3, tc.path)
			cf.machine.nextKV.Value, _ = makeSubordinateJSONValue(t, 3, tc.path, tc.json)
			_, _, err := cf.processSubordinateValue(context.Background(), cf.table, nil, "/tbl/1/0")
			require.NoError(t, err)
		}

		require.NoError(t, cf.finishJSONContainsFilter())
		require.True(t, cf.lastRowPassesJSONContainsFilter)
		require.Nil(t, cf.subordinateJSONBuilders)
	})

	t.Run("empty-array-contained-by", func(t *testing.T) {
		cf := makeJSONCFetcher()
		right, err := jsonutil.ParseJSON(`[]`)
		require.NoError(t, err)
		require.NoError(t, cf.ConfigureJSONContainsFilter(0, encodeJSONQueryPath("a"), true, right, false))
		cf.resetJSONProgramsForRow()

		rowKey := keys.MakeFamilyKey(keys.SystemSQLCodec.IndexPrefix(1, 1), 0)
		for _, tc := range []struct {
			path []keys.SubordinatePathSegment
			json string
		}{
			{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, json: `{}`},
			{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, json: `[]`},
		} {
			cf.machine.nextKV.Key = keys.MakeSubordinatePathKey(rowKey, 3, tc.path)
			cf.machine.nextKV.Value, _ = makeSubordinateJSONValue(t, 3, tc.path, tc.json)
			_, _, err := cf.processSubordinateValue(context.Background(), cf.table, nil, "/tbl/1/0")
			require.NoError(t, err)
		}

		require.NoError(t, cf.finishJSONContainsFilter())
		require.True(t, cf.lastRowPassesJSONContainsFilter)
		require.Nil(t, cf.subordinateJSONBuilders)
	})
}

func TestJSONContainsFilterSharesSelectedPathMaterialization(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeSingleColumnCFetcherWithOutputs(types.Jsonb, 3, "doc", types.Jsonb)
	right, err := jsonutil.ParseJSON(`{"b":[20]}`)
	require.NoError(t, err)
	require.NoError(t, cf.ConfigureJSONContainsFilter(
		0, encodeJSONQueryPath("a"), false, right, false,
	))
	require.NoError(t, cf.ConfigureJSONAccessPrograms([]row.JSONAccessSpec{
		{ColIdx: 0, Kind: row.JSONAccessFetchJSONPath, Path: encodeJSONQueryPath("a"), Materialize: false},
	}, 1))
	cf.resetJSONProgramsForRow()

	rowKey := keys.MakeFamilyKey(keys.SystemSQLCodec.IndexPrefix(1, 1), 0)
	for _, tc := range []struct {
		path []keys.SubordinatePathSegment
		json string
	}{
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, json: `{}`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, json: `{"b":[10,20]}`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}, {Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"}}, json: `[10,20]`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}, {Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"}, {Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0}}, json: `10`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}, {Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"}, {Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1}}, json: `20`},
	} {
		cf.machine.nextKV.Key = keys.MakeSubordinatePathKey(rowKey, 3, tc.path)
		cf.machine.nextKV.Value, _ = makeSubordinateJSONValue(t, 3, tc.path, tc.json)
		_, _, err := cf.processSubordinateValue(context.Background(), cf.table, nil, "/tbl/1/0")
		require.NoError(t, err)
	}

	require.False(t, cf.jsonContainsFilters[0].program.SawSubordinate())
	require.NoError(t, cf.finishJSONContainsFilter())
	require.True(t, cf.lastRowPassesJSONContainsFilter)
	require.NoError(t, cf.finalizeJSONAccessPrograms())
	require.JSONEq(t, `{"b":[10,20]}`, cf.machine.colvecs.Vecs[1].JSON().Get(0).String())
}

func TestJSONContainedByFilterSharesSelectedPathMaterialization(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeSingleColumnCFetcherWithOutputs(types.Jsonb, 3, "doc", types.Jsonb)
	right, err := jsonutil.ParseJSON(`{"b":[10,20],"extra":true}`)
	require.NoError(t, err)
	require.NoError(t, cf.ConfigureJSONContainsFilter(
		0, encodeJSONQueryPath("a"), true, right, false,
	))
	require.NoError(t, cf.ConfigureJSONAccessPrograms([]row.JSONAccessSpec{
		{ColIdx: 0, Kind: row.JSONAccessFetchJSONPath, Path: encodeJSONQueryPath("a"), Materialize: false},
	}, 1))
	cf.resetJSONProgramsForRow()

	rowKey := keys.MakeFamilyKey(keys.SystemSQLCodec.IndexPrefix(1, 1), 0)
	for _, tc := range []struct {
		path []keys.SubordinatePathSegment
		json string
	}{
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, json: `{}`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, json: `{"b":[10,20]}`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}, {Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"}}, json: `[10,20]`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}, {Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"}, {Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0}}, json: `10`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}, {Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"}, {Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1}}, json: `20`},
	} {
		cf.machine.nextKV.Key = keys.MakeSubordinatePathKey(rowKey, 3, tc.path)
		cf.machine.nextKV.Value, _ = makeSubordinateJSONValue(t, 3, tc.path, tc.json)
		_, _, err := cf.processSubordinateValue(context.Background(), cf.table, nil, "/tbl/1/0")
		require.NoError(t, err)
	}

	require.False(t, cf.jsonContainsFilters[0].program.SawSubordinate())
	require.NoError(t, cf.finishJSONContainsFilter())
	require.True(t, cf.lastRowPassesJSONContainsFilter)
	require.NoError(t, cf.finalizeJSONAccessPrograms())
	require.JSONEq(t, `{"b":[10,20]}`, cf.machine.colvecs.Vecs[1].JSON().Get(0).String())
}

func TestJSONContainsFilterSharedSelectedPathMissing(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeSingleColumnCFetcherWithOutputs(types.Jsonb, 3, "doc", types.Jsonb)
	right, err := jsonutil.ParseJSON(`{"b":[20]}`)
	require.NoError(t, err)
	require.NoError(t, cf.ConfigureJSONContainsFilter(
		0, encodeJSONQueryPath("a"), false, right, false,
	))
	require.NoError(t, cf.ConfigureJSONAccessPrograms([]row.JSONAccessSpec{
		{ColIdx: 0, Kind: row.JSONAccessFetchJSONPath, Path: encodeJSONQueryPath("a"), Materialize: false},
	}, 1))
	cf.resetJSONProgramsForRow()

	rowKey := keys.MakeFamilyKey(keys.SystemSQLCodec.IndexPrefix(1, 1), 0)
	cf.machine.nextKV.Key = keys.MakeSubordinatePathKey(rowKey, 3, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}})
	cf.machine.nextKV.Value, _ = makeSubordinateJSONValue(t, 3, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `{}`)
	_, _, err = cf.processSubordinateValue(context.Background(), cf.table, nil, "/tbl/1/0")
	require.NoError(t, err)

	require.False(t, cf.jsonContainsFilters[0].program.SawSubordinate())
	require.NoError(t, cf.finishJSONContainsFilter())
	require.False(t, cf.lastRowPassesJSONContainsFilter)
	require.NoError(t, cf.finalizeJSONAccessPrograms())
	require.True(t, cf.machine.colvecs.Nulls[1].NullAt(0))
}

func TestMultipleJSONContainsFiltersNoMaterialize(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeJSONCFetcher()
	rightA, err := jsonutil.ParseJSON(`{"b":[20]}`)
	require.NoError(t, err)
	require.NoError(t, cf.ConfigureJSONContainsFilter(
		0, encodeJSONQueryPath("a"), false, rightA, false,
	))
	rightRoot, err := jsonutil.ParseJSON(`{"b":[10,20],"c":null}`)
	require.NoError(t, err)
	require.NoError(t, cf.ConfigureJSONContainsFilter(
		0, encodeJSONQueryPath("a"), true, rightRoot, false,
	))
	cf.resetJSONProgramsForRow()

	rowKey := keys.MakeFamilyKey(keys.SystemSQLCodec.IndexPrefix(1, 1), 0)
	for _, tc := range []struct {
		path []keys.SubordinatePathSegment
		json string
	}{
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, json: `{}`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, json: `{"b":[10,20],"c":null}`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}, {Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"}}, json: `[10,20]`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}, {Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"}, {Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0}}, json: `10`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}, {Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"}, {Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1}}, json: `20`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}, {Kind: keys.SubordinatePathObjectKey, ObjectKey: "c"}}, json: `null`},
	} {
		cf.machine.nextKV.Key = keys.MakeSubordinatePathKey(rowKey, 3, tc.path)
		cf.machine.nextKV.Value, _ = makeSubordinateJSONValue(t, 3, tc.path, tc.json)
		_, _, err := cf.processSubordinateValue(context.Background(), cf.table, nil, "/tbl/1/0")
		require.NoError(t, err)
	}

	require.Len(t, cf.jsonContainsFilters, 2)
	require.NoError(t, cf.finishJSONContainsFilter())
	require.True(t, cf.lastRowPassesJSONContainsFilter)
	require.Nil(t, cf.subordinateJSONBuilders)
}

func TestMultipleJSONContainsFiltersShareSelectedCache(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeJSONCFetcher()
	rightA, err := jsonutil.ParseJSON(`{"b":[20]}`)
	require.NoError(t, err)
	require.NoError(t, cf.ConfigureJSONContainsFilter(
		0, encodeJSONQueryPath("a"), false, rightA, true,
	))
	rightRoot, err := jsonutil.ParseJSON(`{"b":[10,20],"c":null}`)
	require.NoError(t, err)
	require.NoError(t, cf.ConfigureJSONContainsFilter(
		0, encodeJSONQueryPath("a"), true, rightRoot, true,
	))
	cf.resetJSONProgramsForRow()

	rowKey := keys.MakeFamilyKey(keys.SystemSQLCodec.IndexPrefix(1, 1), 0)
	for _, tc := range []struct {
		path []keys.SubordinatePathSegment
		json string
	}{
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, json: `{}`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, json: `{"b":[10,20],"c":null}`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}, {Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"}}, json: `[10,20]`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}, {Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"}, {Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0}}, json: `10`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}, {Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"}, {Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1}}, json: `20`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}, {Kind: keys.SubordinatePathObjectKey, ObjectKey: "c"}}, json: `null`},
	} {
		cf.machine.nextKV.Key = keys.MakeSubordinatePathKey(rowKey, 3, tc.path)
		cf.machine.nextKV.Value, _ = makeSubordinateJSONValue(t, 3, tc.path, tc.json)
		_, _, err := cf.processSubordinateValue(context.Background(), cf.table, nil, "/tbl/1/0")
		require.NoError(t, err)
	}

	require.Len(t, cf.jsonContainsFilters, 2)
	require.NoError(t, cf.finishJSONContainsFilter())
	require.True(t, cf.lastRowPassesJSONContainsFilter)
	require.Equal(t, 2, cf.jsonContainsFilters[0].selected.cache.NumContainsResults())
}

func TestFinalizeJSONAccessProgramsWritesDerivedColumns(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeSingleColumnCFetcherWithOutputs(types.Jsonb, 3, "doc", types.String, types.Jsonb)
	require.NoError(t, cf.ConfigureJSONAccessPrograms([]row.JSONAccessSpec{
		{ColIdx: 0, Kind: row.JSONAccessFetchTextPath, Path: encodeJSONQueryPath("a", "b", "-1"), Materialize: false},
		{ColIdx: 0, Kind: row.JSONAccessFetchJSONPath, Path: encodeJSONQueryPath("a"), Materialize: false},
	}, 1))
	cf.resetJSONProgramsForRow()

	rowKey := keys.MakeFamilyKey(keys.SystemSQLCodec.IndexPrefix(1, 1), 0)
	for _, tc := range []struct {
		path []keys.SubordinatePathSegment
		json string
	}{
		{
			path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}},
			json: `{}`,
		},
		{
			path: []keys.SubordinatePathSegment{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
			},
			json: `{"b":[10,20]}`,
		},
		{
			path: []keys.SubordinatePathSegment{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
			},
			json: `[10,20]`,
		},
		{
			path: []keys.SubordinatePathSegment{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
				{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0},
			},
			json: `10`,
		},
		{
			path: []keys.SubordinatePathSegment{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
				{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1},
			},
			json: `20`,
		},
	} {
		cf.machine.nextKV.Key = keys.MakeSubordinatePathKey(rowKey, 3, tc.path)
		cf.machine.nextKV.Value, _ = makeSubordinateJSONValue(t, 3, tc.path, tc.json)
		_, _, err := cf.processSubordinateValue(context.Background(), cf.table, nil, "/tbl/1/0")
		require.NoError(t, err)
	}

	require.NoError(t, cf.finalizeJSONAccessPrograms())
	require.Equal(t, "20", string(cf.machine.colvecs.BytesCols[cf.machine.colvecs.ColsMap[1]].Get(0)))
	require.JSONEq(t, `{"b":[10,20]}`, cf.machine.colvecs.Vecs[2].JSON().Get(0).String())
	require.Nil(t, cf.subordinateJSONBuilders)
}

func TestFinalizeJSONAccessProgramsSharesSelectedPathCacheAcrossKinds(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeSingleColumnCFetcherWithOutputs(types.Jsonb, 3, "doc", types.Jsonb, types.String)
	require.NoError(t, cf.ConfigureJSONAccessPrograms([]row.JSONAccessSpec{
		{ColIdx: 0, Kind: row.JSONAccessFetchJSONPath, Path: encodeJSONQueryPath("a", "b", "-1"), Materialize: false},
		{ColIdx: 0, Kind: row.JSONAccessFetchTextPath, Path: encodeJSONQueryPath("a", "b", "-1"), Materialize: false},
	}, 1))
	cf.resetJSONProgramsForRow()

	rowKey := keys.MakeFamilyKey(keys.SystemSQLCodec.IndexPrefix(1, 1), 0)
	for _, tc := range []struct {
		path []keys.SubordinatePathSegment
		json string
	}{
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, json: `{"a":{"b":[10,20]}}`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, json: `{"b":[10,20]}`},
		{path: []keys.SubordinatePathSegment{
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
		}, json: `[10,20]`},
		{path: []keys.SubordinatePathSegment{
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
			{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1},
		}, json: `20`},
	} {
		cf.machine.nextKV.Key = keys.MakeSubordinatePathKey(rowKey, 3, tc.path)
		cf.machine.nextKV.Value, _ = makeSubordinateJSONValue(t, 3, tc.path, tc.json)
		_, _, err := cf.processSubordinateValue(context.Background(), cf.table, nil, "/tbl/1/0")
		require.NoError(t, err)
	}

	require.NoError(t, cf.finalizeJSONAccessPrograms())
	require.Equal(t, "20", cf.machine.colvecs.Vecs[1].JSON().Get(0).String())
	require.Equal(t, "20", string(cf.machine.colvecs.BytesCols[cf.machine.colvecs.ColsMap[2]].Get(0)))
	require.Len(t, cf.jsonSharedSelectedPaths, 1)
	jsonDatum, err := cf.jsonSharedSelectedPaths[0].resultDatum(row.JSONAccessFetchJSONPath)
	require.NoError(t, err)
	textDatum, err := cf.jsonSharedSelectedPaths[0].resultDatum(row.JSONAccessFetchTextPath)
	require.NoError(t, err)
	require.Equal(t, "20", jsonDatum.(*tree.DJSON).JSON.String())
	require.Equal(t, tree.NewDString("20"), textDatum)
	require.False(t, cf.jsonAccessPrograms[0].shared.haveCached)
	require.False(t, cf.jsonAccessPrograms[1].shared.haveCached)
}

func TestFinalizeSubordinateArraysErrorsOnGap(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeArrayCFetcher()
	cf.subordinateArrays = map[int]*subordinateArrayBuilder{
		0: newSubordinateArrayBuilder(types.Int),
	}
	cf.subordinateArrays[0].Set(1, tree.NewDInt(20))

	err := cf.finalizeSubordinateValues()
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

func TestProcessValueSingleRejectsJSONEncoding(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeJSONCFetcher()
	j, err := jsonutil.ParseJSON(`{"a":1}`)
	require.NoError(t, err)
	cf.machine.nextKV.Value, err = valueside.MarshalLegacy(types.Jsonb, tree.NewDJSON(j))
	require.NoError(t, err)

	_, _, err = cf.processValueSingle(context.Background(), cf.table, 3, "/tbl/1/0")
	require.Error(t, err)
	require.Regexp(t, "incompatible data layout", err.Error())
}

func TestProcessValueBytesRejectsJSONEncoding(t *testing.T) {
	defer leaktest.AfterTest(t)()

	cf := makeJSONCFetcher()
	j, err := jsonutil.ParseJSON(`{"a":1}`)
	require.NoError(t, err)

	var buf []byte
	buf, err = valueside.Encode(buf, valueside.MakeColumnIDDelta(0, 3), tree.NewDJSON(j), nil)
	require.NoError(t, err)

	_, _, err = cf.processValueBytes(context.Background(), cf.table, buf, "/tbl/1/0")
	require.Error(t, err)
	require.Regexp(t, "JSON type encoded inline in the row value", err.Error())
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

	cf.setFetcher(nil, rowinfra.RowLimit(9))
	require.Nil(t, cf.machine.lastRowPrefix)
	require.Equal(t, 9, cf.machine.limitHint)
	require.Equal(t, stateResetBatch, cf.machine.state[0])
	require.Equal(t, stateInitFetch, cf.machine.state[1])
}
