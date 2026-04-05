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
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/semistrict/ratel/pkg/sql/sem/tree"
)

func TestWat2WasmAddInts(t *testing.T) {
	wat := `(module
		(func (export "invoke") (param i64 i64) (result i64)
			local.get 0
			local.get 1
			i64.add))`

	wasm, err := Wat2Wasm(wat)
	if err != nil {
		t.Fatalf("Wat2Wasm: %v", err)
	}
	if len(wasm) == 0 {
		t.Fatal("Wat2Wasm produced empty output")
	}
	// Verify magic number.
	if wasm[0] != 0x00 || wasm[1] != 0x61 || wasm[2] != 0x73 || wasm[3] != 0x6D {
		t.Fatalf("bad magic: %x", wasm[:4])
	}
}

func TestWat2WasmNamedParams(t *testing.T) {
	wat := `(module
		(func (export "invoke") (param $a i64) (param $b i64) (result i64)
			local.get $a
			local.get $b
			i64.sub))`

	wasm, err := Wat2Wasm(wat)
	if err != nil {
		t.Fatalf("Wat2Wasm: %v", err)
	}
	if len(wasm) == 0 {
		t.Fatal("Wat2Wasm produced empty output")
	}
}

func TestWat2WasmFoldedInstructions(t *testing.T) {
	wat := `(module
		(func (export "invoke") (param i64 i64) (result i64)
			(i64.add (local.get 0) (local.get 1))))`

	wasm, err := Wat2Wasm(wat)
	if err != nil {
		t.Fatalf("Wat2Wasm: %v", err)
	}
	if len(wasm) == 0 {
		t.Fatal("Wat2Wasm produced empty output")
	}
}

func TestWat2WasmWithLocals(t *testing.T) {
	wat := `(module
		(func (export "invoke") (param $x i64) (result i64)
			(local $tmp i64)
			local.get $x
			i64.const 2
			i64.mul
			local.set $tmp
			local.get $tmp))`

	wasm, err := Wat2Wasm(wat)
	if err != nil {
		t.Fatalf("Wat2Wasm: %v", err)
	}
	if len(wasm) == 0 {
		t.Fatal("Wat2Wasm produced empty output")
	}
}

func TestWat2WasmMultipleFunctions(t *testing.T) {
	wat := `(module
		(func $double (param i64) (result i64)
			local.get 0
			i64.const 2
			i64.mul)
		(func (export "invoke") (param i64) (result i64)
			local.get 0
			call $double))`

	wasm, err := Wat2Wasm(wat)
	if err != nil {
		t.Fatalf("Wat2Wasm: %v", err)
	}
	if len(wasm) == 0 {
		t.Fatal("Wat2Wasm produced empty output")
	}
}

func TestWat2WasmFloat64(t *testing.T) {
	wat := `(module
		(func (export "invoke") (param f64 f64) (result f64)
			local.get 0
			local.get 1
			f64.add))`

	wasm, err := Wat2Wasm(wat)
	if err != nil {
		t.Fatalf("Wat2Wasm: %v", err)
	}
	if len(wasm) == 0 {
		t.Fatal("Wat2Wasm produced empty output")
	}
}

func TestWat2WasmBoolToI32(t *testing.T) {
	wat := `(module
		(func (export "invoke") (param i32 i32) (result i32)
			local.get 0
			local.get 1
			i32.and))`

	wasm, err := Wat2Wasm(wat)
	if err != nil {
		t.Fatalf("Wat2Wasm: %v", err)
	}
	if len(wasm) == 0 {
		t.Fatal("Wat2Wasm produced empty output")
	}
}

func TestWat2WasmComments(t *testing.T) {
	wat := `(module
		;; This is a line comment
		(func (export "invoke") (param i64) (result i64)
			(; block comment ;)
			local.get 0
			i64.const 1
			i64.add))`

	wasm, err := Wat2Wasm(wat)
	if err != nil {
		t.Fatalf("Wat2Wasm: %v", err)
	}
	if len(wasm) == 0 {
		t.Fatal("Wat2Wasm produced empty output")
	}
}

func TestWat2WasmErrors(t *testing.T) {
	tests := []struct {
		name string
		wat  string
	}{
		{"empty", ""},
		{"no module", "(func (export \"invoke\") (param i64) (result i64) local.get 0)"},
		{"bad type", "(module (func (export \"invoke\") (param i128) (result i64) local.get 0))"},
		{"unknown instr", "(module (func (export \"invoke\") (result i64) i64.invalid))"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Wat2Wasm(tc.wat)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestRegistryCompileAndCall(t *testing.T) {
	ctx := t.Context()
	reg, err := NewRegistry(ctx)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	defer reg.Close(ctx)

	wat := `(module
		(func (export "invoke") (param i64 i64) (result i64)
			local.get 0
			local.get 1
			i64.add))`

	wasm, err := Wat2Wasm(wat)
	if err != nil {
		t.Fatalf("Wat2Wasm: %v", err)
	}

	err = reg.CompileAndRegister(ctx, "add_ints", wasm, "invoke",
		[]ValType{ValI64, ValI64}, ValI64, 0)
	if err != nil {
		t.Fatalf("CompileAndRegister: %v", err)
	}

	fn, err := reg.MakeFn("add_ints")
	if err != nil {
		t.Fatalf("MakeFn: %v", err)
	}

	a := tree.NewDInt(3)
	b := tree.NewDInt(4)
	result, err := fn(nil, tree.Datums{a, b})
	if err != nil {
		t.Fatalf("fn call: %v", err)
	}

	expected := tree.NewDInt(7)
	if result.Compare(nil, expected) != 0 {
		t.Fatalf("expected %s, got %s", expected, result)
	}
}

func TestRegistryFloat64Multiply(t *testing.T) {
	ctx := t.Context()
	reg, err := NewRegistry(ctx)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	defer reg.Close(ctx)

	wat := `(module
		(func (export "invoke") (param f64 f64) (result f64)
			local.get 0
			local.get 1
			f64.mul))`

	wasm, err := Wat2Wasm(wat)
	if err != nil {
		t.Fatalf("Wat2Wasm: %v", err)
	}

	err = reg.CompileAndRegister(ctx, "mul_floats", wasm, "invoke",
		[]ValType{ValF64, ValF64}, ValF64, 0)
	if err != nil {
		t.Fatalf("CompileAndRegister: %v", err)
	}

	fn, err := reg.MakeFn("mul_floats")
	if err != nil {
		t.Fatalf("MakeFn: %v", err)
	}

	a := tree.NewDFloat(2.5)
	b := tree.NewDFloat(4.0)
	result, err := fn(nil, tree.Datums{a, b})
	if err != nil {
		t.Fatalf("fn call: %v", err)
	}

	expected := tree.NewDFloat(10.0)
	if result.Compare(nil, expected) != 0 {
		t.Fatalf("expected %s, got %s", expected, result)
	}
}

func TestRegistryBoolAnd(t *testing.T) {
	ctx := t.Context()
	reg, err := NewRegistry(ctx)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	defer reg.Close(ctx)

	wat := `(module
		(func (export "invoke") (param i32 i32) (result i32)
			local.get 0
			local.get 1
			i32.and))`

	wasm, err := Wat2Wasm(wat)
	if err != nil {
		t.Fatalf("Wat2Wasm: %v", err)
	}

	err = reg.CompileAndRegister(ctx, "bool_and", wasm, "invoke",
		[]ValType{ValI32, ValI32}, ValI32, 0)
	if err != nil {
		t.Fatalf("CompileAndRegister: %v", err)
	}

	fn, err := reg.MakeFn("bool_and")
	if err != nil {
		t.Fatalf("MakeFn: %v", err)
	}

	result, err := fn(nil, tree.Datums{tree.DBoolTrue, tree.DBoolFalse})
	if err != nil {
		t.Fatalf("fn call: %v", err)
	}
	if result != tree.DBoolFalse {
		t.Fatalf("expected false, got %s", result)
	}

	result, err = fn(nil, tree.Datums{tree.DBoolTrue, tree.DBoolTrue})
	if err != nil {
		t.Fatalf("fn call: %v", err)
	}
	if result != tree.DBoolTrue {
		t.Fatalf("expected true, got %s", result)
	}
}

func TestRegistryCallWithHelper(t *testing.T) {
	ctx := t.Context()
	reg, err := NewRegistry(ctx)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	defer reg.Close(ctx)

	wat := `(module
		(func $double (param i64) (result i64)
			local.get 0
			i64.const 2
			i64.mul)
		(func (export "invoke") (param i64) (result i64)
			local.get 0
			call $double))`

	wasm, err := Wat2Wasm(wat)
	if err != nil {
		t.Fatalf("Wat2Wasm: %v", err)
	}

	err = reg.CompileAndRegister(ctx, "double", wasm, "invoke",
		[]ValType{ValI64}, ValI64, 0)
	if err != nil {
		t.Fatalf("CompileAndRegister: %v", err)
	}

	fn, err := reg.MakeFn("double")
	if err != nil {
		t.Fatalf("MakeFn: %v", err)
	}

	result, err := fn(nil, tree.Datums{tree.NewDInt(21)})
	if err != nil {
		t.Fatalf("fn call: %v", err)
	}
	expected := tree.NewDInt(42)
	if result.Compare(nil, expected) != 0 {
		t.Fatalf("expected %s, got %s", expected, result)
	}
}

func TestRegistryDeregister(t *testing.T) {
	ctx := t.Context()
	reg, err := NewRegistry(ctx)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	defer reg.Close(ctx)

	wat := `(module
		(func (export "invoke") (param i64) (result i64)
			local.get 0))`

	wasm, err := Wat2Wasm(wat)
	if err != nil {
		t.Fatalf("Wat2Wasm: %v", err)
	}

	err = reg.CompileAndRegister(ctx, "identity", wasm, "invoke",
		[]ValType{ValI64}, ValI64, 0)
	if err != nil {
		t.Fatalf("CompileAndRegister: %v", err)
	}

	if !reg.Deregister(ctx, "identity") {
		t.Fatal("Deregister returned false for existing function")
	}
	if reg.Deregister(ctx, "identity") {
		t.Fatal("Deregister returned true for already-removed function")
	}

	_, err = reg.MakeFn("identity")
	if err == nil {
		t.Fatal("MakeFn should fail for deregistered function")
	}
}

func TestRegistrySignatureMismatch(t *testing.T) {
	ctx := t.Context()
	reg, err := NewRegistry(ctx)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	defer reg.Close(ctx)

	wat := `(module
		(func (export "invoke") (param i64 i64) (result i64)
			local.get 0
			local.get 1
			i64.add))`

	wasm, err := Wat2Wasm(wat)
	if err != nil {
		t.Fatalf("Wat2Wasm: %v", err)
	}

	// Wrong number of params.
	err = reg.CompileAndRegister(ctx, "bad", wasm, "invoke",
		[]ValType{ValI64}, ValI64, 0)
	if err == nil {
		t.Fatal("expected error for wrong param count")
	}

	// Wrong param type.
	err = reg.CompileAndRegister(ctx, "bad", wasm, "invoke",
		[]ValType{ValI64, ValF64}, ValI64, 0)
	if err == nil {
		t.Fatal("expected error for wrong param type")
	}

	// Wrong result type.
	err = reg.CompileAndRegister(ctx, "bad", wasm, "invoke",
		[]ValType{ValI64, ValI64}, ValF64, 0)
	if err == nil {
		t.Fatal("expected error for wrong result type")
	}

	// Wrong export name.
	err = reg.CompileAndRegister(ctx, "bad", wasm, "nonexistent",
		[]ValType{ValI64, ValI64}, ValI64, 0)
	if err == nil {
		t.Fatal("expected error for missing export")
	}
}

func TestMarshalUnmarshal(t *testing.T) {
	// INT
	intDatum := tree.NewDInt(42)
	v, err := MarshalDatum(intDatum, ValI64)
	if err != nil {
		t.Fatalf("MarshalDatum INT: %v", err)
	}
	result, err := UnmarshalDatum(v, ValI64)
	if err != nil {
		t.Fatalf("UnmarshalDatum INT: %v", err)
	}
	if result.Compare(nil, intDatum) != 0 {
		t.Fatalf("INT roundtrip: expected %s, got %s", intDatum, result)
	}

	// Negative INT
	negDatum := tree.NewDInt(-100)
	v, err = MarshalDatum(negDatum, ValI64)
	if err != nil {
		t.Fatalf("MarshalDatum neg INT: %v", err)
	}
	result, err = UnmarshalDatum(v, ValI64)
	if err != nil {
		t.Fatalf("UnmarshalDatum neg INT: %v", err)
	}
	if result.Compare(nil, negDatum) != 0 {
		t.Fatalf("neg INT roundtrip: expected %s, got %s", negDatum, result)
	}

	// FLOAT
	floatDatum := tree.NewDFloat(3.14)
	v, err = MarshalDatum(floatDatum, ValF64)
	if err != nil {
		t.Fatalf("MarshalDatum FLOAT: %v", err)
	}
	result, err = UnmarshalDatum(v, ValF64)
	if err != nil {
		t.Fatalf("UnmarshalDatum FLOAT: %v", err)
	}
	if result.Compare(nil, floatDatum) != 0 {
		t.Fatalf("FLOAT roundtrip: expected %s, got %s", floatDatum, result)
	}

	// BOOL true
	v, err = MarshalDatum(tree.DBoolTrue, ValI32)
	if err != nil {
		t.Fatalf("MarshalDatum BOOL true: %v", err)
	}
	result, err = UnmarshalDatum(v, ValI32)
	if err != nil {
		t.Fatalf("UnmarshalDatum BOOL true: %v", err)
	}
	if result != tree.DBoolTrue {
		t.Fatalf("BOOL true roundtrip: expected true, got %s", result)
	}

	// BOOL false
	v, err = MarshalDatum(tree.DBoolFalse, ValI32)
	if err != nil {
		t.Fatalf("MarshalDatum BOOL false: %v", err)
	}
	result, err = UnmarshalDatum(v, ValI32)
	if err != nil {
		t.Fatalf("UnmarshalDatum BOOL false: %v", err)
	}
	if result != tree.DBoolFalse {
		t.Fatalf("BOOL false roundtrip: expected false, got %s", result)
	}

	// NULL
	_, err = MarshalDatum(tree.DNull, ValI64)
	if err == nil {
		t.Fatal("MarshalDatum NULL should fail")
	}
}

func TestFuelExhaustion(t *testing.T) {
	ctx := t.Context()
	reg, err := NewRegistry(ctx)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	defer reg.Close(ctx)

	// An infinite loop: br 0 jumps back to the beginning of the loop.
	wat := `(module
		(func (export "invoke") (param i64) (result i64)
			(loop
				br 0)
			local.get 0))`

	wasm, err := Wat2Wasm(wat)
	if err != nil {
		t.Fatalf("Wat2Wasm: %v", err)
	}

	// Register with a very short timeout.
	err = reg.CompileAndRegister(ctx, "infinite", wasm, "invoke",
		[]ValType{ValI64}, ValI64, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("CompileAndRegister: %v", err)
	}

	fn, err := reg.MakeFn("infinite")
	if err != nil {
		t.Fatalf("MakeFn: %v", err)
	}

	_, err = fn(nil, tree.Datums{tree.NewDInt(1)})
	if err == nil {
		t.Fatal("expected error for infinite loop, got nil")
	}
	// The error should mention time limit.
	errStr := err.Error()
	if !strings.Contains(errStr, "time limit") && !strings.Contains(errStr, "context deadline") && !strings.Contains(errStr, "module closed") {
		t.Fatalf("expected timeout-related error, got: %s", errStr)
	}
}

func TestInvalidWasm(t *testing.T) {
	ctx := t.Context()
	reg, err := NewRegistry(ctx)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	defer reg.Close(ctx)

	// Completely invalid bytes.
	err = reg.CompileAndRegister(ctx, "invalid", []byte("not wasm at all"), "invoke",
		[]ValType{ValI64}, ValI64, 0)
	if err == nil {
		t.Fatal("expected error for invalid WASM bytes")
	}

	// Valid WASM header but truncated.
	err = reg.CompileAndRegister(ctx, "truncated", []byte{0x00, 0x61, 0x73, 0x6D, 0x01, 0x00}, "invoke",
		[]ValType{ValI64}, ValI64, 0)
	if err == nil {
		t.Fatal("expected error for truncated WASM")
	}
}

func TestConcurrentCalls(t *testing.T) {
	ctx := t.Context()
	reg, err := NewRegistry(ctx)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	defer reg.Close(ctx)

	wat := `(module
		(func (export "invoke") (param i64 i64) (result i64)
			local.get 0
			local.get 1
			i64.add))`

	wasm, err := Wat2Wasm(wat)
	if err != nil {
		t.Fatalf("Wat2Wasm: %v", err)
	}

	err = reg.CompileAndRegister(ctx, "add", wasm, "invoke",
		[]ValType{ValI64, ValI64}, ValI64, 0)
	if err != nil {
		t.Fatalf("CompileAndRegister: %v", err)
	}

	fn, err := reg.MakeFn("add")
	if err != nil {
		t.Fatalf("MakeFn: %v", err)
	}

	// Run 100 concurrent calls.
	var wg sync.WaitGroup
	errs := make(chan error, 100)
	for i := range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a := tree.NewDInt(tree.DInt(i))
			b := tree.NewDInt(tree.DInt(i + 1))
			result, callErr := fn(nil, tree.Datums{a, b})
			if callErr != nil {
				errs <- callErr
				return
			}
			expected := tree.NewDInt(tree.DInt(2*i + 1))
			if result.Compare(nil, expected) != 0 {
				errs <- fmt.Errorf("expected %s, got %s", expected, result)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Errorf("concurrent call error: %v", e)
	}
}
