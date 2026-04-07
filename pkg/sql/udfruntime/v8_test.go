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
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	v8 "github.com/tommie/v8go"
)

func TestV8Basic(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	val, err := ctx.RunScript("1 + 2", "test.js")
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}
	if val.Int32() != 3 {
		t.Fatalf("expected 3, got %d", val.Int32())
	}
}

func TestRegistryJavaScript(t *testing.T) {
	reg := NewRegistry()
	defer reg.Close()

	jsBody := `function invoke(a, b) { return a + b; }`

	err := reg.CompileAndRegisterJS("js_add", jsBody,
		[]ValType{ValI64, ValI64}, ValI64, 0)
	if err != nil {
		t.Fatalf("CompileAndRegisterJS: %v", err)
	}

	fn, err := reg.MakeFn("js_add")
	if err != nil {
		t.Fatalf("MakeFn: %v", err)
	}

	result, err := fn(nil, tree.Datums{tree.NewDInt(10), tree.NewDInt(20)})
	if err != nil {
		t.Fatalf("fn call: %v", err)
	}
	expected := tree.NewDInt(30)
	if result.Compare(nil, expected) != 0 {
		t.Fatalf("expected %s, got %s", expected, result)
	}
}

func TestRegistryJavaScriptFloat(t *testing.T) {
	reg := NewRegistry()
	defer reg.Close()

	jsBody := `function invoke(a, b) { return a * b; }`

	err := reg.CompileAndRegisterJS("js_mul", jsBody,
		[]ValType{ValF64, ValF64}, ValF64, 0)
	if err != nil {
		t.Fatalf("CompileAndRegisterJS: %v", err)
	}

	fn, err := reg.MakeFn("js_mul")
	if err != nil {
		t.Fatalf("MakeFn: %v", err)
	}

	result, err := fn(nil, tree.Datums{tree.NewDFloat(3.0), tree.NewDFloat(4.5)})
	if err != nil {
		t.Fatalf("fn call: %v", err)
	}
	expected := tree.NewDFloat(13.5)
	if result.Compare(nil, expected) != 0 {
		t.Fatalf("expected %s, got %s", expected, result)
	}
}

func TestRegistryDeregister(t *testing.T) {
	reg := NewRegistry()
	defer reg.Close()

	jsBody := `function invoke(a) { return a; }`
	err := reg.CompileAndRegisterJS("identity", jsBody,
		[]ValType{ValI64}, ValI64, 0)
	if err != nil {
		t.Fatalf("CompileAndRegisterJS: %v", err)
	}

	if !reg.Deregister("identity") {
		t.Fatal("Deregister returned false for existing function")
	}
	if reg.Deregister("identity") {
		t.Fatal("Deregister returned true for already-removed function")
	}
	_, err = reg.MakeFn("identity")
	if err == nil {
		t.Fatal("MakeFn should fail for deregistered function")
	}
}

func TestRegistryFuelExhaustion(t *testing.T) {
	reg := NewRegistry()
	defer reg.Close()

	jsBody := `function invoke(a) { while(true){} return a; }`
	err := reg.CompileAndRegisterJS("infinite", jsBody,
		[]ValType{ValI64}, ValI64, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("CompileAndRegisterJS: %v", err)
	}

	fn, err := reg.MakeFn("infinite")
	if err != nil {
		t.Fatalf("MakeFn: %v", err)
	}

	_, err = fn(nil, tree.Datums{tree.NewDInt(1)})
	if err == nil {
		t.Fatal("expected error for infinite loop")
	}
}

func TestRegistryConcurrentCalls(t *testing.T) {
	reg := NewRegistry()
	defer reg.Close()

	jsBody := `function invoke(a, b) { return a + b; }`
	err := reg.CompileAndRegisterJS("concurrent_add", jsBody,
		[]ValType{ValI64, ValI64}, ValI64, 0)
	if err != nil {
		t.Fatalf("CompileAndRegisterJS: %v", err)
	}

	fn, err := reg.MakeFn("concurrent_add")
	if err != nil {
		t.Fatalf("MakeFn: %v", err)
	}

	// V8 isolates are single-threaded, so concurrent calls must be serialized.
	// This test verifies correctness, not parallelism.
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := range 20 {
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

func TestJSErrorLineNumbers(t *testing.T) {
	// First: understand what V8 gives us for error line numbers.
	// Create a multi-line function with an error on a specific line.
	reg := NewRegistry()
	defer reg.Close()

	// Error is on line 4 of the user's source (null.foo).
	jsBody := `function invoke(a) {
  var x = a + 1;
  var y = x * 2;
  return null.foo;
}`

	err := reg.CompileAndRegisterJS("bad_func", jsBody,
		[]ValType{ValI64}, ValI64, 0)
	if err != nil {
		t.Fatalf("CompileAndRegisterJS: %v", err)
	}

	fn, err := reg.MakeFn("bad_func")
	if err != nil {
		t.Fatalf("MakeFn: %v", err)
	}

	_, err = fn(nil, tree.Datums{tree.NewDInt(1)})
	if err == nil {
		t.Fatal("expected error from null.foo")
	}

	// The error should reference the function name and ideally line 4.
	errStr := err.Error()
	t.Logf("error message: %s", errStr)

	if !strings.Contains(errStr, "bad_func") {
		t.Errorf("error should mention function name 'bad_func', got: %s", errStr)
	}
	// V8 should report the line where null.foo occurs.
	if !strings.Contains(errStr, "Cannot read properties of null") &&
		!strings.Contains(errStr, "null") {
		t.Errorf("error should mention null property access, got: %s", errStr)
	}
}

func TestJSErrorWithSourceURL(t *testing.T) {
	// Test that //# sourceURL= in user code is respected by V8.
	reg := NewRegistry()
	defer reg.Close()

	jsBody := `//# sourceURL=my_pricing_logic.js
function invoke(a) {
  var x = a + 1;
  return null.foo;
}`

	err := reg.CompileAndRegisterJS("with_source_url", jsBody,
		[]ValType{ValI64}, ValI64, 0)
	if err != nil {
		t.Fatalf("CompileAndRegisterJS: %v", err)
	}

	fn, err := reg.MakeFn("with_source_url")
	if err != nil {
		t.Fatalf("MakeFn: %v", err)
	}

	_, err = fn(nil, tree.Datums{tree.NewDInt(1)})
	if err == nil {
		t.Fatal("expected error")
	}

	errStr := err.Error()
	t.Logf("error with sourceURL: %s", errStr)

	// V8 should use the sourceURL name in the error.
	if !strings.Contains(errStr, "my_pricing_logic.js") {
		t.Errorf("error should reference sourceURL 'my_pricing_logic.js', got: %s", errStr)
	}
}

func TestIsValidIdentifier(t *testing.T) {
	valid := []string{"foo", "add_ints", "myFunc", "_private", "x1"}
	for _, name := range valid {
		if !isValidIdentifier(name) {
			t.Errorf("expected %q to be valid", name)
		}
	}

	invalid := []string{
		"",
		"1foo",                       // starts with digit
		"foo bar",                    // space
		"foo;evil()",                 // semicolon injection
		"foo\nbar",                   // newline
		"foo.bar",                    // dot
		"foo-bar",                    // hyphen
		"foo;evil();//",              // full JS injection
		"__wasm_bytes_x`);evil();//", // template literal injection
	}
	for _, name := range invalid {
		if isValidIdentifier(name) {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}

func TestRegistryRejectsInvalidNames(t *testing.T) {
	reg := NewRegistry()
	defer reg.Close()

	err := reg.CompileAndRegisterJS("foo;evil()", "function invoke() { return 1; }",
		[]ValType{}, ValI64, 0)
	if err == nil {
		t.Fatal("expected error for invalid function name")
	}
	if !strings.Contains(err.Error(), "invalid function name") {
		t.Fatalf("expected 'invalid function name' error, got: %s", err)
	}
}

func TestRegistryRejectsOversizedModule(t *testing.T) {
	reg := NewRegistry()
	defer reg.Close()

	bigBody := strings.Repeat("x", MaxModuleSize+1)
	err := reg.CompileAndRegisterJS("big", bigBody, []ValType{}, ValI64, 0)
	if err == nil {
		t.Fatal("expected error for oversized JS body")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected 'too large' error, got: %s", err)
	}
}

func TestBatchOperatorCorrectness(t *testing.T) {
	// Verify that the batch Call() path produces correct results,
	// not just that it doesn't crash (which the benchmark tests).
	reg := NewRegistry()
	defer reg.Close()

	err := reg.CompileAndRegisterJS("square",
		"function invoke(x) { return x * x; }",
		[]ValType{ValI64}, ValI64, 0)
	if err != nil {
		t.Fatal(err)
	}

	tc := reg.NewTxnContext(nil, context.Background(), nil, nil)
	defer tc.Close()

	batch := []tree.Datums{
		{tree.NewDInt(0)},
		{tree.NewDInt(1)},
		{tree.NewDInt(2)},
		{tree.NewDInt(3)},
		{tree.NewDInt(10)},
		{tree.NewDInt(-5)},
	}

	results, err := reg.Call(tc, "square", batch)
	if err != nil {
		t.Fatal(err)
	}

	expected := []int64{0, 1, 4, 9, 100, 25}
	for i, want := range expected {
		got := int64(*results[i].(*tree.DInt))
		if got != want {
			t.Errorf("row %d: square(%d) = %d, want %d",
				i, int64(*batch[i][0].(*tree.DInt)), got, want)
		}
	}
}

func TestBatchOperatorCorrectnessMultiArg(t *testing.T) {
	reg := NewRegistry()
	defer reg.Close()

	err := reg.CompileAndRegisterJS("add",
		"function invoke(a, b) { return a + b; }",
		[]ValType{ValI64, ValI64}, ValI64, 0)
	if err != nil {
		t.Fatal(err)
	}

	tc := reg.NewTxnContext(nil, context.Background(), nil, nil)
	defer tc.Close()

	batch := []tree.Datums{
		{tree.NewDInt(1), tree.NewDInt(2)},
		{tree.NewDInt(100), tree.NewDInt(-50)},
		{tree.NewDInt(0), tree.NewDInt(0)},
	}

	results, err := reg.Call(tc, "add", batch)
	if err != nil {
		t.Fatal(err)
	}

	expected := []int64{3, 50, 0}
	for i, want := range expected {
		got := int64(*results[i].(*tree.DInt))
		if got != want {
			t.Errorf("row %d: got %d, want %d", i, got, want)
		}
	}
}

func TestBatchOperatorCorrectnessString(t *testing.T) {
	reg := NewRegistry()
	defer reg.Close()

	err := reg.CompileAndRegisterJS("upper",
		"function invoke(s) { return s.toUpperCase(); }",
		[]ValType{ValString}, ValString, 0)
	if err != nil {
		t.Fatal(err)
	}

	tc := reg.NewTxnContext(nil, context.Background(), nil, nil)
	defer tc.Close()

	batch := []tree.Datums{
		{tree.NewDString("hello")},
		{tree.NewDString("world")},
		{tree.NewDString("")},
	}

	results, err := reg.Call(tc, "upper", batch)
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"HELLO", "WORLD", ""}
	for i, want := range expected {
		got := string(*results[i].(*tree.DString))
		if got != want {
			t.Errorf("row %d: got %q, want %q", i, got, want)
		}
	}
}

func TestConcurrentCreateDrop(t *testing.T) {
	reg := NewRegistry()
	defer reg.Close()

	// Pre-register a function.
	err := reg.CompileAndRegisterJS("volatile_fn",
		"function invoke(x) { return x + 1; }",
		[]ValType{ValI64}, ValI64, 0)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 200)

	// Hammer CREATE and DROP concurrently.
	for i := range 50 {
		wg.Add(2)

		// Goroutine that creates functions.
		go func() {
			defer wg.Done()
			name := fmt.Sprintf("fn_%d", i)
			err := reg.CompileAndRegisterJS(name,
				fmt.Sprintf("function invoke(x) { return x + %d; }", i),
				[]ValType{ValI64}, ValI64, 0)
			if err != nil {
				errs <- fmt.Errorf("create %s: %w", name, err)
			}
		}()

		// Goroutine that drops functions.
		go func() {
			defer wg.Done()
			name := fmt.Sprintf("fn_%d", i)
			// May or may not exist yet -- that's fine.
			reg.Deregister(name)
		}()
	}

	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}

var _ = time.Second
