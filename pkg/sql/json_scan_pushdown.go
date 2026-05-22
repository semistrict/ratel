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
	"sort"
	"strconv"

	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfrapb"
	"github.com/cockroachdb/cockroach/pkg/sql/opt/exec"
	"github.com/cockroachdb/cockroach/pkg/sql/physicalplan"
	"github.com/cockroachdb/cockroach/pkg/sql/row"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree/treebin"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree/treecmp"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
)

const (
	jsonPathStepKeyPrefix        = "k:"
	jsonPathStepIndexPrefix      = "i:"
	jsonPathStepKeyOrIndexPrefix = "p:"
)

type jsonPointLookupSpanBuilder struct {
	rowKey roachpb.Key
	spans  roachpb.Spans
	seen   map[string]struct{}
}

func newJSONPointLookupSpanBuilder(rowKey roachpb.Key) *jsonPointLookupSpanBuilder {
	b := &jsonPointLookupSpanBuilder{
		rowKey: append(roachpb.Key(nil), rowKey...),
		seen:   make(map[string]struct{}),
	}
	b.addExactKey(b.rowKey)
	return b
}

func (b *jsonPointLookupSpanBuilder) addSpan(span roachpb.Span) {
	key := string(span.Key) + "\x00" + string(span.EndKey)
	if _, ok := b.seen[key]; ok {
		return
	}
	b.seen[key] = struct{}{}
	b.spans = append(b.spans, span)
}

func (b *jsonPointLookupSpanBuilder) addExactKey(key roachpb.Key) {
	cp := append(roachpb.Key(nil), key...)
	b.addSpan(roachpb.Span{Key: cp, EndKey: cp.Next()})
}

func (b *jsonPointLookupSpanBuilder) addPathPrefix(prefix roachpb.Key) {
	cp := append(roachpb.Key(nil), prefix...)
	b.addSpan(roachpb.Span{Key: cp, EndKey: cp.PrefixEnd()})
}

func (b *jsonPointLookupSpanBuilder) addRootHeader(colID descpb.ColumnID) {
	b.addExactKey(roachpb.Key(keys.MakeSubordinatePathKey(b.rowKey, uint32(colID),
		[]keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}})))
}

func subordinateJSONStoredPath(path []keys.SubordinatePathSegment) []keys.SubordinatePathSegment {
	stored := make([]keys.SubordinatePathSegment, 0, len(path)+1)
	stored = append(stored, keys.SubordinatePathSegment{Kind: keys.SubordinatePathHeader})
	stored = append(stored, path...)
	return stored
}

func (b *jsonPointLookupSpanBuilder) addExistsKeys(colID descpb.ColumnID, objectKeys []string) {
	b.addRootHeader(colID)
	for _, objectKey := range objectKeys {
		b.addExactKey(roachpb.Key(keys.MakeSubordinatePathKey(b.rowKey, uint32(colID), subordinateJSONStoredPath([]keys.SubordinatePathSegment{{
			Kind:      keys.SubordinatePathObjectKey,
			ObjectKey: objectKey,
		}}))))
	}
}

func (b *jsonPointLookupSpanBuilder) addSelectedPath(colID descpb.ColumnID, path []keys.SubordinatePathSegment) {
	storedPath := subordinateJSONStoredPath(path)
	b.addRootHeader(colID)
	for i := 2; i < len(storedPath); i++ {
		b.addExactKey(roachpb.Key(keys.MakeSubordinatePathKey(
			b.rowKey, uint32(colID), storedPath[:i],
		)))
	}
	b.addPathPrefix(roachpb.Key(keys.MakeSubordinatePathPrefix(
		b.rowKey, uint32(colID), storedPath,
	)))
}

func (b *jsonPointLookupSpanBuilder) finish() roachpb.Spans {
	sort.Slice(b.spans, func(i, j int) bool {
		if cmp := b.spans[i].Key.Compare(b.spans[j].Key); cmp != 0 {
			return cmp < 0
		}
		return b.spans[i].EndKey.Compare(b.spans[j].EndKey) < 0
	})
	return b.spans
}

func exactSingleRowSentinelKey(spans roachpb.Spans) (roachpb.Key, bool) {
	if len(spans) != 1 {
		var rowKey roachpb.Key
		for i := range spans {
			span := spans[i]
			if !span.EndKey.Equal(span.Key.Next()) {
				continue
			}
			prefixLen, err := keys.GetRowPrefixLength(span.Key)
			if err != nil || prefixLen != len(span.Key)-1 {
				continue
			}
			if rowKey != nil {
				if !rowKey.Equal(span.Key) {
					return nil, false
				}
				continue
			}
			rowKey = append(roachpb.Key(nil), span.Key...)
		}
		if rowKey == nil {
			return nil, false
		}
		return rowKey, true
	}
	span := spans[0]
	if !span.EndKey.Equal(span.Key.PrefixEnd()) {
		return nil, false
	}
	prefixLen, err := keys.GetRowPrefixLength(span.Key)
	if err != nil || prefixLen != len(span.Key)-1 {
		return nil, false
	}
	return append(roachpb.Key(nil), span.Key...), true
}

func addStaticJSONPathSpans(
	builder *jsonPointLookupSpanBuilder, colID descpb.ColumnID, path []string,
) (bool, error) {
	if len(path) == 0 {
		return false, nil
	}
	segments, ok, err := row.TryStaticSubordinateJSONPath(path)
	if err != nil || !ok {
		return ok, err
	}
	builder.addSelectedPath(colID, segments)
	return true, nil
}

func encodeJSONPathKeyStep(step string) string {
	return jsonPathStepKeyPrefix + strconv.Quote(step)
}

func encodeJSONPathIndexStep(idx int64) string {
	return jsonPathStepIndexPrefix + strconv.FormatInt(idx, 10)
}

func encodeJSONPathKeyOrIndexStep(step string) string {
	return jsonPathStepKeyOrIndexPrefix + strconv.Quote(step)
}

func stripTransparentTypedCasts(expr tree.TypedExpr) tree.TypedExpr {
	for {
		cast, ok := expr.(*tree.CastExpr)
		if !ok {
			return expr
		}
		child, ok := cast.Expr.(tree.TypedExpr)
		if !ok || child.ResolvedType().Family() != cast.ResolvedType().Family() {
			return expr
		}
		expr = child
	}
}

func evalConstDatum(evalCtx *eval.Context, expr tree.TypedExpr) (tree.Datum, bool, error) {
	expr = stripTransparentTypedCasts(expr)
	if tree.ContainsVars(expr) {
		return nil, false, nil
	}
	d, err := eval.Expr(context.Background(), evalCtx, expr)
	if err != nil {
		return nil, false, err
	}
	return d, true, nil
}

func extractJSONStringKeyFromTypedExpr(
	evalCtx *eval.Context, expr tree.TypedExpr,
) (string, bool, error) {
	d, ok, err := evalConstDatum(evalCtx, expr)
	if err != nil || !ok || d == tree.DNull {
		return "", ok, err
	}
	s, ok := d.(*tree.DString)
	if !ok {
		return "", false, nil
	}
	return string(*s), true, nil
}

func extractJSONStringArrayFromTypedExpr(
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
	keys := make([]string, arr.Len())
	for i := 0; i < arr.Len(); i++ {
		s, ok := arr.Array[i].(*tree.DString)
		if !ok {
			return nil, false, nil
		}
		keys[i] = string(*s)
	}
	return keys, true, nil
}

func extractJSONFetchOperandPathStepFromTypedExpr(
	evalCtx *eval.Context, expr tree.TypedExpr,
) (string, bool, error) {
	d, ok, err := evalConstDatum(evalCtx, expr)
	if err != nil || !ok || d == tree.DNull {
		return "", ok, err
	}
	switch t := d.(type) {
	case *tree.DString:
		return encodeJSONPathKeyOrIndexStep(string(*t)), true, nil
	case *tree.DInt:
		return encodeJSONPathIndexStep(int64(*t)), true, nil
	default:
		return "", false, nil
	}
}

func extractJSONPathFromTypedExpr(
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
	for i := 0; i < arr.Len(); i++ {
		s, ok := arr.Array[i].(*tree.DString)
		if !ok {
			return nil, false, nil
		}
		path[i] = encodeJSONPathKeyOrIndexStep(string(*s))
	}
	return path, true, nil
}

func extractJSONSourceAndPathFromTypedExpr(
	evalCtx *eval.Context, sourceCols []catalog.Column, expr tree.TypedExpr,
) (sourceColIdx int, path []string, ok bool, err error) {
	expr = stripTransparentTypedCasts(expr)
	switch t := expr.(type) {
	case *tree.IndexedVar:
		if t.Idx < 0 || t.Idx >= len(sourceCols) {
			return 0, nil, false, nil
		}
		if sourceCols[t.Idx].GetType().Family() != types.JsonFamily {
			return 0, nil, false, nil
		}
		return t.Idx, nil, true, nil

	case *tree.BinaryExpr:
		switch t.Operator.Symbol {
		case treebin.JSONFetchVal:
			sourceColIdx, path, ok, err = extractJSONSourceAndPathFromTypedExpr(evalCtx, sourceCols, t.TypedLeft())
			if err != nil || !ok {
				return 0, nil, ok, err
			}
			step, ok, err := extractJSONFetchOperandPathStepFromTypedExpr(evalCtx, t.TypedRight())
			if err != nil || !ok {
				return 0, nil, ok, err
			}
			return sourceColIdx, append(path, step), true, nil

		case treebin.JSONFetchValPath:
			sourceColIdx, path, ok, err = extractJSONSourceAndPathFromTypedExpr(evalCtx, sourceCols, t.TypedLeft())
			if err != nil || !ok {
				return 0, nil, ok, err
			}
			suffix, ok, err := extractJSONPathFromTypedExpr(evalCtx, t.TypedRight())
			if err != nil || !ok {
				return 0, nil, ok, err
			}
			return sourceColIdx, append(path, suffix...), true, nil
		}
	}
	return 0, nil, false, nil
}

func extractJSONAccessProgramFromTypedExpr(
	evalCtx *eval.Context,
	sourceCols []catalog.Column,
	planToStreamColMap []int,
	expr tree.TypedExpr,
) (*execinfrapb.JSONAccessSpec, bool, error) {
	expr = stripTransparentTypedCasts(expr)
	switch t := expr.(type) {
	case *tree.ComparisonExpr:
		sourceColIdx, pathPrefix, ok, err := extractJSONSourceAndPathFromTypedExpr(evalCtx, sourceCols, t.TypedLeft())
		if err != nil || !ok || len(pathPrefix) != 0 {
			return nil, ok && len(pathPrefix) == 0, err
		}
		streamIdx := planToStreamColMap[sourceColIdx]
		if streamIdx < 0 {
			return nil, false, nil
		}
		switch t.Operator.Symbol {
		case treecmp.JSONExists:
			key, ok, err := extractJSONStringKeyFromTypedExpr(evalCtx, t.TypedRight())
			if err != nil || !ok {
				return nil, ok, err
			}
			return &execinfrapb.JSONAccessSpec{
				SourceColIdx: uint32(streamIdx),
				Kind:         uint32(exec.JSONAccessExists),
				Key:          key,
			}, true, nil
		case treecmp.JSONSomeExists:
			keys, ok, err := extractJSONStringArrayFromTypedExpr(evalCtx, t.TypedRight())
			if err != nil || !ok {
				return nil, ok, err
			}
			return &execinfrapb.JSONAccessSpec{
				SourceColIdx: uint32(streamIdx),
				Kind:         uint32(exec.JSONAccessExistsAny),
				Keys:         keys,
			}, true, nil
		case treecmp.JSONAllExists:
			keys, ok, err := extractJSONStringArrayFromTypedExpr(evalCtx, t.TypedRight())
			if err != nil || !ok {
				return nil, ok, err
			}
			return &execinfrapb.JSONAccessSpec{
				SourceColIdx: uint32(streamIdx),
				Kind:         uint32(exec.JSONAccessExistsAll),
				Keys:         keys,
			}, true, nil
		default:
			return nil, false, nil
		}

	case *tree.BinaryExpr:
		sourceColIdx, pathPrefix, ok, err := extractJSONSourceAndPathFromTypedExpr(evalCtx, sourceCols, t.TypedLeft())
		if err != nil || !ok {
			return nil, ok, err
		}
		streamIdx := planToStreamColMap[sourceColIdx]
		if streamIdx < 0 {
			return nil, false, nil
		}

		switch t.Operator.Symbol {
		case treebin.JSONFetchVal, treebin.JSONFetchText:
			step, ok, err := extractJSONFetchOperandPathStepFromTypedExpr(evalCtx, t.TypedRight())
			if err != nil || !ok {
				return nil, ok, err
			}
			kind := exec.JSONAccessFetchJSONPath
			if t.Operator.Symbol == treebin.JSONFetchText {
				kind = exec.JSONAccessFetchTextPath
			}
			return &execinfrapb.JSONAccessSpec{
				SourceColIdx: uint32(streamIdx),
				Kind:         uint32(kind),
				Path:         append(pathPrefix, step),
			}, true, nil

		case treebin.JSONFetchValPath, treebin.JSONFetchTextPath:
			suffix, ok, err := extractJSONPathFromTypedExpr(evalCtx, t.TypedRight())
			if err != nil || !ok {
				return nil, ok, err
			}
			kind := exec.JSONAccessFetchJSONPath
			if t.Operator.Symbol == treebin.JSONFetchTextPath {
				kind = exec.JSONAccessFetchTextPath
			}
			return &execinfrapb.JSONAccessSpec{
				SourceColIdx: uint32(streamIdx),
				Kind:         uint32(kind),
				Path:         append(pathPrefix, suffix...),
			}, true, nil

		default:
			return nil, false, nil
		}
	}
	return nil, false, nil
}

func extractJSONPathCompareFilterFromTypedExpr(
	evalCtx *eval.Context,
	exprCtx physicalplan.ExprContext,
	sourceCols []catalog.Column,
	planToStreamColMap []int,
	expr tree.TypedExpr,
) (*execinfrapb.JSONPathCompareFilterSpec, bool, error) {
	cmp, ok := stripTransparentTypedCasts(expr).(*tree.ComparisonExpr)
	if !ok {
		return nil, false, nil
	}

	type candidate struct {
		left  tree.TypedExpr
		right tree.TypedExpr
		mode  exec.JSONPathFilterMode
	}
	var candidates []candidate
	switch cmp.Operator.Symbol {
	case treecmp.EQ:
		candidates = append(candidates, candidate{cmp.TypedLeft(), cmp.TypedRight(), exec.JSONPathFilterEq})
		candidates = append(candidates, candidate{cmp.TypedRight(), cmp.TypedLeft(), exec.JSONPathFilterEq})
	case treecmp.NE:
		candidates = append(candidates, candidate{cmp.TypedLeft(), cmp.TypedRight(), exec.JSONPathFilterNe})
		candidates = append(candidates, candidate{cmp.TypedRight(), cmp.TypedLeft(), exec.JSONPathFilterNe})
	case treecmp.LT:
		candidates = append(candidates, candidate{cmp.TypedLeft(), cmp.TypedRight(), exec.JSONPathFilterLt})
		candidates = append(candidates, candidate{cmp.TypedRight(), cmp.TypedLeft(), exec.JSONPathFilterGt})
	case treecmp.LE:
		candidates = append(candidates, candidate{cmp.TypedLeft(), cmp.TypedRight(), exec.JSONPathFilterLe})
		candidates = append(candidates, candidate{cmp.TypedRight(), cmp.TypedLeft(), exec.JSONPathFilterGe})
	case treecmp.GT:
		candidates = append(candidates, candidate{cmp.TypedLeft(), cmp.TypedRight(), exec.JSONPathFilterGt})
		candidates = append(candidates, candidate{cmp.TypedRight(), cmp.TypedLeft(), exec.JSONPathFilterLt})
	case treecmp.GE:
		candidates = append(candidates, candidate{cmp.TypedLeft(), cmp.TypedRight(), exec.JSONPathFilterGe})
		candidates = append(candidates, candidate{cmp.TypedRight(), cmp.TypedLeft(), exec.JSONPathFilterLe})
	default:
		return nil, false, nil
	}

	for _, c := range candidates {
		access, ok, err := extractJSONAccessProgramFromTypedExpr(evalCtx, sourceCols, planToStreamColMap, c.left)
		if err != nil || !ok {
			continue
		}
		if access.Kind != uint32(exec.JSONAccessFetchJSONPath) && access.Kind != uint32(exec.JSONAccessFetchTextPath) {
			continue
		}
		if tree.ContainsVars(c.right) {
			continue
		}
		rightExpr, err := physicalplan.MakeExpression(context.Background(), c.right, exprCtx, nil /* indexVarMap */)
		if err != nil {
			return nil, false, err
		}
		return &execinfrapb.JSONPathCompareFilterSpec{
			SourceColIdx: access.SourceColIdx,
			Kind:         access.Kind,
			Path:         append([]string(nil), access.Path...),
			Mode:         uint32(c.mode),
			Right:        rightExpr,
		}, true, nil
	}
	return nil, false, nil
}

func extractJSONExistsFilterFromTypedExpr(
	evalCtx *eval.Context,
	sourceCols []catalog.Column,
	planToStreamColMap []int,
	expr tree.TypedExpr,
) (*execinfrapb.JSONExistsFilterSpec, bool, error) {
	access, ok, err := extractJSONAccessProgramFromTypedExpr(evalCtx, sourceCols, planToStreamColMap, expr)
	if err != nil || !ok {
		return nil, ok, err
	}
	switch exec.JSONAccessKind(access.Kind) {
	case exec.JSONAccessExists, exec.JSONAccessExistsAny, exec.JSONAccessExistsAll:
	default:
		return nil, false, nil
	}
	return &execinfrapb.JSONExistsFilterSpec{
		SourceColIdx: access.SourceColIdx,
		Kind:         access.Kind,
		Key:          access.Key,
		Keys:         append([]string(nil), access.Keys...),
	}, true, nil
}

func fetchedColumnProjected(post execinfrapb.PostProcessSpec, fetchedCols int, idx int) bool {
	if !post.Projection {
		return idx < fetchedCols
	}
	for _, col := range post.OutputColumns {
		if int(col) == idx {
			return true
		}
	}
	return false
}

func maybeOptimizeExactPointLookupJSONTableReaderSpans(
	spans roachpb.Spans,
	sourceCols []catalog.Column,
	post execinfrapb.PostProcessSpec,
	jsonExistsFilter *execinfrapb.JSONExistsFilterSpec,
	jsonAccesses []execinfrapb.JSONAccessSpec,
	jsonPathCompareFilter *execinfrapb.JSONPathCompareFilterSpec,
) (roachpb.Spans, bool, error) {
	rowKey, ok := exactSingleRowSentinelKey(spans)
	if !ok {
		return nil, false, nil
	}
	builder := newJSONPointLookupSpanBuilder(rowKey)
	targeted := false

	if jsonExistsFilter != nil {
		if fetchedColumnProjected(post, len(sourceCols), int(jsonExistsFilter.SourceColIdx)) {
			return nil, false, nil
		}
		colID := sourceCols[int(jsonExistsFilter.SourceColIdx)].GetID()
		switch exec.JSONAccessKind(jsonExistsFilter.Kind) {
		case exec.JSONAccessExists:
			builder.addExistsKeys(colID, []string{jsonExistsFilter.Key})
		case exec.JSONAccessExistsAny, exec.JSONAccessExistsAll:
			builder.addExistsKeys(colID, jsonExistsFilter.Keys)
		default:
			return nil, false, nil
		}
		targeted = true
	}

	for i := range jsonAccesses {
		access := jsonAccesses[i]
		if fetchedColumnProjected(post, len(sourceCols), int(access.SourceColIdx)) {
			return nil, false, nil
		}
		colID := sourceCols[int(access.SourceColIdx)].GetID()
		switch exec.JSONAccessKind(access.Kind) {
		case exec.JSONAccessExists:
			builder.addExistsKeys(colID, []string{access.Key})
		case exec.JSONAccessExistsAny, exec.JSONAccessExistsAll:
			builder.addExistsKeys(colID, access.Keys)
		case exec.JSONAccessFetchJSONPath, exec.JSONAccessFetchTextPath:
			ok, err := addStaticJSONPathSpans(builder, colID, access.Path)
			if err != nil || !ok {
				return nil, false, err
			}
		default:
			return nil, false, nil
		}
		targeted = true
	}

	if jsonPathCompareFilter != nil {
		if fetchedColumnProjected(post, len(sourceCols), int(jsonPathCompareFilter.SourceColIdx)) {
			return nil, false, nil
		}
		colID := sourceCols[int(jsonPathCompareFilter.SourceColIdx)].GetID()
		ok, err := addStaticJSONPathSpans(builder, colID, jsonPathCompareFilter.Path)
		if err != nil || !ok {
			return nil, false, err
		}
		targeted = true
	}

	if !targeted {
		return nil, false, nil
	}
	return builder.finish(), true, nil
}

func explicitIdentityPost(post execinfrapb.PostProcessSpec, numResultCols int) execinfrapb.PostProcessSpec {
	if post.Projection || len(post.RenderExprs) > 0 {
		return post
	}
	post.Projection = true
	post.OutputColumns = make([]uint32, numResultCols)
	for i := 0; i < numResultCols; i++ {
		post.OutputColumns[i] = uint32(i)
	}
	return post
}

func flattenTypedAndExpr(expr tree.TypedExpr) []tree.TypedExpr {
	if and, ok := stripTransparentTypedCasts(expr).(*tree.AndExpr); ok {
		left := flattenTypedAndExpr(and.TypedLeft())
		right := flattenTypedAndExpr(and.TypedRight())
		out := make([]tree.TypedExpr, 0, len(left)+len(right))
		out = append(out, left...)
		out = append(out, right...)
		return out
	}
	return []tree.TypedExpr{expr}
}

func joinTypedAndExpr(exprs []tree.TypedExpr) tree.TypedExpr {
	switch len(exprs) {
	case 0:
		return nil
	case 1:
		return exprs[0]
	}
	out := exprs[0]
	for _, expr := range exprs[1:] {
		out = tree.NewTypedAndExpr(out, expr)
	}
	return out
}

func (dsp *DistSQLPlanner) tryPushJSONRenderIntoTableReaders(
	p *PhysicalPlan, scan *scanNode, n *renderNode, planCtx *PlanningCtx,
) (bool, error) {
	post := p.GetLastStagePost()
	if len(post.RenderExprs) > 0 {
		return false, nil
	}

	jsonAccesses := make([]execinfrapb.JSONAccessSpec, 0, len(n.render))
	outputCols := make([]uint32, len(n.render))
	nextDerived := len(p.GetResultTypes())
	for i, expr := range n.render {
		if iv, ok := stripTransparentTypedCasts(expr).(*tree.IndexedVar); ok {
			streamIdx := p.PlanToStreamColMap[iv.Idx]
			if streamIdx < 0 {
				return false, nil
			}
			outputCols[i] = uint32(streamIdx)
			continue
		}
		spec, ok, err := extractJSONAccessProgramFromTypedExpr(
			planCtx.EvalContext(), scan.cols, p.PlanToStreamColMap, expr,
		)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
		jsonAccesses = append(jsonAccesses, *spec)
		outputCols[i] = uint32(nextDerived)
		nextDerived++
	}
	finalPost := post
	finalPost.Projection = true
	finalPost.OutputColumns = outputCols
	finalPost.RenderExprs = nil

	reoptimizeExistingJSON := false
	for _, pIdx := range p.ResultRouters {
		proc := &p.Processors[pIdx]
		tr := proc.Spec.Core.TableReader
		if tr == nil {
			return false, nil
		}
		if tr.JsonPathCompareFilter != nil || len(tr.JsonAccesses) > 0 {
			reoptimizeExistingJSON = true
		}
		if len(jsonAccesses) > 0 {
			tr.JsonAccesses = append(tr.JsonAccesses, jsonAccesses...)
		}
		if optimized, ok, err := maybeOptimizeExactPointLookupJSONTableReaderSpans(
			tr.Spans, scan.cols, finalPost, tr.JsonExistsFilter, tr.JsonAccesses, tr.JsonPathCompareFilter,
		); err != nil {
			return false, err
		} else if ok {
			tr.Spans = optimized
		}
	}
	if len(jsonAccesses) == 0 && !reoptimizeExistingJSON {
		return false, nil
	}

	typs, err := getTypesForPlanResult(n, nil /* planToStreamColMap */)
	if err != nil {
		return false, err
	}
	newColMap := identityMap(p.PlanToStreamColMap, len(n.render))
	p.SetMergeOrdering(dsp.convertOrdering(n.reqOrdering, newColMap))
	p.SetLastStagePost(finalPost, typs)
	p.PlanToStreamColMap = newColMap
	return true, nil
}

func (dsp *DistSQLPlanner) tryPushJSONFilterIntoTableReaders(
	p *PhysicalPlan, scan *scanNode, n *filterNode, planCtx *PlanningCtx,
) (tree.TypedExpr, bool, error) {
	post := p.GetLastStagePost()
	if len(post.RenderExprs) > 0 {
		return nil, false, nil
	}
	finalPost := explicitIdentityPost(post, len(p.GetResultTypes()))
	postAdjusted := !post.Projection && len(post.RenderExprs) == 0

	conjuncts := flattenTypedAndExpr(n.filter)
	var (
		existsSpec   *execinfrapb.JSONExistsFilterSpec
		pathSpec     *execinfrapb.JSONPathCompareFilterSpec
		residual     []tree.TypedExpr
		pushedFilter bool
	)
	for _, conjunct := range conjuncts {
		if existsSpec == nil {
			candidate, ok, err := extractJSONExistsFilterFromTypedExpr(
				planCtx.EvalContext(), scan.cols, p.PlanToStreamColMap, conjunct,
			)
			if err != nil {
				return nil, false, err
			}
			if ok {
				existsSpec = candidate
				pushedFilter = true
				continue
			}
		}
		if pathSpec == nil {
			candidate, ok, err := extractJSONPathCompareFilterFromTypedExpr(
				planCtx.EvalContext(), planCtx, scan.cols, p.PlanToStreamColMap, conjunct,
			)
			if err != nil {
				return nil, false, err
			}
			if ok {
				pathSpec = candidate
				pushedFilter = true
				continue
			}
		}
		residual = append(residual, conjunct)
	}
	if !pushedFilter {
		return nil, false, nil
	}
	for _, pIdx := range p.ResultRouters {
		proc := &p.Processors[pIdx]
		tr := proc.Spec.Core.TableReader
		if tr == nil {
			return nil, false, nil
		}
		if existsSpec != nil {
			if tr.JsonExistsFilter != nil {
				return nil, false, nil
			}
			tr.JsonExistsFilter = existsSpec
		}
		if pathSpec != nil {
			if tr.JsonPathCompareFilter != nil {
				return nil, false, nil
			}
			tr.JsonPathCompareFilter = pathSpec
		}
		if optimized, ok, err := maybeOptimizeExactPointLookupJSONTableReaderSpans(
			tr.Spans, scan.cols, finalPost, tr.JsonExistsFilter, tr.JsonAccesses, tr.JsonPathCompareFilter,
		); err != nil {
			return nil, false, err
		} else if ok {
			tr.Spans = optimized
		}
	}
	if postAdjusted {
		p.SetLastStagePost(finalPost, p.GetResultTypes())
	}
	return joinTypedAndExpr(residual), true, nil
}
