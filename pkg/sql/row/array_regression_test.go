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
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/semistrict/ratel/pkg/base"
	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/kv"
	"github.com/semistrict/ratel/pkg/roachpb"
	"github.com/semistrict/ratel/pkg/settings/cluster"
	"github.com/semistrict/ratel/pkg/sql/catalog"
	"github.com/semistrict/ratel/pkg/sql/catalog/descpb"
	"github.com/semistrict/ratel/pkg/sql/catalog/desctestutils"
	"github.com/semistrict/ratel/pkg/sql/catalog/tabledesc"
	"github.com/semistrict/ratel/pkg/sql/opt/exec"
	"github.com/semistrict/ratel/pkg/sql/rowenc"
	"github.com/semistrict/ratel/pkg/sql/rowenc/valueside"
	"github.com/semistrict/ratel/pkg/sql/rowinfra"
	"github.com/semistrict/ratel/pkg/sql/sem/tree"
	"github.com/semistrict/ratel/pkg/sql/types"
	"github.com/semistrict/ratel/pkg/testutils/serverutils"
	"github.com/semistrict/ratel/pkg/testutils/sqlutils"
	"github.com/semistrict/ratel/pkg/util"
	"github.com/semistrict/ratel/pkg/util/encoding"
	jsonutil "github.com/semistrict/ratel/pkg/util/json"
	"github.com/semistrict/ratel/pkg/util/leaktest"
	"github.com/semistrict/ratel/pkg/util/protoutil"
	"github.com/stretchr/testify/require"
)

func makeArraySubordinateFetcher(t testing.TB, materialize bool, left tree.Datum) *Fetcher {
	t.Helper()

	var colIdxMap catalog.TableColMap
	colIdxMap.Set(2, 0)

	rf := &Fetcher{
		table: tableInfo{
			spec: descpb.IndexFetchSpec{
				FetchedColumns: []descpb.IndexFetchSpec_Column{{
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
		alloc: &tree.DatumAlloc{},
	}
	rf.ConfigureArrayEqualsAnyFilter(tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings()), 0, left, materialize)
	return rf
}

func makeSingleColumnFetcher(t testing.TB, typ *types.T, colID descpb.ColumnID, name string) *Fetcher {
	t.Helper()

	var colIdxMap catalog.TableColMap
	colIdxMap.Set(colID, 0)
	var neededValueColsByIdx util.FastIntSet
	neededValueColsByIdx.Add(0)

	return &Fetcher{
		table: tableInfo{
			spec: descpb.IndexFetchSpec{
				FetchedColumns: []descpb.IndexFetchSpec_Column{{
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
		alloc: &tree.DatumAlloc{},
	}
}

func makeTwoColumnFetcher(
	t testing.TB,
	firstType *types.T,
	firstID descpb.ColumnID,
	firstName string,
	secondType *types.T,
	secondID descpb.ColumnID,
	secondName string,
) *Fetcher {
	t.Helper()

	var colIdxMap catalog.TableColMap
	colIdxMap.Set(firstID, 0)
	colIdxMap.Set(secondID, 1)
	var neededValueColsByIdx util.FastIntSet
	neededValueColsByIdx.Add(0)
	neededValueColsByIdx.Add(1)

	return &Fetcher{
		table: tableInfo{
			spec: descpb.IndexFetchSpec{
				FetchedColumns: []descpb.IndexFetchSpec_Column{
					{ColumnID: firstID, Name: firstName, Type: firstType},
					{ColumnID: secondID, Name: secondName, Type: secondType},
				},
			},
			colIdxMap:            colIdxMap,
			neededValueColsByIdx: neededValueColsByIdx,
			neededValueCols:      2,
			row:                  make(rowenc.EncDatumRow, 2),
			decodedRow:           make(tree.Datums, 2),
			keyVals:              make([]rowenc.EncDatum, 0),
			extraVals:            make([]rowenc.EncDatum, 0),
			indexColIdx:          []int{-1},
			timestampOutputIdx:   noOutputColumn,
			oidOutputIdx:         noOutputColumn,
		},
		alloc: &tree.DatumAlloc{},
	}
}

func makeJSONExistsFetcher(t testing.TB, key string, materialize bool) *Fetcher {
	t.Helper()
	rf := makeSingleColumnFetcher(t, types.Jsonb, 7, "doc")
	require.NoError(t, rf.ConfigureJSONExistsFilter(0, JSONAccessExists, key, nil, materialize))
	return rf
}

func makeJSONFetchPathFetcher(t testing.TB, path []string, asText bool, materialize bool) *Fetcher {
	t.Helper()
	rf := makeSingleColumnFetcher(t, types.Jsonb, 7, "doc")
	encodedPath := make([]string, 0, len(path))
	for _, step := range path {
		encodedPath = append(encodedPath, jsonPathStepKeyOrIndexPrefix+strconv.Quote(step))
	}
	kind := JSONAccessFetchJSONPath
	if asText {
		kind = JSONAccessFetchTextPath
	}
	require.NoError(t, rf.ConfigureJSONAccessPrograms([]JSONAccessSpec{{
		ColIdx:      0,
		Kind:        kind,
		Path:        encodedPath,
		Materialize: materialize,
	}}))
	return rf
}

func TestTryStaticSubordinateJSONPath(t *testing.T) {
	defer leaktest.AfterTest(t)()

	t.Run("static object path", func(t *testing.T) {
		path, ok, err := TryStaticSubordinateJSONPath([]string{
			jsonPathStepKeyPrefix + strconv.Quote("needle"),
			jsonPathStepKeyOrIndexPrefix + strconv.Quote("tiny"),
		})
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, []keys.SubordinatePathSegment{
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "needle"},
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "tiny"},
		}, path)
	})

	t.Run("reject ambiguous numeric key-or-index", func(t *testing.T) {
		path, ok, err := TryStaticSubordinateJSONPath([]string{
			jsonPathStepKeyOrIndexPrefix + strconv.Quote("0"),
		})
		require.NoError(t, err)
		require.False(t, ok)
		require.Nil(t, path)
	})

	t.Run("reject negative index", func(t *testing.T) {
		path, ok, err := TryStaticSubordinateJSONPath([]string{
			jsonPathStepIndexPrefix + "-1",
		})
		require.NoError(t, err)
		require.False(t, ok)
		require.Nil(t, path)
	})
}

func TestLongestStaticSubordinateJSONPathPrefix(t *testing.T) {
	defer leaktest.AfterTest(t)()

	t.Run("stops at negative index after static object prefix", func(t *testing.T) {
		path, ok, err := LongestStaticSubordinateJSONPathPrefix([]string{
			jsonPathStepKeyOrIndexPrefix + strconv.Quote("needle"),
			jsonPathStepKeyOrIndexPrefix + strconv.Quote("-1"),
			jsonPathStepKeyOrIndexPrefix + strconv.Quote("tiny"),
		})
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, []keys.SubordinatePathSegment{
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "needle"},
		}, path)
	})

	t.Run("stops before ambiguous numeric key-or-index", func(t *testing.T) {
		path, ok, err := LongestStaticSubordinateJSONPathPrefix([]string{
			jsonPathStepKeyOrIndexPrefix + strconv.Quote("0"),
			jsonPathStepKeyOrIndexPrefix + strconv.Quote("tiny"),
		})
		require.NoError(t, err)
		require.True(t, ok)
		require.Empty(t, path)
	})
}

func TestSubordinateJSONRowHeadLookupSpecs(t *testing.T) {
	defer leaktest.AfterTest(t)()

	t.Run("static object prefix before negative index", func(t *testing.T) {
		rf := makeSingleColumnFetcher(t, types.Jsonb, 7, "doc")
		rf.hasSubordinateColumns = true
		rf.table.spec.MaxKeysPerRow = 1
		require.NoError(t, rf.ConfigureJSONPathCompareFilter(
			tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings()),
			0,
			JSONAccessFetchTextPath,
			[]string{
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("needle"),
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("-1"),
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("tiny"),
			},
			exec.JSONPathFilterEq,
			tree.NewDString("x"),
			false, /* materialize */
		))

		lookups, ok, err := rf.subordinateJSONRowHeadLookupSpecs()
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, []SubordinateJSONRowLookupSpec{{
			ColID: 7,
			SelectedPaths: [][]keys.SubordinatePathSegment{{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "needle"},
			}},
		}}, lookups)
	})

	t.Run("allows reverse scans with static prefix", func(t *testing.T) {
		rf := makeSingleColumnFetcher(t, types.Jsonb, 7, "doc")
		rf.hasSubordinateColumns = true
		rf.reverse = true
		rf.table.spec.MaxKeysPerRow = 1
		require.NoError(t, rf.ConfigureJSONPathCompareFilter(
			tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings()),
			0,
			JSONAccessFetchTextPath,
			[]string{
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("needle"),
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("-1"),
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("tiny"),
			},
			exec.JSONPathFilterEq,
			tree.NewDString("x"),
			false, /* materialize */
		))

		lookups, ok, err := rf.subordinateJSONRowHeadLookupSpecs()
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, []SubordinateJSONRowLookupSpec{{
			ColID: 7,
			SelectedPaths: [][]keys.SubordinatePathSegment{{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "needle"},
			}},
		}}, lookups)
	})

	t.Run("rejects ambiguous numeric key-or-index prefix", func(t *testing.T) {
		rf := makeSingleColumnFetcher(t, types.Jsonb, 7, "doc")
		rf.hasSubordinateColumns = true
		rf.table.spec.MaxKeysPerRow = 1
		require.NoError(t, rf.ConfigureJSONPathCompareFilter(
			tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings()),
			0,
			JSONAccessFetchTextPath,
			[]string{
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("0"),
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("tiny"),
			},
			exec.JSONPathFilterEq,
			tree.NewDString("x"),
			false, /* materialize */
		))

		lookups, ok, err := rf.subordinateJSONRowHeadLookupSpecs()
		require.NoError(t, err)
		require.False(t, ok)
		require.Nil(t, lookups)
	})

	t.Run("includes exists keys without selected paths", func(t *testing.T) {
		rf := makeSingleColumnFetcher(t, types.Jsonb, 7, "doc")
		rf.hasSubordinateColumns = true
		rf.table.spec.MaxKeysPerRow = 1
		require.NoError(t, rf.ConfigureJSONExistsFilter(0, JSONAccessExistsAny, "", []string{"b", "a"}, false))

		lookups, ok, err := rf.subordinateJSONRowHeadLookupSpecs()
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, []SubordinateJSONRowLookupSpec{{
			ColID:         7,
			SelectedPaths: nil,
			ExistsKeys:    []string{"a", "b"},
		}}, lookups)
	})

	t.Run("rejects projected source json even when max keys per row is one", func(t *testing.T) {
		rf := makeSingleColumnFetcher(t, types.Jsonb, 7, "doc")
		rf.hasSubordinateColumns = true
		rf.table.spec.MaxKeysPerRow = 1
		require.NoError(t, rf.ConfigureJSONExistsFilter(0, JSONAccessExists, "a", nil, true /* materialize */))

		lookups, ok, err := rf.subordinateJSONRowHeadLookupSpecs()
		require.NoError(t, err)
		require.False(t, ok)
		require.Nil(t, lookups)
	})

	t.Run("allows row head lookup with no json programs when only non-subordinate values are needed", func(t *testing.T) {
		rf := makeSingleColumnFetcher(t, types.String, 8, "pad")
		rf.hasSubordinateColumns = true
		rf.table.spec.MaxKeysPerRow = 0

		lookups, ok, err := rf.subordinateJSONRowHeadLookupSpecs()
		require.NoError(t, err)
		require.True(t, ok)
		require.Nil(t, lookups)
	})

	t.Run("allows max keys per row when all needed value cols are looked-up json", func(t *testing.T) {
		rf := makeSingleColumnFetcher(t, types.Jsonb, 7, "doc")
		rf.hasSubordinateColumns = true
		rf.table.spec.MaxKeysPerRow = 2
		require.NoError(t, rf.ConfigureJSONPathCompareFilter(
			tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings()),
			0,
			JSONAccessFetchTextPath,
			[]string{
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("needle"),
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("-1"),
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("tiny"),
			},
			exec.JSONPathFilterEq,
			tree.NewDString("x"),
			false, /* materialize */
		))

		lookups, ok, err := rf.subordinateJSONRowHeadLookupSpecs()
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, []SubordinateJSONRowLookupSpec{{
			ColID: 7,
			SelectedPaths: [][]keys.SubordinatePathSegment{{
				{Kind: keys.SubordinatePathObjectKey, ObjectKey: "needle"},
			}},
		}}, lookups)
	})

	t.Run("rejects max keys per row when non-json value column is also needed", func(t *testing.T) {
		rf := makeTwoColumnFetcher(t, types.Jsonb, 7, "doc", types.String, 8, "pad")
		rf.hasSubordinateColumns = true
		rf.table.spec.MaxKeysPerRow = 2
		require.NoError(t, rf.ConfigureJSONPathCompareFilter(
			tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings()),
			0,
			JSONAccessFetchTextPath,
			[]string{
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("needle"),
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("-1"),
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("tiny"),
			},
			exec.JSONPathFilterEq,
			tree.NewDString("x"),
			false, /* materialize */
		))

		lookups, ok, err := rf.subordinateJSONRowHeadLookupSpecs()
		require.NoError(t, err)
		require.False(t, ok)
		require.Nil(t, lookups)
	})

	t.Run("rejects max keys per row when source json column must be materialized", func(t *testing.T) {
		rf := makeSingleColumnFetcher(t, types.Jsonb, 7, "doc")
		rf.hasSubordinateColumns = true
		rf.table.spec.MaxKeysPerRow = 2
		require.NoError(t, rf.ConfigureJSONPathCompareFilter(
			tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings()),
			0,
			JSONAccessFetchTextPath,
			[]string{
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("needle"),
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("-1"),
				jsonPathStepKeyOrIndexPrefix + strconv.Quote("tiny"),
			},
			exec.JSONPathFilterEq,
			tree.NewDString("x"),
			true, /* materialize */
		))

		lookups, ok, err := rf.subordinateJSONRowHeadLookupSpecs()
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
			rf := makeSingleColumnFetcher(b, types.Jsonb, 7, "doc")
			rf.hasSubordinateColumns = true
			rf.table.spec.MaxKeysPerRow = 1
			require.NoError(b, rf.ConfigureJSONPathCompareFilter(
				evalCtx, 0, JSONAccessFetchTextPath, encodedPath, exec.JSONPathFilterEq, tree.NewDString("x"), false,
			))
			lookups, ok, err := rf.subordinateJSONRowHeadLookupSpecs()
			require.NoError(b, err)
			require.True(b, ok)
			require.Len(b, lookups, 1)
		}
	})

	b.Run("max_keys_per_row_2_json_only", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			rf := makeSingleColumnFetcher(b, types.Jsonb, 7, "doc")
			rf.hasSubordinateColumns = true
			rf.table.spec.MaxKeysPerRow = 2
			require.NoError(b, rf.ConfigureJSONPathCompareFilter(
				evalCtx, 0, JSONAccessFetchTextPath, encodedPath, exec.JSONPathFilterEq, tree.NewDString("x"), false,
			))
			lookups, ok, err := rf.subordinateJSONRowHeadLookupSpecs()
			require.NoError(b, err)
			require.True(b, ok)
			require.Len(b, lookups, 1)
		}
	})
}

func makeJSONPathCompareFetcher(
	t testing.TB, path []string, asText bool, mode exec.JSONPathFilterMode, right tree.Datum, materialize bool,
) *Fetcher {
	t.Helper()
	rf := makeSingleColumnFetcher(t, types.Jsonb, 7, "doc")
	encodedPath := make([]string, 0, len(path))
	for _, step := range path {
		encodedPath = append(encodedPath, jsonPathStepKeyOrIndexPrefix+strconv.Quote(step))
	}
	kind := JSONAccessFetchJSONPath
	if asText {
		kind = JSONAccessFetchTextPath
	}
	require.NoError(t, rf.ConfigureJSONPathCompareFilter(
		tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings()),
		0,
		kind,
		encodedPath,
		mode,
		right,
		materialize,
	))
	return rf
}

func TestConfigureJSONAccessProgramsSharesPathStateWithJSONPathCompareFilter(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeSingleColumnFetcher(t, types.Jsonb, 7, "doc")
	encodedPath := []string{
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("a"),
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("b"),
	}
	require.NoError(t, rf.ConfigureJSONPathCompareFilter(
		tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings()),
		0,
		JSONAccessFetchTextPath,
		encodedPath,
		exec.JSONPathFilterEq,
		tree.NewDString("20"),
		false, /* materialize */
	))
	require.NoError(t, rf.ConfigureJSONAccessPrograms([]JSONAccessSpec{{
		ColIdx:      0,
		Kind:        JSONAccessFetchJSONPath,
		Path:        encodedPath,
		Materialize: false,
	}}))

	require.Len(t, rf.jsonSharedAccessPrograms, 1)
	require.NotNil(t, rf.jsonPathCompareFilter)
	require.Len(t, rf.jsonAccessPrograms, 1)
	require.Same(t, rf.jsonPathCompareFilter.shared, rf.jsonAccessPrograms[0].shared)
}

func TestConfigureJSONContainsFilterSharesSelectedPathStateWithJSONAccessPrograms(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeSingleColumnFetcher(t, types.Jsonb, 7, "doc")
	encodedPath := []string{
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("a"),
	}
	right, err := jsonutil.ParseJSON(`{"b":[30]}`)
	require.NoError(t, err)
	require.NoError(t, rf.ConfigureJSONContainsFilter(0, encodedPath, false, right, false))
	require.NoError(t, rf.ConfigureJSONAccessPrograms([]JSONAccessSpec{{
		ColIdx:      0,
		Kind:        JSONAccessFetchJSONPath,
		Path:        encodedPath,
		Materialize: false,
	}}))

	require.Len(t, rf.jsonSharedSelectedPaths, 1)
	require.Len(t, rf.jsonContainsFilters, 1)
	require.NotNil(t, rf.jsonContainsFilters[0].selected)
	require.Len(t, rf.jsonAccessPrograms, 1)
	require.Same(t, rf.jsonContainsFilters[0].selected, rf.jsonAccessPrograms[0].shared.selected)
	require.Len(t, rf.jsonContainsFilters[0].selected.contains, 1)
	require.Len(t, rf.jsonContainsFilters[0].selected.access, 1)
}

func observeSelectedJSONPathStateForTests(t *testing.T, selected *sharedJSONSelectedPathState) {
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

	rf := makeSingleColumnFetcher(t, types.Jsonb, 7, "doc")
	encodedPath := []string{jsonPathStepKeyOrIndexPrefix + strconv.Quote("a")}
	require.NoError(t, rf.ConfigureJSONAccessPrograms([]JSONAccessSpec{
		{
			ColIdx:      0,
			Kind:        JSONAccessFetchJSONPath,
			Path:        encodedPath,
			Materialize: false,
		},
		{
			ColIdx:      0,
			Kind:        JSONAccessFetchTextPath,
			Path:        encodedPath,
			Materialize: false,
		},
	}))

	require.Len(t, rf.jsonSharedAccessPrograms, 1)
	shared := rf.jsonSharedAccessPrograms[0]
	require.NotNil(t, shared.selected)

	observeSelectedJSONPathStateForTests(t, shared.selected)

	json1, err := shared.resultDatum(JSONAccessFetchJSONPath)
	require.NoError(t, err)
	json2, err := shared.resultDatum(JSONAccessFetchJSONPath)
	require.NoError(t, err)
	require.Same(t, json1, json2)
	require.False(t, shared.haveCached)

	text1, err := shared.resultDatum(JSONAccessFetchTextPath)
	require.NoError(t, err)
	text2, err := shared.resultDatum(JSONAccessFetchTextPath)
	require.NoError(t, err)
	require.Same(t, text1, text2)
	require.False(t, shared.haveCached)

	shared.reset()
	require.False(t, shared.haveCached)
	observeSelectedJSONPathStateForTests(t, shared.selected)
	text3, err := shared.resultDatum(JSONAccessFetchTextPath)
	require.NoError(t, err)
	require.Equal(t, text1, text3)
}

func TestSharedJSONSelectedPathStateCachesMaterializedJSONAcrossKinds(t *testing.T) {
	defer leaktest.AfterTest(t)()

	selected := &sharedJSONSelectedPathState{
		selector: NewJSONSelectedPathState([]string{jsonPathStepKeyOrIndexPrefix + strconv.Quote("a")}),
		access:   []*sharedJSONAccessProgramState{{}},
	}
	observeSelectedJSONPathStateForTests(t, selected)

	jsonDatum, err := selected.resultDatum(JSONAccessFetchJSONPath)
	require.NoError(t, err)
	require.NotEqual(t, tree.DNull, jsonDatum)

	selected.builder = nil

	textDatum, err := selected.resultDatum(JSONAccessFetchTextPath)
	require.NoError(t, err)
	require.Equal(t, tree.NewDString("x"), textDatum)
}

func makeSubordinateKV(t *testing.T, colID descpb.ColumnID, elemIdx int, d tree.Datum) (roachpb.KeyValue, []byte) {
	t.Helper()

	rowKey := keys.MakeFamilyKey(keys.SystemSQLCodec.IndexPrefix(1, 1), 0)
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
	return roachpb.KeyValue{
		Key:   keys.MakeSubordinateKey(rowKey, uint32(colID), uint32(elemIdx)),
		Value: value,
	}, remaining
}

func makeSubordinateJSONKV(
	t *testing.T, colID descpb.ColumnID, path []keys.SubordinatePathSegment, s string,
) (roachpb.KeyValue, []byte) {
	t.Helper()

	rowKey := keys.MakeFamilyKey(keys.SystemSQLCodec.IndexPrefix(1, 1), 0)
	j, err := jsonutil.ParseJSON(s)
	require.NoError(t, err)
	value, err := rowenc.EncodeSubordinateJSONValue(j)
	require.NoError(t, err)
	return roachpb.KeyValue{
		Key:   keys.MakeSubordinatePathKey(rowKey, uint32(colID), path),
		Value: value,
	}, nil
}

func makeJSONContainsFetcher(
	t *testing.T, path []string, containedBy bool, right string, materialize bool,
) *Fetcher {
	t.Helper()
	rf := makeSingleColumnFetcher(t, types.Jsonb, 7, "doc")
	encodedPath := make([]string, 0, len(path))
	for _, step := range path {
		encodedPath = append(encodedPath, jsonPathStepKeyOrIndexPrefix+strconv.Quote(step))
	}
	j, err := jsonutil.ParseJSON(right)
	require.NoError(t, err)
	require.NoError(t, rf.ConfigureJSONContainsFilter(0, encodedPath, containedBy, j, materialize))
	return rf
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
				spec: descpb.IndexFetchSpec{
					FetchedColumns: []descpb.IndexFetchSpec_Column{{Type: types.IntArray}},
				},
				row: rowenc.EncDatumRow{{Datum: d}},
			},
			alloc: &tree.DatumAlloc{},
			arrayEqualsAnyFilter: &arrayEqualsAnyFilterState{
				evalCtx: tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings()),
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
			spec: descpb.IndexFetchSpec{
				FetchedColumns: []descpb.IndexFetchSpec_Column{{Name: "vals", Type: types.IntArray}},
			},
			row: rowenc.EncDatumRow{{Datum: arr}},
		},
		alloc: &tree.DatumAlloc{},
		arrayEqualsAnyFilter: &arrayEqualsAnyFilterState{
			evalCtx: tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings()),
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
			spec: descpb.IndexFetchSpec{
				FetchedColumns: []descpb.IndexFetchSpec_Column{{Type: types.IntArray}},
			},
			row: make(rowenc.EncDatumRow, 1),
		},
		alloc: &tree.DatumAlloc{},
		arrayEqualsAnyFilter: &arrayEqualsAnyFilterState{
			evalCtx: tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings()),
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
	rf.ConfigureArrayEqualsAnyFilter(tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings()), 3, tree.NewDInt(20), false)
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
	require.NoError(t, rf.table.row[0].EnsureDecoded(types.IntArray, rf.alloc))
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
	require.NoError(t, rf.table.row[0].EnsureDecoded(types.Int, rf.alloc))
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

func TestSubordinateJSONBuilderMaterialize(t *testing.T) {
	defer leaktest.AfterTest(t)()

	builder := &subordinateJSONBuilder{}
	require.NoError(t, builder.Set([]keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, subordinateJSONNodeObject, nil))
	require.NoError(t, builder.Set([]keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, subordinateJSONNodeArray, nil))
	require.NoError(t, builder.Set([]keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
		{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0},
	}, subordinateJSONNodeScalar, jsonutil.TrueJSONValue))
	require.NoError(t, builder.Set([]keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
	}, subordinateJSONNodeScalar, jsonutil.NullJSONValue))

	d, err := builder.Materialize()
	require.NoError(t, err)
	require.Equal(t, `{"a": [true], "b": null}`, d.JSON.String())
}

func TestProcessSubordinateKVMaterializesJSON(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeSingleColumnFetcher(t, types.Jsonb, 7, "doc")

	kv, remaining := makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `{"ignored":"root-kind-only"}`)
	_, _, err := rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)

	kv, remaining = makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, `[1,{"b":null}]`)
	_, _, err = rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)

	kv, remaining = makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
		{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0},
	}, `1`)
	_, _, err = rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)

	kv, remaining = makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
		{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1},
	}, `{"b":null}`)
	_, _, err = rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)

	kv, remaining = makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
		{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
	}, `null`)
	_, _, err = rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)

	require.NoError(t, rf.finalizeRow())
	require.NoError(t, rf.table.row[0].EnsureDecoded(types.Jsonb, rf.alloc))
	require.Equal(t, `{"a": [1, {"b": null}]}`, rf.table.row[0].Datum.(*tree.DJSON).JSON.String())
}

func TestFinishJSONExistsFilterUnsetColumn(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONExistsFetcher(t, "a", false)
	require.NoError(t, rf.finishJSONExistsFilter())
	require.False(t, rf.RowPassesJSONExistsFilter())
}

func TestFinishJSONExistsFilterRejectsInlineJSON(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONExistsFetcher(t, "a", false)
	j, err := jsonutil.ParseJSON(`{"a":1}`)
	require.NoError(t, err)
	rf.table.row[0] = rowenc.EncDatum{Datum: tree.NewDJSON(j)}

	err = rf.finishJSONExistsFilter()
	require.Error(t, err)
	require.Regexp(t, "inline JSON encountered", err.Error())
}

func TestProcessSubordinateKVJSONExistsObjectKey(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONExistsFetcher(t, "a", false)

	kv, remaining := makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `{}`)
	_, _, err := rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)
	kv, remaining = makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, `1`)
	_, _, err = rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)

	require.NoError(t, rf.finishJSONExistsFilter())
	require.True(t, rf.RowPassesJSONExistsFilter())
	require.True(t, rf.jsonExistsFilter.shared.haveCached)
	require.Equal(t, tree.DBoolTrue, rf.jsonExistsFilter.shared.cachedResult)
}

func TestProcessSubordinateKVJSONExistsArrayString(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONExistsFetcher(t, "a", false)

	kv, remaining := makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `[]`)
	_, _, err := rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)
	kv, remaining = makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0}}, `"a"`)
	_, _, err = rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)

	require.NoError(t, rf.finishJSONExistsFilter())
	require.True(t, rf.RowPassesJSONExistsFilter())
}

func TestProcessSubordinateKVJSONExistsScalarString(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONExistsFetcher(t, "a", false)

	kv, remaining := makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `"a"`)
	_, _, err := rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)

	require.NoError(t, rf.finishJSONExistsFilter())
	require.True(t, rf.RowPassesJSONExistsFilter())
}

func observeJSONPathMatcherKV(
	t *testing.T, m *subordinateJSONPathMatcher, colID descpb.ColumnID, path []keys.SubordinatePathSegment, json string,
) {
	t.Helper()

	kv, _ := makeSubordinateJSONKV(t, colID, path, json)
	kind, childCount, j, err := rowenc.DecodeSubordinateJSONValueWithCardinality(kv.Value)
	require.NoError(t, err)
	require.NoError(t, m.Observe(path, kind, childCount, j))
}

func TestSubordinateJSONPathMatcherWholeValue(t *testing.T) {
	defer leaktest.AfterTest(t)()

	m := newSubordinateJSONPathMatcher(nil)
	observeJSONPathMatcherKV(t, m, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `{}`)
	observeJSONPathMatcherKV(t, m, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, `[1,{"b":null}]`)
	observeJSONPathMatcherKV(t, m, 7, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
		{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0},
	}, `1`)
	observeJSONPathMatcherKV(t, m, 7, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
		{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1},
	}, `{"b":null}`)
	observeJSONPathMatcherKV(t, m, 7, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
		{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
	}, `null`)

	j, err := m.Materialize()
	require.NoError(t, err)
	require.NotNil(t, j)
	require.Equal(t, `{"a": [1, {"b": null}]}`, (*j).String())
}

func TestSubordinateJSONPathMatcherNestedPath(t *testing.T) {
	defer leaktest.AfterTest(t)()

	m := newSubordinateJSONPathMatcher([]string{
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("a"),
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("1"),
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("b"),
	})
	observeJSONPathMatcherKV(t, m, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `{}`)
	observeJSONPathMatcherKV(t, m, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, `[1,{"b":null}]`)
	observeJSONPathMatcherKV(t, m, 7, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
		{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1},
	}, `{"b":null}`)
	observeJSONPathMatcherKV(t, m, 7, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
		{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
	}, `null`)

	j, err := m.Materialize()
	require.NoError(t, err)
	require.NotNil(t, j)
	require.Equal(t, `null`, (*j).String())
}

func TestSubordinateJSONPathMatcherMissingPath(t *testing.T) {
	defer leaktest.AfterTest(t)()

	m := newSubordinateJSONPathMatcher([]string{jsonPathStepKeyOrIndexPrefix + strconv.Quote("missing")})
	observeJSONPathMatcherKV(t, m, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `{}`)
	observeJSONPathMatcherKV(t, m, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, `1`)

	j, err := m.Materialize()
	require.NoError(t, err)
	require.Nil(t, j)
}

func TestSubordinateJSONPathMatcherNegativeArrayIndex(t *testing.T) {
	defer leaktest.AfterTest(t)()

	m := newSubordinateJSONPathMatcher([]string{jsonPathStepKeyOrIndexPrefix + strconv.Quote("-1")})
	observeJSONPathMatcherKV(t, m, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `["x","y"]`)
	observeJSONPathMatcherKV(t, m, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1}}, `"y"`)

	txt, err := m.MaterializeText()
	require.NoError(t, err)
	require.NotNil(t, txt)
	require.Equal(t, "y", *txt)
}

func TestSubordinateJSONPathMatcherMaterializeText(t *testing.T) {
	defer leaktest.AfterTest(t)()

	m := newSubordinateJSONPathMatcher([]string{
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("a"),
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("0"),
	})
	observeJSONPathMatcherKV(t, m, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `{}`)
	observeJSONPathMatcherKV(t, m, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, `["x"]`)
	observeJSONPathMatcherKV(t, m, 7, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
		{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0},
	}, `"x"`)

	txt, err := m.MaterializeText()
	require.NoError(t, err)
	require.NotNil(t, txt)
	require.Equal(t, "x", *txt)
}

func TestSubordinateJSONPathMatcherMaterializeTextStoredPaths(t *testing.T) {
	defer leaktest.AfterTest(t)()

	m := newSubordinateJSONPathMatcher([]string{
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("a"),
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("0"),
	})
	observeJSONPathMatcherKV(t, m, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `{}`)
	observeJSONPathMatcherKV(t, m, 7, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathHeader},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
	}, `["x"]`)
	observeJSONPathMatcherKV(t, m, 7, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathHeader},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
		{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0},
	}, `"x"`)

	txt, err := m.MaterializeText()
	require.NoError(t, err)
	require.NotNil(t, txt)
	require.Equal(t, "x", *txt)
}

func TestJSONAccessProgramExists(t *testing.T) {
	defer leaktest.AfterTest(t)()

	p := newJSONExistsProgram("a")
	kv, _ := makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `{}`)
	kind, childCount, j, err := rowenc.DecodeSubordinateJSONValueWithCardinality(kv.Value)
	require.NoError(t, err)
	require.NoError(t, p.Observe([]keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, kind, childCount, j))

	kv, _ = makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, `1`)
	kind, childCount, j, err = rowenc.DecodeSubordinateJSONValueWithCardinality(kv.Value)
	require.NoError(t, err)
	require.NoError(t, p.Observe([]keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, kind, childCount, j))

	d, err := p.ResultDatum()
	require.NoError(t, err)
	require.Equal(t, tree.DBoolTrue, d)
}

func TestJSONAccessProgramExistsStoredPaths(t *testing.T) {
	defer leaktest.AfterTest(t)()

	p := newJSONExistsProgram("a")
	kv, _ := makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `{}`)
	kind, childCount, j, err := rowenc.DecodeSubordinateJSONValueWithCardinality(kv.Value)
	require.NoError(t, err)
	require.NoError(t, p.Observe([]keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, kind, childCount, j))

	kv, _ = makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathHeader},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
	}, `1`)
	kind, childCount, j, err = rowenc.DecodeSubordinateJSONValueWithCardinality(kv.Value)
	require.NoError(t, err)
	require.NoError(t, p.Observe([]keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathHeader},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
	}, kind, childCount, j))

	d, err := p.ResultDatum()
	require.NoError(t, err)
	require.Equal(t, tree.DBoolTrue, d)
}

func TestJSONAccessProgramExistsAny(t *testing.T) {
	defer leaktest.AfterTest(t)()

	p := newJSONExistsAnyProgram([]string{"x", "a"})
	kv, _ := makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `{}`)
	kind, childCount, j, err := rowenc.DecodeSubordinateJSONValueWithCardinality(kv.Value)
	require.NoError(t, err)
	require.NoError(t, p.Observe([]keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, kind, childCount, j))

	kv, _ = makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, `1`)
	kind, childCount, j, err = rowenc.DecodeSubordinateJSONValueWithCardinality(kv.Value)
	require.NoError(t, err)
	require.NoError(t, p.Observe([]keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, kind, childCount, j))

	d, err := p.ResultDatum()
	require.NoError(t, err)
	require.Equal(t, tree.DBoolTrue, d)
}

func TestJSONAccessProgramExistsAll(t *testing.T) {
	defer leaktest.AfterTest(t)()

	p := newJSONExistsAllProgram([]string{"a", "b"})
	kv, _ := makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `{}`)
	kind, childCount, j, err := rowenc.DecodeSubordinateJSONValueWithCardinality(kv.Value)
	require.NoError(t, err)
	require.NoError(t, p.Observe([]keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, kind, childCount, j))

	for _, key := range []string{"a", "b"} {
		kv, _ = makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: key}}, `1`)
		kind, childCount, j, err = rowenc.DecodeSubordinateJSONValueWithCardinality(kv.Value)
		require.NoError(t, err)
		require.NoError(t, p.Observe([]keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: key}}, kind, childCount, j))
	}

	d, err := p.ResultDatum()
	require.NoError(t, err)
	require.Equal(t, tree.DBoolTrue, d)
}

func TestJSONAccessProgramExistsSetNullRow(t *testing.T) {
	defer leaktest.AfterTest(t)()

	for _, p := range []*JSONAccessProgram{
		newJSONExistsAnyProgram([]string{"a"}),
		newJSONExistsAllProgram([]string{"a"}),
	} {
		d, err := p.ResultDatum()
		require.NoError(t, err)
		require.Equal(t, tree.DNull, d)
	}
}

func TestJSONAccessProgramFetchTextPath(t *testing.T) {
	defer leaktest.AfterTest(t)()

	p := newJSONFetchTextPathProgram([]string{
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("a"),
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("0"),
	})
	kv, _ := makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `{}`)
	kind, childCount, j, err := rowenc.DecodeSubordinateJSONValueWithCardinality(kv.Value)
	require.NoError(t, err)
	require.NoError(t, p.Observe([]keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, kind, childCount, j))

	kv, _ = makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, `["x"]`)
	kind, childCount, j, err = rowenc.DecodeSubordinateJSONValueWithCardinality(kv.Value)
	require.NoError(t, err)
	require.NoError(t, p.Observe([]keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, kind, childCount, j))

	kv, _ = makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
		{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0},
	}, `"x"`)
	kind, childCount, j, err = rowenc.DecodeSubordinateJSONValueWithCardinality(kv.Value)
	require.NoError(t, err)
	require.NoError(t, p.Observe([]keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
		{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0},
	}, kind, childCount, j))

	d, err := p.ResultDatum()
	require.NoError(t, err)
	require.Equal(t, tree.NewDString("x"), d)
}

func TestJSONSelectedPathStateMatchesStoredChildBeforeRootHeader(t *testing.T) {
	defer leaktest.AfterTest(t)()

	selected := NewJSONSelectedPathState([]string{
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("a"),
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("b"),
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("1"),
	})
	relPath, ok, err := selected.SelectPath([]keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathHeader},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
		{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1},
	}, rowenc.SubordinateJSONScalar, 0)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, relPath)
}

func TestJSONSelectedPathStateMatchesStoredDescendantBeforeAncestorHeaders(t *testing.T) {
	defer leaktest.AfterTest(t)()

	selected := NewJSONSelectedPathState([]string{
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("a"),
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("b"),
	})
	relPath, ok, err := selected.SelectPath([]keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathHeader},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "c"},
	}, rowenc.SubordinateJSONScalar, 0)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathHeader},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "c"},
	}, relPath)
}

func TestFinishJSONAccessProgramsRejectInlineJSON(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONFetchPathFetcher(t, []string{"a"}, false, false)
	j, err := jsonutil.ParseJSON(`{"a":1}`)
	require.NoError(t, err)
	rf.table.row[0] = rowenc.EncDatum{Datum: tree.NewDJSON(j)}

	err = rf.finishJSONAccessPrograms()
	require.Error(t, err)
	require.Regexp(t, "inline JSON encountered", err.Error())
}

func TestFinishJSONAccessProgramsSharesSelectedPathCacheAcrossKinds(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeSingleColumnFetcher(t, types.Jsonb, 7, "doc")
	path := []string{
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("a"),
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("b"),
		jsonPathStepKeyOrIndexPrefix + strconv.Quote("-1"),
	}
	require.NoError(t, rf.ConfigureJSONAccessPrograms([]JSONAccessSpec{
		{ColIdx: 0, Kind: JSONAccessFetchJSONPath, Path: path, Materialize: false},
		{ColIdx: 0, Kind: JSONAccessFetchTextPath, Path: path, Materialize: false},
	}))

	kv, remaining := makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `{"a":{"b":[10,20]}}`)
	_, _, err := rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)
	kv, remaining = makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
	}, `{"b":[10,20]}`)
	_, _, err = rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)
	kv, remaining = makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
	}, `[10,20]`)
	_, _, err = rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)
	kv, remaining = makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
		{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1},
	}, `20`)
	_, _, err = rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)

	require.NoError(t, rf.finishJSONAccessPrograms())
	require.Len(t, rf.lastRowJSONAccessProgramResults, 2)
	require.Equal(t, `20`, rf.lastRowJSONAccessProgramResults[0].(*tree.DJSON).JSON.String())
	require.Equal(t, tree.NewDString("20"), rf.lastRowJSONAccessProgramResults[1])
	require.Len(t, rf.jsonSharedSelectedPaths, 1)
	require.True(t, rf.jsonSharedSelectedPaths[0].cache.haveJSON)
	require.True(t, rf.jsonSharedSelectedPaths[0].cache.haveText)
	require.False(t, rf.jsonAccessPrograms[0].shared.haveCached)
	require.False(t, rf.jsonAccessPrograms[1].shared.haveCached)
}

func TestFinishJSONPathCompareFilterUnsetColumn(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONPathCompareFetcher(t, []string{"a"}, true, exec.JSONPathFilterEq, tree.NewDString("x"), false)
	require.NoError(t, rf.finishJSONPathCompareFilter())
	require.False(t, rf.RowPassesJSONPathCompareFilter())
}

func TestFinishJSONPathCompareFilterRejectsInlineJSON(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONPathCompareFetcher(t, []string{"a"}, true, exec.JSONPathFilterEq, tree.NewDString("x"), false)
	j, err := jsonutil.ParseJSON(`{"a":"x"}`)
	require.NoError(t, err)
	rf.table.row[0] = rowenc.EncDatum{Datum: tree.NewDJSON(j)}

	err = rf.finishJSONPathCompareFilter()
	require.Error(t, err)
	require.Regexp(t, "inline JSON encountered", err.Error())
}

func TestProcessSubordinateKVJSONPathCompareFilterText(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONPathCompareFetcher(t, []string{"a", "-1"}, true, exec.JSONPathFilterEq, tree.NewDString("y"), false)

	kv, remaining := makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `{"a":["x","y"]}`)
	_, _, err := rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)
	kv, remaining = makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, `["x","y"]`)
	_, _, err = rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)
	kv, remaining = makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
		{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1},
	}, `"y"`)
	_, _, err = rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)

	require.NoError(t, rf.finishJSONPathCompareFilter())
	require.True(t, rf.RowPassesJSONPathCompareFilter())
	require.False(t, rf.jsonPathCompareFilter.shared.haveCached)
	require.True(t, rf.jsonPathCompareFilter.shared.selected.cache.haveText)
	require.Equal(t, tree.NewDString("y"), rf.jsonPathCompareFilter.shared.selected.cache.cachedText)
}

func TestProcessSubordinateKVJSONPathCompareFilterTextLessThan(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONPathCompareFetcher(t, []string{"a", "-1"}, true, exec.JSONPathFilterLt, tree.NewDString("z"), false)

	kv, remaining := makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `{"a":["x","y"]}`)
	_, _, err := rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)
	kv, remaining = makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, `["x","y"]`)
	_, _, err = rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)
	kv, remaining = makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
		{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1},
	}, `"y"`)
	_, _, err = rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)

	require.NoError(t, rf.finishJSONPathCompareFilter())
	require.True(t, rf.RowPassesJSONPathCompareFilter())
}

func TestFinishJSONPathCompareFilterIsNullUnsetColumn(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONPathCompareFetcher(t, []string{"a"}, true, exec.JSONPathFilterIsNull, nil, false)
	require.NoError(t, rf.finishJSONPathCompareFilter())
	require.True(t, rf.RowPassesJSONPathCompareFilter())
}

func TestFinishJSONPathCompareFilterIsNotNullUnsetColumn(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONPathCompareFetcher(t, []string{"a"}, true, exec.JSONPathFilterIsNotNull, nil, false)
	require.NoError(t, rf.finishJSONPathCompareFilter())
	require.False(t, rf.RowPassesJSONPathCompareFilter())
}

func TestFinishJSONContainsFilterUnsetColumn(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONContainsFetcher(t, []string{"a"}, false, `{"b":[20]}`, false)
	require.NoError(t, rf.finishJSONContainsFilter())
	require.False(t, rf.RowPassesJSONContainsFilter())
}

func TestFinishJSONContainsFilterRejectsInlineJSON(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONContainsFetcher(t, []string{"a"}, false, `{"b":[20]}`, false)
	j, err := jsonutil.ParseJSON(`{"a":{"b":[10,20]}}`)
	require.NoError(t, err)
	rf.table.row[0] = rowenc.EncDatum{Datum: tree.NewDJSON(j)}

	err = rf.finishJSONContainsFilter()
	require.Error(t, err)
	require.Regexp(t, "inline JSON encountered in JSON contains filter fallback", err.Error())
}

func TestProcessSubordinateKVJSONContainsFilter(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONContainsFetcher(t, []string{"a"}, false, `{"b":[20]}`, false)

	for _, tc := range []struct {
		path []keys.SubordinatePathSegment
		json string
	}{
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, json: `{}`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, json: `{"b":[10,20]}`},
		{path: []keys.SubordinatePathSegment{
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
		}, json: `[10,20]`},
		{path: []keys.SubordinatePathSegment{
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
			{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0},
		}, json: `10`},
		{path: []keys.SubordinatePathSegment{
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
			{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1},
		}, json: `20`},
	} {
		kv, remaining := makeSubordinateJSONKV(t, 7, tc.path, tc.json)
		_, _, err := rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
		require.NoError(t, err)
	}

	require.NoError(t, rf.finishJSONContainsFilter())
	require.True(t, rf.RowPassesJSONContainsFilter())
	require.Nil(t, rf.subordinateJSONBuilders)
}

func TestProcessSubordinateKVJSONContainedByFilter(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONContainsFetcher(t, []string{"a"}, true, `{"b":[10,20],"extra":true}`, false)

	for _, tc := range []struct {
		path []keys.SubordinatePathSegment
		json string
	}{
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, json: `{}`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, json: `{"b":[10,20]}`},
		{path: []keys.SubordinatePathSegment{
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
		}, json: `[10,20]`},
		{path: []keys.SubordinatePathSegment{
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
			{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0},
		}, json: `10`},
		{path: []keys.SubordinatePathSegment{
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
			{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1},
		}, json: `20`},
	} {
		kv, remaining := makeSubordinateJSONKV(t, 7, tc.path, tc.json)
		_, _, err := rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
		require.NoError(t, err)
	}

	require.NoError(t, rf.finishJSONContainsFilter())
	require.True(t, rf.RowPassesJSONContainsFilter())
	require.Nil(t, rf.subordinateJSONBuilders)
}

func TestProcessSubordinateKVJSONContainsArrayOfObjectsFilter(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONContainsFetcher(t, []string{"a"}, false, `[{"x":1},{"y":2}]`, false)

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
		kv, remaining := makeSubordinateJSONKV(t, 7, tc.path, tc.json)
		_, _, err := rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
		require.NoError(t, err)
	}

	require.NoError(t, rf.finishJSONContainsFilter())
	require.True(t, rf.RowPassesJSONContainsFilter())
	require.Nil(t, rf.subordinateJSONBuilders)
}

func TestProcessSubordinateKVJSONContainedByArrayOfObjectsFilter(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONContainsFetcher(t, []string{"a"}, true, `[{"x":1,"z":0},{"y":2}]`, false)

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
		kv, remaining := makeSubordinateJSONKV(t, 7, tc.path, tc.json)
		_, _, err := rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
		require.NoError(t, err)
	}

	require.NoError(t, rf.finishJSONContainsFilter())
	require.True(t, rf.RowPassesJSONContainsFilter())
	require.Nil(t, rf.subordinateJSONBuilders)
}

func TestProcessSubordinateKVJSONContainsFilterSharesSelectedPathMaterialization(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONContainsFetcher(t, []string{"a"}, false, `{"b":[20]}`, false)
	require.NoError(t, rf.ConfigureJSONAccessPrograms([]JSONAccessSpec{
		{ColIdx: 0, Kind: JSONAccessFetchJSONPath, Path: []string{jsonPathStepKeyOrIndexPrefix + strconv.Quote("a")}, Materialize: false},
	}))

	for _, tc := range []struct {
		path []keys.SubordinatePathSegment
		json string
	}{
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, json: `{}`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, json: `{"b":[10,20]}`},
		{path: []keys.SubordinatePathSegment{
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
		}, json: `[10,20]`},
		{path: []keys.SubordinatePathSegment{
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
			{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0},
		}, json: `10`},
		{path: []keys.SubordinatePathSegment{
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
			{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1},
		}, json: `20`},
	} {
		kv, remaining := makeSubordinateJSONKV(t, 7, tc.path, tc.json)
		_, _, err := rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
		require.NoError(t, err)
	}

	require.False(t, rf.jsonContainsFilters[0].program.SawSubordinate())
	require.NoError(t, rf.finishJSONContainsFilter())
	require.True(t, rf.RowPassesJSONContainsFilter())
	require.NoError(t, rf.finishJSONAccessPrograms())
	require.JSONEq(t, `{"b":[10,20]}`, rf.JSONAccessProgramResults()[0].(*tree.DJSON).JSON.String())
}

func TestProcessSubordinateKVJSONContainedByFilterSharesSelectedPathMaterialization(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONContainsFetcher(t, []string{"a"}, true, `{"b":[10,20],"extra":true}`, false)
	require.NoError(t, rf.ConfigureJSONAccessPrograms([]JSONAccessSpec{
		{ColIdx: 0, Kind: JSONAccessFetchJSONPath, Path: []string{jsonPathStepKeyOrIndexPrefix + strconv.Quote("a")}, Materialize: false},
	}))

	for _, tc := range []struct {
		path []keys.SubordinatePathSegment
		json string
	}{
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, json: `{}`},
		{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, json: `{"b":[10,20]}`},
		{path: []keys.SubordinatePathSegment{
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
		}, json: `[10,20]`},
		{path: []keys.SubordinatePathSegment{
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
			{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0},
		}, json: `10`},
		{path: []keys.SubordinatePathSegment{
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
			{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1},
		}, json: `20`},
	} {
		kv, remaining := makeSubordinateJSONKV(t, 7, tc.path, tc.json)
		_, _, err := rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
		require.NoError(t, err)
	}

	require.False(t, rf.jsonContainsFilters[0].program.SawSubordinate())
	require.NoError(t, rf.finishJSONContainsFilter())
	require.True(t, rf.RowPassesJSONContainsFilter())
	require.NoError(t, rf.finishJSONAccessPrograms())
	require.JSONEq(t, `{"b":[10,20]}`, rf.JSONAccessProgramResults()[0].(*tree.DJSON).JSON.String())
}

func TestFinishJSONContainsFilterSharedSelectedPathMissing(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONContainsFetcher(t, []string{"a"}, false, `{"b":[20]}`, false)
	require.NoError(t, rf.ConfigureJSONAccessPrograms([]JSONAccessSpec{
		{ColIdx: 0, Kind: JSONAccessFetchJSONPath, Path: []string{jsonPathStepKeyOrIndexPrefix + strconv.Quote("a")}, Materialize: false},
	}))

	kv, remaining := makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `{}`)
	_, _, err := rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)

	require.False(t, rf.jsonContainsFilters[0].program.SawSubordinate())
	require.NoError(t, rf.finishJSONContainsFilter())
	require.False(t, rf.RowPassesJSONContainsFilter())
	require.NoError(t, rf.finishJSONAccessPrograms())
	require.Equal(t, tree.DNull, rf.JSONAccessProgramResults()[0])
}

func TestProcessSubordinateKVMultipleJSONContainsFilters(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONContainsFetcher(t, []string{"a"}, false, `{"b":[20]}`, false)
	right, err := jsonutil.ParseJSON(`{"b":[10,20],"c":null}`)
	require.NoError(t, err)
	require.NoError(t, rf.ConfigureJSONContainsFilter(
		0,
		[]string{jsonPathStepKeyOrIndexPrefix + strconv.Quote("a")},
		true,
		right,
		false,
	))

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
		kv, remaining := makeSubordinateJSONKV(t, 7, tc.path, tc.json)
		_, _, err := rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
		require.NoError(t, err)
	}

	require.Len(t, rf.jsonContainsFilters, 2)
	require.NoError(t, rf.finishJSONContainsFilter())
	require.True(t, rf.RowPassesJSONContainsFilter())
}

func TestProcessSubordinateKVMultipleJSONContainsFiltersShareSelectedCache(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONContainsFetcher(t, []string{"a"}, false, `{"b":[20]}`, true)
	right, err := jsonutil.ParseJSON(`{"b":[10,20],"c":null}`)
	require.NoError(t, err)
	require.NoError(t, rf.ConfigureJSONContainsFilter(
		0,
		[]string{jsonPathStepKeyOrIndexPrefix + strconv.Quote("a")},
		true,
		right,
		true,
	))

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
		kv, remaining := makeSubordinateJSONKV(t, 7, tc.path, tc.json)
		_, _, err := rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
		require.NoError(t, err)
	}

	require.Len(t, rf.jsonContainsFilters, 2)
	require.NoError(t, rf.finishJSONContainsFilter())
	require.True(t, rf.RowPassesJSONContainsFilter())
	require.Equal(t, 2, rf.jsonContainsFilters[0].selected.cache.NumContainsResults())
}

func TestProcessSubordinateKVJSONContainsNullAndEmptyFilter(t *testing.T) {
	defer leaktest.AfterTest(t)()

	t.Run("json-null", func(t *testing.T) {
		rf := makeJSONContainsFetcher(t, nil, false, `null`, false)
		kv, remaining := makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `null`)
		_, _, err := rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
		require.NoError(t, err)

		require.NoError(t, rf.finishJSONContainsFilter())
		require.True(t, rf.RowPassesJSONContainsFilter())
		require.Nil(t, rf.subordinateJSONBuilders)
	})

	t.Run("empty-object-contained-by", func(t *testing.T) {
		rf := makeJSONContainsFetcher(t, []string{"a"}, true, `{}`, false)
		for _, tc := range []struct {
			path []keys.SubordinatePathSegment
			json string
		}{
			{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, json: `{}`},
			{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, json: `{}`},
		} {
			kv, remaining := makeSubordinateJSONKV(t, 7, tc.path, tc.json)
			_, _, err := rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
			require.NoError(t, err)
		}

		require.NoError(t, rf.finishJSONContainsFilter())
		require.True(t, rf.RowPassesJSONContainsFilter())
		require.Nil(t, rf.subordinateJSONBuilders)
	})

	t.Run("empty-array-contained-by", func(t *testing.T) {
		rf := makeJSONContainsFetcher(t, []string{"a"}, true, `[]`, false)
		for _, tc := range []struct {
			path []keys.SubordinatePathSegment
			json string
		}{
			{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, json: `{}`},
			{path: []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, json: `[]`},
		} {
			kv, remaining := makeSubordinateJSONKV(t, 7, tc.path, tc.json)
			_, _, err := rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
			require.NoError(t, err)
		}

		require.NoError(t, rf.finishJSONContainsFilter())
		require.True(t, rf.RowPassesJSONContainsFilter())
		require.Nil(t, rf.subordinateJSONBuilders)
	})
}

func TestProcessSubordinateKVJSONFetchPathJSON(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONFetchPathFetcher(t, []string{"a", "1", "b"}, false, false)

	kv, remaining := makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `{}`)
	_, _, err := rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)
	kv, remaining = makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, `[1,{"b":null}]`)
	_, _, err = rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)
	kv, remaining = makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
		{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1},
	}, `{"b":null}`)
	_, _, err = rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)
	kv, remaining = makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
		{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 1},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "b"},
	}, `null`)
	_, _, err = rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)

	require.NoError(t, rf.finalizeRow())
	require.Equal(t, tree.NewDJSON(jsonutil.NullJSONValue), rf.JSONAccessProgramResults()[0])
}

func TestProcessSubordinateKVJSONFetchPathText(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONFetchPathFetcher(t, []string{"a", "0"}, true, false)

	kv, remaining := makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `{}`)
	_, _, err := rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)
	kv, remaining = makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, `["x"]`)
	_, _, err = rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)
	kv, remaining = makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"},
		{Kind: keys.SubordinatePathArrayIndex, ArrayIdx: 0},
	}, `"x"`)
	_, _, err = rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)

	require.NoError(t, rf.finalizeRow())
	require.Equal(t, tree.NewDString("x"), rf.JSONAccessProgramResults()[0])
}

func TestProcessSubordinateKVJSONFetchPathMissing(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeJSONFetchPathFetcher(t, []string{"missing"}, false, false)

	kv, remaining := makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}, `{}`)
	_, _, err := rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)
	kv, remaining = makeSubordinateJSONKV(t, 7, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathObjectKey, ObjectKey: "a"}}, `1`)
	_, _, err = rf.processSubordinateKV(context.Background(), &rf.table, kv, remaining, "/tbl/1/0")
	require.NoError(t, err)

	require.NoError(t, rf.finalizeRow())
	require.Equal(t, tree.DNull, rf.JSONAccessProgramResults()[0])
}

func TestProcessValueSingleRejectsJSONEncoding(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeSingleColumnFetcher(t, types.Jsonb, 7, "doc")
	j, err := jsonutil.ParseJSON(`{"a":1}`)
	require.NoError(t, err)
	kvValue, err := valueside.MarshalLegacy(types.Jsonb, tree.NewDJSON(j))
	require.NoError(t, err)

	_, _, err = rf.processValueSingle(
		context.Background(), &rf.table, 7, roachpb.KeyValue{Value: kvValue}, "/tbl/1/0",
	)
	require.Error(t, err)
	require.Regexp(t, "subordinate-encoded type stored as single-column family value", err.Error())
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
	require.NoError(t, rf.table.row[0].EnsureDecoded(types.Int, rf.alloc))
	require.Equal(t, "42", rf.table.row[0].Datum.String())
}

func TestProcessValueSingleTraceKV(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rf := makeSingleColumnFetcher(t, types.Int, 7, "v")
	rf.traceKV = true
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
	rf.traceKV = true

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
	rf.traceKV = true
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
	rf.traceKV = true
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
	var neededValueColsByIdx util.FastIntSet
	neededValueColsByIdx.AddRange(0, 1)

	tsCol := descpb.ColumnID(10)
	oidCol := descpb.ColumnID(11)
	colIdxMap.Set(tsCol, 2)
	colIdxMap.Set(oidCol, 3)

	rf := &Fetcher{
		table: tableInfo{
			spec: descpb.IndexFetchSpec{
				TableName: "t",
				FetchedColumns: []descpb.IndexFetchSpec_Column{
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
		alloc: &tree.DatumAlloc{},
	}
	rf.table.row[0] = rowenc.DatumToEncDatum(types.Int, tree.NewDInt(42))
	rf.valueColsFound = 1
	rf.table.rowLastModified.WallTime = 123

	require.NoError(t, rf.finalizeRow())
	require.NoError(t, rf.table.row[1].EnsureDecoded(types.String, rf.alloc))
	require.Equal(t, tree.DNull, rf.table.row[1].Datum)
	require.NoError(t, rf.table.row[2].EnsureDecoded(types.Decimal, rf.alloc))
	require.NotEqual(t, tree.DNull, rf.table.row[2].Datum)
	require.NoError(t, rf.table.row[3].EnsureDecoded(types.Oid, rf.alloc))
	require.Equal(t, rf.table.tableOid, rf.table.row[3].Datum)
}

func TestFinalizeRowAllowsDeletedRowMissingNonNullableValue(t *testing.T) {
	defer leaktest.AfterTest(t)()

	var colIdxMap catalog.TableColMap
	colIdxMap.Set(7, 0)
	var neededValueColsByIdx util.FastIntSet
	neededValueColsByIdx.Add(0)

	rf := &Fetcher{
		table: tableInfo{
			spec: descpb.IndexFetchSpec{
				TableName: "t",
				IndexName: "primary",
				FetchedColumns: []descpb.IndexFetchSpec_Column{{
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
		alloc: &tree.DatumAlloc{},
	}

	require.NoError(t, rf.finalizeRow())
	require.NoError(t, rf.table.row[0].EnsureDecoded(types.Int, rf.alloc))
	require.Equal(t, tree.DNull, rf.table.row[0].Datum)
}

func TestFinalizeRowSkipsPredicateOnlyArrayColumn(t *testing.T) {
	defer leaktest.AfterTest(t)()

	var colIdxMap catalog.TableColMap
	colIdxMap.Set(7, 0)
	var neededValueColsByIdx util.FastIntSet
	neededValueColsByIdx.Add(0)

	rf := &Fetcher{
		table: tableInfo{
			spec: descpb.IndexFetchSpec{
				FetchedColumns: []descpb.IndexFetchSpec_Column{{
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
		alloc: &tree.DatumAlloc{},
		arrayEqualsAnyFilter: &arrayEqualsAnyFilterState{
			evalCtx:        tree.NewTestingEvalContext(cluster.MakeTestingClusterSettings()),
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

	rf := initFetcher(t, args, false /* reverseScan */, &tree.DatumAlloc{}, nil /* memMon */)
	require.NoError(t, rf.StartScan(
		ctx,
		kv.NewTxn(ctx, kvDB, 0),
		spans,
		rowinfra.NoBytesLimit,
		rowinfra.NoRowLimit,
		false, /* traceKV */
		false, /* forceProductionKVBatchSize */
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
	require.NoError(t, dst[0].EnsureDecoded(types.String, rf.alloc))
	require.NoError(t, dst[1].EnsureDecoded(types.Int, rf.alloc))
	require.Equal(t, "a", string(tree.MustBeDString(dst[0].Datum)))
	require.EqualValues(t, 1, tree.MustBeDInt(dst[1].Datum))
	require.NoError(t, dst[2].EnsureDecoded(types.String, rf.alloc))
	require.Equal(t, "keep", string(tree.MustBeDString(dst[2].Datum)))
	require.Equal(t, rf.table.rowLastModified, rf.RowLastModified())

	ok, err = rf.NextRowInto(ctx, dst, encMap)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, dst[0].EnsureDecoded(types.String, rf.alloc))
	require.NoError(t, dst[1].EnsureDecoded(types.Int, rf.alloc))
	require.Equal(t, "b", string(tree.MustBeDString(dst[0].Datum)))
	require.EqualValues(t, 2, tree.MustBeDInt(dst[1].Datum))

	ok, err = rf.NextRowInto(ctx, dst, encMap)
	require.NoError(t, err)
	require.False(t, ok)

	rf = initFetcher(t, args, false /* reverseScan */, &tree.DatumAlloc{}, nil /* memMon */)
	spans = roachpb.Spans{tableDesc.IndexSpan(keys.SystemSQLCodec, tableDesc.GetPrimaryIndexID())}
	require.NoError(t, rf.StartScan(
		ctx,
		kv.NewTxn(ctx, kvDB, 0),
		spans,
		rowinfra.NoBytesLimit,
		rowinfra.NoRowLimit,
		false, /* traceKV */
		false, /* forceProductionKVBatchSize */
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

func TestRecursiveJSONRoundTripThroughKV(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()

	s, sqlDB, kvDB := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)

	sqlRunner := sqlutils.MakeSQLRunner(sqlDB)
	sqlRunner.Exec(t, `CREATE DATABASE d`)
	sqlRunner.Exec(t, `CREATE TABLE d.json_roundtrip (k INT PRIMARY KEY, doc JSONB)`)
	sqlRunner.Exec(t, `INSERT INTO d.json_roundtrip VALUES (1, '{"a":[1,{"b":null}],"c":true}')`)

	tableDesc := desctestutils.TestingGetPublicTableDescriptor(
		kvDB, keys.SystemSQLCodec, "d", "json_roundtrip",
	)
	span := tableDesc.IndexSpan(keys.SystemSQLCodec, tableDesc.GetPrimaryIndexID())
	kvs, err := kvDB.Scan(ctx, span.Key, span.EndKey, 0)
	require.NoError(t, err)
	require.Greater(t, len(kvs), 1)

	// The primary row value should no longer inline the JSON payload.
	rawTuple, err := kvs[0].Value.GetTuple()
	require.NoError(t, err)
	require.Empty(t, rawTuple)

	rf := initFetcher(t, initFetcherArgs{
		tableDesc: tableDesc,
		indexIdx:  0,
		columns:   []int{1},
	}, false /* reverseScan */, &tree.DatumAlloc{}, nil /* memMon */)
	require.NoError(t, rf.StartScan(
		ctx,
		kv.NewTxn(ctx, kvDB, 0),
		roachpb.Spans{span},
		rowinfra.NoBytesLimit,
		rowinfra.NoRowLimit,
		false, /* traceKV */
		false, /* forceProductionKVBatchSize */
	))

	row, err := rf.NextRowDecoded(ctx)
	require.NoError(t, err)
	require.Len(t, row, 1)
	require.IsType(t, &tree.DJSON{}, row[0])
	require.Equal(t, `{"a": [1, {"b": null}], "c": true}`, row[0].(*tree.DJSON).JSON.String())

	row, err = rf.NextRowDecoded(ctx)
	require.NoError(t, err)
	require.Nil(t, row)
}

func TestRecursiveJSONUpdateRemovesOldSubtree(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()

	s, sqlDB, kvDB := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)

	sqlRunner := sqlutils.MakeSQLRunner(sqlDB)
	sqlRunner.Exec(t, `CREATE DATABASE d`)
	sqlRunner.Exec(t, `CREATE TABLE d.json_update (k INT PRIMARY KEY, doc JSONB)`)
	sqlRunner.Exec(t, `INSERT INTO d.json_update VALUES (1, '{"a":[1,{"b":null}],"c":true}')`)
	sqlRunner.Exec(t, `UPDATE d.json_update SET doc = '5' WHERE k = 1`)

	tableDesc := desctestutils.TestingGetPublicTableDescriptor(
		kvDB, keys.SystemSQLCodec, "d", "json_update",
	)
	span := tableDesc.IndexSpan(keys.SystemSQLCodec, tableDesc.GetPrimaryIndexID())
	kvs, err := kvDB.Scan(ctx, span.Key, span.EndKey, 0)
	require.NoError(t, err)

	require.Len(t, kvs, 2)
	require.Empty(t, mustGetTuple(t, *kvs[0].Value))

	rowPrefix, colID, path, err := keys.DecodeSubordinatePathKey(kvs[1].Key)
	require.NoError(t, err)
	require.Equal(t, uint32(tableDesc.PublicColumns()[1].GetID()), colID)
	require.Equal(t, roachpb.Key(mustPrimaryKeyPrefix(t, tableDesc, kvDB, span)), roachpb.Key(rowPrefix))
	require.Equal(t, "$", subordinatePathString(path))

	kind, j, err := rowenc.DecodeSubordinateJSONValue(*kvs[1].Value)
	require.NoError(t, err)
	require.Equal(t, rowenc.SubordinateJSONScalar, kind)
	require.NotNil(t, j)
	require.Equal(t, `5`, j.String())

	rf := initFetcher(t, initFetcherArgs{
		tableDesc: tableDesc,
		indexIdx:  0,
		columns:   []int{1},
	}, false /* reverseScan */, &tree.DatumAlloc{}, nil /* memMon */)
	require.NoError(t, rf.StartScan(
		ctx,
		kv.NewTxn(ctx, kvDB, 0),
		roachpb.Spans{span},
		rowinfra.NoBytesLimit,
		rowinfra.NoRowLimit,
		false, /* traceKV */
		false, /* forceProductionKVBatchSize */
	))
	row, err := rf.NextRowDecoded(ctx)
	require.NoError(t, err)
	require.Len(t, row, 1)
	require.Equal(t, `5`, row[0].(*tree.DJSON).JSON.String())
}

func TestRecursiveJSONInserterOverwriteRemovesOldSubtree(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()

	s, sqlDB, kvDB := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)

	sqlRunner := sqlutils.MakeSQLRunner(sqlDB)
	sqlRunner.Exec(t, `CREATE DATABASE d`)
	sqlRunner.Exec(t, `CREATE TABLE d.json_upsert (k INT PRIMARY KEY, doc JSONB)`)
	sqlRunner.Exec(t, `INSERT INTO d.json_upsert VALUES (1, '{"a":{"b":[10,20]}}')`)

	tableDesc := desctestutils.TestingGetPublicTableDescriptor(
		kvDB, keys.SystemSQLCodec, "d", "json_upsert",
	)
	st := cluster.MakeTestingClusterSettings()
	txn := kv.NewTxn(ctx, kvDB, 0)
	ri, err := MakeInserter(
		ctx,
		txn,
		keys.SystemSQLCodec,
		tableDesc,
		tableDesc.PublicColumns(),
		&tree.DatumAlloc{},
		&st.SV,
		false, /* internal */
		nil,   /* metrics */
	)
	require.NoError(t, err)

	j, err := jsonutil.ParseJSON(`{"a":{"b":[99]}}`)
	require.NoError(t, err)

	b := &kv.Batch{}
	require.NoError(t, ri.InsertRow(
		ctx, b, []tree.Datum{tree.NewDInt(1), tree.NewDJSON(j)}, PartialIndexUpdateHelper{}, true, /* overwrite */
		false, /* traceKV */
	))
	require.NoError(t, txn.Run(ctx, b))
	require.NoError(t, txn.Commit(ctx))

	span := tableDesc.IndexSpan(keys.SystemSQLCodec, tableDesc.GetPrimaryIndexID())
	kvs, err := kvDB.Scan(ctx, span.Key, span.EndKey, 0)
	require.NoError(t, err)

	var sawOldArrayTail bool
	for i := 1; i < len(kvs); i++ {
		_, _, path, err := keys.DecodeSubordinatePathKey(kvs[i].Key)
		require.NoError(t, err)
		if subordinatePathString(path) == "$.a.b[1]" {
			sawOldArrayTail = true
		}
	}
	require.False(t, sawOldArrayTail)

	rf := initFetcher(t, initFetcherArgs{
		tableDesc: tableDesc,
		indexIdx:  0,
		columns:   []int{1},
	}, false /* reverseScan */, &tree.DatumAlloc{}, nil /* memMon */)
	require.NoError(t, rf.StartScan(
		ctx,
		kv.NewTxn(ctx, kvDB, 0),
		roachpb.Spans{span},
		rowinfra.NoBytesLimit,
		rowinfra.NoRowLimit,
		false, /* traceKV */
		false, /* forceProductionKVBatchSize */
	))
	row, err := rf.NextRowDecoded(ctx)
	require.NoError(t, err)
	require.Len(t, row, 1)
	require.Equal(t, `{"a": {"b": [99]}}`, row[0].(*tree.DJSON).JSON.String())
}

func TestRecursiveJSONInserterOverwriteMultiRowRemovesOldSubtree(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()

	s, sqlDB, kvDB := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)

	sqlRunner := sqlutils.MakeSQLRunner(sqlDB)
	sqlRunner.Exec(t, `CREATE DATABASE d`)
	sqlRunner.Exec(t, `CREATE TABLE d.json_upsert_multi (k INT PRIMARY KEY, doc JSONB)`)
	sqlRunner.Exec(t, `INSERT INTO d.json_upsert_multi VALUES (1, '{"a":{"b":[10,20]}}'), (2, NULL)`)

	tableDesc := desctestutils.TestingGetPublicTableDescriptor(
		kvDB, keys.SystemSQLCodec, "d", "json_upsert_multi",
	)
	st := cluster.MakeTestingClusterSettings()
	txn := kv.NewTxn(ctx, kvDB, 0)
	ri, err := MakeInserter(
		ctx,
		txn,
		keys.SystemSQLCodec,
		tableDesc,
		tableDesc.PublicColumns(),
		&tree.DatumAlloc{},
		&st.SV,
		false, /* internal */
		nil,   /* metrics */
	)
	require.NoError(t, err)

	j1, err := jsonutil.ParseJSON(`{"a":{"b":[99]}}`)
	require.NoError(t, err)
	j2, err := jsonutil.ParseJSON(`{"a":{"b":[5]}}`)
	require.NoError(t, err)

	b := &kv.Batch{}
	require.NoError(t, ri.InsertRow(
		ctx, b, []tree.Datum{tree.NewDInt(1), tree.NewDJSON(j1)}, PartialIndexUpdateHelper{}, true, /* overwrite */
		false, /* traceKV */
	))
	require.NoError(t, ri.InsertRow(
		ctx, b, []tree.Datum{tree.NewDInt(2), tree.NewDJSON(j2)}, PartialIndexUpdateHelper{}, true, /* overwrite */
		false, /* traceKV */
	))
	require.NoError(t, txn.Run(ctx, b))
	require.NoError(t, txn.Commit(ctx))

	sqlRunner.CheckQueryResults(t,
		`SELECT k, coalesce(doc#>>'{a,b,-1}', 'NULL') FROM d.json_upsert_multi WHERE doc ? 'a' ORDER BY k`,
		[][]string{{"1", "99"}, {"2", "5"}},
	)
}

func mustDJSON(t testing.TB, raw string) *tree.DJSON {
	t.Helper()
	j, err := jsonutil.ParseJSON(raw)
	require.NoError(t, err)
	return tree.NewDJSON(j)
}

func makeLargeRootArrayJSONDoc(targetBytes int) string {
	var b strings.Builder
	b.Grow(targetBytes + targetBytes/8)
	b.WriteByte('[')

	chunk := strings.Repeat("x", 240)
	for i := 0; b.Len() < targetBytes; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		if i == 10 {
			b.WriteString(`{"test":"v"}`)
			continue
		}
		fmt.Fprintf(&b, `{"junk":"%s","i":%d}`, chunk, i)
	}

	b.WriteByte(']')
	return b.String()
}

func makeLargeRootObjectJSONDoc(targetBytes int) string {
	var b strings.Builder
	b.Grow(targetBytes + targetBytes/8)
	b.WriteString(`{"test":"v","tail_delete":"gone"`)

	chunk := strings.Repeat("x", 240)
	for i := 0; b.Len() < targetBytes; i++ {
		fmt.Fprintf(&b, `,"k%06d":"%s"`, i, chunk)
	}

	b.WriteByte('}')
	return b.String()
}

func TestRecursiveJSONUpdaterLocalMutations(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()

	s, sqlDB, kvDB := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)

	sqlRunner := sqlutils.MakeSQLRunner(sqlDB)
	sqlRunner.Exec(t, `CREATE DATABASE d`)
	sqlRunner.Exec(t, `CREATE TABLE d.t (k INT PRIMARY KEY, doc JSONB)`)

	st := cluster.MakeTestingClusterSettings()

	applyMutation := func(
		t *testing.T, initial string, mutation SubordinateJSONMutationOp, expected string,
	) {
		t.Helper()
		sqlRunner.Exec(t, `DELETE FROM d.t`)
		sqlRunner.Exec(t, `INSERT INTO d.t VALUES (1, $1::JSONB)`, initial)

		tableDesc := desctestutils.TestingGetPublicTableDescriptor(
			kvDB, keys.SystemSQLCodec, "d", "t",
		)
		txn := kv.NewTxn(ctx, kvDB, 0)
		ru, err := MakeUpdater(
			ctx,
			txn,
			keys.SystemSQLCodec,
			tableDesc,
			[]catalog.Column{tableDesc.PublicColumns()[1]},
			tableDesc.PublicColumns(),
			UpdaterDefault,
			&tree.DatumAlloc{},
			&st.SV,
			false, /* internal */
			nil,   /* metrics */
		)
		require.NoError(t, err)

		b := &kv.Batch{}
		require.NoError(t, ru.UpdateSubordinateJSONRow(
			ctx,
			txn,
			b,
			[]tree.Datum{tree.NewDInt(1), tree.DNull},
			mutation,
			false, /* traceKV */
		))
		require.NoError(t, txn.Run(ctx, b))
		require.NoError(t, txn.Commit(ctx))
		sqlRunner.CheckQueryResults(t, `SELECT doc::STRING FROM d.t`, [][]string{{expected}})
	}

	t.Run("root object concat", func(t *testing.T) {
		applyMutation(t,
			`{"test":"old","keep":1}`,
			SubordinateJSONMutationOp{
				ColID: 2,
				Kind:  SubordinateJSONMutationConcat,
				Value: mustDJSON(t, `{"test":"new","added":2}`).JSON,
			},
			`{"added": 2, "keep": 1, "test": "new"}`,
		)
	})

	t.Run("root object delete key", func(t *testing.T) {
		applyMutation(t,
			`{"test":"old","keep":1}`,
			SubordinateJSONMutationOp{
				ColID: 2,
				Kind:  SubordinateJSONMutationDeleteKey,
				Key:   "test",
			},
			`{"keep": 1}`,
		)
	})

	t.Run("root object set key", func(t *testing.T) {
		applyMutation(t,
			`{"test":"old","keep":1}`,
			SubordinateJSONMutationOp{
				ColID:         2,
				Kind:          SubordinateJSONMutationSetPath,
				Path:          []string{"test"},
				Value:         mustDJSON(t, `"new"`).JSON,
				CreateMissing: false,
			},
			`{"keep": 1, "test": "new"}`,
		)
	})

	t.Run("root array append", func(t *testing.T) {
		applyMutation(t,
			`[{"test":"a"},{"test":"b"}]`,
			SubordinateJSONMutationOp{
				ColID: 2,
				Kind:  SubordinateJSONMutationConcat,
				Value: mustDJSON(t, `[{"test":"c"}]`).JSON,
			},
			`[{"test": "a"}, {"test": "b"}, {"test": "c"}]`,
		)
	})

	t.Run("root array delete last", func(t *testing.T) {
		applyMutation(t,
			`[{"test":"a"},{"test":"b"}]`,
			SubordinateJSONMutationOp{
				ColID: 2,
				Kind:  SubordinateJSONMutationDeleteLastArrayElement,
			},
			`[{"test": "a"}]`,
		)
	})

	t.Run("root array object key set", func(t *testing.T) {
		applyMutation(t,
			`[{"test":"a"},{"test":"b"}]`,
			SubordinateJSONMutationOp{
				ColID:         2,
				Kind:          SubordinateJSONMutationSetPath,
				Path:          []string{"1", "test"},
				Value:         mustDJSON(t, `"updated"`).JSON,
				CreateMissing: false,
			},
			`[{"test": "a"}, {"test": "updated"}]`,
		)
	})
}

func TestRecursiveJSONUpdaterLargeRootArrayMutationsStayLocal(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()

	s, sqlDB, kvDB := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)

	sqlRunner := sqlutils.MakeSQLRunner(sqlDB)
	sqlRunner.Exec(t, `CREATE DATABASE d`)
	sqlRunner.Exec(t, `CREATE TABLE d.t (k INT PRIMARY KEY, doc JSONB)`)

	st := cluster.MakeTestingClusterSettings()
	const targetBytes = 128 << 10
	testCases := []struct {
		name               string
		initialDoc         string
		mutation           SubordinateJSONMutationOp
		expectedProjection string
	}{
		{
			name:       "append root array element",
			initialDoc: makeLargeRootArrayJSONDoc(targetBytes),
			mutation: SubordinateJSONMutationOp{
				ColID: 2,
				Kind:  SubordinateJSONMutationConcat,
				Value: mustDJSON(t, `[{"test":"appended"}]`).JSON,
			},
			expectedProjection: `CASE WHEN jsonb_array_length(doc) > 10 AND doc->10->>'test' = 'v' AND doc->(jsonb_array_length(doc) - 1)->>'test' = 'appended' THEN 1 ELSE 0 END`,
		},
		{
			name:       "delete last root array element",
			initialDoc: makeLargeRootArrayJSONDoc(targetBytes),
			mutation: SubordinateJSONMutationOp{
				ColID: 2,
				Kind:  SubordinateJSONMutationDeleteLastArrayElement,
			},
			expectedProjection: `CASE WHEN jsonb_array_length(doc) >= 10 AND doc->10->>'test' = 'v' THEN 1 ELSE 0 END`,
		},
		{
			name:       "set root array element key",
			initialDoc: makeLargeRootArrayJSONDoc(targetBytes),
			mutation: SubordinateJSONMutationOp{
				ColID:         2,
				Kind:          SubordinateJSONMutationSetPath,
				Path:          []string{"10", "test"},
				Value:         mustDJSON(t, `"updated"`).JSON,
				CreateMissing: false,
			},
			expectedProjection: `CASE WHEN doc->10->>'test' = 'updated' THEN 1 ELSE 0 END`,
		},
		{
			name:       "append root object key",
			initialDoc: makeLargeRootObjectJSONDoc(targetBytes),
			mutation: SubordinateJSONMutationOp{
				ColID: 2,
				Kind:  SubordinateJSONMutationConcat,
				Value: mustDJSON(t, `{"appended":"v"}`).JSON,
			},
			expectedProjection: `CASE WHEN doc->>'test' = 'v' AND doc->>'appended' = 'v' THEN 1 ELSE 0 END`,
		},
		{
			name:       "delete root object key",
			initialDoc: makeLargeRootObjectJSONDoc(targetBytes),
			mutation: SubordinateJSONMutationOp{
				ColID: 2,
				Kind:  SubordinateJSONMutationDeleteKey,
				Key:   "tail_delete",
			},
			expectedProjection: `CASE WHEN doc->>'tail_delete' IS NULL AND doc->>'test' = 'v' THEN 1 ELSE 0 END`,
		},
		{
			name:       "set root object key",
			initialDoc: makeLargeRootObjectJSONDoc(targetBytes),
			mutation: SubordinateJSONMutationOp{
				ColID:         2,
				Kind:          SubordinateJSONMutationSetPath,
				Path:          []string{"test"},
				Value:         mustDJSON(t, `"updated"`).JSON,
				CreateMissing: false,
			},
			expectedProjection: `CASE WHEN doc->>'test' = 'updated' THEN 1 ELSE 0 END`,
		},
	}

	applyLocalMutation := func(
		t *testing.T, initialDoc string, mutation SubordinateJSONMutationOp, expectedProjection string,
	) {
		t.Helper()
		sqlRunner.Exec(t, `DELETE FROM d.t`)
		sqlRunner.Exec(t, `INSERT INTO d.t VALUES (1, $1::JSONB)`, initialDoc)

		tableDesc := desctestutils.TestingGetPublicTableDescriptor(
			kvDB, keys.SystemSQLCodec, "d", "t",
		)
		txn := kv.NewTxn(ctx, kvDB, 0)
		ru, err := MakeUpdater(
			ctx,
			txn,
			keys.SystemSQLCodec,
			tableDesc,
			[]catalog.Column{tableDesc.PublicColumns()[1]},
			tableDesc.PublicColumns(),
			UpdaterDefault,
			&tree.DatumAlloc{},
			&st.SV,
			false, /* internal */
			nil,   /* metrics */
		)
		require.NoError(t, err)

		oldValues := []tree.Datum{tree.NewDInt(1), tree.DNull}
		primaryIndexKey, err := ru.Helper.encodePrimaryIndex(ru.FetchColIDtoRowIndex, oldValues)
		require.NoError(t, err)
		rowKey := keys.MakeFamilyKey(primaryIndexKey, 0)
		headerKey := roachpb.Key(keys.MakeSubordinatePathKey(rowKey, 2, []keys.SubordinatePathSegment{
			{Kind: keys.SubordinatePathHeader},
		}))

		b := &kv.Batch{}
		localApplied, err := ru.tryApplyLocalSubordinateJSONMutation(
			ctx, txn, b, rowKey, headerKey, mutation, false, /* traceKV */
		)
		require.NoError(t, err)
		require.True(t, localApplied)
		require.Less(t, int64(b.ApproximateMutationBytes()), int64(1<<20))

		require.NoError(t, txn.Run(ctx, b))
		require.NoError(t, txn.Commit(ctx))
		sqlRunner.CheckQueryResults(t, `SELECT `+expectedProjection+` FROM d.t`, [][]string{{"1"}})
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			applyLocalMutation(t, tc.initialDoc, tc.mutation, tc.expectedProjection)
		})
	}
}

func TestRecursiveJSONUpdaterClearColumn(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()

	s, sqlDB, kvDB := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)

	sqlRunner := sqlutils.MakeSQLRunner(sqlDB)
	sqlRunner.Exec(t, `CREATE DATABASE d`)
	sqlRunner.Exec(t, `CREATE TABLE d.t (k INT PRIMARY KEY, doc JSONB)`)
	sqlRunner.Exec(t, `INSERT INTO d.t VALUES (1, '{"test":"old","keep":1}')`)

	tableDesc := desctestutils.TestingGetPublicTableDescriptor(
		kvDB, keys.SystemSQLCodec, "d", "t",
	)
	st := cluster.MakeTestingClusterSettings()
	txn := kv.NewTxn(ctx, kvDB, 0)
	ru, err := MakeUpdater(
		ctx,
		txn,
		keys.SystemSQLCodec,
		tableDesc,
		[]catalog.Column{tableDesc.PublicColumns()[1]},
		tableDesc.PublicColumns(),
		UpdaterDefault,
		&tree.DatumAlloc{},
		&st.SV,
		false, /* internal */
		nil,   /* metrics */
	)
	require.NoError(t, err)

	b := &kv.Batch{}
	require.NoError(t, ru.ClearSubordinateJSONColumn(
		ctx, b, []tree.Datum{tree.NewDInt(1), tree.DNull}, 2, false, /* traceKV */
	))
	require.NoError(t, txn.Run(ctx, b))
	require.NoError(t, txn.Commit(ctx))

	sqlRunner.CheckQueryResults(t, `SELECT doc IS NULL FROM d.t`, [][]string{{"true"}})
}

func mustGetTuple(t *testing.T, v roachpb.Value) []byte {
	t.Helper()
	raw, err := v.GetTuple()
	require.NoError(t, err)
	return raw
}

func subordinatePathString(path []keys.SubordinatePathSegment) string {
	var s string
	for _, seg := range path {
		switch seg.Kind {
		case keys.SubordinatePathHeader:
			s += "$"
		case keys.SubordinatePathObjectKey:
			s += "." + seg.ObjectKey
		case keys.SubordinatePathArrayIndex:
			s += "[" + tree.NewDInt(tree.DInt(seg.ArrayIdx)).String() + "]"
		}
	}
	return s
}

func mustPrimaryKeyPrefix(
	t *testing.T, tableDesc catalog.TableDescriptor, kvDB *kv.DB, span roachpb.Span,
) []byte {
	t.Helper()
	kvs, err := kvDB.Scan(context.Background(), span.Key, span.EndKey, 1)
	require.NoError(t, err)
	require.NotEmpty(t, kvs)
	prefixLen, err := keys.GetRowPrefixLength(kvs[0].Key)
	require.NoError(t, err)
	return kvs[0].Key[:prefixLen]
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
	rf := initFetcher(t, initFetcherArgs{
		tableDesc: tableDesc,
		indexIdx:  0,
		columns:   []int{0},
	}, false /* reverseScan */, &tree.DatumAlloc{}, nil /* memMon */)

	require.NoError(t, rf.StartScan(
		ctx,
		kv.NewTxn(ctx, kvDB, 0),
		roachpb.Spans{tableDesc.IndexSpan(keys.SystemSQLCodec, tableDesc.GetPrimaryIndexID())},
		rowinfra.NoBytesLimit,
		rowinfra.NoRowLimit,
		false, /* traceKV */
		false, /* forceProductionKVBatchSize */
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
