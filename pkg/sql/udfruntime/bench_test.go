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
	"testing"

	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
)

// --- Single-row (batch of 1) via MakeFn compatibility path ---

func BenchmarkMakeFnJS(b *testing.B) {
	reg := NewRegistry()
	defer reg.Close()

	err := reg.CompileAndRegisterJS("add", "function invoke(a, b) { return a + b; }",
		[]ValType{ValI64, ValI64}, ValI64, 0)
	if err != nil {
		b.Fatal(err)
	}
	fn, err := reg.MakeFn("add")
	if err != nil {
		b.Fatal(err)
	}

	args := tree.Datums{tree.NewDInt(3), tree.NewDInt(4)}

	b.ResetTimer()
	for range b.N {
		if _, err := fn(nil, args); err != nil {
			b.Fatal(err)
		}
	}
}

// --- Batch calls ---

func BenchmarkBatchJS1(b *testing.B) {
	reg := NewRegistry()
	defer reg.Close()

	err := reg.CompileAndRegisterJS("add", "function invoke(a, b) { return a + b; }",
		[]ValType{ValI64, ValI64}, ValI64, 0)
	if err != nil {
		b.Fatal(err)
	}

	tc := reg.NewTxnContext(nil, context.Background(), nil, nil)
	defer tc.Close()

	batch := []tree.Datums{{tree.NewDInt(3), tree.NewDInt(4)}}

	b.ResetTimer()
	for range b.N {
		if _, err := reg.Call(tc, "add", batch); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBatchWasm1(b *testing.B) {
	reg := NewRegistry()
	defer reg.Close()

	wat := `(module
		(func (export "invoke") (param i64 i64) (result i64)
			local.get 0 local.get 1 i64.add))`
	wasmBytes, err := Wat2Wasm(wat)
	if err != nil {
		b.Fatal(err)
	}
	err = reg.CompileAndRegisterWasm("add", wasmBytes, "invoke",
		[]ValType{ValI64, ValI64}, ValI64, 0)
	if err != nil {
		b.Fatal(err)
	}

	tc := reg.NewTxnContext(nil, context.Background(), nil, nil)
	defer tc.Close()

	batch := []tree.Datums{{tree.NewDInt(3), tree.NewDInt(4)}}

	b.ResetTimer()
	for range b.N {
		if _, err := reg.Call(tc, "add", batch); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBatchJS1000(b *testing.B) {
	reg := NewRegistry()
	defer reg.Close()

	err := reg.CompileAndRegisterJS("double", "function invoke(x) { return x * 2; }",
		[]ValType{ValI64}, ValI64, 0)
	if err != nil {
		b.Fatal(err)
	}

	tc := reg.NewTxnContext(nil, context.Background(), nil, nil)
	defer tc.Close()

	batch := make([]tree.Datums, 1000)
	for i := range batch {
		batch[i] = tree.Datums{tree.NewDInt(tree.DInt(i))}
	}

	b.ResetTimer()
	for range b.N {
		if _, err := reg.Call(tc, "double", batch); err != nil {
			b.Fatal(err)
		}
	}
}

// --- Batch with async sql`` ---

func BenchmarkBatchAsyncSQL1(b *testing.B) {
	reg := NewRegistry()
	defer reg.Close()

	exec := &nopExecutor{
		rows: []tree.Datums{{tree.NewDInt(42)}},
		cols: []ResultColumn{{Name: "n"}},
	}

	err := reg.CompileAndRegisterJS("count",
		"async function invoke(x) { var r = await sql`SELECT count(*) as n FROM t WHERE a > ${x}`; return r[0].n; }",
		[]ValType{ValI64}, ValI64, 0)
	if err != nil {
		b.Fatal(err)
	}

	tc := reg.NewTxnContext(exec, context.Background(), nil, nil)
	defer tc.Close()

	batch := []tree.Datums{{tree.NewDInt(10)}}

	b.ResetTimer()
	for range b.N {
		if _, err := reg.Call(tc, "count", batch); err != nil {
			b.Fatal(err)
		}
	}
}

// --- Marshaling benchmarks ---

func BenchmarkMarshalDatumToJS(b *testing.B) {
	args := tree.Datums{
		tree.NewDString("Alice"),
		tree.NewDFloat(11.49),
		tree.NewDInt(2),
		tree.NewDString(`["electronics","sale"]`),
	}
	types := []ValType{ValString, ValF64, ValI64, ValString}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j, arg := range args {
			s, err := MarshalDatumToJS(arg, types[j], false)
			if err != nil {
				b.Fatal(err)
			}
			_ = s
		}
	}
}

func BenchmarkWriteDatumJSON(b *testing.B) {
	args := tree.Datums{
		tree.NewDString("Alice"),
		tree.NewDFloat(11.49),
		tree.NewDInt(2),
		tree.NewDString(`["electronics","sale"]`),
	}
	types := []ValType{ValString, ValF64, ValI64, ValString}
	var buf bytes.Buffer

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		for j, arg := range args {
			if err := WriteDatumJSON(&buf, arg, types[j]); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// nopExecutor returns a fixed result immediately.
type nopExecutor struct {
	rows []tree.Datums
	cols []ResultColumn
}

func (e *nopExecutor) QueryBufferedEx(
	ctx context.Context, opName string, txn interface{}, override interface{},
	stmt string, qargs ...interface{},
) ([]tree.Datums, []ResultColumn, error) {
	return e.rows, e.cols, nil
}
