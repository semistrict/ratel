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

package sql

import (
	"context"
	gosql "database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/colinfo"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/desctestutils"
	"github.com/cockroachdb/cockroach/pkg/sql/clusterunique"
	"github.com/cockroachdb/cockroach/pkg/sql/parser"
	"github.com/cockroachdb/cockroach/pkg/sql/row"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondatapb"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

func TestPlanSubordinateJSONDirectUpdate(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	ctx := context.Background()
	s, sqlDB, kvDB := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)

	db := sqlDB
	execCfg := s.ExecutorConfig().(ExecutorConfig)

	require.NoError(t, execSQL(ctx, db, `CREATE DATABASE d`))

	testCases := []struct {
		name         string
		stmt         string
		expectedKind string
	}{
		{
			name:         "delete object key",
			stmt:         `UPDATE d.t SET j = j - 'tail_delete' WHERE id = 1`,
			expectedKind: "delete-key",
		},
		{
			name:         "update object key",
			stmt:         `UPDATE d.t SET j = jsonb_set(j, '{test}', '"updated"'::JSONB, false) WHERE id = 1`,
			expectedKind: "set-path",
		},
		{
			name:         "append object key",
			stmt:         `UPDATE d.t SET j = j || '{"appended":"v"}'::JSONB WHERE id = 1`,
			expectedKind: "concat",
		},
		{
			name:         "delete last array element",
			stmt:         `UPDATE d.t SET j = j - (jsonb_array_length(j) - 1) WHERE id = 1`,
			expectedKind: "delete-last-array-element",
		},
		{
			name:         "update array element key",
			stmt:         `UPDATE d.t SET j = jsonb_set(j, '{0,test}', '"updated"'::JSONB, false) WHERE id = 1`,
			expectedKind: "set-path",
		},
	}

	sessionModes := []struct {
		name          string
		distSQLMode   sessiondatapb.DistSQLExecMode
		vectorizeMode sessiondatapb.VectorizeExecMode
	}{
		{name: "default"},
		{
			name:          "distsql-always-vectorize-on",
			distSQLMode:   sessiondatapb.DistSQLAlways,
			vectorizeMode: sessiondatapb.VectorizeOn,
		},
	}

	for _, nullable := range []bool{false, true} {
		nullability := "not-null"
		colDef := "j JSONB NOT NULL"
		if nullable {
			nullability = "nullable"
			colDef = "j JSONB"
		}
		t.Run(nullability, func(t *testing.T) {
			require.NoError(t, execSQL(ctx, db, `DROP TABLE IF EXISTS d.t`))
			require.NoError(t, execSQL(ctx, db, `CREATE TABLE d.t (id INT PRIMARY KEY, `+colDef+`)`))
			require.NoError(t, execSQL(ctx, db, `INSERT INTO d.t VALUES (1, '{"test":"v","tail_delete":"gone","arr":[{"test":"v"}]}'::JSONB)`))

			desc := desctestutils.TestingGetPublicTableDescriptor(kvDB, keys.SystemSQLCodec, "d", "t")
			jsonCol := desc.PublicColumns()[1]
			require.Equal(t, "j", jsonCol.GetName())

			for _, mode := range sessionModes {
				mode := mode
				t.Run(mode.name, func(t *testing.T) {
					for _, tc := range testCases {
						t.Run(tc.name, func(t *testing.T) {
							p := makeInternalPlannerForStatement(
								t, ctx, kvDB, s.NodeID(), &execCfg, sessiondatapb.SessionData{}, mode.distSQLMode, mode.vectorizeMode, tc.stmt,
							)
							defer p.curPlan.close(ctx)

							main := p.curPlan.main.planNode
							rowCount, ok := main.(*rowCountNode)
							require.True(t, ok, "expected rowCountNode, got %T", main)

							upd, ok := rowCount.source.(*updateNode)
							require.True(t, ok, "expected updateNode, got %T", rowCount.source)
							r, ok := upd.source.(*renderNode)
							require.True(t, ok, "expected renderNode source, got %T", upd.source)
							scan, ok := r.source.plan.(*scanNode)
							require.True(t, ok, "expected scanNode source, got %T", r.source.plan)
							require.NotNil(t, upd.run.subordinateJSONMutation)
							require.Equal(t, jsonCol.GetID(), upd.run.subordinateJSONMutation.colID)
							require.Equal(t, tc.expectedKind, subordinateJSONMutationKindName(upd.run.subordinateJSONMutation.kind))

							require.NotContains(t, scan.colCfg.wantedColumns, tree.ColumnID(jsonCol.GetID()))
							require.NotContains(t, resultColumnNames(scan.resultColumns), jsonCol.GetName())
						})
					}
				})
			}
		})
	}
}

func TestExecuteSubordinateJSONDirectUpdateUsesLocalMutation(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	ctx := context.Background()
	s, sqlDB, kvDB := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)

	db := sqlDB
	require.NoError(t, execSQL(ctx, db, `SET distsql = always`))
	require.NoError(t, execSQL(ctx, db, `SET vectorize = on`))
	require.NoError(t, execSQL(ctx, db, `CREATE DATABASE d`))

	const targetBytes = 128 << 10
	testCases := []struct {
		name          string
		initialDoc    string
		stmt          string
		verifyQuery   string
		expected      []string
		expectedKind  row.SubordinateJSONMutationKind
		maxBatchBytes int64
	}{
		{
			name:          "append root array element",
			initialDoc:    makeLargeRootArrayUpdateTestDoc(targetBytes),
			stmt:          `UPDATE d.t SET j = j || '[{"test":"appended"}]'::JSONB WHERE id = 1`,
			verifyQuery:   `SELECT j->10->>'test', j->(jsonb_array_length(j) - 1)->>'test' FROM d.t WHERE id = 1`,
			expected:      []string{"v", "appended"},
			expectedKind:  row.SubordinateJSONMutationConcat,
			maxBatchBytes: 1 << 20,
		},
		{
			name:          "update root array element key",
			initialDoc:    makeLargeRootArrayUpdateTestDoc(targetBytes),
			stmt:          `UPDATE d.t SET j = jsonb_set(j, '{10,test}', '"updated"'::JSONB, false) WHERE id = 1`,
			verifyQuery:   `SELECT j->10->>'test' FROM d.t WHERE id = 1`,
			expected:      []string{"updated"},
			expectedKind:  row.SubordinateJSONMutationSetPath,
			maxBatchBytes: 1 << 20,
		},
		{
			name:       "delete last root array element",
			initialDoc: makeLargeRootArrayUpdateTestDoc(targetBytes),
			stmt:       `UPDATE d.t SET j = j - (jsonb_array_length(j) - 1) WHERE id = 1`,
			verifyQuery: fmt.Sprintf(
				`SELECT j->10->>'test', CAST(jsonb_array_length(j) AS STRING) FROM d.t WHERE id = 1`,
			),
			expected:      []string{"v", fmt.Sprintf("%d", rootArrayLenFromJSON(t, makeLargeRootArrayUpdateTestDoc(targetBytes))-1)},
			expectedKind:  row.SubordinateJSONMutationDeleteLastArrayElement,
			maxBatchBytes: 1 << 20,
		},
		{
			name:          "append root object key",
			initialDoc:    makeLargeRootObjectUpdateTestDoc(targetBytes),
			stmt:          `UPDATE d.t SET j = j || '{"appended":"v"}'::JSONB WHERE id = 1`,
			verifyQuery:   `SELECT j->>'test', j->>'appended' FROM d.t WHERE id = 1`,
			expected:      []string{"v", "v"},
			expectedKind:  row.SubordinateJSONMutationConcat,
			maxBatchBytes: 1 << 20,
		},
		{
			name:          "update root object key",
			initialDoc:    makeLargeRootObjectUpdateTestDoc(targetBytes),
			stmt:          `UPDATE d.t SET j = jsonb_set(j, '{test}', '"updated"'::JSONB, false) WHERE id = 1`,
			verifyQuery:   `SELECT j->>'test' FROM d.t WHERE id = 1`,
			expected:      []string{"updated"},
			expectedKind:  row.SubordinateJSONMutationSetPath,
			maxBatchBytes: 1 << 20,
		},
		{
			name:          "delete root object key",
			initialDoc:    makeLargeRootObjectUpdateTestDoc(targetBytes),
			stmt:          `UPDATE d.t SET j = j - 'tail_delete' WHERE id = 1`,
			verifyQuery:   `SELECT CASE WHEN j->>'tail_delete' IS NULL AND j->>'test' = 'v' THEN 'ok' ELSE 'bad' END FROM d.t WHERE id = 1`,
			expected:      []string{"ok"},
			expectedKind:  row.SubordinateJSONMutationDeleteKey,
			maxBatchBytes: 1 << 20,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, execSQL(ctx, db, `DROP TABLE IF EXISTS d.t`))
			require.NoError(t, execSQL(ctx, db, `CREATE TABLE d.t (id INT PRIMARY KEY, j JSONB)`))
			_, err := db.ExecContext(ctx, `INSERT INTO d.t VALUES (1, $1::JSONB)`, tc.initialDoc)
			require.NoError(t, err)
			desc := desctestutils.TestingGetPublicTableDescriptor(kvDB, keys.SystemSQLCodec, "d", "t")

			var results []row.TestingSubordinateJSONMutationResult
			prev := row.TestingSubordinateJSONMutationApplied
			row.TestingSubordinateJSONMutationApplied = func(r row.TestingSubordinateJSONMutationResult) {
				results = append(results, r)
			}
			defer func() {
				row.TestingSubordinateJSONMutationApplied = prev
			}()

			_, err = db.ExecContext(ctx, tc.stmt)
			require.NoError(t, err)
			var filtered []row.TestingSubordinateJSONMutationResult
			for _, r := range results {
				if r.TableID == desc.GetID() {
					filtered = append(filtered, r)
				}
			}
			require.Len(t, filtered, 1)
			require.Equal(t, tc.expectedKind, filtered[0].MutationKind)
			require.True(t, filtered[0].LocalApplied)
			require.False(t, filtered[0].FellBackToGenericRewrite)
			require.Less(t, filtered[0].ApproximateMutationBytes, tc.maxBatchBytes)

			rowVals := db.QueryRowContext(ctx, tc.verifyQuery)
			got := make([]gosql.NullString, len(tc.expected))
			dest := make([]any, len(got))
			for i := range got {
				dest[i] = &got[i]
			}
			require.NoError(t, rowVals.Scan(dest...))
			for i := range tc.expected {
				require.True(t, got[i].Valid)
				require.Equal(t, tc.expected[i], got[i].String)
			}
		})
	}
}

func makeInternalPlannerForStatement(
	t testing.TB,
	ctx context.Context,
	db *kv.DB,
	nodeID roachpb.NodeID,
	execCfg *ExecutorConfig,
	sessionData sessiondatapb.SessionData,
	distSQLMode sessiondatapb.DistSQLExecMode,
	vectorizeMode sessiondatapb.VectorizeExecMode,
	sql string,
) *planner {
	t.Helper()

	internalPlanner, cleanup := NewInternalPlanner(
		"test",
		kv.NewTxn(ctx, db, nodeID),
		username.RootUserName(),
		&MemoryMetrics{},
		execCfg,
		sessionData,
	)
	t.Cleanup(cleanup)

	p := internalPlanner.(*planner)
	p.SessionData().DistSQLMode = distSQLMode
	p.SessionData().VectorizeMode = vectorizeMode

	stmt, err := parser.ParseOne(sql)
	require.NoError(t, err)

	p.stmt = makeStatement(stmt, clusterunique.ID{})
	require.NoError(t, p.makeOptimizerPlan(ctx))
	return p
}

func execSQL(ctx context.Context, db *gosql.DB, stmt string) error {
	_, err := db.ExecContext(ctx, stmt)
	return err
}

func makeLargeRootArrayUpdateTestDoc(targetBytes int) string {
	var b strings.Builder
	b.Grow(targetBytes + targetBytes/8)
	b.WriteByte('[')
	chunk := strings.Repeat("x", 240)
	for i := 0; b.Len() < targetBytes; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		if i == 10 {
			b.WriteString(`{"test":"v"}`)
			continue
		}
		fmt.Fprintf(&b, `{"junk":"%s","i":%d}`, chunk, i)
	}
	b.WriteByte(']')
	return b.String()
}

func makeLargeRootObjectUpdateTestDoc(targetBytes int) string {
	var b strings.Builder
	b.Grow(targetBytes + targetBytes/8)
	b.WriteString(`{"test":"v","tail_delete":"gone"`)
	chunk := strings.Repeat("x", 240)
	for i := 0; b.Len() < targetBytes; i++ {
		fmt.Fprintf(&b, `,"k%06d":"%s"`, i, chunk)
	}
	b.WriteByte('}')
	return b.String()
}

func rootArrayLenFromJSON(t testing.TB, doc string) int {
	t.Helper()
	var arr []any
	require.NoError(t, json.Unmarshal([]byte(doc), &arr))
	return len(arr)
}

func subordinateJSONMutationKindName(kind row.SubordinateJSONMutationKind) string {
	switch kind {
	case row.SubordinateJSONMutationConcat:
		return "concat"
	case row.SubordinateJSONMutationDeleteKey:
		return "delete-key"
	case row.SubordinateJSONMutationDeleteLastArrayElement:
		return "delete-last-array-element"
	case row.SubordinateJSONMutationSetPath:
		return "set-path"
	default:
		return "unknown"
	}
}

func resultColumnNames(cols colinfo.ResultColumns) []string {
	names := make([]string, len(cols))
	for i := range cols {
		names[i] = cols[i].Name
	}
	return names
}

func catalogColumnNames(cols []catalog.Column) []string {
	names := make([]string, len(cols))
	for i := range cols {
		names[i] = cols[i].GetName()
	}
	return names
}
