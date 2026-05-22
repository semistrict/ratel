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
	"context"
	"strings"

	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgcode"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgerror"
	"github.com/cockroachdb/cockroach/pkg/sql/row"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree/treebin"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/errors"
)

type subordinateJSONMutationPlan struct {
	colID           descpb.ColumnID
	valueSourceIdx  int
	kind            row.SubordinateJSONMutationKind
	key             string
	path            []string
	createMissing   bool
	nullOnValueNull bool
}

func (p *subordinateJSONMutationPlan) buildOp(sourceVals tree.Datums) (row.SubordinateJSONMutationOp, bool, error) {
	op := row.SubordinateJSONMutationOp{
		ColID:         p.colID,
		Kind:          p.kind,
		Key:           p.key,
		Path:          append([]string(nil), p.path...),
		CreateMissing: p.createMissing,
	}
	if p.valueSourceIdx < 0 {
		return op, false, nil
	}
	if sourceVals[p.valueSourceIdx] == tree.DNull {
		if p.nullOnValueNull {
			return op, true, nil
		}
		return op, false, pgerror.New(pgcode.NullValueNotAllowed, "NULL JSON mutation operand")
	}
	dj, ok := sourceVals[p.valueSourceIdx].(*tree.DJSON)
	if !ok || dj == nil {
		return op, false, errors.AssertionFailedf("expected JSON datum at source index %d", p.valueSourceIdx)
	}
	op.Value = dj.JSON
	return op, false, nil
}

func tryPlanSubordinateJSONDirectUpdate(
	ctx context.Context,
	evalCtx *eval.Context,
	tableDesc catalog.TableDescriptor,
	source planNode,
	fetchCols []catalog.Column,
	updateCols []catalog.Column,
	rowsNeeded bool,
	checks checkSet,
) (*subordinateJSONMutationPlan, error) {
	if rowsNeeded || !checks.Empty() || len(updateCols) != 1 {
		return nil, nil
	}
	if len(tableDesc.DeletableNonPrimaryIndexes()) != 0 {
		return nil, nil
	}
	updateCol := updateCols[0]
	if updateCol.GetType().Family() != types.JsonFamily {
		return nil, nil
	}

	r, ok := source.(*renderNode)
	if !ok {
		return nil, nil
	}
	scan, ok := r.source.plan.(*scanNode)
	if !ok {
		return nil, nil
	}

	updateToFetch := row.ColMapping([]catalog.Column{updateCol}, fetchCols)
	if len(updateToFetch) != 1 {
		return nil, errors.AssertionFailedf("expected one update column mapping, got %d", len(updateToFetch))
	}
	fetchIdx := updateToFetch[0]
	if fetchIdx < 0 || len(r.render) < len(fetchCols)+1 {
		return nil, nil
	}
	oldExpr, ok := stripTransparentTypedCasts(r.render[fetchIdx]).(*tree.IndexedVar)
	if !ok {
		return nil, nil
	}
	plan, replacement, ok, err := tryBuildSubordinateJSONMutationPlan(evalCtx, updateCol, r.render[len(fetchCols)], oldExpr.Idx, len(fetchCols))
	if err != nil || !ok {
		return nil, err
	}

	nullJSON, ok := eval.ReType(tree.DNull, types.Jsonb)
	if !ok {
		return nil, errors.AssertionFailedf("failed to construct typed NULL JSON")
	}
	r.render[fetchIdx] = nullJSON.(tree.TypedExpr)
	if replacement == nil {
		r.render[len(fetchCols)] = nullJSON.(tree.TypedExpr)
	} else {
		r.render[len(fetchCols)] = replacement
	}

	if exprUsesIndexedVar(r.render, oldExpr.Idx) {
		return nil, nil
	}
	if err := dropRenderSourceColumn(r, scan, oldExpr.Idx); err != nil {
		return nil, err
	}
	return plan, nil
}

func tryBuildSubordinateJSONMutationPlan(
	evalCtx *eval.Context,
	updateCol catalog.Column,
	expr tree.TypedExpr,
	sourceIdx int,
	valueSourceIdx int,
) (*subordinateJSONMutationPlan, tree.TypedExpr, bool, error) {
	expr = stripTransparentTypedCasts(expr)
	switch t := expr.(type) {
	case *tree.BinaryExpr:
		switch t.Operator.Symbol {
		case treebin.Concat:
			left, ok := stripTransparentTypedCasts(t.TypedLeft()).(*tree.IndexedVar)
			if !ok || left.Idx != sourceIdx || containsIndexedVar(t.TypedRight()) {
				return nil, nil, false, nil
			}
			return &subordinateJSONMutationPlan{
				colID:           updateCol.GetID(),
				valueSourceIdx:  valueSourceIdx,
				kind:            row.SubordinateJSONMutationConcat,
				nullOnValueNull: true,
			}, stripTransparentTypedCasts(t.TypedRight()), true, nil
		case treebin.Minus:
			left, ok := stripTransparentTypedCasts(t.TypedLeft()).(*tree.IndexedVar)
			if !ok || left.Idx != sourceIdx {
				return nil, nil, false, nil
			}
			if key, ok, err := extractJSONStringKeyFromTypedExpr(evalCtx, t.TypedRight()); err != nil {
				return nil, nil, false, err
			} else if ok {
				return &subordinateJSONMutationPlan{
					colID:          updateCol.GetID(),
					valueSourceIdx: -1,
					kind:           row.SubordinateJSONMutationDeleteKey,
					key:            key,
				}, nil, true, nil
			}
			if isDeleteLastJSONArrayExpr(evalCtx, sourceIdx, t.TypedRight()) {
				return &subordinateJSONMutationPlan{
					colID:          updateCol.GetID(),
					valueSourceIdx: -1,
					kind:           row.SubordinateJSONMutationDeleteLastArrayElement,
				}, nil, true, nil
			}
		}
	case *tree.FuncExpr:
		if !strings.EqualFold(t.Func.String(), "jsonb_set") {
			return nil, nil, false, nil
		}
		if len(t.Exprs) != 3 && len(t.Exprs) != 4 {
			return nil, nil, false, nil
		}
		source, ok := stripTransparentTypedCasts(t.Exprs[0].(tree.TypedExpr)).(*tree.IndexedVar)
		if !ok || source.Idx != sourceIdx {
			return nil, nil, false, nil
		}
		path, ok, err := extractJSONStringPathArray(evalCtx, t.Exprs[1].(tree.TypedExpr))
		if err != nil || !ok || containsIndexedVar(t.Exprs[2].(tree.TypedExpr)) {
			return nil, nil, false, err
		}
		createMissing := true
		if len(t.Exprs) == 4 {
			d, ok, err := evalConstDatum(evalCtx, t.Exprs[3].(tree.TypedExpr))
			if err != nil || !ok || d == tree.DNull {
				return nil, nil, false, err
			}
			createMissing = bool(tree.MustBeDBool(d))
		}
		return &subordinateJSONMutationPlan{
			colID:           updateCol.GetID(),
			valueSourceIdx:  valueSourceIdx,
			kind:            row.SubordinateJSONMutationSetPath,
			path:            path,
			createMissing:   createMissing,
			nullOnValueNull: true,
		}, stripTransparentTypedCasts(t.Exprs[2].(tree.TypedExpr)), true, nil
	}
	return nil, nil, false, nil
}

func extractJSONStringPathArray(
	evalCtx *eval.Context, expr tree.TypedExpr,
) ([]string, bool, error) {
	d, ok, err := evalConstDatum(evalCtx, expr)
	if err != nil || !ok || d == tree.DNull {
		return nil, ok, err
	}
	arr, ok := d.(*tree.DArray)
	if !ok {
		return nil, false, nil
	}
	path := make([]string, arr.Len())
	for i := range arr.Array {
		if arr.Array[i] == tree.DNull {
			return nil, false, pgerror.Newf(pgcode.NullValueNotAllowed, "path element at position %d is null", i+1)
		}
		s, ok := arr.Array[i].(*tree.DString)
		if !ok {
			return nil, false, nil
		}
		path[i] = string(*s)
	}
	return path, true, nil
}

func isDeleteLastJSONArrayExpr(evalCtx *eval.Context, sourceIdx int, expr tree.TypedExpr) bool {
	expr = stripTransparentTypedCasts(expr)
	bin, ok := expr.(*tree.BinaryExpr)
	if !ok || bin.Operator.Symbol != treebin.Minus {
		return false
	}
	left, ok := stripTransparentTypedCasts(bin.TypedLeft()).(*tree.FuncExpr)
	if !ok || !strings.EqualFold(left.Func.String(), "jsonb_array_length") || len(left.Exprs) != 1 {
		return false
	}
	arg, ok := stripTransparentTypedCasts(left.Exprs[0].(tree.TypedExpr)).(*tree.IndexedVar)
	if !ok || arg.Idx != sourceIdx {
		return false
	}
	d, ok, err := evalConstDatum(evalCtx, bin.TypedRight())
	return err == nil && ok && d != tree.DNull && int64(tree.MustBeDInt(d)) == 1
}

func containsIndexedVar(expr tree.TypedExpr) bool {
	found := false
	_, _ = tree.SimpleVisit(expr, func(expr tree.Expr) (bool, tree.Expr, error) {
		if _, ok := expr.(*tree.IndexedVar); ok {
			found = true
			return false, expr, nil
		}
		return true, expr, nil
	})
	return found
}

func exprUsesIndexedVar(exprs []tree.TypedExpr, idx int) bool {
	for i := range exprs {
		used := false
		_, _ = tree.SimpleVisit(exprs[i], func(expr tree.Expr) (bool, tree.Expr, error) {
			if iv, ok := expr.(*tree.IndexedVar); ok && iv.Idx == idx {
				used = true
				return false, expr, nil
			}
			return true, expr, nil
		})
		if used {
			return true
		}
	}
	return false
}

type indexedVarRemapVisitor struct {
	removed int
}

func (v indexedVarRemapVisitor) VisitPre(expr tree.Expr) (bool, tree.Expr) {
	iv, ok := expr.(*tree.IndexedVar)
	if !ok {
		return true, expr
	}
	if iv.Idx == v.removed {
		panic(errors.AssertionFailedf("indexed var %d was not removed before remap", v.removed))
	}
	if iv.Idx < v.removed {
		return false, tree.NewTypedOrdinalReference(iv.Idx, iv.ResolvedType())
	}
	return false, tree.NewTypedOrdinalReference(iv.Idx-1, iv.ResolvedType())
}

func (indexedVarRemapVisitor) VisitPost(expr tree.Expr) tree.Expr { return expr }

func dropRenderSourceColumn(r *renderNode, scan *scanNode, sourceIdx int) error {
	newWanted := append([]tree.ColumnID(nil), scan.colCfg.wantedColumns[:sourceIdx]...)
	newWanted = append(newWanted, scan.colCfg.wantedColumns[sourceIdx+1:]...)
	scan.colCfg.wantedColumns = newWanted
	if err := scan.initDescDefaults(scan.colCfg); err != nil {
		return err
	}
	r.source.columns = scan.resultColumns
	r.ivarHelper = tree.MakeIndexedVarHelper(r, len(r.source.columns))
	visitor := indexedVarRemapVisitor{removed: sourceIdx}
	for i := range r.render {
		expr, changed := tree.WalkExpr(visitor, r.render[i])
		if changed {
			r.render[i] = r.ivarHelper.Rebind(expr.(tree.TypedExpr))
			continue
		}
		r.render[i] = r.ivarHelper.Rebind(r.render[i])
	}
	return nil
}
