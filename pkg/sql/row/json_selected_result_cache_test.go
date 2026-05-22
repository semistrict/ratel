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
	"testing"

	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	jsonutil "github.com/cockroachdb/cockroach/pkg/util/json"
	"github.com/stretchr/testify/require"
)

func TestJSONSelectedPathResultCacheCachesAcrossKinds(t *testing.T) {
	var builder SubordinateJSONBuilder
	scalar, err := jsonutil.ParseJSON(`1`)
	require.NoError(t, err)
	require.NoError(t, builder.Set(
		[]keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}},
		SubordinateJSONNodeScalar,
		scalar,
	))

	var cache JSONSelectedPathResultCache
	jsonDatum, err := cache.ResultDatum(&builder, JSONAccessFetchJSONPath)
	require.NoError(t, err)
	require.Equal(t, `1`, jsonDatum.(*tree.DJSON).JSON.String())

	textDatum, err := cache.ResultDatum(nil, JSONAccessFetchTextPath)
	require.NoError(t, err)
	require.Equal(t, "1", string(*textDatum.(*tree.DString)))

	cache.Reset()

	textDatum, err = cache.ResultDatum(&builder, JSONAccessFetchTextPath)
	require.NoError(t, err)
	require.Equal(t, "1", string(*textDatum.(*tree.DString)))
}

func TestJSONSelectedPathResultCacheMissingPath(t *testing.T) {
	var cache JSONSelectedPathResultCache

	jsonDatum, err := cache.ResultDatum(nil, JSONAccessFetchJSONPath)
	require.NoError(t, err)
	require.Equal(t, "NULL", jsonDatum.String())

	textDatum, err := cache.ResultDatum(nil, JSONAccessFetchTextPath)
	require.NoError(t, err)
	require.Equal(t, "NULL", textDatum.String())
}

func TestJSONSelectedPathResultCacheCachesContainmentResults(t *testing.T) {
	var builder SubordinateJSONBuilder
	doc, err := jsonutil.ParseJSON(`1`)
	require.NoError(t, err)
	require.NoError(t, builder.Set(
		[]keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}},
		SubordinateJSONNodeScalar,
		doc,
	))

	right, err := jsonutil.ParseJSON(`1`)
	require.NoError(t, err)
	program, err := NewJSONContainsProgram(nil, right, false)
	require.NoError(t, err)

	var cache JSONSelectedPathResultCache
	contains, err := cache.ContainsResult(&builder, program)
	require.NoError(t, err)
	require.True(t, contains)
	require.Len(t, cache.contains, 1)

	contains, err = cache.ContainsResult(nil, program)
	require.NoError(t, err)
	require.True(t, contains)
	require.Len(t, cache.contains, 1)
}
