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
	"fmt"
	"testing"

	"github.com/semistrict/ratel/pkg/sql/row"
	"github.com/semistrict/ratel/pkg/testutils/inproc"
	"github.com/semistrict/ratel/pkg/testutils/inproc/internal/planassert"
	"github.com/stretchr/testify/require"
)

type jsonAccessRow struct {
	id       int
	exists   sql.NullBool
	rootJSON sql.NullString
	rootText sql.NullString
	pathJSON sql.NullString
	pathText sql.NullString
}

func startSubordinateJSONCluster(
	t *testing.T, vectorizeMode string,
) (context.Context, *inproc.Cluster, *sql.DB) {
	t.Helper()

	c := inproc.StartCluster(t, 1)
	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `SET distsql = always`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `SET vectorize = `+vectorizeMode)
	require.NoError(t, err)

	return ctx, c, db
}

func createSubordinateJSONTable(t *testing.T, ctx context.Context, db *sql.DB, rows string) {
	t.Helper()

	_, err := db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, j JSONB)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO t VALUES `+rows)
	require.NoError(t, err)
}

func createSubordinateJSONVirtualTable(t *testing.T, ctx context.Context, db *sql.DB, rows string) {
	t.Helper()

	_, err := db.ExecContext(ctx, `
		CREATE TABLE t (
			id INT PRIMARY KEY,
			j JSONB,
			ja JSONB AS (j->'a') VIRTUAL,
			jt STRING AS (j#>>'{a,b,-1}') VIRTUAL
		)
	`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO t (id, j) VALUES `+rows)
	require.NoError(t, err)
}

func createSubordinateJSONJoinTables(t *testing.T, ctx context.Context, db *sql.DB, jsonRows string, ids string) {
	t.Helper()

	createSubordinateJSONTable(t, ctx, db, jsonRows)
	_, err := db.ExecContext(ctx, `CREATE TABLE u (id INT PRIMARY KEY)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO u VALUES `+ids)
	require.NoError(t, err)
}

func queryIDs(
	t *testing.T, ctx context.Context, db *sql.DB, query string, args ...any,
) []int {
	t.Helper()

	rows, err := db.QueryContext(ctx, query, args...)
	require.NoError(t, err)
	defer rows.Close()

	var got []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		got = append(got, id)
	}
	require.NoError(t, rows.Err())
	return got
}

func TestSubordinateJSONPointLookupDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `(1, '{"needle":{"tiny":"v"},"junk":{"a":"b","c":"d"}}')`)

	var got sql.NullString
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT j->'needle'->>'tiny' FROM t WHERE id = 1`,
	).Scan(&got))
	require.Equal(t, sql.NullString{String: "v", Valid: true}, got)

	var id int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT id FROM t WHERE id = 1 AND j->'needle'->>'tiny' = 'v'`,
	).Scan(&id))
	require.Equal(t, 1, id)

	plan := planassert.VecVerbose(t, ctx, db, `SELECT j->'needle'->>'tiny' FROM t WHERE id = 1`)
	planassert.UsesColBatchScan(t, plan)
}

func TestSubordinateJSONPointLookupFilterOnlyDefaultVectorizedPlan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `(1, '{"needle":{"tiny":"v"},"junk":{"a":"b","c":"d"}}')`)

	vecPlan := planassert.VecVerbose(t, ctx, db, `SELECT id FROM t WHERE id = 1 AND j->'needle'->>'tiny' = 'v'`)
	t.Logf("VEC PLAN:\n%s", vecPlan)
	planassert.UsesColBatchScan(t, vecPlan)

	distsqlPlan := planassert.DistSQLJSON(t, ctx, db, `SELECT id FROM t WHERE id = 1 AND j->'needle'->>'tiny' = 'v'`)
	t.Logf("DISTSQL JSON: %s", distsqlPlan)
	planassert.NotContains(t, distsqlPlan, `"Spans: /1/0-/1/1"`)
}

func TestSubordinateJSONDotAccessCaseSensitiveDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `(1, '{"Needle":{"Tiny":"v"},"needle":{"tiny":"w"}}')`)

	var upper sql.NullString
	var lower sql.NullString
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT j.Needle.Tiny, j.needle.tiny
		FROM t
		WHERE id = 1
	`).Scan(&upper, &lower))
	require.Equal(t, sql.NullString{String: `"v"`, Valid: true}, upper)
	require.Equal(t, sql.NullString{String: `"w"`, Valid: true}, lower)

	plan := planassert.VecVerbose(t, ctx, db, `
		SELECT id
		FROM t
		WHERE id = 1 AND j.Needle.Tiny = '"v"'::JSONB
	`)
	planassert.UsesColBatchScan(t, plan)

	var id int
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT id
		FROM t
		WHERE id = 1 AND j.Needle.Tiny = '"v"'::JSONB
	`).Scan(&id))
	require.Equal(t, 1, id)
}

func TestSubordinateJSONDotAccessLongPathDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `(1, '{"Needle":{"Layer":{"Deep":{"Tiny":"v"}}}}')`)

	var val sql.NullString
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT j.Needle.Layer.Deep.Tiny
		FROM t
		WHERE id = 1
	`).Scan(&val))
	require.Equal(t, sql.NullString{String: `"v"`, Valid: true}, val)

	plan := planassert.VecVerbose(t, ctx, db, `
		SELECT id
		FROM t
		WHERE id = 1 AND j.Needle.Layer.Deep.Tiny = '"v"'::JSONB
	`)
	planassert.UsesColBatchScan(t, plan)

	var id int
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT id
		FROM t
		WHERE id = 1 AND j.Needle.Layer.Deep.Tiny = '"v"'::JSONB
	`).Scan(&id))
	require.Equal(t, 1, id)
}

func TestSubordinateJSONAccessProjection(t *testing.T) {
	testSubordinateJSONAccessProjection(t, nil)
}

func TestSubordinateJSONAccessProjectionRowEngine(t *testing.T) {
	testSubordinateJSONAccessProjection(t, func(ctx context.Context, db *sql.DB) {
		_, err := db.ExecContext(ctx, `SET distsql = off`)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `SET vectorize = off`)
		require.NoError(t, err)
	})
}

func TestSubordinateJSONAccessProjectionDistSQL(t *testing.T) {
	testSubordinateJSONAccessProjection(t, func(ctx context.Context, db *sql.DB) {
		_, err := db.ExecContext(ctx, `SET distsql = always`)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `SET vectorize = on`)
		require.NoError(t, err)
	})
}

func testSubordinateJSONAccessProjection(
	t *testing.T, configure func(ctx context.Context, db *sql.DB),
) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)
	if configure != nil {
		configure(ctx, db)
	}

	_, err := db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, j JSONB)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO t VALUES
			(1, '{"a": {"b": [10, 20], "c": null}, "z": 7}'),
			(2, '{"a": {"b": [30]}, "z": 8}'),
			(3, '{"q": "r", "z": 9}'),
			(4, NULL),
			(5, 'null')
	`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `
		SELECT
			id,
			j ? 'a',
			j->'z',
			j->>'z',
			j#>'{a,b,1}',
			j#>>'{a,b,1}'
		FROM t
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	var got []jsonAccessRow
	for rows.Next() {
		var row jsonAccessRow
		require.NoError(t, rows.Scan(
			&row.id,
			&row.exists,
			&row.rootJSON,
			&row.rootText,
			&row.pathJSON,
			&row.pathText,
		))
		got = append(got, row)
	}
	require.NoError(t, rows.Err())

	require.Equal(t, []jsonAccessRow{
		{
			id:       1,
			exists:   sql.NullBool{Bool: true, Valid: true},
			rootJSON: sql.NullString{String: "7", Valid: true},
			rootText: sql.NullString{String: "7", Valid: true},
			pathJSON: sql.NullString{String: "20", Valid: true},
			pathText: sql.NullString{String: "20", Valid: true},
		},
		{
			id:       2,
			exists:   sql.NullBool{Bool: true, Valid: true},
			rootJSON: sql.NullString{String: "8", Valid: true},
			rootText: sql.NullString{String: "8", Valid: true},
		},
		{
			id:       3,
			exists:   sql.NullBool{Bool: false, Valid: true},
			rootJSON: sql.NullString{String: "9", Valid: true},
			rootText: sql.NullString{String: "9", Valid: true},
		},
		{
			id: 4,
		},
		{
			id:     5,
			exists: sql.NullBool{Bool: false, Valid: true},
		},
	}, got)
}

func TestSubordinateJSONExistsFilterOnlyDistSQL(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `SET distsql = always`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `SET vectorize = on`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, j JSONB)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO t VALUES
			(1, '{"a": {"b": [10, 20]}, "z": 7}'),
			(2, '{"a": {"b": [30]}, "z": 8}'),
			(3, '{"q": "r", "z": 9}'),
			(4, NULL),
			(5, 'null')
	`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `SELECT id FROM t WHERE j ? $1 ORDER BY id DESC`, "a")
	require.NoError(t, err)
	defer rows.Close()

	var got []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		got = append(got, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{2, 1}, got)
}

func TestSubordinateJSONExistsFilterOnlyUsesRowHeadLookupFetcher(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `(1, '{"needle":[{"tiny":"v"}],"junk":{"a":"b","c":"d"}}')`)
	plan := planassert.DistSQLJSON(t, ctx, db, `SELECT id FROM t WHERE j ? 'needle'`)
	planassert.NotContains(t, plan, `"Filterer/`)

	var seen []row.SubordinateJSONRowLookupSpec
	prev := row.TestingSubordinateJSONRowHeadFetcherCreated
	row.TestingSubordinateJSONRowHeadFetcherCreated = func(lookups []row.SubordinateJSONRowLookupSpec) {
		seen = append([]row.SubordinateJSONRowLookupSpec(nil), lookups...)
	}
	defer func() {
		row.TestingSubordinateJSONRowHeadFetcherCreated = prev
	}()

	var id int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT id FROM t WHERE j ? 'needle'`).Scan(&id))
	require.Equal(t, 1, id)
	require.Len(t, seen, 1)
	require.Empty(t, seen[0].SelectedPaths)
	require.Equal(t, []string{"needle"}, seen[0].ExistsKeys)
}

func TestSubordinateJSONColumnarExperimentalAlways(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7, "1": "obj"}'),
		(2, '{"a": {"b": [30]}, "z": 8, "1": "obj2"}'),
		(3, '{"a": {"b": []}, "z": 9, "1": "obj3"}'),
		(4, NULL)
	`)

	rows, err := db.QueryContext(ctx, `SELECT id FROM t WHERE j ? 'a' ORDER BY id DESC`)
	require.NoError(t, err)
	defer rows.Close()

	var gotExists []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		gotExists = append(gotExists, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{3, 2, 1}, gotExists)

	rows, err = db.QueryContext(ctx, `SELECT id FROM t WHERE j#>>'{a,b,-1}' < '25' ORDER BY id DESC`)
	require.NoError(t, err)
	defer rows.Close()

	var gotCompare []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		gotCompare = append(gotCompare, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{1}, gotCompare)

	rows, err = db.QueryContext(ctx, `SELECT id, j->>'z' FROM t ORDER BY id`)
	require.NoError(t, err)
	defer rows.Close()

	type rowResult struct {
		id int
		z  sql.NullString
	}
	var gotProjection []rowResult
	for rows.Next() {
		var row rowResult
		require.NoError(t, rows.Scan(&row.id, &row.z))
		gotProjection = append(gotProjection, row)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []rowResult{
		{id: 1, z: sql.NullString{String: "7", Valid: true}},
		{id: 2, z: sql.NullString{String: "8", Valid: true}},
		{id: 3, z: sql.NullString{String: "9", Valid: true}},
		{id: 4},
	}, gotProjection)
}

func TestSubordinateJSONColumnarExperimentalAlwaysAdvanced(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "1": "obj"}'),
		(2, '{"a": {"b": [20]}, "1": "obj2"}'),
		(3, '{"a": {"b": []}, "q": "r"}'),
		(4, NULL),
		(5, 'null')
	`)

	rows, err := db.QueryContext(ctx, `
		SELECT id
		FROM t
		WHERE j ?| ARRAY['missing', 'a']
		ORDER BY id DESC
	`)
	require.NoError(t, err)
	defer rows.Close()

	var gotAny []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		gotAny = append(gotAny, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{3, 2, 1}, gotAny)

	rows, err = db.QueryContext(ctx, `
		SELECT id
		FROM t
		WHERE j#>>'{a,b,-1}' = '20' AND id < 2
		ORDER BY id DESC
	`)
	require.NoError(t, err)
	defer rows.Close()

	var gotResidual []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		gotResidual = append(gotResidual, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{1}, gotResidual)

	rows, err = db.QueryContext(ctx, `
		SELECT id, j
		FROM t
		WHERE j#>>'{a,b,-1}' = '20'
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	type projectionRow struct {
		id int
		j  sql.NullString
	}
	var gotProjection []projectionRow
	for rows.Next() {
		var row projectionRow
		require.NoError(t, rows.Scan(&row.id, &row.j))
		gotProjection = append(gotProjection, row)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []projectionRow{
		{id: 1, j: sql.NullString{String: `{"1": "obj", "a": {"b": [10, 20]}}`, Valid: true}},
		{id: 2, j: sql.NullString{String: `{"1": "obj2", "a": {"b": [20]}}`, Valid: true}},
	}, gotProjection)

	rows, err = db.QueryContext(ctx, `
		SELECT id
		FROM t
		WHERE j#>>'{a,b,1}' IS NULL
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	var gotIsNull []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		gotIsNull = append(gotIsNull, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{2, 3, 4, 5}, gotIsNull)
}

func TestSubordinateJSONColumnarExplainUsesColBatchScan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()
	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [30]}, "z": 8}'),
		(3, NULL)
	`)

	plan := planassert.VecVerbose(t, ctx, db, `SELECT id FROM t WHERE j#>>'{a,b,-1}' = '20'`)
	planassert.UsesColBatchScan(t, plan)
}

func TestSubordinateJSONColumnarExplainExistsSetUsesColBatchScan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [30]}, "z": 8}'),
		(3, '{"q": "r"}'),
		(4, NULL)
	`)

	plan := planassert.VecVerbose(t, ctx, db, `SELECT id FROM t WHERE j ?| ARRAY['missing', 'a']`)
	planassert.UsesColBatchScan(t, plan)
}

func TestSubordinateJSONColumnarExplainProjectionUsesColBatchScan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [30]}, "z": 8}'),
		(3, NULL)
	`)

	plan := planassert.VecVerbose(t, ctx, db, `SELECT j->>'z' FROM t ORDER BY id`)
	planassert.UsesColBatchScan(t, plan)
}

func TestSubordinateJSONContainmentDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [30]}, "z": 8}'),
		(3, '{"q": "r"}'),
		(4, NULL),
		(5, 'null')
	`)

	rows, err := db.QueryContext(ctx, `
		SELECT id
		FROM t
		WHERE j @> '{"a": {"b": [10, 20]}}'
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	var gotContains []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		gotContains = append(gotContains, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{1}, gotContains)

	rows, err = db.QueryContext(ctx, `
		SELECT id
		FROM t
		WHERE j->'a' @> '{"b": [30]}'
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	var gotFetchContains []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		gotFetchContains = append(gotFetchContains, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{2}, gotFetchContains)

	rows, err = db.QueryContext(ctx, `
		SELECT id
		FROM t
		WHERE j <@ '{"a": {"b": [10, 20]}, "z": 7, "extra": 1}'
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	var gotContainedBy []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		gotContainedBy = append(gotContainedBy, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{1}, gotContainedBy)
}

func TestSubordinateJSONContainmentWithResidualFilterDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [30]}, "z": 8}'),
		(3, '{"a": {"b": [30]}, "z": 9}')
	`)

	got := queryIDs(t, ctx, db, `
		SELECT id
		FROM t
		WHERE j->'a' @> '{"b": [30]}' AND id < 3
		ORDER BY id DESC
	`)
	require.Equal(t, []int{2}, got)
}

func TestSubordinateJSONMultipleContainmentFiltersDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [30]}, "z": 8}'),
		(3, '{"a": {"b": [30]}, "z": 9}'),
		(4, '{"a": {"b": [20], "c": null}, "z": 7}')
	`)

	got := queryIDs(t, ctx, db, `
		SELECT id
		FROM t
		WHERE j @> '{"z": 8}'
		  AND j->'a' @> '{"b": [30]}'
		ORDER BY id
	`)
	require.Equal(t, []int{2}, got)

	got = queryIDs(t, ctx, db, `
		SELECT id
		FROM t
		WHERE j->'a' @> '{"b": [20]}'
		  AND j->'a' <@ '{"b": [20], "c": null}'
		ORDER BY id
	`)
	require.Equal(t, []int{4}, got)
}

func TestSubordinateJSONContainmentProjectionDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [30]}, "z": 8}'),
		(3, '{"q": "r"}')
	`)

	rows, err := db.QueryContext(ctx, `
		SELECT id, j
		FROM t
		WHERE j @> '{"z": 8}'
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	type rowResult struct {
		id int
		j  sql.NullString
	}
	var got []rowResult
	for rows.Next() {
		var row rowResult
		require.NoError(t, rows.Scan(&row.id, &row.j))
		got = append(got, row)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []rowResult{{
		id: 2,
		j:  sql.NullString{String: `{"a": {"b": [30]}, "z": 8}`, Valid: true},
	}}, got)
}

func TestSubordinateJSONContainmentResidualProjectionDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [20]}, "z": 8}'),
		(3, '{"a": {"b": [20]}, "q": "r"}'),
		(4, '{"a": {"b": [30]}, "z": 9}'),
		(5, NULL)
	`)

	type rowResult struct {
		j sql.NullString
	}
	rows, err := db.QueryContext(ctx, `
		SELECT j
		FROM t
		WHERE j @> '{"a": {"b": [20]}}'
		  AND id > 1
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	var got []rowResult
	for rows.Next() {
		var row rowResult
		require.NoError(t, rows.Scan(&row.j))
		got = append(got, row)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []rowResult{
		{j: sql.NullString{String: `{"a": {"b": [20]}, "z": 8}`, Valid: true}},
		{j: sql.NullString{String: `{"a": {"b": [20]}, "q": "r"}`, Valid: true}},
	}, got)
}

func TestSubordinateJSONSamePathContainmentProjectionDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [30]}, "z": 8}'),
		(3, '{"q": "r"}')
	`)

	rows, err := db.QueryContext(ctx, `
		SELECT id, j#>'{a}'
		FROM t
		WHERE j#>'{a}' @> '{"b": [30]}'
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	type rowResult struct {
		id int
		j  sql.NullString
	}
	var got []rowResult
	for rows.Next() {
		var row rowResult
		require.NoError(t, rows.Scan(&row.id, &row.j))
		got = append(got, row)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []rowResult{{
		id: 2,
		j:  sql.NullString{String: `{"b": [30]}`, Valid: true},
	}}, got)
}

func TestSubordinateJSONSamePathContainmentTextAccessDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}}'),
		(2, '{"a": {"b": [30]}}'),
		(3, '{"q": "r"}')
	`)

	rows, err := db.QueryContext(ctx, `
		SELECT id, j#>>'{a,b,0}'
		FROM t
		WHERE j#>'{a}' @> '{"b": [30]}'
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	type rowResult struct {
		id int
		v  sql.NullString
	}
	var got []rowResult
	for rows.Next() {
		var row rowResult
		require.NoError(t, rows.Scan(&row.id, &row.v))
		got = append(got, row)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []rowResult{{
		id: 2,
		v:  sql.NullString{String: `30`, Valid: true},
	}}, got)
}

func TestSubordinateJSONSamePathContainmentCompareAndProjectionDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}}'),
		(2, '{"a": {"b": [30]}}'),
		(3, '{"a": {"b": [40]}}')
	`)

	rows, err := db.QueryContext(ctx, `
		SELECT id, j#>>'{a,b,0}'
		FROM t
		WHERE j#>'{a}' @> '{"b": [30]}'
		  AND j#>>'{a,b,0}' = '30'
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	type rowResult struct {
		id int
		v  sql.NullString
	}
	var got []rowResult
	for rows.Next() {
		var row rowResult
		require.NoError(t, rows.Scan(&row.id, &row.v))
		got = append(got, row)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []rowResult{{
		id: 2,
		v:  sql.NullString{String: `30`, Valid: true},
	}}, got)
}

func TestSubordinateJSONContainedByProjectionDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [30]}, "z": 8}'),
		(3, '{"q": "r"}')
	`)

	rows, err := db.QueryContext(ctx, `
		SELECT id, j
		FROM t
		WHERE j <@ '{"a": {"b": [30]}, "z": 8, "extra": 1}'
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	type rowResult struct {
		id int
		j  sql.NullString
	}
	var got []rowResult
	for rows.Next() {
		var row rowResult
		require.NoError(t, rows.Scan(&row.id, &row.j))
		got = append(got, row)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []rowResult{{
		id: 2,
		j:  sql.NullString{String: `{"a": {"b": [30]}, "z": 8}`, Valid: true},
	}}, got)
}

func TestSubordinateJSONContainmentArrayOfObjectsDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": [{"x": 1, "extra": 9}, {"y": 2}], "z": 7}'),
		(2, '{"a": [{"x": 1}], "z": 8}'),
		(3, '{"a": [{"y": 2}], "z": 9}')
	`)

	got := queryIDs(t, ctx, db, `
		SELECT id
		FROM t
		WHERE j->'a' @> '[{"x": 1}, {"y": 2}]'
		ORDER BY id
	`)
	require.Equal(t, []int{1}, got)

	got = queryIDs(t, ctx, db, `
		SELECT id
		FROM t
		WHERE j->'a' <@ '[{"x": 1, "extra": 9}, {"y": 2, "extra": true}]'
		ORDER BY id
	`)
	require.Equal(t, []int{1, 2, 3}, got)
}

func TestSubordinateJSONContainmentDuplicateArrayScalarsDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": [1, 1, 2]}'),
		(2, '{"a": [1, 2]}'),
		(3, '{"a": [1, 1]}')
	`)

	got := queryIDs(t, ctx, db, `
		SELECT id
		FROM t
		WHERE j->'a' @> '[1, 1]'
		ORDER BY id
	`)
	require.Equal(t, []int{1, 2, 3}, got)

	got = queryIDs(t, ctx, db, `
		SELECT id
		FROM t
		WHERE j->'a' <@ '[1, 1, 2]'
		ORDER BY id
	`)
	require.Equal(t, []int{1, 2, 3}, got)
}

func TestSubordinateJSONContainmentNullAndEmptyDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{}'),
		(2, '[]'),
		(3, 'null'),
		(4, NULL),
		(5, '{"a": null}'),
		(6, '{"a": []}'),
		(7, '{"a": {}}')
	`)

	got := queryIDs(t, ctx, db, `
		SELECT id
		FROM t
		WHERE j @> '{}'
		ORDER BY id
	`)
	require.Equal(t, []int{1, 5, 6, 7}, got)

	got = queryIDs(t, ctx, db, `
		SELECT id
		FROM t
		WHERE j @> 'null'
		ORDER BY id
	`)
	require.Equal(t, []int{3}, got)

	got = queryIDs(t, ctx, db, `
		SELECT id
		FROM t
		WHERE j->'a' <@ '{}'
		ORDER BY id
	`)
	require.Equal(t, []int{7}, got)

	got = queryIDs(t, ctx, db, `
		SELECT id
		FROM t
		WHERE j->'a' <@ '[]'
		ORDER BY id
	`)
	require.Equal(t, []int{6}, got)

	got = queryIDs(t, ctx, db, `
		SELECT id
		FROM t
		WHERE j->'a' <@ 'null'
		ORDER BY id
	`)
	require.Equal(t, []int{5}, got)
}

func TestSubordinateJSONContainmentNullAndEmptyProjectionDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{}'),
		(2, '[]'),
		(3, 'null'),
		(4, NULL),
		(5, '{"a": null}'),
		(6, '{"a": []}'),
		(7, '{"a": {}}')
	`)

	rows, err := db.QueryContext(ctx, `
		SELECT id, j->'a'
		FROM t
		WHERE j->'a' <@ 'null'
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	type rowResult struct {
		id int
		j  sql.NullString
	}
	var got []rowResult
	for rows.Next() {
		var row rowResult
		require.NoError(t, rows.Scan(&row.id, &row.j))
		got = append(got, row)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []rowResult{
		{id: 5, j: sql.NullString{String: `null`, Valid: true}},
	}, got)

	rows, err = db.QueryContext(ctx, `
		SELECT id, j->'a'
		FROM t
		WHERE j->'a' <@ '[]'
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	got = got[:0]
	for rows.Next() {
		var row rowResult
		require.NoError(t, rows.Scan(&row.id, &row.j))
		got = append(got, row)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []rowResult{
		{id: 6, j: sql.NullString{String: `[]`, Valid: true}},
	}, got)

	rows, err = db.QueryContext(ctx, `
		SELECT id, j->'a'
		FROM t
		WHERE j->'a' <@ '{}'
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	got = got[:0]
	for rows.Next() {
		var row rowResult
		require.NoError(t, rows.Scan(&row.id, &row.j))
		got = append(got, row)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []rowResult{
		{id: 7, j: sql.NullString{String: `{}`, Valid: true}},
	}, got)
}

func TestSubordinateJSONCombinedProgramsDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [20]}, "z": null}'),
		(3, '{"a": {"b": [20]}, "q": "r"}'),
		(4, '{"a": {"b": [30]}, "z": 9}'),
		(5, NULL)
	`)

	rows, err := db.QueryContext(ctx, `
		SELECT id, j#>>'{a,b,-1}'
		FROM t
		WHERE j ? 'a'
		  AND j->'a' @> '{"b": [20]}'
		  AND j->>'z' IS NOT NULL
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	type rowResult struct {
		id int
		v  sql.NullString
	}
	var got []rowResult
	for rows.Next() {
		var row rowResult
		require.NoError(t, rows.Scan(&row.id, &row.v))
		got = append(got, row)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []rowResult{
		{id: 1, v: sql.NullString{String: `20`, Valid: true}},
	}, got)
}

func TestSubordinateJSONCombinedProgramsResidualFilterDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [20]}, "z": 8}'),
		(3, '{"a": {"b": [20]}, "q": "r"}'),
		(4, '{"a": {"b": [30]}, "z": 9}'),
		(5, NULL)
	`)

	rows, err := db.QueryContext(ctx, `
		SELECT j#>>'{a,b,-1}'
		FROM t
		WHERE j ? 'a'
		  AND j->'a' @> '{"b": [20]}'
		  AND id > 1
		ORDER BY 1
	`)
	require.NoError(t, err)
	defer rows.Close()

	var got []sql.NullString
	for rows.Next() {
		var v sql.NullString
		require.NoError(t, rows.Scan(&v))
		got = append(got, v)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []sql.NullString{
		{String: `20`, Valid: true},
		{String: `20`, Valid: true},
	}, got)
}

func TestSubordinateJSONContainmentColumnarExplainUsesColBatchScan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [30]}, "z": 8}'),
		(3, NULL)
	`)

	plan := planassert.VecVerbose(t, ctx, db, `SELECT id FROM t WHERE j @> '{"a": {"b": [10, 20]}}'`)
	planassert.UsesColBatchScan(t, plan)

	plan = planassert.VecVerbose(t, ctx, db, `SELECT id FROM t WHERE j->'a' @> '{"b": [30]}'`)
	planassert.UsesColBatchScan(t, plan)

	plan = planassert.VecVerbose(t, ctx, db, `SELECT id FROM t WHERE j <@ '{"a": {"b": [10, 20]}, "z": 7, "extra": 1}'`)
	planassert.UsesColBatchScan(t, plan)

	plan = planassert.VecVerbose(t, ctx, db, `SELECT id FROM t WHERE j->'a' @> '[{"x": 1}, {"y": 2}]'`)
	planassert.UsesColBatchScan(t, plan)

	plan = planassert.VecVerbose(t, ctx, db, `SELECT id FROM t WHERE j @> 'null'`)
	planassert.UsesColBatchScan(t, plan)

	plan = planassert.VecVerbose(t, ctx, db, `SELECT id FROM t WHERE j->'a' <@ '[]'`)
	planassert.UsesColBatchScan(t, plan)

	plan = planassert.VecVerbose(t, ctx, db, `SELECT id FROM t WHERE j->'a' <@ '{}'`)
	planassert.UsesColBatchScan(t, plan)
}

func TestSubordinateJSONCombinedProgramsColumnarExplainUsesColBatchScan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [20]}, "z": null}'),
		(3, '{"a": {"b": [20]}, "q": "r"}'),
		(4, '{"a": {"b": [30]}, "z": 9}')
	`)

	plan := planassert.VecVerbose(t, ctx, db, `
		SELECT id, j#>>'{a,b,-1}'
		FROM t
		WHERE j ? 'a'
		  AND j->'a' @> '{"b": [20]}'
		  AND j->>'z' IS NOT NULL
		ORDER BY id
	`)
	planassert.UsesColBatchScan(t, plan)
}

func TestSubordinateJSONCombinedProgramsResidualFilterColumnarExplainUsesColBatchScan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [20]}, "z": 8}'),
		(3, '{"a": {"b": [20]}, "q": "r"}'),
		(4, '{"a": {"b": [30]}, "z": 9}')
	`)

	plan := planassert.VecVerbose(t, ctx, db, `
		SELECT j#>>'{a,b,-1}'
		FROM t
		WHERE j ? 'a'
		  AND j->'a' @> '{"b": [20]}'
		  AND id > 1
		ORDER BY 1
	`)
	planassert.UsesColBatchScan(t, plan)
}

func TestSubordinateJSONSamePathMixedKindsDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}}'),
		(2, '{"a": {"b": [30]}}'),
		(3, '{"q": "r"}'),
		(4, NULL)
	`)

	rows, err := db.QueryContext(ctx, `
		SELECT id, j#>'{a,b,-1}'
		FROM t
		WHERE j#>>'{a,b,-1}' = '20'
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	type rowResult struct {
		id int
		j  sql.NullString
	}
	var got []rowResult
	for rows.Next() {
		var row rowResult
		require.NoError(t, rows.Scan(&row.id, &row.j))
		got = append(got, row)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []rowResult{
		{id: 1, j: sql.NullString{String: `20`, Valid: true}},
	}, got)
}

func TestSubordinateJSONSamePathMixedKindsColumnarExplainUsesColBatchScan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}}'),
		(2, '{"a": {"b": [30]}}'),
		(3, '{"q": "r"}')
	`)

	plan := planassert.VecVerbose(t, ctx, db, `
		SELECT id, j#>'{a,b,-1}'
		FROM t
		WHERE j#>>'{a,b,-1}' = '20'
		ORDER BY id
	`)
	planassert.UsesColBatchScan(t, plan)
}

func TestSubordinateJSONSamePathJSONAndTextProjectionDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}}'),
		(2, '{"a": {"b": [30]}}'),
		(3, '{"q": "r"}'),
		(4, NULL)
	`)

	rows, err := db.QueryContext(ctx, `
		SELECT id, j#>'{a,b,-1}', j#>>'{a,b,-1}'
		FROM t
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	type rowResult struct {
		id       int
		pathJSON sql.NullString
		pathText sql.NullString
	}
	var got []rowResult
	for rows.Next() {
		var row rowResult
		require.NoError(t, rows.Scan(&row.id, &row.pathJSON, &row.pathText))
		got = append(got, row)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []rowResult{
		{id: 1, pathJSON: sql.NullString{String: `20`, Valid: true}, pathText: sql.NullString{String: `20`, Valid: true}},
		{id: 2, pathJSON: sql.NullString{String: `30`, Valid: true}, pathText: sql.NullString{String: `30`, Valid: true}},
		{id: 3},
		{id: 4},
	}, got)
}

func TestSubordinateJSONSamePathJSONAndTextProjectionColumnarExplainUsesColBatchScan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}}'),
		(2, '{"a": {"b": [30]}}'),
		(3, '{"q": "r"}')
	`)

	plan := planassert.VecVerbose(t, ctx, db, `
		SELECT id, j#>'{a,b,-1}', j#>>'{a,b,-1}'
		FROM t
		ORDER BY id
	`)
	planassert.UsesColBatchScan(t, plan)
}

func TestSubordinateJSONSamePathJSONAndTextProjectionResidualFilterDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}}'),
		(2, '{"a": {"b": [30]}}'),
		(3, '{"a": {"b": [40, 50]}}'),
		(4, NULL)
	`)

	rows, err := db.QueryContext(ctx, `
		SELECT id, j#>'{a,b,-1}', j#>>'{a,b,-1}'
		FROM t
		WHERE id > 1
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	type rowResult struct {
		id       int
		pathJSON sql.NullString
		pathText sql.NullString
	}
	var got []rowResult
	for rows.Next() {
		var row rowResult
		require.NoError(t, rows.Scan(&row.id, &row.pathJSON, &row.pathText))
		got = append(got, row)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []rowResult{
		{id: 2, pathJSON: sql.NullString{String: `30`, Valid: true}, pathText: sql.NullString{String: `30`, Valid: true}},
		{id: 3, pathJSON: sql.NullString{String: `50`, Valid: true}, pathText: sql.NullString{String: `50`, Valid: true}},
		{id: 4},
	}, got)
}

func TestSubordinateJSONSamePathJSONAndTextProjectionResidualFilterColumnarExplainUsesColBatchScan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}}'),
		(2, '{"a": {"b": [30]}}'),
		(3, '{"a": {"b": [40, 50]}}')
	`)

	plan := planassert.VecVerbose(t, ctx, db, `
		SELECT id, j#>'{a,b,-1}', j#>>'{a,b,-1}'
		FROM t
		WHERE id > 1
		ORDER BY id
	`)
	planassert.UsesColBatchScan(t, plan)
}

func TestSubordinateJSONCastWrappedProgramsDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [30]}, "z": 8}'),
		(3, '{"q": "r"}'),
		(4, NULL)
	`)

	got := queryIDs(t, ctx, db, `
		SELECT id
		FROM t
		WHERE (j->'a')::JSONB @> '{"b": [30]}'
		ORDER BY id
	`)
	require.Equal(t, []int{2}, got)

	got = queryIDs(t, ctx, db, `
		SELECT id
		FROM t
		WHERE (j#>>'{a,b,-1}')::STRING = '20'
		ORDER BY id
	`)
	require.Equal(t, []int{1}, got)

	got = queryIDs(t, ctx, db, `
		SELECT id
		FROM t
		WHERE j ? CAST('a' AS STRING)
		ORDER BY id
	`)
	require.Equal(t, []int{1, 2}, got)

	got = queryIDs(t, ctx, db, `
		SELECT id
		FROM t
		WHERE j ?| CAST(ARRAY['missing', 'a'] AS STRING[])
		ORDER BY id
	`)
	require.Equal(t, []int{1, 2}, got)

	got = queryIDs(t, ctx, db, `
		SELECT id
		FROM t
		WHERE j ?& CAST(ARRAY['a', 'z'] AS STRING[])
		ORDER BY id
	`)
	require.Equal(t, []int{1, 2}, got)

	got = queryIDs(t, ctx, db, `
		SELECT id
		FROM t
		WHERE j#>>CAST('{a,b,-1}' AS STRING[]) = CAST('20' AS STRING)
		ORDER BY id
	`)
	require.Equal(t, []int{1}, got)
}

func TestSubordinateJSONCastWrappedProgramsColumnarExplainUsesColBatchScan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [30]}, "z": 8}'),
		(3, '{"q": "r"}'),
		(4, NULL)
	`)

	plan := planassert.VecVerbose(t, ctx, db, `SELECT id FROM t WHERE (j->'a')::JSONB @> '{"b": [30]}'`)
	planassert.UsesColBatchScan(t, plan)

	plan = planassert.VecVerbose(t, ctx, db, `SELECT id FROM t WHERE (j#>>'{a,b,-1}')::STRING = '20'`)
	planassert.UsesColBatchScan(t, plan)

	plan = planassert.VecVerbose(t, ctx, db, `SELECT id FROM t WHERE j ? CAST('a' AS STRING)`)
	planassert.UsesColBatchScan(t, plan)

	plan = planassert.VecVerbose(t, ctx, db, `SELECT id FROM t WHERE j ?| CAST(ARRAY['missing', 'a'] AS STRING[])`)
	planassert.UsesColBatchScan(t, plan)

	plan = planassert.VecVerbose(t, ctx, db, `SELECT id FROM t WHERE j ?& CAST(ARRAY['a', 'z'] AS STRING[])`)
	planassert.UsesColBatchScan(t, plan)

	plan = planassert.VecVerbose(t, ctx, db, `SELECT id FROM t WHERE j#>>CAST('{a,b,-1}' AS STRING[]) = CAST('20' AS STRING)`)
	planassert.UsesColBatchScan(t, plan)
}

func TestSubordinateJSONVirtualColumnsDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONVirtualTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [30]}, "z": 8}'),
		(3, '{"q": "r"}'),
		(4, NULL)
	`)

	got := queryIDs(t, ctx, db, `
		SELECT id
		FROM t
		WHERE jt = '20'
		ORDER BY id
	`)
	require.Equal(t, []int{1}, got)

	got = queryIDs(t, ctx, db, `
		SELECT id
		FROM t
		WHERE ja @> '{"b": [30]}'
		ORDER BY id
	`)
	require.Equal(t, []int{2}, got)
}

func TestSubordinateJSONVirtualColumnsColumnarExplainUsesColBatchScan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONVirtualTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [30]}, "z": 8}'),
		(3, '{"q": "r"}'),
		(4, NULL)
	`)

	plan := planassert.VecVerbose(t, ctx, db, `SELECT id FROM t WHERE jt = '20'`)
	planassert.UsesColBatchScan(t, plan)

	plan = planassert.VecVerbose(t, ctx, db, `SELECT id FROM t WHERE ja @> '{"b": [30]}'`)
	planassert.UsesColBatchScan(t, plan)
}

func TestSubordinateJSONJoinPushdownDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONJoinTables(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [30]}, "z": 8}'),
		(3, '{"q": "r"}')
	`, `
		(1), (2), (3)
	`)

	got := queryIDs(t, ctx, db, `
		SELECT t.id
		FROM t
		JOIN u USING (id)
		WHERE t.j#>>'{a,b,-1}' = '20'
		ORDER BY t.id
	`)
	require.Equal(t, []int{1}, got)

	got = queryIDs(t, ctx, db, `
		SELECT t.id
		FROM t
		JOIN u USING (id)
		WHERE t.j @> '{"z": 8}'
		ORDER BY t.id
	`)
	require.Equal(t, []int{2}, got)
}

func TestSubordinateJSONJoinPushdownColumnarExplainUsesColBatchScan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONJoinTables(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [30]}, "z": 8}'),
		(3, '{"q": "r"}')
	`, `
		(1), (2), (3)
	`)

	plan := planassert.VecVerbose(t, ctx, db, `
		SELECT t.id
		FROM t
		JOIN u USING (id)
		WHERE t.j#>>'{a,b,-1}' = '20'
	`)
	planassert.UsesColBatchScan(t, plan)

	plan = planassert.VecVerbose(t, ctx, db, `
		SELECT t.id
		FROM t
		JOIN u USING (id)
		WHERE t.j @> '{"z": 8}'
	`)
	planassert.UsesColBatchScan(t, plan)
}

func TestSubordinateJSONContainmentColumnarExplainResidualUsesColBatchScan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [30]}, "z": 8}'),
		(3, '{"a": {"b": [30]}, "z": 9}')
	`)

	plan := planassert.VecVerbose(t, ctx, db, `SELECT id FROM t WHERE j->'a' @> '{"b": [30]}' AND id < 3`)
	planassert.UsesColBatchScan(t, plan)
}

func TestSubordinateJSONMultipleContainmentColumnarExplainUsesColBatchScan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [30]}, "z": 8}'),
		(3, '{"a": {"b": [20], "c": null}, "z": 7}')
	`)

	plan := planassert.VecVerbose(t, ctx, db, `
		SELECT id
		FROM t
		WHERE j @> '{"z": 8}'
		  AND j->'a' @> '{"b": [30]}'
	`)
	planassert.UsesColBatchScan(t, plan)

	plan = planassert.VecVerbose(t, ctx, db, `
		SELECT id
		FROM t
		WHERE j->'a' @> '{"b": [20]}'
		  AND j->'a' <@ '{"b": [20], "c": null}'
	`)
	planassert.UsesColBatchScan(t, plan)
}

func TestSubordinateJSONContainmentColumnarExplainProjectionUsesColBatchScan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [30]}, "z": 8}'),
		(3, '{"q": "r"}')
	`)

	plan := planassert.VecVerbose(t, ctx, db, `SELECT id, j FROM t WHERE j @> '{"z": 8}' ORDER BY id`)
	planassert.UsesColBatchScan(t, plan)
}

func TestSubordinateJSONContainmentColumnarExplainResidualProjectionUsesColBatchScan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [20]}, "z": 8}'),
		(3, '{"a": {"b": [20]}, "q": "r"}'),
		(4, '{"a": {"b": [30]}, "z": 9}')
	`)

	plan := planassert.VecVerbose(t, ctx, db, `
		SELECT j
		FROM t
		WHERE j @> '{"a": {"b": [20]}}'
		  AND id > 1
		ORDER BY id
	`)
	planassert.UsesColBatchScan(t, plan)
}

func TestSubordinateJSONSamePathContainmentColumnarExplainProjectionUsesColBatchScan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [30]}, "z": 8}'),
		(3, '{"q": "r"}')
	`)

	plan := planassert.VecVerbose(t, ctx, db, `SELECT id, j#>'{a}' FROM t WHERE j#>'{a}' @> '{"b": [30]}' ORDER BY id`)
	planassert.UsesColBatchScan(t, plan)
}

func TestSubordinateJSONSamePathDualContainmentProjectionDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [20], "c": null}, "z": 8}'),
		(3, '{"a": {"b": [20], "c": true}, "z": 9}'),
		(4, '{"q": "r"}')
	`)

	rows, err := db.QueryContext(ctx, `
		SELECT id, j#>'{a}'
		FROM t
		WHERE j#>'{a}' @> '{"b": [20]}'
		  AND j#>'{a}' <@ '{"b": [20], "c": null}'
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	var got []string
	for rows.Next() {
		var id int
		var subtree string
		require.NoError(t, rows.Scan(&id, &subtree))
		got = append(got, fmt.Sprintf("%d=%s", id, subtree))
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []string{
		`2={"b": [20], "c": null}`,
	}, got)
}

func TestSubordinateJSONSamePathContainmentTextAccessColumnarExplainUsesColBatchScan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}}'),
		(2, '{"a": {"b": [30]}}'),
		(3, '{"q": "r"}')
	`)

	plan := planassert.VecVerbose(t, ctx, db, `
		SELECT id, j#>>'{a,b,0}'
		FROM t
		WHERE j#>'{a}' @> '{"b": [30]}'
		ORDER BY id
	`)
	planassert.UsesColBatchScan(t, plan)
}

func TestSubordinateJSONSamePathDualContainmentProjectionColumnarExplainUsesColBatchScan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [20], "c": null}, "z": 8}'),
		(3, '{"a": {"b": [20], "c": true}, "z": 9}')
	`)

	plan := planassert.VecVerbose(t, ctx, db, `
		SELECT id, j#>'{a}'
		FROM t
		WHERE j#>'{a}' @> '{"b": [20]}'
		  AND j#>'{a}' <@ '{"b": [20], "c": null}'
		ORDER BY id
	`)
	planassert.UsesColBatchScan(t, plan)
}

func TestSubordinateJSONSamePathContainmentCompareAndProjectionColumnarExplainUsesColBatchScan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}}'),
		(2, '{"a": {"b": [30]}}'),
		(3, '{"a": {"b": [40]}}')
	`)

	plan := planassert.VecVerbose(t, ctx, db, `
		SELECT id, j#>>'{a,b,0}'
		FROM t
		WHERE j#>'{a}' @> '{"b": [30]}'
		  AND j#>>'{a,b,0}' = '30'
		ORDER BY id
	`)
	planassert.UsesColBatchScan(t, plan)
}

func TestSubordinateJSONContainedByColumnarExplainProjectionUsesColBatchScan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [30]}, "z": 8}'),
		(3, '{"q": "r"}')
	`)

	plan := planassert.VecVerbose(t, ctx, db, `SELECT id, j FROM t WHERE j <@ '{"a": {"b": [30]}, "z": 8, "extra": 1}' ORDER BY id`)
	planassert.UsesColBatchScan(t, plan)
}

func TestSubordinateJSONExistsAllFilterOnlyDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [30]}}'),
		(3, '{"b": 1, "c": 2}'),
		(4, NULL),
		(5, 'null')
	`)

	got := queryIDs(t, ctx, db, `
		SELECT id
		FROM t
		WHERE j ?& ARRAY['a', 'z']
		ORDER BY id DESC
	`)
	require.Equal(t, []int{1}, got)
}

func TestSubordinateJSONColumnarExplainExistsAllUsesColBatchScan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "z": 7}'),
		(2, '{"a": {"b": [30]}}'),
		(3, '{"b": 1, "c": 2}'),
		(4, NULL)
	`)

	plan := planassert.VecVerbose(t, ctx, db, `SELECT id FROM t WHERE j ?& ARRAY['a', 'z']`)
	planassert.UsesColBatchScan(t, plan)
}

func TestSubordinateJSONExistsSetFilterOnlyDistSQL(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `SET distsql = always`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `SET vectorize = on`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, j JSONB)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO t VALUES
			(1, '{"a": {"b": [10, 20]}, "z": 7}'),
			(2, '{"a": {"b": [30]}, "z": 8}'),
			(3, '{"q": "r", "z": 9}'),
			(4, NULL),
			(5, 'null')
	`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `
		SELECT id
		FROM t
		WHERE j ?| ARRAY['missing', 'a']
		ORDER BY id DESC
	`)
	require.NoError(t, err)
	defer rows.Close()

	var gotAny []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		gotAny = append(gotAny, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{2, 1}, gotAny)

	rows, err = db.QueryContext(ctx, `
		SELECT id
		FROM t
		WHERE j ?& ARRAY['a', 'z']
		ORDER BY id DESC
	`)
	require.NoError(t, err)
	defer rows.Close()

	var gotAll []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		gotAll = append(gotAll, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{2, 1}, gotAll)
}

func TestSubordinateJSONExistsSetProjectionDistSQL(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `SET distsql = always`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `SET vectorize = on`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, j JSONB)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO t VALUES
			(1, '{"a": {"b": [10, 20]}, "z": 7}'),
			(2, '{"a": {"b": [30]}, "z": 8}'),
			(3, '{"q": "r", "z": 9}'),
			(4, NULL),
			(5, 'null')
	`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `
		SELECT id, j ?| ARRAY['a', 'missing'], j ?& ARRAY['a', 'z']
		FROM t
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	type existsSetRow struct {
		id     int
		anyKey sql.NullBool
		allKey sql.NullBool
	}
	var got []existsSetRow
	for rows.Next() {
		var row existsSetRow
		require.NoError(t, rows.Scan(&row.id, &row.anyKey, &row.allKey))
		got = append(got, row)
	}
	require.NoError(t, rows.Err())

	require.Equal(t, []existsSetRow{
		{
			id:     1,
			anyKey: sql.NullBool{Bool: true, Valid: true},
			allKey: sql.NullBool{Bool: true, Valid: true},
		},
		{
			id:     2,
			anyKey: sql.NullBool{Bool: true, Valid: true},
			allKey: sql.NullBool{Bool: true, Valid: true},
		},
		{
			id:     3,
			anyKey: sql.NullBool{Bool: false, Valid: true},
			allKey: sql.NullBool{Bool: false, Valid: true},
		},
		{id: 4},
		{
			id:     5,
			anyKey: sql.NullBool{Bool: false, Valid: true},
			allKey: sql.NullBool{Bool: false, Valid: true},
		},
	}, got)
}

func TestSubordinateJSONNegativeIndexProjectionDistSQL(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `SET distsql = always`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `SET vectorize = on`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, j JSONB)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO t VALUES
			(1, '{"a": {"b": [10, 20]}, "1": "obj"}'),
			(2, '{"a": {"b": [30]}, "1": "obj2"}'),
			(3, '{"a": {"b": []}, "1": "obj3"}')
	`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `
		SELECT
			id,
			j#>'{a,b,-1}',
			j#>>'{a,b,-1}',
			j->'1'
		FROM t
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	type negativeIdxRow struct {
		id       int
		pathJSON sql.NullString
		pathText sql.NullString
		keyJSON  sql.NullString
	}
	var got []negativeIdxRow
	for rows.Next() {
		var row negativeIdxRow
		require.NoError(t, rows.Scan(&row.id, &row.pathJSON, &row.pathText, &row.keyJSON))
		got = append(got, row)
	}
	require.NoError(t, rows.Err())

	require.Equal(t, []negativeIdxRow{
		{
			id:       1,
			pathJSON: sql.NullString{String: "20", Valid: true},
			pathText: sql.NullString{String: "20", Valid: true},
			keyJSON:  sql.NullString{String: "\"obj\"", Valid: true},
		},
		{
			id:       2,
			pathJSON: sql.NullString{String: "30", Valid: true},
			pathText: sql.NullString{String: "30", Valid: true},
			keyJSON:  sql.NullString{String: "\"obj2\"", Valid: true},
		},
		{
			id:      3,
			keyJSON: sql.NullString{String: "\"obj3\"", Valid: true},
		},
	}, got)
}

func TestSubordinateJSONPathCompareFilterOnlyDistSQL(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `SET distsql = always`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `SET vectorize = on`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, j JSONB)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO t VALUES
			(1, '{"a": {"b": [10, 20]}, "1": "obj"}'),
			(2, '{"a": {"b": [30]}, "1": "obj2"}'),
			(3, '{"a": {"b": []}, "1": "obj3"}'),
			(4, NULL)
	`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `
		SELECT id
		FROM t
		WHERE j#>>'{a,b,-1}' = '20'
		ORDER BY id DESC
	`)
	require.NoError(t, err)
	defer rows.Close()

	var got []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		got = append(got, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{1}, got)
}

func TestSubordinateJSONPathCompareJSONFilterOnlyDistSQL(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `SET distsql = always`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `SET vectorize = on`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, j JSONB)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO t VALUES
			(1, '{"a": {"b": [10, 20]}, "1": "obj"}'),
			(2, '{"a": {"b": [30]}, "1": "obj2"}'),
			(3, '{"a": {"b": []}, "1": "obj3"}'),
			(4, NULL)
	`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `
		SELECT id
		FROM t
		WHERE j->'1' = '"obj"'::jsonb
		ORDER BY id DESC
	`)
	require.NoError(t, err)
	defer rows.Close()

	var got []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		got = append(got, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{1}, got)
}

func TestSubordinateJSONPathCompareNonEqFilterOnlyDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "1": "obj"}'),
		(2, '{"a": {"b": [30]}, "1": "obj2"}'),
		(3, '{"a": {"b": []}, "1": "obj3"}'),
		(4, NULL)
	`)

	rows, err := db.QueryContext(ctx, `
		SELECT id
		FROM t
		WHERE j#>>'{a,b,-1}' < '25'
		ORDER BY id DESC
	`)
	require.NoError(t, err)
	defer rows.Close()

	var gotLt []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		gotLt = append(gotLt, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{1}, gotLt)

	rows, err = db.QueryContext(ctx, `
		SELECT id
		FROM t
		WHERE j->'1' <> '"obj"'::jsonb
		ORDER BY id DESC
	`)
	require.NoError(t, err)
	defer rows.Close()

	var gotNe []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		gotNe = append(gotNe, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{3, 2}, gotNe)
}

func TestSubordinateJSONColumnarExplainPathCompareNonEqUsesColBatchScan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}, "1": "obj"}'),
		(2, '{"a": {"b": [30]}, "1": "obj2"}'),
		(3, '{"a": {"b": []}, "1": "obj3"}'),
		(4, NULL)
	`)

	plan := planassert.VecVerbose(t, ctx, db, `SELECT id FROM t WHERE j#>>'{a,b,-1}' < '25'`)
	planassert.UsesColBatchScan(t, plan)

	plan = planassert.VecVerbose(t, ctx, db, `SELECT id FROM t WHERE j->'1' <> '"obj"'::jsonb`)
	planassert.UsesColBatchScan(t, plan)
}

func TestSubordinateJSONPathCompareWithResidualFilterDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}}'),
		(2, '{"a": {"b": [20]}}'),
		(3, '{"a": {"b": [10]}}')
	`)

	rows, err := db.QueryContext(ctx, `
		SELECT id
		FROM t
		WHERE j#>>'{a,b,-1}' = '20' AND id < 2
		ORDER BY id DESC
	`)
	require.NoError(t, err)
	defer rows.Close()

	var got []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		got = append(got, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{1}, got)
}

func TestSubordinateJSONPathCompareResidualProjectionDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}}'),
		(2, '{"a": {"b": [20]}}'),
		(3, '{"a": {"b": [10]}}')
	`)

	rows, err := db.QueryContext(ctx, `
		SELECT j
		FROM t
		WHERE j#>>'{a,b,-1}' = '20' AND id < 2
		ORDER BY id DESC
	`)
	require.NoError(t, err)
	defer rows.Close()

	type rowResult struct {
		j sql.NullString
	}
	var got []rowResult
	for rows.Next() {
		var row rowResult
		require.NoError(t, rows.Scan(&row.j))
		got = append(got, row)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []rowResult{{
		j: sql.NullString{String: `{"a": {"b": [10, 20]}}`, Valid: true},
	}}, got)
}

func TestSubordinateJSONColumnarExplainPathCompareResidualProjectionUsesColBatchScan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}}'),
		(2, '{"a": {"b": [20]}}'),
		(3, '{"a": {"b": [10]}}')
	`)

	plan := planassert.VecVerbose(t, ctx, db, `
		SELECT j
		FROM t
		WHERE j#>>'{a,b,-1}' = '20' AND id < 2
	`)
	planassert.UsesColBatchScan(t, plan)
}

func TestSubordinateJSONPathCompareProjectionDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, 20]}}'),
		(2, '{"a": {"b": [30]}}'),
		(3, '{"a": {"b": [10]}}')
	`)

	rows, err := db.QueryContext(ctx, `
		SELECT id, j
		FROM t
		WHERE j#>>'{a,b,-1}' = '20'
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	type rowResult struct {
		id int
		j  sql.NullString
	}
	var got []rowResult
	for rows.Next() {
		var row rowResult
		require.NoError(t, rows.Scan(&row.id, &row.j))
		got = append(got, row)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []rowResult{{
		id: 1,
		j:  sql.NullString{String: `{"a": {"b": [10, 20]}}`, Valid: true},
	}}, got)
}

func TestSubordinateJSONExistsProjectionDistSQL(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `SET distsql = always`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `SET vectorize = on`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, j JSONB)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO t VALUES
			(1, '{"a": {"b": [10, 20]}, "z": 7}'),
			(2, '{"a": {"b": [30]}, "z": 8}'),
			(3, '{"q": "r", "z": 9}')
	`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `
		SELECT id, j
		FROM t
		WHERE j ? 'a'
		ORDER BY id DESC
	`)
	require.NoError(t, err)
	defer rows.Close()

	type rowResult struct {
		id int
		j  sql.NullString
	}
	var got []rowResult
	for rows.Next() {
		var row rowResult
		require.NoError(t, rows.Scan(&row.id, &row.j))
		got = append(got, row)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []rowResult{
		{id: 2, j: sql.NullString{String: `{"a": {"b": [30]}, "z": 8}`, Valid: true}},
		{id: 1, j: sql.NullString{String: `{"a": {"b": [10, 20]}, "z": 7}`, Valid: true}},
	}, got)
}

func TestSubordinateJSONPathIsNullFilterDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, null]}}'),
		(2, '{"a": {"b": [30]}}'),
		(3, NULL)
	`)

	rows, err := db.QueryContext(ctx, `
		SELECT id
		FROM t
		WHERE j#>>'{a,b,1}' IS NULL
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	var got []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		got = append(got, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{1, 2, 3}, got)
}

func TestSubordinateJSONPathIsNotNullFilterDistSQL(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "on")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, null]}}'),
		(2, '{"a": {"b": [30]}}'),
		(3, NULL)
	`)

	rows, err := db.QueryContext(ctx, `
		SELECT id
		FROM t
		WHERE j#>>'{a,b,0}' IS NOT NULL
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	var got []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		got = append(got, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{1, 2}, got)
}

func TestSubordinateJSONColumnarExplainPathNullFiltersUseColBatchScan(t *testing.T) {
	ctx, c, db := startSubordinateJSONCluster(t, "experimental_always")
	defer c.Stop()

	createSubordinateJSONTable(t, ctx, db, `
		(1, '{"a": {"b": [10, null]}}'),
		(2, '{"a": {"b": [30]}}'),
		(3, NULL)
	`)

	plan := planassert.VecVerbose(t, ctx, db, `SELECT id FROM t WHERE j#>>'{a,b,1}' IS NULL`)
	planassert.UsesColBatchScan(t, plan)

	plan = planassert.VecVerbose(t, ctx, db, `SELECT id FROM t WHERE j#>>'{a,b,0}' IS NOT NULL`)
	planassert.UsesColBatchScan(t, plan)
}
