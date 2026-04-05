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
	"testing"

	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
)

type mockExecutor struct {
	responses map[string]mockResponse
	queries   []capturedQuery
}

type mockResponse struct {
	rows []tree.Datums
	cols []ResultColumn
}

type capturedQuery struct {
	sql  string
	args []interface{}
}

func (m *mockExecutor) QueryBufferedEx(
	ctx context.Context, opName string, txn interface{}, override interface{},
	stmt string, qargs ...interface{},
) ([]tree.Datums, []ResultColumn, error) {
	m.queries = append(m.queries, capturedQuery{sql: stmt, args: qargs})
	for prefix, resp := range m.responses {
		if strings.HasPrefix(strings.TrimSpace(stmt), prefix) {
			return resp.rows, resp.cols, nil
		}
	}
	return nil, nil, fmt.Errorf("unexpected SQL: %s", stmt)
}

func TestSQLTaggedTemplate(t *testing.T) {
	reg := NewRegistry()
	defer reg.Close()

	mock := &mockExecutor{
		responses: map[string]mockResponse{
			"SELECT count(*)": {
				rows: []tree.Datums{{tree.NewDInt(42)}},
				cols: []ResultColumn{{Name: "n"}},
			},
		},
	}

	jsBody := "async function invoke(min_age) {\n  var rows = await sql`SELECT count(*) as n FROM users WHERE age > ${min_age}`;\n  return rows[0].n;\n}"

	err := reg.CompileAndRegisterJS("count_users", jsBody,
		[]ValType{ValI64}, ValI64, 0)
	if err != nil {
		t.Fatalf("CompileAndRegisterJS: %v", err)
	}

	tc := reg.NewTxnContext(mock, context.Background(), nil, nil)
	defer tc.Close()

	results, err := reg.Call(tc, "count_users", []tree.Datums{tree.Datums{tree.NewDInt(25)}})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	result := results[0]

	expected := tree.NewDInt(42)
	if result.Compare(nil, expected) != 0 {
		t.Fatalf("expected %s, got %s", expected, result)
	}

	if len(mock.queries) != 1 {
		t.Fatalf("expected 1 query, got %d", len(mock.queries))
	}
	q := mock.queries[0]
	if !strings.Contains(q.sql, "$1") {
		t.Fatalf("expected parameterized SQL with $1, got: %s", q.sql)
	}
	t.Logf("SQL: %s, args: %v", q.sql, q.args)
}

func TestSQLTaggedTemplateMultipleParams(t *testing.T) {
	reg := NewRegistry()
	defer reg.Close()

	mock := &mockExecutor{
		responses: map[string]mockResponse{
			"SELECT name": {
				rows: []tree.Datums{
					{tree.NewDString("Alice")},
					{tree.NewDString("Bob")},
				},
				cols: []ResultColumn{{Name: "name"}},
			},
		},
	}

	jsBody := "async function invoke(min_age, max_age) {\n  var rows = await sql`SELECT name FROM users WHERE age > ${min_age} AND age < ${max_age}`;\n  return rows.length;\n}"

	err := reg.CompileAndRegisterJS("range_users", jsBody,
		[]ValType{ValI64, ValI64}, ValI64, 0)
	if err != nil {
		t.Fatalf("CompileAndRegisterJS: %v", err)
	}

	tc := reg.NewTxnContext(mock, context.Background(), nil, nil)
	defer tc.Close()

	results, err := reg.Call(tc, "range_users", []tree.Datums{tree.Datums{tree.NewDInt(20), tree.NewDInt(30)}})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	result := results[0]

	expected := tree.NewDInt(2)
	if result.Compare(nil, expected) != 0 {
		t.Fatalf("expected %s, got %s", expected, result)
	}

	q := mock.queries[0]
	if !strings.Contains(q.sql, "$1") || !strings.Contains(q.sql, "$2") {
		t.Fatalf("expected $1 and $2 in SQL, got: %s", q.sql)
	}
	t.Logf("SQL: %s, args: %v", q.sql, q.args)
}

func TestSQLTaggedTemplateRowObjects(t *testing.T) {
	reg := NewRegistry()
	defer reg.Close()

	mock := &mockExecutor{
		responses: map[string]mockResponse{
			"SELECT name, age": {
				rows: []tree.Datums{
					{tree.NewDString("Alice"), tree.NewDInt(30)},
				},
				cols: []ResultColumn{{Name: "name"}, {Name: "age"}},
			},
		},
	}

	jsBody := "async function invoke() {\n  var rows = await sql`SELECT name, age FROM users LIMIT 1`;\n  if (rows.length === 0) return -1;\n  return rows[0].age;\n}"

	err := reg.CompileAndRegisterJS("get_age", jsBody,
		[]ValType{}, ValI64, 0)
	if err != nil {
		t.Fatalf("CompileAndRegisterJS: %v", err)
	}

	tc := reg.NewTxnContext(mock, context.Background(), nil, nil)
	defer tc.Close()

	results, err := reg.Call(tc, "get_age", []tree.Datums{tree.Datums{}})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	result := results[0]

	expected := tree.NewDInt(30)
	if result.Compare(nil, expected) != 0 {
		t.Fatalf("expected %s, got %s", expected, result)
	}
}

func TestSQLTaggedTemplateMultipleQueries(t *testing.T) {
	reg := NewRegistry()
	defer reg.Close()

	mock := &mockExecutor{
		responses: map[string]mockResponse{
			"SELECT manager_id": {
				rows: []tree.Datums{{tree.NewDInt(7)}},
				cols: []ResultColumn{{Name: "manager_id"}},
			},
			"SELECT count(*)": {
				rows: []tree.Datums{{tree.NewDInt(15)}},
				cols: []ResultColumn{{Name: "n"}},
			},
		},
	}

	jsBody := "async function invoke(dept) {\n  var mgr = await sql`SELECT manager_id FROM departments WHERE name = ${dept}`;\n  if (mgr.length === 0) return -1;\n  var reports = await sql`SELECT count(*) as n FROM employees WHERE manager_id = ${mgr[0].manager_id}`;\n  return reports[0].n;\n}"

	err := reg.CompileAndRegisterJS("count_reports", jsBody,
		[]ValType{ValI64}, ValI64, 0)
	if err != nil {
		t.Fatalf("CompileAndRegisterJS: %v", err)
	}

	tc := reg.NewTxnContext(mock, context.Background(), nil, nil)
	defer tc.Close()

	results, err := reg.Call(tc, "count_reports", []tree.Datums{tree.Datums{tree.NewDInt(1)}})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	result := results[0]

	expected := tree.NewDInt(15)
	if result.Compare(nil, expected) != 0 {
		t.Fatalf("expected %s, got %s", expected, result)
	}

	if len(mock.queries) != 2 {
		t.Fatalf("expected 2 queries, got %d", len(mock.queries))
	}
	t.Logf("Query 1: %s, args: %v", mock.queries[0].sql, mock.queries[0].args)
	t.Logf("Query 2: %s, args: %v", mock.queries[1].sql, mock.queries[1].args)
}
