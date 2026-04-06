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
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	crdbJSON "github.com/cockroachdb/cockroach/pkg/util/json"
	jsoniter "github.com/json-iterator/go"
	v8 "github.com/tommie/v8go"
)

// DefaultTimeout is the default execution timeout for a single UDF call.
const DefaultTimeout = 100 * time.Millisecond

// MaxModuleSize is the maximum size of a WASM binary or JS source body.
const MaxModuleSize = 10 << 20 // 10MB

// Language identifies the UDF source language.
type Language string

const (
	LangWasm       Language = "wasm"
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
	bufPool     sync.Pool            // pool of *bytes.Buffer
}

type compiledFunc struct {
	jsSetup    string // evaluated once per context to define the function
	jsCall     string // call expression prefix: "invoke(" or "__wasm_inst_X.exports.invoke("
	language   Language
	paramTypes []ValType
	resultType ValType
	timeout    time.Duration
}

// NewRegistry creates a new V8-backed UDF registry.
func NewRegistry() *Registry {
	r := &Registry{
		iso:   v8.NewIsolate(),
		funcs: make(map[string]*compiledFunc),
		bufPool: sync.Pool{New: func() interface{} {
			return &bytes.Buffer{}
		}},
	}
	r.sqlTemplate = r.makeAsyncSQLTemplate()
	return r
}

func (r *Registry) getBuf() *bytes.Buffer {
	buf := r.bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func (r *Registry) putBuf(buf *bytes.Buffer) {
	r.bufPool.Put(buf)
}

// CompileAndRegisterWasm compiles a WASM binary and registers it.
func (r *Registry) CompileAndRegisterWasm(
	name string,
	wasmBytes []byte,
	exportName string,
	paramTypes []ValType,
	resultType ValType,
	timeout time.Duration,
) error {
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	if !isValidIdentifier(name) {
		return fmt.Errorf("invalid function name %q: must be a valid identifier", name)
	}
	if len(wasmBytes) > MaxModuleSize {
		return fmt.Errorf("WASM module too large: %d bytes (max %d)", len(wasmBytes), MaxModuleSize)
	}

	jsArray := wasmBytesToJSArray(wasmBytes)
	jsSetup := fmt.Sprintf(`
		const __wasm_bytes_%s = new Uint8Array(%s);
		const __wasm_mod_%s = new WebAssembly.Module(__wasm_bytes_%s);
		const __wasm_inst_%s = new WebAssembly.Instance(__wasm_mod_%s);
	`, name, jsArray, name, name, name, name)

	ctx := v8.NewContext(r.iso)
	_, err := ctx.RunScript(jsSetup, "setup_"+name+".js")
	ctx.Close()
	if err != nil {
		return fmt.Errorf("compiling WASM module %q: %w", name, err)
	}

	cf := &compiledFunc{
		jsSetup:    jsSetup,
		jsCall:     fmt.Sprintf("__wasm_inst_%s.exports.%s(", name, exportName),
		language:   LangWasm,
		paramTypes: paramTypes,
		resultType: resultType,
		timeout:    timeout,
	}

	r.mu.Lock()
	r.funcs[name] = cf
	r.mu.Unlock()
	return nil
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
		jsCall:     "invoke(",
		language:   LangJavaScript,
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

	// Detect async functions: if the function is declared with "async function",
	// it returns Promises and can't be batched via JSON.stringify.
	// Fall back to sequential per-row execution.
	if strings.Contains(cf.jsSetup, "async function") {
		return r.callSequential(tc, cf, name, argsBatch)
	}

	// Lazily install the __batch helper that takes a JSON string of args,
	// parses it inside V8, calls invoke for each row, and returns
	// JSON.stringify of the results. All in one JS function call.
	batchKey := "__batch_" + name
	if !tc.setupDone[batchKey] {
		wrapBigInt := cf.resultType == ValI64 && cf.language == LangWasm
		var mapExpr string
		if wrapBigInt {
			mapExpr = "Number(" + cf.jsCall + "...a))"
		} else {
			mapExpr = cf.jsCall + "...a)"
		}
		batchSetup := fmt.Sprintf(
			"function %s(jsonStr) { return JSON.stringify(JSON.parse(jsonStr).map(a => { %s return %s; })); }",
			batchKey, jsBatchArgHydration(cf.paramTypes), mapExpr)
		if _, err := tc.v8ctx.RunScript(batchSetup, batchKey+".js"); err != nil {
			return nil, fmt.Errorf("installing batch helper: %w", err)
		}
		tc.setupDone[batchKey] = true
	}

	// Build JSON array of args in Go: [[arg0_0,arg0_1],[arg1_0,arg1_1],...]
	buf := r.getBuf()
	buf.Grow(len(argsBatch)*20 + 32)
	buf.WriteByte('[')
	for i, args := range argsBatch {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteByte('[')
		for j, arg := range args {
			if j > 0 {
				buf.WriteByte(',')
			}
			if err := WriteDatumJSON(buf, arg, cf.paramTypes[j]); err != nil {
				r.putBuf(buf)
				return nil, fmt.Errorf("row %d arg %d: %w", i, j, err)
			}
		}
		buf.WriteByte(']')
	}
	buf.WriteByte(']')

	// Create a V8 string value from the buffer (one CGO call, no parsing),
	// then call the batch function directly (one CGO call). The batch
	// function does JSON.parse + invoke + JSON.stringify all in JS.
	byts := buf.Bytes()
	argsStr := unsafe.String(&byts[0], len(byts))
	strVal, err := v8.NewValue(r.iso, argsStr)
	r.putBuf(buf)
	if err != nil {
		return nil, fmt.Errorf("UDF %q: creating args string: %w", name, err)
	}

	timer := time.AfterFunc(cf.timeout*time.Duration(len(argsBatch)), func() {
		r.iso.TerminateExecution()
	})

	val, err := tc.v8ctx.Global().MethodCall(batchKey, strVal)

	// If the batch contains async functions (sql``), the result will be
	// a Promise wrapping the array. Pump until resolved.
	if err == nil {
		if prom, promErr := val.AsPromise(); promErr == nil {
			for prom.State() == v8.Pending {
				r.drainAsyncResults(tc.v8ctx)
				if r.callState.pending.Load() > 0 {
					r.waitAsyncResults(tc.v8ctx)
				}
				tc.v8ctx.PerformMicrotaskCheckpoint()
			}
			if prom.State() == v8.Rejected {
				timer.Stop()
				return nil, fmt.Errorf("UDF %q rejected: %s", name, prom.Result().String())
			}
			val = prom.Result()
		}
	}
	timer.Stop()

	if err != nil {
		if jsErr, ok := err.(*v8.JSError); ok {
			if jsErr.StackTrace != "" && jsErr.StackTrace != jsErr.Message {
				return nil, fmt.Errorf("UDF %q: %s", name, jsErr.StackTrace)
			}
		}
		return nil, fmt.Errorf("UDF %q: %w", name, err)
	}

	// Parse the JSON string in Go -- one CGO call instead of N GetIdx calls.
	jsonStr := val.String()

	// Parse JSON results. Each element can be null (→ DNull) or a typed value.
	var raws []jsoniter.RawMessage
	if err := jsoniter.Unmarshal([]byte(jsonStr), &raws); err != nil {
		return nil, fmt.Errorf("UDF %q: parsing results: %w", name, err)
	}

	results := make([]tree.Datum, len(argsBatch))
	for i, raw := range raws {
		if len(raw) == 0 || string(raw) == "null" {
			results[i] = tree.DNull
			continue
		}
		switch cf.resultType {
		case ValI64:
			var n float64
			if err := jsoniter.Unmarshal(raw, &n); err != nil {
				return nil, fmt.Errorf("UDF %q result %d: %w", name, i, err)
			}
			d := tree.DInt(int64(n))
			results[i] = &d
		case ValF64:
			var n float64
			if err := jsoniter.Unmarshal(raw, &n); err != nil {
				return nil, fmt.Errorf("UDF %q result %d: %w", name, i, err)
			}
			d := tree.DFloat(n)
			results[i] = &d
		case ValI32:
			var n float64
			if err := jsoniter.Unmarshal(raw, &n); err != nil {
				return nil, fmt.Errorf("UDF %q result %d: %w", name, i, err)
			}
			if n != 0 {
				results[i] = tree.DBoolTrue
			} else {
				results[i] = tree.DBoolFalse
			}
		case ValString:
			var s string
			if err := jsoniter.Unmarshal(raw, &s); err != nil {
				return nil, fmt.Errorf("UDF %q result %d: %w", name, i, err)
			}
			d := tree.DString(s)
			results[i] = &d
		case ValTimestamp:
			var s string
			if err := jsoniter.Unmarshal(raw, &s); err != nil {
				return nil, fmt.Errorf("UDF %q result %d: %w", name, i, err)
			}
			t, _, err := tree.ParseDTimestamp(nil, s, time.Microsecond)
			if err != nil {
				return nil, fmt.Errorf("UDF %q result %d: parsing timestamp %q: %w", name, i, s, err)
			}
			results[i] = t
		case ValJSON:
			j, err := crdbJSON.ParseJSON(string(raw))
			if err != nil {
				return nil, fmt.Errorf("UDF %q result %d: parsing JSON: %w", name, i, err)
			}
			results[i] = tree.NewDJSON(j)
		default:
			return nil, fmt.Errorf("unsupported result type: 0x%02x", byte(cf.resultType))
		}
	}

	return results, nil
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
		// JS Date.toISOString() or valueOf() -- we get the string representation.
		s := val.String()
		t, _, err := tree.ParseDTimestamp(nil, s, time.Microsecond)
		if err != nil {
			return nil, fmt.Errorf("parsing timestamp result %q: %w", s, err)
		}
		return t, nil
	case ValJSON:
		s := val.String()
		j, err := crdbJSON.ParseJSON(s)
		if err != nil {
			return nil, fmt.Errorf("parsing JSON result: %w", err)
		}
		return tree.NewDJSON(j), nil
	default:
		return nil, fmt.Errorf("unsupported result type: 0x%02x", byte(cf.resultType))
	}
}

// callSequential executes an async UDF one row at a time, pumping Promises.
// Used for functions that contain sql“ calls (which return Promises).
// Caller must hold execMu.
func (r *Registry) callSequential(
	tc *TxnContext, cf *compiledFunc, name string, argsBatch []tree.Datums,
) ([]tree.Datum, error) {
	forWasm := cf.language == LangWasm
	results := make([]tree.Datum, len(argsBatch))

	for i, args := range argsBatch {
		jsArgs := make([]string, len(args))
		for j, arg := range args {
			s, err := MarshalDatumToJS(arg, cf.paramTypes[j], forWasm)
			if err != nil {
				return nil, fmt.Errorf("row %d arg %d: %w", i, j, err)
			}
			jsArgs[j] = s
		}

		callExpr := cf.jsCall + strings.Join(jsArgs, ", ") + ")"
		if cf.resultType == ValI64 && cf.language == LangWasm {
			callExpr = "Number(" + callExpr + ")"
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

// GetSignature returns the param/result types and language for a registered function.
func (r *Registry) GetSignature(name string) (paramTypes []ValType, resultType ValType, lang Language, ok bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cf, exists := r.funcs[name]
	if !exists {
		return nil, 0, "", false
	}
	return cf.paramTypes, cf.resultType, cf.language, true
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

// wasmBytesToJSArray converts WASM bytes to a JS array literal like "[0,97,115,109,...]"
func wasmBytesToJSArray(b []byte) string {
	parts := make([]string, len(b))
	for i, v := range b {
		parts[i] = fmt.Sprintf("%d", v)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func jsBatchArgHydration(paramTypes []ValType) string {
	var b strings.Builder
	for i, vt := range paramTypes {
		if vt != ValTimestamp {
			continue
		}
		fmt.Fprintf(&b, "a[%d] = new Date(a[%d]); ", i, i)
	}
	return b.String()
}
