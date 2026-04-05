// Copyright 2024 Oxide Computer Company
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

package colexec

import (
	"context"
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/sql/colconv"
	"github.com/cockroachdb/cockroach/pkg/sql/colexecop"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/sql/udfruntime"
	"github.com/stretchr/testify/require"
)

// BenchmarkUDFVectorized benchmarks UDF execution through the vectorized
// engine's defaultBuiltinFuncOperator, which is the real code path for
// SELECT udf(col) FROM table.
//
// This measures the per-row overhead of: vectorized batch → datum conversion
// → Overload.Fn call (V8) → datum back to columnar.
func BenchmarkUDFVectorized(b *testing.B) {
	ctx := context.Background()
	st := cluster.MakeTestingClusterSettings()
	evalCtx := tree.MakeTestingEvalContext(st)
	defer evalCtx.Stop(ctx)

	// Register a UDF as if CREATE FUNCTION had been called.
	reg := udfruntime.NewRegistry()
	defer reg.Close()

	err := reg.CompileAndRegisterJS("bench_double",
		"function invoke(x) { return x * 2; }",
		[]udfruntime.ValType{udfruntime.ValI64}, udfruntime.ValI64, 0)
	require.NoError(b, err)

	fn, err := reg.MakeFn("bench_double")
	require.NoError(b, err)

	overload := tree.Overload{
		Types:      tree.ArgTypes{{Name: "x", Typ: types.Int}},
		ReturnType: tree.FixedReturnType(types.Int),
		Fn:         fn,
		Volatility: tree.VolatilityImmutable,
		Info:       "bench UDF",
	}

	funcDef := tree.NewFunctionDefinition(
		"bench_double",
		&tree.FunctionProperties{Category: "UDF"},
		[]tree.Overload{overload},
	)
	tree.RegisterFunction("bench_double", funcDef)
	defer tree.UnregisterFunction("bench_double")

	// Build a FuncExpr that resolves to our UDF.
	funcExpr := &tree.FuncExpr{
		Func:  tree.ResolvableFunctionReference{FunctionReference: funcDef},
		Type:  0, // NormalClass
		Exprs: tree.Exprs{&tree.IndexedVar{Idx: 0}},
	}
	semaCtx := tree.MakeSemaContext()
	semaCtx.IVarContainer = &MockTypeContext{Typs: []*types.T{types.Int}}
	typedExpr, err := funcExpr.TypeCheck(ctx, &semaCtx, types.Int)
	require.NoError(b, err)
	resolvedFuncExpr := typedExpr.(*tree.FuncExpr)

	for _, batchSize := range []int{1, 64, 1024} {
		b.Run(fmt.Sprintf("batch=%d", batchSize), func(b *testing.B) {
			inputTyps := []*types.T{types.Int}
			allTyps := []*types.T{types.Int, types.Int} // input + output
			outputIdx := 1

			batch := testAllocator.NewMemBatchWithMaxCapacity(allTyps)
			col := batch.ColVec(0).Int64()
			for i := 0; i < batchSize; i++ {
				col[i] = int64(i)
			}
			batch.SetLength(batchSize)

			source := colexecop.NewRepeatableBatchSource(testAllocator, batch, allTyps)

			op := &defaultBuiltinFuncOperator{
				OneInputHelper:      colexecop.MakeOneInputHelper(source),
				allocator:           testAllocator,
				evalCtx:             &evalCtx,
				funcExpr:            resolvedFuncExpr,
				outputIdx:           outputIdx,
				columnTypes:         inputTyps,
				outputType:          types.Int,
				toDatumConverter:    colconv.NewVecToDatumConverter(len(inputTyps), []int{0}, false),
				datumToVecConverter: colconv.GetDatumToPhysicalFn(types.Int),
				row:                 make(tree.Datums, 1),
				argumentCols:        []int{0},
			}
			op.Init(ctx)

			b.SetBytes(int64(8 * batchSize))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				op.Next()
			}
		})
	}
}

// BenchmarkUDFVectorizedBatched benchmarks the new udfBatchOperator which
// calls the V8 registry with the entire batch at once.
func BenchmarkUDFVectorizedBatched(b *testing.B) {
	ctx := context.Background()

	reg := udfruntime.NewRegistry()
	defer reg.Close()

	err := reg.CompileAndRegisterJS("bench_double",
		"function invoke(x) { return x * 2; }",
		[]udfruntime.ValType{udfruntime.ValI64}, udfruntime.ValI64, 0)
	require.NoError(b, err)

	for _, batchSize := range []int{1, 64, 1024} {
		b.Run(fmt.Sprintf("batch=%d", batchSize), func(b *testing.B) {
			inputTyps := []*types.T{types.Int}
			allTyps := []*types.T{types.Int, types.Int}
			outputIdx := 1

			batch := testAllocator.NewMemBatchWithMaxCapacity(allTyps)
			col := batch.ColVec(0).Int64()
			for i := 0; i < batchSize; i++ {
				col[i] = int64(i)
			}
			batch.SetLength(batchSize)

			source := colexecop.NewRepeatableBatchSource(testAllocator, batch, allTyps)

			op := &udfBatchOperator{
				OneInputHelper:      colexecop.MakeOneInputHelper(source),
				allocator:           testAllocator,
				registry:            reg,
				funcName:            "bench_double",
				columnTypes:         inputTyps,
				argumentCols:        []int{0},
				outputIdx:           outputIdx,
				outputType:          types.Int,
				toDatumConverter:    colconv.NewVecToDatumConverter(len(inputTyps), []int{0}, false),
				datumToVecConverter: colconv.GetDatumToPhysicalFn(types.Int),
			}
			op.Init(ctx)

			b.SetBytes(int64(8 * batchSize))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				op.Next()
			}
		})
	}
}

// MockTypeContext for the benchmark (reuse from colexectestutils).
type MockTypeContext struct {
	Typs []*types.T
}

func (p *MockTypeContext) IndexedVarEval(idx int, ctx *tree.EvalContext) (tree.Datum, error) {
	return tree.DNull.Eval(ctx)
}
func (p *MockTypeContext) IndexedVarResolvedType(idx int) *types.T {
	return p.Typs[idx]
}
func (p *MockTypeContext) IndexedVarNodeFormatter(idx int) tree.NodeFormatter {
	n := tree.Name(fmt.Sprintf("$%d", idx))
	return &n
}
