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
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/util/json"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
)

// TestTypeMapping_String tests TEXT/STRING ↔ JS String.
func TestTypeMapping_String(t *testing.T) {
	reg := NewRegistry()
	defer reg.Close()

	// String concatenation.
	err := reg.CompileAndRegisterJS("concat_str",
		`function invoke(a, b) { return a + ' ' + b; }`,
		[]ValType{ValString, ValString}, ValString, 0)
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	tc := reg.NewTxnContext(nil, context.Background(), nil, nil)
	defer tc.Close()

	a := tree.NewDString("hello")
	b := tree.NewDString("world")
	results, err := reg.Call(tc, "concat_str", []tree.Datums{{a, b}})
	if err != nil {
		t.Fatalf("call: %v", err)
	}

	expected := tree.NewDString("hello world")
	if results[0].Compare(nil, expected) != 0 {
		t.Fatalf("expected %s, got %s", expected, results[0])
	}
}

// TestTypeMapping_StringLength tests string methods work in JS.
func TestTypeMapping_StringLength(t *testing.T) {
	reg := NewRegistry()
	defer reg.Close()

	err := reg.CompileAndRegisterJS("str_len",
		`function invoke(s) { return s.length; }`,
		[]ValType{ValString}, ValI64, 0)
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	tc := reg.NewTxnContext(nil, context.Background(), nil, nil)
	defer tc.Close()

	results, err := reg.Call(tc, "str_len", []tree.Datums{{tree.NewDString("hello")}})
	if err != nil {
		t.Fatalf("call: %v", err)
	}

	expected := tree.NewDInt(5)
	if results[0].Compare(nil, expected) != 0 {
		t.Fatalf("expected %s, got %s", expected, results[0])
	}
}

// TestTypeMapping_Bytes tests BYTEA ↔ JS Uint8Array.
func TestTypeMapping_Bytes(t *testing.T) {
	reg := NewRegistry()
	defer reg.Close()

	// Return the length of a byte array.
	err := reg.CompileAndRegisterJS("byte_len",
		`function invoke(b) { return b.length; }`,
		[]ValType{ValBytes}, ValI64, 0)
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	tc := reg.NewTxnContext(nil, context.Background(), nil, nil)
	defer tc.Close()

	input := tree.NewDBytes(tree.DBytes("abc"))
	results, err := reg.Call(tc, "byte_len", []tree.Datums{{input}})
	if err != nil {
		t.Fatalf("call: %v", err)
	}

	expected := tree.NewDInt(3)
	if results[0].Compare(nil, expected) != 0 {
		t.Fatalf("expected %s, got %s", expected, results[0])
	}
}

// TestTypeMapping_Timestamp tests TIMESTAMP ↔ JS Date.
func TestTypeMapping_Timestamp(t *testing.T) {
	reg := NewRegistry()
	defer reg.Close()

	// Extract the year from a timestamp.
	err := reg.CompileAndRegisterJS("get_year",
		`function invoke(ts) { return ts.getFullYear(); }`,
		[]ValType{ValTimestamp}, ValI64, 0)
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	tc := reg.NewTxnContext(nil, context.Background(), nil, nil)
	defer tc.Close()

	ts, _ := tree.MakeDTimestamp(timeutil.Now(), time.Microsecond)
	results, err := reg.Call(tc, "get_year", []tree.Datums{{ts}})
	if err != nil {
		t.Fatalf("call: %v", err)
	}

	expected := tree.NewDInt(2026)
	if results[0].Compare(nil, expected) != 0 {
		t.Fatalf("expected %s, got %s", expected, results[0])
	}
}

// TestTypeMapping_TimestampReturn tests returning a Date from JS as TIMESTAMP.
func TestTypeMapping_TimestampReturn(t *testing.T) {
	reg := NewRegistry()
	defer reg.Close()

	err := reg.CompileAndRegisterJS("make_ts",
		`function invoke(year) { return new Date(year, 0, 1); }`,
		[]ValType{ValI64}, ValTimestamp, 0)
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	tc := reg.NewTxnContext(nil, context.Background(), nil, nil)
	defer tc.Close()

	results, err := reg.Call(tc, "make_ts", []tree.Datums{{tree.NewDInt(2025)}})
	if err != nil {
		t.Fatalf("call: %v", err)
	}

	// The result should be a DTimestamp.
	ts, ok := results[0].(*tree.DTimestamp)
	if !ok {
		t.Fatalf("expected DTimestamp, got %T: %s", results[0], results[0])
	}
	if ts.Time.Year() != 2025 {
		t.Fatalf("expected year 2025, got %d", ts.Time.Year())
	}
}

// TestTypeMapping_JSON tests JSON/JSONB ↔ native JS object.
func TestTypeMapping_JSON(t *testing.T) {
	reg := NewRegistry()
	defer reg.Close()

	// Access properties of a JSON object.
	err := reg.CompileAndRegisterJS("json_name",
		`function invoke(obj) { return obj.name; }`,
		[]ValType{ValJSON}, ValString, 0)
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	tc := reg.NewTxnContext(nil, context.Background(), nil, nil)
	defer tc.Close()

	j, err := json.ParseJSON(`{"name": "Alice", "age": 30}`)
	if err != nil {
		t.Fatal(err)
	}
	input := tree.NewDJSON(j)
	results, err := reg.Call(tc, "json_name", []tree.Datums{{input}})
	if err != nil {
		t.Fatalf("call: %v", err)
	}

	expected := tree.NewDString("Alice")
	if results[0].Compare(nil, expected) != 0 {
		t.Fatalf("expected %s, got %s", expected, results[0])
	}
}

// TestTypeMapping_JSONReturn tests returning a JS object as JSONB.
func TestTypeMapping_JSONReturn(t *testing.T) {
	reg := NewRegistry()
	defer reg.Close()

	err := reg.CompileAndRegisterJS("make_json",
		`function invoke(name, age) { return {name: name, age: age}; }`,
		[]ValType{ValString, ValI64}, ValJSON, 0)
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	tc := reg.NewTxnContext(nil, context.Background(), nil, nil)
	defer tc.Close()

	results, err := reg.Call(tc, "make_json",
		[]tree.Datums{{tree.NewDString("Bob"), tree.NewDInt(25)}})
	if err != nil {
		t.Fatalf("call: %v", err)
	}

	dj, ok := results[0].(*tree.DJSON)
	if !ok {
		t.Fatalf("expected DJSON, got %T: %s", results[0], results[0])
	}
	// Verify the JSON contains the expected fields.
	s := dj.JSON.String()
	if s != `{"age": 25, "name": "Bob"}` && s != `{"name": "Bob", "age": 25}` {
		t.Fatalf("unexpected JSON: %s", s)
	}
}

// TestTypeMapping_JSONArray tests JSON array round-trip.
func TestTypeMapping_JSONArray(t *testing.T) {
	reg := NewRegistry()
	defer reg.Close()

	err := reg.CompileAndRegisterJS("json_first",
		`function invoke(arr) { return arr[0]; }`,
		[]ValType{ValJSON}, ValI64, 0)
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	tc := reg.NewTxnContext(nil, context.Background(), nil, nil)
	defer tc.Close()

	j, err := json.ParseJSON(`[42, 99]`)
	if err != nil {
		t.Fatal(err)
	}
	results, err := reg.Call(tc, "json_first", []tree.Datums{{tree.NewDJSON(j)}})
	if err != nil {
		t.Fatalf("call: %v", err)
	}

	expected := tree.NewDInt(42)
	if results[0].Compare(nil, expected) != 0 {
		t.Fatalf("expected %s, got %s", expected, results[0])
	}
}

// TestTypeMapping_NullHandling tests NULL pass-through: NULL inputs
// become JS null, and JS null/undefined returns become SQL NULL.
func TestTypeMapping_NullHandling(t *testing.T) {
	reg := NewRegistry()
	defer reg.Close()

	// NULL input → JS null → function returns 0
	err := reg.CompileAndRegisterJS("null_input",
		`function invoke(x) { return x === null ? 0 : 1; }`,
		[]ValType{ValString}, ValI64, 0)
	if err != nil {
		t.Fatalf("register null_input: %v", err)
	}

	tc := reg.NewTxnContext(nil, context.Background(), nil, nil)
	defer tc.Close()

	results, err := reg.Call(tc, "null_input", []tree.Datums{{tree.DNull}})
	if err != nil {
		t.Fatalf("null_input: %v", err)
	}
	expected := tree.NewDInt(0)
	if results[0].Compare(nil, expected) != 0 {
		t.Fatalf("null_input: expected %s, got %s", expected, results[0])
	}

	// Non-NULL input → returns 1
	results, err = reg.Call(tc, "null_input", []tree.Datums{{tree.NewDString("hello")}})
	if err != nil {
		t.Fatalf("non-null input: %v", err)
	}
	expected = tree.NewDInt(1)
	if results[0].Compare(nil, expected) != 0 {
		t.Fatalf("non-null input: expected %s, got %s", expected, results[0])
	}

	// NULL output: JS function returns null → SQL NULL
	err = reg.CompileAndRegisterJS("null_output",
		`function invoke(x) { return null; }`,
		[]ValType{ValI64}, ValI64, 0)
	if err != nil {
		t.Fatalf("register null_output: %v", err)
	}

	results, err = reg.Call(tc, "null_output", []tree.Datums{{tree.NewDInt(1)}})
	if err != nil {
		t.Fatalf("null_output: %v", err)
	}
	if results[0] != tree.DNull {
		t.Fatalf("null_output: expected DNull, got %s", results[0])
	}

	// NULL round-trip: NULL in → null in JS → null out → SQL NULL
	err = reg.CompileAndRegisterJS("null_passthrough",
		`function invoke(x) { return x; }`,
		[]ValType{ValString}, ValString, 0)
	if err != nil {
		t.Fatalf("register null_passthrough: %v", err)
	}

	results, err = reg.Call(tc, "null_passthrough", []tree.Datums{{tree.DNull}})
	if err != nil {
		t.Fatalf("null_passthrough: %v", err)
	}
	if results[0] != tree.DNull {
		t.Fatalf("null_passthrough: expected DNull, got %s", results[0])
	}
}

// TestTypeMapping_SQLTypeToValType tests that all plv8-compatible SQL types
// are recognized by SQLTypeToValType.
func TestTypeMapping_SQLTypeToValType(t *testing.T) {
	tests := []struct {
		typ     *types.T
		want    ValType
		wantErr bool
	}{
		{types.Bool, ValI32, false},
		{types.Int, ValI64, false},
		{types.Int2, ValI64, false},
		{types.Int4, ValI64, false},
		{types.Float, ValF64, false},
		{types.Float4, ValF64, false},
		{types.String, ValString, false},
		{types.Bytes, ValBytes, false},
		{types.Timestamp, ValTimestamp, false},
		{types.TimestampTZ, ValTimestamp, false},
		{types.Jsonb, ValJSON, false},
		// Unsupported types should error.
		{types.Uuid, 0, true},
		{types.INet, 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.typ.SQLString(), func(t *testing.T) {
			got, err := SQLTypeToValType(tc.typ)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for type %s", tc.typ.SQLString())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for type %s: %v", tc.typ.SQLString(), err)
			}
			if got != tc.want {
				t.Fatalf("type %s: expected ValType %d, got %d", tc.typ.SQLString(), tc.want, got)
			}
		})
	}
}
