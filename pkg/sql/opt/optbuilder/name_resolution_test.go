// Copyright 2018 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package optbuilder

import (
	"context"
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/sql/catalog/colinfo"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/colinfo/colinfotestutils"
	"github.com/cockroachdb/cockroach/pkg/sql/parser"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
)

var _ colinfotestutils.ColumnItemResolverTester = &scope{}

// GetColumnItemResolver is part of the sqlutils.ColumnItemResolverTester
// interface.
func (s *scope) GetColumnItemResolver() colinfo.ColumnItemResolver {
	return s
}

// AddTable is part of the sqlutils.ColumnItemResolverTester interface.
func (s *scope) AddTable(tabName tree.TableName, colNames []tree.Name) {
	for _, col := range colNames {
		s.cols = append(s.cols, scopeColumn{name: scopeColName(col), table: tabName})
	}
}

// ResolveQualifiedStarTestResults is part of the
// sqlutils.ColumnItemResolverTester interface.
func (s *scope) ResolveQualifiedStarTestResults(
	srcName *tree.TableName, srcMeta colinfo.ColumnSourceMeta,
) (string, string, error) {
	s, ok := srcMeta.(*scope)
	if !ok {
		return "", "", fmt.Errorf("resolver did not return *scope, found %T instead", srcMeta)
	}
	nl := make(tree.NameList, 0, len(s.cols))
	for i := range s.cols {
		col := s.cols[i]
		if col.table == *srcName && col.visibility == visible {
			nl = append(nl, col.name.ReferenceName())
		}
	}
	return srcName.String(), nl.String(), nil
}

// ResolveColumnItemTestResults is part of the
// sqlutils.ColumnItemResolverTester interface.
func (s *scope) ResolveColumnItemTestResults(
	colRes colinfo.ColumnResolutionResult,
) (string, error) {
	col, ok := colRes.(*scopeColumn)
	if !ok {
		return "", fmt.Errorf("resolver did not return *scopeColumn, found %T instead", colRes)
	}
	return fmt.Sprintf("%s.%s", col.table.String(), col.name.ReferenceName()), nil
}

func TestResolveQualifiedStar(t *testing.T) {
	s := &scope{}
	colinfotestutils.RunResolveQualifiedStarTest(t, s)
}

func TestResolveColumnItem(t *testing.T) {
	s := &scope{}
	colinfotestutils.RunResolveColumnItemTest(t, s)
}

func TestResolveJSONDottedPath(t *testing.T) {
	semaCtx := tree.MakeSemaContext()
	s := &scope{
		builder: &Builder{
			ctx:     context.Background(),
			semaCtx: &semaCtx,
		},
	}
	s.cols = append(s.cols, scopeColumn{
		name:       scopeColName("j"),
		table:      tree.MakeUnqualifiedTableName("t"),
		typ:        types.Jsonb,
		visibility: visible,
	})

	testCases := []struct {
		expr     string
		expected string
	}{
		{expr: `j.Foo`, expected: `j->'Foo':::STRING`},
		{expr: `t.j.Foo.Bar`, expected: `(j->'Foo':::STRING)->'Bar':::STRING`},
		{expr: `j.Foo.Bar.Baz.Quux`, expected: `(((j->'Foo':::STRING)->'Bar':::STRING)->'Baz':::STRING)->'Quux':::STRING`},
	}

	for _, tc := range testCases {
		t.Run(tc.expr, func(t *testing.T) {
			expr, err := parser.ParseExpr(tc.expr)
			if err != nil {
				t.Fatal(err)
			}
			typed := s.resolveType(expr, types.Any)
			if actual := tree.Serialize(typed); actual != tc.expected {
				t.Fatalf("expected %s, got %s", tc.expected, actual)
			}
		})
	}
}

func TestResolveJSONDottedPathPrefersQualifiedColumn(t *testing.T) {
	semaCtx := tree.MakeSemaContext()
	s := &scope{
		builder: &Builder{
			ctx:     context.Background(),
			semaCtx: &semaCtx,
		},
	}
	s.cols = append(s.cols, scopeColumn{
		name:       scopeColName("foo"),
		table:      tree.MakeUnqualifiedTableName("j"),
		typ:        types.Int,
		visibility: visible,
	})

	expr, err := parser.ParseExpr(`j.foo`)
	if err != nil {
		t.Fatal(err)
	}
	typed := s.resolveType(expr, types.Any)
	if actual := tree.Serialize(typed); actual != `foo` {
		t.Fatalf("expected foo, got %s", actual)
	}
	if actual := typed.ResolvedType(); actual != types.Int {
		t.Fatalf("expected %s, got %s", types.Int, actual)
	}
}
