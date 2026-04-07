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

// MaxModuleSize is the maximum size of a JS source body.
const MaxModuleSize = 10 << 20 // 10MB

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
	jsSetup     string // evaluated once per context to define the function
	poolResetJS string // evaluated before each batch: resets pool + sets dateArgs
	paramTypes  []ValType
	resultType  ValType
	timeout     time.Duration
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

	// Precompute the pool reset script that also sets dateArgs for timestamp hydration.
	dateArgsJS := "[]"
	for i, pt := range paramTypes {
		if pt == ValTimestamp {
			if dateArgsJS == "[]" {
				dateArgsJS = fmt.Sprintf("[%d", i)
			} else {
				dateArgsJS += fmt.Sprintf(",%d", i)
			}
		}
	}
	if dateArgsJS != "[]" {
		dateArgsJS += "]"
	}
	poolResetJS := "__pool.reset(); __pool.dateArgs = " + dateArgsJS + ";"

	cf := &compiledFunc{
		jsSetup:     jsBody,
		poolResetJS: poolResetJS,
		paramTypes:  paramTypes,
		resultType:  resultType,
		timeout:     timeout,
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
// Uses a sliding-window pool: Go submits work to V8 via __pool.submit(),
// V8 invokes the user function, and Go collects results via __pool.collect().
// Sync functions complete immediately; async functions (Promises) are
// tracked and resolved via microtask pumping. No sync/async classification
// is needed — both paths are unified.
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

	// Initialize callState for this invocation so sql`` callbacks have
	// access to the executor, context, and result channel.
	r.callState = asyncCallState{
		executor: tc.executor,
		ctx:      tc.goCtx,
		txn:      tc.txn,
		override: tc.override,
		results:  make(chan asyncSQLResult, 64),
	}

	// Lazily evaluate function setup.
	if !tc.setupDone[name] {
		_, err := tc.v8ctx.RunScript(cf.jsSetup, name+".js")
		if err != nil {
			return nil, err
		}
		tc.setupDone[name] = true
	}

	// Lazily install the pool JS into this V8 context.
	if !tc.setupDone["__pool"] {
		if _, err := tc.v8ctx.RunScript(poolJS, "pool.js"); err != nil {
			return nil, fmt.Errorf("installing pool: %w", err)
		}
		tc.setupDone["__pool"] = true
	}

	timer := time.AfterFunc(cf.timeout*time.Duration(len(argsBatch)), func() {
		r.iso.TerminateExecution()
	})
	defer timer.Stop()

	// Reset pool state and set timestamp hydration indices for this function.
	if _, err := tc.v8ctx.RunScript(cf.poolResetJS, "pool_reset.js"); err != nil {
		return nil, fmt.Errorf("pool reset: %w", err)
	}

	results := make([]tree.Datum, len(argsBatch))
	remaining := argsBatch
	submitted := 0
	collected := 0

	for collected < len(argsBatch) {
		// 1. Submit up to poolBatchSize - inflight new rows.
		inflight := submitted - collected
		toSubmit := poolBatchSize - inflight
		if toSubmit > len(remaining) {
			toSubmit = len(remaining)
		}
		if toSubmit > 0 {
			chunk := remaining[:toSubmit]
			remaining = remaining[toSubmit:]

			buf := r.getBuf()
			buf.Grow(len(chunk)*20 + 32)
			buf.WriteByte('[')
			for i, args := range chunk {
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
						return nil, fmt.Errorf("row %d arg %d: %w", submitted+i, j, err)
					}
				}
				buf.WriteByte(']')
			}
			buf.WriteByte(']')

			byts := buf.Bytes()
			argsStr := unsafe.String(&byts[0], len(byts))
			strVal, err := v8.NewValue(r.iso, argsStr)
			r.putBuf(buf)
			if err != nil {
				return nil, fmt.Errorf("UDF %q: creating args string: %w", name, err)
			}

			_, err = tc.v8ctx.Global().MethodCall("__pool_submit", strVal)
			if err != nil {
				return nil, r.formatJSError(name, err)
			}

			submitted += toSubmit

			// Pump microtasks to let any sql`` goroutines start.
			tc.v8ctx.PerformMicrotaskCheckpoint()
		}

		// 2. Collect completed results.
		val, err := tc.v8ctx.Global().MethodCall("__pool_collect")
		if err != nil {
			return nil, r.formatJSError(name, err)
		}

		if !val.IsNull() && !val.IsUndefined() {
			n, err := r.parsePoolResults(val.String(), name, cf, results)
			if err != nil {
				return nil, err
			}
			collected += n
		} else if submitted-collected > 0 {
			// Nothing ready yet but we have in-flight async work.
			// Drain async SQL results and pump microtasks.
			r.drainAsyncResults(tc.v8ctx)
			if r.callState.pending.Load() > 0 {
				r.waitAsyncResults(tc.v8ctx)
			}
			tc.v8ctx.PerformMicrotaskCheckpoint()
		}
	}

	return results, nil
}

// parsePoolResults parses the JSON output from __pool.collect():
// [[idx, value], ...] or [[idx, null, "error"], ...].
// Places results at results[idx]. Returns the count parsed.
func (r *Registry) parsePoolResults(
	jsonStr string, name string, cf *compiledFunc, results []tree.Datum,
) (int, error) {
	var entries []jsoniter.RawMessage
	if err := jsoniter.UnmarshalFromString(jsonStr, &entries); err != nil {
		return 0, fmt.Errorf("parsing pool results: %w", err)
	}

	for _, raw := range entries {
		// Each entry is [idx, value] or [idx, null, "error"].
		var tuple []jsoniter.RawMessage
		if err := jsoniter.Unmarshal(raw, &tuple); err != nil {
			return 0, fmt.Errorf("parsing pool entry: %w", err)
		}
		if len(tuple) < 2 {
			return 0, fmt.Errorf("pool entry too short: %s", string(raw))
		}

		var idx int
		if err := jsoniter.Unmarshal(tuple[0], &idx); err != nil {
			return 0, fmt.Errorf("parsing pool idx: %w", err)
		}
		if idx < 0 || idx >= len(results) {
			return 0, fmt.Errorf("pool idx out of range: %d", idx)
		}

		// Check for error (3-element tuple).
		if len(tuple) >= 3 {
			var errMsg string
			if err := jsoniter.Unmarshal(tuple[2], &errMsg); err != nil {
				return 0, fmt.Errorf("parsing pool error: %w", err)
			}
			return 0, fmt.Errorf("UDF %q row %d: %s", name, idx, errMsg)
		}

		// Unmarshal the value.
		valRaw := tuple[1]
		d, err := r.unmarshalRawResult(valRaw, cf)
		if err != nil {
			return 0, fmt.Errorf("UDF result %d: %w", idx, err)
		}
		results[idx] = d
	}

	return len(entries), nil
}

// unmarshalRawResult converts a raw JSON value to a Datum based on the
// function's result type.
func (r *Registry) unmarshalRawResult(raw jsoniter.RawMessage, cf *compiledFunc) (tree.Datum, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return tree.DNull, nil
	}
	switch cf.resultType {
	case ValI64:
		var n float64
		if err := jsoniter.Unmarshal(raw, &n); err != nil {
			return nil, err
		}
		d := tree.DInt(int64(n))
		return &d, nil
	case ValF64:
		var n float64
		if err := jsoniter.Unmarshal(raw, &n); err != nil {
			return nil, err
		}
		d := tree.DFloat(n)
		return &d, nil
	case ValI32:
		s := string(raw)
		if s == "true" || s == "1" {
			return tree.DBoolTrue, nil
		} else if s == "false" || s == "0" || s == "" {
			return tree.DBoolFalse, nil
		}
		var n float64
		if err := jsoniter.Unmarshal(raw, &n); err != nil {
			return nil, err
		}
		if n != 0 {
			return tree.DBoolTrue, nil
		}
		return tree.DBoolFalse, nil
	case ValString:
		var s string
		if err := jsoniter.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		d := tree.DString(s)
		return &d, nil
	case ValTimestamp:
		var s string
		if err := jsoniter.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		t, _, err := tree.ParseDTimestamp(nil, s, time.Microsecond)
		if err != nil {
			return nil, fmt.Errorf("parsing timestamp %q: %w", s, err)
		}
		return t, nil
	case ValJSON:
		j, err := crdbJSON.ParseJSON(string(raw))
		if err != nil {
			return nil, fmt.Errorf("parsing JSON: %w", err)
		}
		return tree.NewDJSON(j), nil
	default:
		return nil, fmt.Errorf("unsupported result type: 0x%02x", byte(cf.resultType))
	}
}

func (r *Registry) formatJSError(name string, err error) error {
	if jsErr, ok := err.(*v8.JSError); ok {
		if jsErr.StackTrace != "" && jsErr.StackTrace != jsErr.Message {
			return fmt.Errorf("UDF %q: %s", name, jsErr.StackTrace)
		}
	}
	return fmt.Errorf("UDF %q: %w", name, err)
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
