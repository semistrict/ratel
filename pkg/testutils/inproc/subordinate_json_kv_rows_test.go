// Copyright 2026 The Ratel Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package inproc_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/desctestutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/inproc"
	"github.com/cockroachdb/cockroach/pkg/testutils/inproc/internal/planassert"
	"github.com/stretchr/testify/require"
)

func TestSubordinateJSONLargeValueScannedKeysBounded(t *testing.T) {
	t.Parallel()

	const deterministicDocBytes = 128 << 10

	ckv := &planassert.KVKeyCounter{}
	c := inproc.StartCluster(t, 1, func(args *base.TestClusterArgs) {
		storeKnobs := &kvserver.StoreTestingKnobs{
			TestingResponseFilter: ckv.ResponseFilter,
		}
		args.ServerArgs.Knobs.Store = storeKnobs
	})
	ctx := context.Background()
	db := c.ServerConn(0)
	defer c.Stop()
	_, err := db.ExecContext(ctx, `SET distsql = always`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `SET vectorize = on`)
	require.NoError(t, err)

	testCases := []struct {
		name           string
		doc            func() string
		rows           int
		stmt           func(table string) string
		maxScannedKeys int64
		verifyQuery    func(table string, doc string) string
		verifyScan     func(*sql.Row, string)
	}{
		{
			name:           "query root array index",
			doc:            func() string { return makeLargeSubordinateJSONBenchRootArrayIndexDoc(deterministicDocBytes) },
			rows:           1,
			stmt:           func(table string) string { return fmt.Sprintf(`SELECT j->10->>'test' FROM %s WHERE id = 1`, table) },
			maxScannedKeys: 24,
			verifyQuery: func(table string, _ string) string {
				return fmt.Sprintf(`SELECT j->10->>'test' FROM %s WHERE id = 1`, table)
			},
			verifyScan: func(row *sql.Row, _ string) {
				var got string
				require.NoError(t, row.Scan(&got))
				require.Equal(t, "v", got)
			},
		},
		{
			name:           "query root object key",
			doc:            func() string { return makeLargeSubordinateJSONBenchRootObjectKeyDoc(deterministicDocBytes) },
			rows:           1,
			stmt:           func(table string) string { return fmt.Sprintf(`SELECT j->>'test' FROM %s WHERE id = 1`, table) },
			maxScannedKeys: 16,
			verifyQuery: func(table string, _ string) string {
				return fmt.Sprintf(`SELECT j->>'test' FROM %s WHERE id = 1`, table)
			},
			verifyScan: func(row *sql.Row, _ string) {
				var got string
				require.NoError(t, row.Scan(&got))
				require.Equal(t, "v", got)
			},
		},
		{
			name: "append root array element",
			doc:  func() string { return makeLargeSubordinateJSONBenchRootArrayIndexDoc(deterministicDocBytes) },
			rows: 1,
			stmt: func(table string) string {
				return fmt.Sprintf(`UPDATE %s SET j = j || '[{"test":"appended"}]'::JSONB WHERE id = 1`, table)
			},
			maxScannedKeys: 8,
			verifyQuery: func(table string, doc string) string {
				return fmt.Sprintf(
					`SELECT jsonb_array_length(j), j->10->>'test', j->%d->>'test' FROM %s WHERE id = 1`,
					rootJSONArrayLen(t, doc), table,
				)
			},
			verifyScan: func(row *sql.Row, doc string) {
				var length int
				var before, appended sql.NullString
				require.NoError(t, row.Scan(&length, &before, &appended))
				require.Equal(t, rootJSONArrayLen(t, doc)+1, length)
				require.Equal(t, "v", before.String)
				require.Equal(t, "appended", appended.String)
			},
		},
		{
			name: "delete last root array element",
			doc:  func() string { return makeLargeSubordinateJSONBenchRootArrayIndexDoc(deterministicDocBytes) },
			rows: 1,
			stmt: func(table string) string {
				return fmt.Sprintf(`UPDATE %s SET j = j - (jsonb_array_length(j) - 1) WHERE id = 1`, table)
			},
			maxScannedKeys: 8,
			verifyQuery: func(table string, _ string) string {
				return fmt.Sprintf(`SELECT jsonb_array_length(j), j->10->>'test' FROM %s WHERE id = 1`, table)
			},
			verifyScan: func(row *sql.Row, doc string) {
				var length int
				var got string
				require.NoError(t, row.Scan(&length, &got))
				require.Equal(t, rootJSONArrayLen(t, doc)-1, length)
				require.Equal(t, "v", got)
			},
		},
		{
			name: "update root array element key",
			doc:  func() string { return makeLargeSubordinateJSONBenchRootArrayIndexDoc(deterministicDocBytes) },
			rows: 1,
			stmt: func(table string) string {
				return fmt.Sprintf(`UPDATE %s SET j = jsonb_set(j, '{10,test}', '"updated"'::JSONB, false) WHERE id = 1`, table)
			},
			maxScannedKeys: 8,
			verifyQuery: func(table string, _ string) string {
				return fmt.Sprintf(`SELECT j->10->>'test' FROM %s WHERE id = 1`, table)
			},
			verifyScan: func(row *sql.Row, _ string) {
				var got string
				require.NoError(t, row.Scan(&got))
				require.Equal(t, "updated", got)
			},
		},
		{
			name: "append root object key",
			doc:  func() string { return makeLargeSubordinateJSONBenchRootObjectKeyDoc(deterministicDocBytes) },
			rows: 1,
			stmt: func(table string) string {
				return fmt.Sprintf(`UPDATE %s SET j = j || '{"appended":"v"}'::JSONB WHERE id = 1`, table)
			},
			maxScannedKeys: 8,
			verifyQuery: func(table string, _ string) string {
				return fmt.Sprintf(`SELECT j->>'test', j->>'appended' FROM %s WHERE id = 1`, table)
			},
			verifyScan: func(row *sql.Row, _ string) {
				var original, appended sql.NullString
				require.NoError(t, row.Scan(&original, &appended))
				require.Equal(t, "v", original.String)
				require.Equal(t, "v", appended.String)
			},
		},
		{
			name: "delete root object key",
			doc:  func() string { return makeLargeSubordinateJSONBenchRootObjectKeyDoc(deterministicDocBytes) },
			rows: 1,
			stmt: func(table string) string {
				return fmt.Sprintf(`UPDATE %s SET j = j - 'tail_delete' WHERE id = 1`, table)
			},
			maxScannedKeys: 8,
			verifyQuery: func(table string, _ string) string {
				return fmt.Sprintf(`SELECT j->>'tail_delete' IS NULL FROM %s WHERE id = 1`, table)
			},
			verifyScan: func(row *sql.Row, _ string) {
				var got bool
				require.NoError(t, row.Scan(&got))
				require.True(t, got)
			},
		},
		{
			name: "update root object key",
			doc:  func() string { return makeLargeSubordinateJSONBenchRootObjectKeyDoc(deterministicDocBytes) },
			rows: 1,
			stmt: func(table string) string {
				return fmt.Sprintf(`UPDATE %s SET j = jsonb_set(j, '{test}', '"updated"'::JSONB, false) WHERE id = 1`, table)
			},
			maxScannedKeys: 8,
			verifyQuery: func(table string, _ string) string {
				return fmt.Sprintf(`SELECT j->>'test' FROM %s WHERE id = 1`, table)
			},
			verifyScan: func(row *sql.Row, _ string) {
				var got string
				require.NoError(t, row.Scan(&got))
				require.Equal(t, "updated", got)
			},
		},
		{
			name: "reverse path filter only",
			doc:  func() string { return makeLargeSubordinateJSONBenchNegativeIndexDoc(deterministicDocBytes) },
			rows: 2,
			stmt: func(table string) string {
				return fmt.Sprintf(`SELECT id FROM %s WHERE j->'needle'->-1->>'tiny' = 'v' ORDER BY id DESC LIMIT 1`, table)
			},
			maxScannedKeys: 12,
			verifyQuery: func(table string, _ string) string {
				return fmt.Sprintf(`SELECT id FROM %s WHERE j->'needle'->-1->>'tiny' = 'v' ORDER BY id DESC LIMIT 1`, table)
			},
			verifyScan: func(row *sql.Row, _ string) {
				var got int
				require.NoError(t, row.Scan(&got))
				require.Equal(t, 2, got)
			},
		},
		{
			name:           "exists filter only",
			doc:            func() string { return makeLargeSubordinateJSONBenchNegativeIndexDoc(deterministicDocBytes) },
			rows:           1,
			stmt:           func(table string) string { return fmt.Sprintf(`SELECT id FROM %s WHERE j ? 'needle'`, table) },
			maxScannedKeys: 8,
			verifyQuery:    func(table string, _ string) string { return fmt.Sprintf(`SELECT id FROM %s WHERE j ? 'needle'`, table) },
			verifyScan: func(row *sql.Row, _ string) {
				var got int
				require.NoError(t, row.Scan(&got))
				require.Equal(t, 1, got)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			table := fmt.Sprintf("subordinate_json_kv_rows_%s", sanitizeSQLIdentifier(t.Name()))
			doc := tc.doc()
			tablePrefix := setupSubordinateJSONKVRowsTable(t, ctx, c, db, table, doc, tc.rows)
			scannedKeys := ckv.Measure(roachpb.Span{Key: tablePrefix, EndKey: tablePrefix.PrefixEnd()}, func() {
				stmt := tc.stmt(table)
				if len(stmt) >= len("SELECT") && stmt[:len("SELECT")] == "SELECT" {
					rows, err := db.QueryContext(ctx, stmt)
					require.NoError(t, err)
					drainRows(t, rows)
					return
				}
				_, err := db.ExecContext(ctx, stmt)
				require.NoError(t, err)
			})
			planassert.UsesAtMostScannedKeys(t, scannedKeys, tc.maxScannedKeys)

			tc.verifyScan(db.QueryRowContext(ctx, tc.verifyQuery(table, doc)), doc)
		})
	}
}

func setupSubordinateJSONKVRowsTable(
	t testing.TB, ctx context.Context, c *inproc.Cluster, db *sql.DB, table string, doc string, rows int,
) roachpb.Key {
	t.Helper()

	_, err := db.ExecContext(ctx, `SET distsql = always`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `SET vectorize = on`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE %s (id INT PRIMARY KEY, j JSONB)`, table))
	require.NoError(t, err)
	for i := 1; i <= rows; i++ {
		_, err = db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s VALUES ($1, $2::JSONB)`, table), i, doc)
		require.NoError(t, err)
	}
	desc := desctestutils.TestingGetPublicTableDescriptor(c.Server(0).DB(), keys.SystemSQLCodec, "defaultdb", table)
	return keys.SystemSQLCodec.TablePrefix(uint32(desc.GetID()))
}

func sanitizeSQLIdentifier(s string) string {
	buf := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= 'a' && ch <= 'z':
			buf = append(buf, ch)
		case ch >= 'A' && ch <= 'Z':
			buf = append(buf, ch+('a'-'A'))
		case ch >= '0' && ch <= '9':
			buf = append(buf, ch)
		default:
			buf = append(buf, '_')
		}
	}
	return string(buf)
}

func rootJSONArrayLen(t testing.TB, doc string) int {
	t.Helper()
	var arr []any
	require.NoError(t, json.Unmarshal([]byte(doc), &arr))
	return len(arr)
}

func drainRows(t testing.TB, rows *sql.Rows) {
	t.Helper()
	defer rows.Close()

	cols, err := rows.Columns()
	require.NoError(t, err)
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		require.NoError(t, rows.Scan(ptrs...))
	}
	require.NoError(t, rows.Err())
}
