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

package wasmruntime

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/semistrict/ratel/pkg/sql/sem/tree"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// DefaultFuelLimit is the default execution timeout for a single WASM function call.
const DefaultFuelLimit = 100 * time.Millisecond

// DefaultMemoryLimitPages is the default memory limit in WASM pages (64KB each).
// 256 pages = 16MB.
const DefaultMemoryLimitPages = 256

// Registry manages compiled WASM modules and provides SQL function callbacks.
type Registry struct {
	runtime wazero.Runtime
	mu      sync.RWMutex
	modules map[string]*compiledFunc
	counter atomic.Uint64
}

// compiledFunc holds a compiled WASM module and its SQL type signature.
type compiledFunc struct {
	compiled   wazero.CompiledModule
	paramTypes []ValType
	resultType ValType
	exportName string
	timeout    time.Duration
	memPages   uint32
}

// NewRegistry creates a new WASM function registry.
// The provided context is used for the lifetime of the underlying runtime.
func NewRegistry(ctx context.Context) (*Registry, error) {
	cfg := wazero.NewRuntimeConfig().
		// No WASI - guarantees determinism.
		WithMemoryLimitPages(DefaultMemoryLimitPages).
		WithCloseOnContextDone(true)

	rt := wazero.NewRuntimeWithConfig(ctx, cfg)
	return &Registry{
		runtime: rt,
		modules: make(map[string]*compiledFunc),
	}, nil
}

// CompileAndRegister compiles a WASM binary module and registers it under the
// given name. The exported function (exportName) must match the expected
// parameter and result types.
func (r *Registry) CompileAndRegister(
	ctx context.Context,
	name string,
	wasmBytes []byte,
	exportName string,
	paramTypes []ValType,
	resultType ValType,
	timeout time.Duration,
) error {
	compiled, err := r.runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return fmt.Errorf("compiling WASM module %q: %w", name, err)
	}

	// Verify the exported function exists and has the right signature.
	exports := compiled.ExportedFunctions()
	fnDef, ok := exports[exportName]
	if !ok {
		compiled.Close(ctx)
		var available []string
		for k := range exports {
			available = append(available, k)
		}
		return fmt.Errorf("WASM module %q does not export function %q (available: %v)", name, exportName, available)
	}

	// Verify parameter count.
	wasmParams := fnDef.ParamTypes()
	wasmResults := fnDef.ResultTypes()
	if len(wasmParams) != len(paramTypes) {
		compiled.Close(ctx)
		return fmt.Errorf("WASM function %q.%s has %d parameters, expected %d",
			name, exportName, len(wasmParams), len(paramTypes))
	}
	if len(wasmResults) != 1 {
		compiled.Close(ctx)
		return fmt.Errorf("WASM function %q.%s has %d results, expected exactly 1",
			name, exportName, len(wasmResults))
	}

	// Verify parameter types match.
	for i, pt := range paramTypes {
		expected := ValTypeToAPI(pt)
		if wasmParams[i] != expected {
			compiled.Close(ctx)
			return fmt.Errorf("WASM function %q.%s parameter %d: expected type 0x%02x, got 0x%02x",
				name, exportName, i, expected, wasmParams[i])
		}
	}

	// Verify result type matches.
	expectedResult := ValTypeToAPI(resultType)
	if wasmResults[0] != expectedResult {
		compiled.Close(ctx)
		return fmt.Errorf("WASM function %q.%s result: expected type 0x%02x, got 0x%02x",
			name, exportName, expectedResult, wasmResults[0])
	}

	if timeout == 0 {
		timeout = DefaultFuelLimit
	}

	cf := &compiledFunc{
		compiled:   compiled,
		paramTypes: paramTypes,
		resultType: resultType,
		exportName: exportName,
		timeout:    timeout,
		memPages:   DefaultMemoryLimitPages,
	}

	r.mu.Lock()
	old, exists := r.modules[name]
	r.modules[name] = cf
	r.mu.Unlock()

	if exists {
		old.compiled.Close(ctx)
	}
	return nil
}

// Deregister removes a WASM function from the registry.
func (r *Registry) Deregister(ctx context.Context, name string) bool {
	r.mu.Lock()
	cf, ok := r.modules[name]
	if ok {
		delete(r.modules, name)
	}
	r.mu.Unlock()

	if ok {
		cf.compiled.Close(ctx)
	}
	return ok
}

// MakeFn returns an Overload.Fn callback for the named WASM function.
// The callback can be used directly as a tree.Overload.Fn.
func (r *Registry) MakeFn(name string) (func(*tree.EvalContext, tree.Datums) (tree.Datum, error), error) {
	r.mu.RLock()
	cf, ok := r.modules[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("WASM function %q not registered", name)
	}

	return func(evalCtx *tree.EvalContext, args tree.Datums) (tree.Datum, error) {
		return r.callFunc(cf, args)
	}, nil
}

func (r *Registry) callFunc(cf *compiledFunc, args tree.Datums) (tree.Datum, error) {
	// Marshal arguments.
	wasmArgs := make([]uint64, len(args))
	for i, arg := range args {
		v, err := MarshalDatum(arg, cf.paramTypes[i])
		if err != nil {
			return nil, fmt.Errorf("marshaling argument %d: %w", i, err)
		}
		wasmArgs[i] = v
	}

	// Create a context with timeout for CPU limiting.
	ctx, cancel := context.WithTimeout(context.Background(), cf.timeout)
	defer cancel()

	// Instantiate a fresh module for this call.
	instanceName := fmt.Sprintf("_wasm_%d", r.counter.Add(1))
	modCfg := wazero.NewModuleConfig().WithName(instanceName)
	inst, err := r.runtime.InstantiateModule(ctx, cf.compiled, modCfg)
	if err != nil {
		return nil, fmt.Errorf("instantiating WASM module: %w", err)
	}
	defer inst.Close(ctx)

	// Call the exported function.
	fn := inst.ExportedFunction(cf.exportName)
	if fn == nil {
		return nil, fmt.Errorf("exported function %q not found in instance", cf.exportName)
	}

	results, err := fn.Call(ctx, wasmArgs...)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("WASM function execution exceeded time limit (%s)", cf.timeout)
		}
		return nil, fmt.Errorf("WASM function execution error: %w", err)
	}

	if len(results) != 1 {
		return nil, fmt.Errorf("WASM function returned %d values, expected 1", len(results))
	}

	return UnmarshalDatum(results[0], cf.resultType)
}

// Close shuts down the registry and releases all WASM resources.
func (r *Registry) Close(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for name, cf := range r.modules {
		cf.compiled.Close(ctx)
		delete(r.modules, name)
	}
	return r.runtime.Close(ctx)
}

// List returns the names of all registered WASM functions.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.modules))
	for name := range r.modules {
		names = append(names, name)
	}
	return names
}

// GetSignature returns the parameter and result types for a registered function.
func (r *Registry) GetSignature(name string) (paramTypes []ValType, resultType ValType, ok bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cf, exists := r.modules[name]
	if !exists {
		return nil, 0, false
	}
	return cf.paramTypes, cf.resultType, true
}

func ValTypeToAPI(vt ValType) api.ValueType {
	switch vt {
	case ValI32:
		return api.ValueTypeI32
	case ValI64:
		return api.ValueTypeI64
	case ValF64:
		return api.ValueTypeF64
	default:
		return api.ValueTypeI64 // fallback
	}
}
