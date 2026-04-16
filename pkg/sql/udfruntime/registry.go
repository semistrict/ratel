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
	"sync"
	"time"

	"github.com/semistrict/ratel/pkg/sql/sem/tree"
	crdbJSON "github.com/semistrict/ratel/pkg/util/json"
	v8 "github.com/tommie/v8go"
)

// DefaultTimeout is the default execution timeout for a single UDF call.
const DefaultTimeout = 100 * time.Millisecond

// MaxModuleSize is the maximum size of a WASM binary or JS source body.
const MaxModuleSize = 10 << 20 // 10MB

// Language identifies the UDF source language.
type Language string

const (
	LangJavaScript Language = "javascript"
)

// Registry manages compiled UDF functions backed by V8.
type Registry struct {
	iso         *v8.Isolate
	mu          sync.RWMutex
	funcs       map[string]*compiledFunc
	execMu      sync.Mutex           // serializes V8 execution (isolates are single-threaded)
	sqlTemplate *v8.FunctionTemplate // async sql`` tagged template (returns Promise)
	callState   asyncCallState       // per-call state, safe because execMu is held
}

type compiledFunc struct {
	jsSetup    string // evaluated once per context to define the function
	paramTypes []ValType
	resultType ValType
	timeout    time.Duration
}

// NewRegistry creates a new V8-backed UDF registry.
func NewRegistry() *Registry {
	r := &Registry{
		iso:   v8.NewIsolate(),
		funcs: make(map[string]*compiledFunc),
	}
	r.sqlTemplate = r.makeAsyncSQLTemplate()
	return r
}

// CompileAndRegisterJS registers a JavaScript function.
func (r *Registry) CompileAndRegisterJS(
	name string,
	jsBody string,
	paramTypes []ValType,
	resultType ValType,
	timeout time.Duration,
) error {
	if !isValidIdentifier(name) {
		return fmt.Errorf("invalid function name %q: must be a valid identifier", name)
	}
	if len(jsBody) > MaxModuleSize {
		return fmt.Errorf("JS function body too large: %d bytes (max %d)", len(jsBody), MaxModuleSize)
	}
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	ctx := v8.NewContext(r.iso)
	_, err := ctx.RunScript(jsBody, "setup_"+name+".js")
	ctx.Close()
	if err != nil {
		return fmt.Errorf("compiling JS function %q: %w", name, err)
	}

	cf := &compiledFunc{
		jsSetup:    jsBody,
		paramTypes: paramTypes,
		resultType: resultType,
		timeout:    timeout,
	}

	r.mu.Lock()
	r.funcs[name] = cf
	r.mu.Unlock()
	return nil
}

// Deregister removes a UDF from the registry.
func (r *Registry) Deregister(name string) bool {
	r.mu.Lock()
	_, ok := r.funcs[name]
	if ok {
		delete(r.funcs, name)
	}
	r.mu.Unlock()
	return ok
}

// MakeFn returns an Overload.Fn callback for compatibility with the SQL
// engine's per-row function interface. This is the SLOW PATH -- it calls
// the batch API with a batch of 1. Callers that process multiple rows
// should use Call() directly with a proper batch.
func (r *Registry) MakeFn(name string) (func(*tree.EvalContext, tree.Datums) (tree.Datum, error), error) {
	r.mu.RLock()
	_, ok := r.funcs[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("UDF %q not registered", name)
	}

	// Cache a TxnContext across calls to avoid the ~60µs cost of
	// v8go.NewContext on every invocation. The cached context is
	// invalidated when the evalCtx changes (new transaction/query).
	var cachedTC *TxnContext
	var cachedEvalCtx *tree.EvalContext

	return func(evalCtx *tree.EvalContext, args tree.Datums) (tree.Datum, error) {
		if cachedTC == nil || evalCtx != cachedEvalCtx {
			if cachedTC != nil {
				cachedTC.Close()
			}
			var executor SQLExecutor
			var txn interface{}
			goCtx := context.Background()
			if evalCtx != nil {
				if ie, ok := evalCtx.UDFSQLExecutor.(SQLExecutor); ok {
					executor = ie
				}
				if evalCtx.Txn != nil {
					txn = evalCtx.Txn
				}
				goCtx = evalCtx.Context
			}
			cachedTC = r.NewTxnContext(executor, goCtx, txn, nil)
			cachedEvalCtx = evalCtx
		}
		results, err := r.Call(cachedTC, name, []tree.Datums{args})
		if err != nil {
			return nil, err
		}
		return results[0], nil
	}, nil
}

// Call executes a UDF for a batch of row arguments within a TxnContext.
// This is the primary execution API. All callers should batch rows.
//
// For pure JS/WASM functions (no sql“), it builds a single JS script
// that calls invoke() for each row and returns results via JSON.stringify.
//
// For async functions using sql“, each row is executed sequentially
// with Promise pumping.
//
// Each element of argsBatch is the Datums for one invocation.
// Returns one Datum per invocation.
func (r *Registry) Call(
	tc *TxnContext, name string, argsBatch []tree.Datums,
) ([]tree.Datum, error) {
	if len(argsBatch) == 0 {
		return nil, nil
	}

	r.mu.RLock()
	cf, ok := r.funcs[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("UDF %q not registered", name)
	}

	r.execMu.Lock()
	defer r.execMu.Unlock()

	// Set up async state for sql`` calls.
	r.callState = asyncCallState{
		executor: tc.executor,
		ctx:      tc.goCtx,
		txn:      tc.txn,
		override: tc.override,
		results:  make(chan asyncSQLResult, 16),
	}
	defer func() { r.callState = asyncCallState{} }()

	// Lazily evaluate function setup.
	if !tc.setupDone[name] {
		_, err := tc.v8ctx.RunScript(cf.jsSetup, name+".js")
		if err != nil {
			return nil, err
		}
		tc.setupDone[name] = true
	}

	return r.callSequential(tc, cf, name, argsBatch)
}

func (r *Registry) unmarshalResult(val *v8.Value, cf *compiledFunc) (tree.Datum, error) {
	switch cf.resultType {
	case ValI64:
		return UnmarshalJSResult(int64(val.Integer()), ValI64)
	case ValF64:
		d := tree.DFloat(val.Number())
		return &d, nil
	case ValI32:
		return UnmarshalJSResult(int64(val.Int32()), ValI32)
	case ValString:
		d := tree.DString(val.String())
		return &d, nil
	case ValTimestamp:
		// The call expression was wrapped in JSON.stringify, so val is a
		// quoted ISO 8601 string like "\"2025-01-01T08:00:00.000Z\"".
		s := val.String()
		if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
			s = s[1 : len(s)-1]
		}
		t, _, err := tree.ParseDTimestamp(nil, s, time.Microsecond)
		if err != nil {
			return nil, fmt.Errorf("parsing timestamp result %q: %w", s, err)
		}
		return t, nil
	case ValJSON:
		// The call expression was wrapped in JSON.stringify, so val is
		// the JSON string of the result object.
		j, err := crdbJSON.ParseJSON(val.String())
		if err != nil {
			return nil, fmt.Errorf("parsing JSON result: %w", err)
		}
		return tree.NewDJSON(j), nil
	default:
		return nil, fmt.Errorf("unsupported result type: 0x%02x", byte(cf.resultType))
	}
}

// callSequential executes a UDF one row at a time, pumping Promises for
// async functions that use sql“. Caller must hold execMu.
func (r *Registry) callSequential(
	tc *TxnContext, cf *compiledFunc, name string, argsBatch []tree.Datums,
) ([]tree.Datum, error) {
	results := make([]tree.Datum, len(argsBatch))

	for i, args := range argsBatch {
		jsArgs := make([]string, len(args))
		for j, arg := range args {
			s, err := MarshalDatumToJS(arg, cf.paramTypes[j])
			if err != nil {
				return nil, fmt.Errorf("row %d arg %d: %w", i, j, err)
			}
			jsArgs[j] = s
		}

		callExpr := "invoke(" + strings.Join(jsArgs, ", ") + ")"
		// Date.toString() gives locale format, Object.toString() gives "[object Object]".
		// Wrap in JSON.stringify so we get ISO 8601 for timestamps and real JSON for objects.
		if cf.resultType == ValTimestamp || cf.resultType == ValJSON {
			callExpr = "JSON.stringify(" + callExpr + ")"
		}

		timer := time.AfterFunc(cf.timeout, func() {
			r.iso.TerminateExecution()
		})

		val, err := tc.v8ctx.RunScript(callExpr, name+"_call.js")
		if err != nil {
			timer.Stop()
			if jsErr, ok := err.(*v8.JSError); ok {
				if jsErr.StackTrace != "" && jsErr.StackTrace != jsErr.Message {
					return nil, fmt.Errorf("UDF %q row %d: %s", name, i, jsErr.StackTrace)
				}
			}
			return nil, fmt.Errorf("UDF %q row %d: %w", name, i, err)
		}

		// If result is a Promise, pump until resolved.
		if prom, promErr := val.AsPromise(); promErr == nil {
			for prom.State() == v8.Pending {
				r.drainAsyncResults(tc.v8ctx)
				if r.callState.pending.Load() > 0 {
					r.waitAsyncResults(tc.v8ctx)
				}
				tc.v8ctx.PerformMicrotaskCheckpoint()
			}
			timer.Stop()
			if prom.State() == v8.Rejected {
				return nil, fmt.Errorf("UDF %q row %d rejected: %s", name, i, prom.Result().String())
			}
			val = prom.Result()
		} else {
			timer.Stop()
		}

		d, err := r.unmarshalResult(val, cf)
		if err != nil {
			return nil, fmt.Errorf("UDF %q row %d: %w", name, i, err)
		}
		results[i] = d
	}

	return results, nil
}

// Close shuts down the registry and releases V8 resources.
func (r *Registry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for name := range r.funcs {
		delete(r.funcs, name)
	}
	r.iso.Dispose()
}

// List returns the names of all registered functions.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.funcs))
	for name := range r.funcs {
		names = append(names, name)
	}
	return names
}

// GetSignature returns the param/result types for a registered function.
func (r *Registry) GetSignature(name string) (paramTypes []ValType, resultType ValType, ok bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cf, exists := r.funcs[name]
	if !exists {
		return nil, 0, false
	}
	return cf.paramTypes, cf.resultType, true
}

// isValidIdentifier checks that a name is safe to embed in generated JS code.
// Only allows alphanumeric characters and underscores.
func isValidIdentifier(name string) bool {
	if len(name) == 0 {
		return false
	}
	for i, ch := range name {
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch == '_' {
			continue
		}
		if i > 0 && ch >= '0' && ch <= '9' {
			continue
		}
		return false
	}
	return true
}
