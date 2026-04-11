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

package sql

import (
	"testing"

	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/roachpb"
	"github.com/semistrict/ratel/pkg/sql/catalog/descpb"
	"github.com/semistrict/ratel/pkg/util/encoding"
	"github.com/semistrict/ratel/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

func TestJSONPointLookupSpanBuilderSelectedPathIncludesAncestorHeaders(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rowKey := keys.MakeFamilyKey(
		encoding.EncodeUvarintAscending(keys.SystemSQLCodec.IndexPrefix(42, 1), 7), 0,
	)
	builder := newJSONPointLookupSpanBuilder(roachpb.Key(rowKey))
	builder.addSelectedPath(5, []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "needle"},
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "tiny"},
	})
	spans := builder.finish()

	require.Contains(t, spans, roachpb.Span{
		Key:    roachpb.Key(keys.MakeSubordinatePathKey(rowKey, 5, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}})),
		EndKey: roachpb.Key(keys.MakeSubordinatePathKey(rowKey, 5, []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}})).Next(),
	})
	require.Contains(t, spans, roachpb.Span{
		Key: roachpb.Key(keys.MakeSubordinatePathKey(rowKey, 5, []keys.SubordinatePathSegment{
			{Kind: keys.SubordinatePathHeader},
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "needle"},
		})),
		EndKey: roachpb.Key(keys.MakeSubordinatePathKey(rowKey, 5, []keys.SubordinatePathSegment{
			{Kind: keys.SubordinatePathHeader},
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "needle"},
		})).Next(),
	})
	require.Contains(t, spans, roachpb.Span{
		Key: roachpb.Key(keys.MakeSubordinatePathPrefix(rowKey, 5, []keys.SubordinatePathSegment{
			{Kind: keys.SubordinatePathHeader},
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "needle"},
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "tiny"},
		})),
		EndKey: roachpb.Key(keys.MakeSubordinatePathPrefix(rowKey, 5, []keys.SubordinatePathSegment{
			{Kind: keys.SubordinatePathHeader},
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "needle"},
			{Kind: keys.SubordinatePathObjectKey, ObjectKey: "tiny"},
		})).PrefixEnd(),
	})
}

func TestJSONPointLookupSpanBuilderSingleStepPathOmitsExtraAncestorSpan(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rowKey := keys.MakeFamilyKey(
		encoding.EncodeUvarintAscending(keys.SystemSQLCodec.IndexPrefix(42, 1), 7), 0,
	)
	builder := newJSONPointLookupSpanBuilder(roachpb.Key(rowKey))
	builder.addSelectedPath(descpb.ColumnID(5), []keys.SubordinatePathSegment{
		{Kind: keys.SubordinatePathObjectKey, ObjectKey: "needle"},
	})
	spans := builder.finish()

	require.Len(t, spans, 3)
}
