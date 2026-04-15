// Copyright 2019 The Cockroach Authors.
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

	"github.com/cockroachdb/errors"
	"github.com/semistrict/ratel/pkg/col/coldata"
	"github.com/semistrict/ratel/pkg/sql/colconv"
	"github.com/semistrict/ratel/pkg/sql/colexec/colexecutils"
	"github.com/semistrict/ratel/pkg/sql/colexecerror"
	"github.com/semistrict/ratel/pkg/sql/colexecop"
	"github.com/semistrict/ratel/pkg/sql/colmem"
	"github.com/semistrict/ratel/pkg/sql/execinfra"
	"github.com/semistrict/ratel/pkg/sql/sem/tree"
	"github.com/semistrict/ratel/pkg/sql/types"
	"github.com/semistrict/ratel/pkg/sql/udfruntime"
)

type defaultBuiltinFuncOperator struct {
	colexecop.OneInputHelper
	allocator           *colmem.Allocator
	evalCtx             *tree.EvalContext
	funcExpr            *tree.FuncExpr
	columnTypes         []*types.T
	argumentCols        []int
	outputIdx           int
	outputType          *types.T
	toDatumConverter    *colconv.VecToDatumConverter
	datumToVecConverter func(tree.Datum) interface{}

	row tree.Datums
}

var _ colexecop.Operator = &defaultBuiltinFuncOperator{}
var _ execinfra.Releasable = &defaultBuiltinFuncOperator{}

func (b *defaultBuiltinFuncOperator) Next() coldata.Batch {
	batch := b.Input.Next()
	n := batch.Length()
	if n == 0 {
		return coldata.ZeroBatch
	}

	sel := batch.Selection()
	output := batch.ColVec(b.outputIdx)
	if output.MaybeHasNulls() {
		// We need to make sure that there are no left over null values in the
		// output vector.
		output.Nulls().UnsetNulls()
	}
	b.allocator.PerformOperation(
		[]coldata.Vec{output},
		func() {
			b.toDatumConverter.ConvertBatchAndDeselect(batch)
			for i := 0; i < n; i++ {
				hasNulls := false

				for j, argumentCol := range b.argumentCols {
					// Note that we don't need to apply sel to index i because
					// vecToDatumConverter returns a "dense" datum column.
					b.row[j] = b.toDatumConverter.GetDatumColumn(argumentCol)[i]
					hasNulls = hasNulls || b.row[j] == tree.DNull
				}

				var (
					res tree.Datum
					err error
				)
				// Some functions cannot handle null arguments.
				if hasNulls && !b.funcExpr.CanHandleNulls() {
					res = tree.DNull
				} else {
					res, err = b.funcExpr.ResolvedOverload().Fn(b.evalCtx, b.row)
					if err != nil {
						colexecerror.ExpectedError(b.funcExpr.MaybeWrapError(err))
					}
				}

				rowIdx := i
				if sel != nil {
					rowIdx = sel[i]
				}

				// Convert the datum into a physical type and write it out.
				if res == tree.DNull {
					output.Nulls().SetNull(rowIdx)
				} else {
					converted := b.datumToVecConverter(res)
					coldata.SetValueAt(output, converted, rowIdx)
				}
			}
		},
	)
	return batch
}

// Release is part of the execinfra.Releasable interface.
func (b *defaultBuiltinFuncOperator) Release() {
	b.toDatumConverter.Release()
}

// udfBatchOperator calls a V8-backed UDF with an entire batch at once,
// avoiding per-row V8 context creation overhead.
type udfBatchOperator struct {
	colexecop.OneInputHelper
	allocator           *colmem.Allocator
	registry            *udfruntime.Registry
	funcName            string
	columnTypes         []*types.T
	argumentCols        []int
	outputIdx           int
	outputType          *types.T
	toDatumConverter    *colconv.VecToDatumConverter
	datumToVecConverter func(tree.Datum) interface{}
	txnCtx              *udfruntime.TxnContext
}

var _ colexecop.Operator = &udfBatchOperator{}
var _ execinfra.Releasable = &udfBatchOperator{}

func (u *udfBatchOperator) Init(ctx context.Context) {
	u.OneInputHelper.Init(ctx)
	// Create a TxnContext that lives for the lifetime of this operator.
	u.txnCtx = u.registry.NewTxnContext(nil, ctx, nil, nil)
}

func (u *udfBatchOperator) Next() coldata.Batch {
	batch := u.Input.Next()
	n := batch.Length()
	if n == 0 {
		return coldata.ZeroBatch
	}

	sel := batch.Selection()
	output := batch.ColVec(u.outputIdx)
	if output.MaybeHasNulls() {
		output.Nulls().UnsetNulls()
	}

	u.allocator.PerformOperation(
		[]coldata.Vec{output},
		func() {
			u.toDatumConverter.ConvertBatchAndDeselect(batch)

			// Build a batch of Datums for the UDF call.
			argsBatch := make([]tree.Datums, n)
			for i := 0; i < n; i++ {
				row := make(tree.Datums, len(u.argumentCols))
				for j, argumentCol := range u.argumentCols {
					row[j] = u.toDatumConverter.GetDatumColumn(argumentCol)[i]
				}
				argsBatch[i] = row
			}

			// Call the UDF with the entire batch.
			results, err := u.registry.Call(u.txnCtx, u.funcName, argsBatch)
			if err != nil {
				colexecerror.ExpectedError(err)
			}

			// Write results to the output vector.
			for i := 0; i < n; i++ {
				rowIdx := i
				if sel != nil {
					rowIdx = sel[i]
				}
				if results[i] == tree.DNull {
					output.Nulls().SetNull(rowIdx)
				} else {
					converted := u.datumToVecConverter(results[i])
					coldata.SetValueAt(output, converted, rowIdx)
				}
			}
		},
	)
	return batch
}

// Release is part of the execinfra.Releasable interface.
func (u *udfBatchOperator) Release() {
	u.toDatumConverter.Release()
	if u.txnCtx != nil {
		u.txnCtx.Close()
		u.txnCtx = nil
	}
}

// NewBuiltinFunctionOperator returns an operator that applies builtin functions.
func NewBuiltinFunctionOperator(
	allocator *colmem.Allocator,
	evalCtx *tree.EvalContext,
	funcExpr *tree.FuncExpr,
	columnTypes []*types.T,
	argumentCols []int,
	outputIdx int,
	input colexecop.Operator,
) (colexecop.Operator, error) {
	overload := funcExpr.ResolvedOverload()
	if overload.FnWithExprs != nil {
		return nil, errors.New("builtins with FnWithExprs are not supported in the vectorized engine")
	}

	// Check if this is a UDF backed by the V8 registry. If so, use the
	// batched operator which calls all rows in a single V8 invocation.
	if reg, ok := evalCtx.UDFRegistry.(*udfruntime.Registry); ok && reg != nil {
		funcName := funcExpr.Func.String()
		if _, _, exists := reg.GetSignature(funcName); exists {
			outputType := funcExpr.ResolvedType()
			input = colexecutils.NewVectorTypeEnforcer(allocator, input, outputType, outputIdx)
			return &udfBatchOperator{
				OneInputHelper:      colexecop.MakeOneInputHelper(input),
				allocator:           allocator,
				registry:            reg,
				funcName:            funcName,
				columnTypes:         columnTypes,
				argumentCols:        argumentCols,
				outputIdx:           outputIdx,
				outputType:          outputType,
				toDatumConverter:    colconv.NewVecToDatumConverter(len(columnTypes), argumentCols, true /* willRelease */),
				datumToVecConverter: colconv.GetDatumToPhysicalFn(outputType),
			}, nil
		}
	}

	switch overload.SpecializedVecBuiltin {
	case tree.SubstringStringIntInt:
		input = colexecutils.NewVectorTypeEnforcer(allocator, input, types.String, outputIdx)
		return newSubstringOperator(
			allocator, columnTypes, argumentCols, outputIdx, input,
		), nil
	default:
		outputType := funcExpr.ResolvedType()
		input = colexecutils.NewVectorTypeEnforcer(allocator, input, outputType, outputIdx)
		return &defaultBuiltinFuncOperator{
			OneInputHelper:      colexecop.MakeOneInputHelper(input),
			allocator:           allocator,
			evalCtx:             evalCtx,
			funcExpr:            funcExpr,
			outputIdx:           outputIdx,
			columnTypes:         columnTypes,
			outputType:          outputType,
			toDatumConverter:    colconv.NewVecToDatumConverter(len(columnTypes), argumentCols, true /* willRelease */),
			datumToVecConverter: colconv.GetDatumToPhysicalFn(outputType),
			row:                 make(tree.Datums, len(argumentCols)),
			argumentCols:        argumentCols,
		}, nil
	}
}
