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

package udfruntime

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	v8 "github.com/tommie/v8go"
)

// asyncSQLResult is sent from a goroutine doing SQL work back to the V8 thread.
type asyncSQLResult struct {
	resolver *v8.PromiseResolver
	rows     []tree.Datums
	cols     []ResultColumn
	err      error
}

// makeAsyncSQLTemplate creates a FunctionTemplate for sql“ that returns
// Promises. SQL execution happens in goroutines; the V8 thread pumps
// results between microtask checkpoints.
func (r *Registry) makeAsyncSQLTemplate() *v8.FunctionTemplate {
	return v8.NewFunctionTemplate(r.iso, func(info *v8.FunctionCallbackInfo) *v8.Value {
		args := info.Args()
		ctx := info.Context()

		if len(args) == 0 {
			r.callState.err = fmt.Errorf("sql`...` requires template strings argument")
			return nil
		}

		// Reconstruct parameterized SQL (same as sync version).
		stringsObj, err := args[0].AsObject()
		if err != nil {
			r.callState.err = fmt.Errorf("sql`...`: %w", err)
			return nil
		}
		lengthVal, err := stringsObj.Get("length")
		if err != nil {
			r.callState.err = fmt.Errorf("sql`...`: %w", err)
			return nil
		}
		numStrings := int(lengthVal.Int32())

		var sqlBuf strings.Builder
		qargs := make([]interface{}, 0, len(args)-1)
		for i := 0; i < numStrings; i++ {
			part, err := stringsObj.GetIdx(uint32(i))
			if err != nil {
				r.callState.err = fmt.Errorf("sql`...`: %w", err)
				return nil
			}
			sqlBuf.WriteString(part.String())
			if i < numStrings-1 && i+1 < len(args) {
				sqlBuf.WriteString(fmt.Sprintf("$%d", i+1))
				qargs = append(qargs, v8ValueToGo(args[i+1]))
			}
		}
		sqlStr := sqlBuf.String()

		// Create a Promise.
		resolver, err := v8.NewPromiseResolver(ctx)
		if err != nil {
			r.callState.err = fmt.Errorf("sql`...`: creating promise: %w", err)
			return nil
		}

		// Capture state for the goroutine.
		state := r.callState
		resultCh := state.results

		// Launch SQL execution in a goroutine.
		state.pending.Add(1)
		go func() {
			rows, cols, err := state.executor.QueryBufferedEx(
				state.ctx, "udf-async-sql", state.txn, state.override,
				sqlStr, qargs...,
			)
			resultCh <- asyncSQLResult{
				resolver: resolver,
				rows:     rows,
				cols:     cols,
				err:      err,
			}
		}()

		return resolver.GetPromise().Value
	})
}

// asyncCallState holds state for an async UDF invocation.
type asyncCallState struct {
	executor SQLExecutor
	ctx      context.Context
	txn      interface{}
	override interface{}
	results  chan asyncSQLResult
	pending  atomic.Int32
	err      error
}

// drainAsyncResults processes completed SQL results, resolving their
// Promises on the V8 thread. Call this between microtask checkpoints.
func (r *Registry) drainAsyncResults(v8ctx *v8.Context) error {
	for {
		select {
		case res := <-r.callState.results:
			r.callState.pending.Add(-1)
			if res.err != nil {
				errVal, _ := v8.NewValue(r.iso, res.err.Error())
				res.resolver.Reject(errVal)
			} else {
				jsArr, err := rowsToJSArray(v8ctx, res.rows, res.cols)
				if err != nil {
					errVal, _ := v8.NewValue(r.iso, err.Error())
					res.resolver.Reject(errVal)
				} else {
					res.resolver.Resolve(jsArr)
				}
			}
			v8ctx.PerformMicrotaskCheckpoint()
		default:
			return nil
		}
	}
}

// waitAsyncResults blocks until all pending SQL goroutines complete,
// draining results and resolving Promises as they arrive.
func (r *Registry) waitAsyncResults(v8ctx *v8.Context) error {
	for r.callState.pending.Load() > 0 {
		res := <-r.callState.results
		r.callState.pending.Add(-1)
		if res.err != nil {
			errVal, _ := v8.NewValue(r.iso, res.err.Error())
			res.resolver.Reject(errVal)
		} else {
			jsArr, err := rowsToJSArray(v8ctx, res.rows, res.cols)
			if err != nil {
				errVal, _ := v8.NewValue(r.iso, err.Error())
				res.resolver.Reject(errVal)
			} else {
				res.resolver.Resolve(jsArr)
			}
		}
		v8ctx.PerformMicrotaskCheckpoint()
	}
	return nil
}
