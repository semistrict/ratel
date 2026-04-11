// Copyright 2016 The Cockroach Authors.
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
//
// This file implements the select code that deals with column references
// and resolving column names in expressions.

package schemaexpr

import (
	"context"

	"github.com/semistrict/ratel/pkg/sql/catalog/colinfo"
	"github.com/semistrict/ratel/pkg/sql/sem/tree"
	"github.com/semistrict/ratel/pkg/sql/sessiondata"
	"github.com/semistrict/ratel/pkg/sql/sqlerrors"
)

// NameResolutionVisitor is a tree.Visitor implementation used to
// resolve the column names in an expression.
type NameResolutionVisitor struct {
	err        error
	iVarHelper tree.IndexedVarHelper
	searchPath sessiondata.SearchPath
	resolver   colinfo.ColumnResolver
}

var _ tree.Visitor = &NameResolutionVisitor{}

func unresolvedNamePartsInNaturalOrder(n *tree.UnresolvedName) ([]string, []string) {
	parts := make([]string, n.NumParts)
	rawParts := make([]string, n.NumParts)
	for i := 0; i < n.NumParts; i++ {
		reversedIdx := n.NumParts - 1 - i
		parts[i] = n.Parts[reversedIdx]
		rawParts[i] = n.RawPart(reversedIdx)
	}
	return parts, rawParts
}

func isUndefinedNameResolutionError(err error) bool {
	return sqlerrors.IsUndefinedColumnError(err) || sqlerrors.IsUndefinedRelationError(err)
}

func resolveJSONPathExpr(base tree.Expr, parts []string, rawParts []string) tree.Expr {
	expr := base
	for i := range parts {
		raw := parts[i]
		if rawParts != nil && i < len(rawParts) && rawParts[i] != "" {
			raw = rawParts[i]
		}
		expr = &tree.ColumnAccessExpr{
			Expr:       expr,
			ColName:    tree.Name(parts[i]),
			RawColName: raw,
		}
	}
	return expr
}

func (v *NameResolutionVisitor) maybeResolveJSONDottedPath(
	n *tree.UnresolvedName,
) (tree.Expr, bool) {
	if n.Star || n.NumParts < 2 {
		return nil, false
	}

	if n.NumParts <= 4 {
		vn, err := n.NormalizeVarName()
		if err != nil {
			v.err = err
			return nil, false
		}
		colItem, ok := vn.(*tree.ColumnItem)
		if !ok {
			return nil, false
		}
		if _, err := colinfo.ResolveColumnItem(context.TODO(), &v.resolver, colItem); err == nil {
			return nil, false
		} else if !isUndefinedNameResolutionError(err) {
			v.err = err
			return nil, false
		}
	}

	parts, rawParts := unresolvedNamePartsInNaturalOrder(n)
	for prefixLen := n.NumParts - 1; prefixLen >= 1; prefixLen-- {
		prefixName := tree.MakeUnresolvedNameWithRawParts(parts[:prefixLen], rawParts[:prefixLen])
		vn, err := (&prefixName).NormalizeVarName()
		if err != nil {
			v.err = err
			return nil, false
		}
		colItem, ok := vn.(*tree.ColumnItem)
		if !ok {
			continue
		}
		_, err = colinfo.ResolveColumnItem(context.TODO(), &v.resolver, colItem)
		if err != nil {
			if isUndefinedNameResolutionError(err) {
				continue
			}
			v.err = err
			return nil, false
		}
		colIdx := v.resolver.ResolverState.ColIdx
		ivar := v.iVarHelper.IndexedVar(colIdx)
		return resolveJSONPathExpr(ivar, parts[prefixLen:], rawParts[prefixLen:]), true
	}

	return nil, false
}

// VisitPre implements tree.Visitor.
func (v *NameResolutionVisitor) VisitPre(expr tree.Expr) (recurse bool, newNode tree.Expr) {
	if v.err != nil {
		return false, expr
	}

	switch t := expr.(type) {
	case *tree.IndexedVar:
		// If the indexed var is a standalone ordinal reference, ensure it
		// becomes a fully bound indexed var.
		t, v.err = v.iVarHelper.BindIfUnbound(t)
		if v.err != nil {
			return false, expr
		}

		return false, t

	case *tree.UnresolvedName:
		if expr, ok := v.maybeResolveJSONDottedPath(t); ok {
			return true, expr
		}
		if v.err != nil {
			return false, expr
		}
		vn, err := t.NormalizeVarName()
		if err != nil {
			v.err = err
			return false, expr
		}
		return v.VisitPre(vn)

	case *tree.ColumnItem:
		_, err := colinfo.ResolveColumnItem(context.TODO(), &v.resolver, t)
		if err != nil {
			v.err = err
			return false, expr
		}

		colIdx := v.resolver.ResolverState.ColIdx
		ivar := v.iVarHelper.IndexedVar(colIdx)
		return true, ivar

	case *tree.FuncExpr:
		// Check for invalid use of *, which, if it is an argument, is the only argument.
		if len(t.Exprs) != 1 {
			break
		}
		vn, ok := t.Exprs[0].(tree.VarName)
		if !ok {
			break
		}
		vn, v.err = vn.NormalizeVarName()
		if v.err != nil {
			return false, expr
		}
		// Save back to avoid re-doing the work later.
		t.Exprs[0] = vn
		return true, t
	}

	return true, expr
}

// VisitPost implements tree.Visitor.
func (*NameResolutionVisitor) VisitPost(expr tree.Expr) tree.Expr { return expr }

// ResolveNamesUsingVisitor resolves the names in the given expression.
func ResolveNamesUsingVisitor(
	v *NameResolutionVisitor,
	expr tree.Expr,
	source *colinfo.DataSourceInfo,
	ivarHelper tree.IndexedVarHelper,
	searchPath sessiondata.SearchPath,
) (tree.Expr, error) {
	*v = NameResolutionVisitor{
		iVarHelper: ivarHelper,
		searchPath: searchPath,
		resolver: colinfo.ColumnResolver{
			Source: source,
		},
	}

	expr, _ = tree.WalkExpr(v, expr)
	return expr, v.err
}
