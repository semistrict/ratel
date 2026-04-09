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
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing permissions and
// limitations under the License.

package inproc_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/lib/pq"
	"github.com/semistrict/ratel/pkg/testutils/inproc"
	"github.com/stretchr/testify/require"
)

func TestSubordinateArrayEqualsAnyFilterOnly(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, vals INT[])`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO t VALUES
			(1, ARRAY[10, 20]),
			(2, ARRAY[30]),
			(3, NULL),
			(4, ARRAY[]::INT[]),
			(5, ARRAY[NULL, 20]),
			(6, ARRAY[99, 100])
	`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `SELECT id FROM t WHERE $1::INT = ANY(vals) ORDER BY id`, 20)
	require.NoError(t, err)
	defer rows.Close()

	var got []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		got = append(got, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{1, 5}, got)
}

func TestSubordinateArrayEqualsAnyFilterOnlyReverseOrder(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, vals INT[])`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO t VALUES
			(1, ARRAY[10, 20, 30]),
			(2, ARRAY[40, 50]),
			(3, ARRAY[20, 60]),
			(4, NULL)
	`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `SELECT id FROM t WHERE $1::INT = ANY(vals) ORDER BY id DESC`, 20)
	require.NoError(t, err)
	defer rows.Close()

	var got []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		got = append(got, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{3, 1}, got)
}

func TestSubordinateArrayEqualsAnyFilterOnlyRowEngine(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `SET distsql = off`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `SET vectorize = off`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, vals INT[])`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO t VALUES
			(1, ARRAY[10, 20]),
			(2, ARRAY[30]),
			(3, ARRAY[20, 40]),
			(4, NULL)
	`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `SELECT id FROM t WHERE $1::INT = ANY(vals) ORDER BY id DESC`, 20)
	require.NoError(t, err)
	defer rows.Close()

	var got []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		got = append(got, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{3, 1}, got)
}

func TestSubordinateArrayEqualsAnyFilterWithDistSQLVectorize(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `SET distsql = always`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `SET vectorize = on`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, vals INT[])`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO t VALUES
			(1, ARRAY[10, 20]),
			(2, ARRAY[30, 40]),
			(3, ARRAY[]::INT[]),
			(4, NULL)
	`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `SELECT id FROM t WHERE $1::INT = ANY(vals) ORDER BY id DESC`, 20)
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

func TestSubordinateArrayEqualsAnyProjectionMaterializesArray(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, vals INT[])`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO t VALUES
			(1, ARRAY[10, 20]),
			(2, ARRAY[30]),
			(3, ARRAY[20, 40]),
			(4, NULL)
	`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `SELECT vals FROM t WHERE $1::INT = ANY(vals) ORDER BY id DESC`, 20)
	require.NoError(t, err)
	defer rows.Close()

	var got [][]int64
	for rows.Next() {
		var vals []int64
		require.NoError(t, rows.Scan(pq.Array(&vals)))
		got = append(got, vals)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, [][]int64{{20, 40}, {10, 20}}, got)
}

func TestSubordinateArrayEqualsAnyProjectionMaterializesArrayRowEngine(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `SET distsql = off`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `SET vectorize = off`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, vals INT[])`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO t VALUES
			(1, ARRAY[10, 20]),
			(2, ARRAY[30]),
			(3, ARRAY[20, 40]),
			(4, NULL)
	`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `SELECT vals FROM t WHERE $1::INT = ANY(vals) ORDER BY id DESC`, 20)
	require.NoError(t, err)
	defer rows.Close()

	var got [][]int64
	for rows.Next() {
		var vals []int64
		require.NoError(t, rows.Scan(pq.Array(&vals)))
		got = append(got, vals)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, [][]int64{{20, 40}, {10, 20}}, got)
}

func TestSubordinateArrayEqualsAnyExpressionNullSemantics(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, vals INT[])`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO t VALUES
			(1, ARRAY[20]),
			(2, ARRAY[NULL, 30]),
			(3, ARRAY[NULL, 20]),
			(4, NULL),
			(5, ARRAY[]::INT[]),
			(6, ARRAY[30])
	`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `SELECT id, $1::INT = ANY(vals) FROM t ORDER BY id`, 20)
	require.NoError(t, err)
	defer rows.Close()

	type result struct {
		id    int
		match sql.NullBool
	}
	var got []result
	for rows.Next() {
		var r result
		require.NoError(t, rows.Scan(&r.id, &r.match))
		got = append(got, r)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []result{
		{id: 1, match: sql.NullBool{Bool: true, Valid: true}},
		{id: 2, match: sql.NullBool{Valid: false}},
		{id: 3, match: sql.NullBool{Bool: true, Valid: true}},
		{id: 4, match: sql.NullBool{Valid: false}},
		{id: 5, match: sql.NullBool{Bool: false, Valid: true}},
		{id: 6, match: sql.NullBool{Bool: false, Valid: true}},
	}, got)

	rows, err = db.QueryContext(ctx, `SELECT id FROM t WHERE $1::INT = ANY(vals) ORDER BY id`, 20)
	require.NoError(t, err)
	defer rows.Close()

	var matched []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		matched = append(matched, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{1, 3}, matched)
}

func TestSubordinateArrayEqualsAnyExpressionNullSemanticsRowEngine(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `SET distsql = off`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `SET vectorize = off`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, vals INT[])`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO t VALUES
			(1, ARRAY[20]),
			(2, ARRAY[NULL, 30]),
			(3, ARRAY[NULL, 20]),
			(4, NULL),
			(5, ARRAY[]::INT[]),
			(6, ARRAY[30])
	`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `SELECT id FROM t WHERE $1::INT = ANY(vals) ORDER BY id`, 20)
	require.NoError(t, err)
	defer rows.Close()

	var matched []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		matched = append(matched, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{1, 3}, matched)

	rows, err = db.QueryContext(ctx, `SELECT id, $1::INT = ANY(vals) FROM t ORDER BY id`, 20)
	require.NoError(t, err)
	defer rows.Close()

	type result struct {
		id    int
		match sql.NullBool
	}
	var got []result
	for rows.Next() {
		var r result
		require.NoError(t, rows.Scan(&r.id, &r.match))
		got = append(got, r)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []result{
		{id: 1, match: sql.NullBool{Bool: true, Valid: true}},
		{id: 2, match: sql.NullBool{Valid: false}},
		{id: 3, match: sql.NullBool{Bool: true, Valid: true}},
		{id: 4, match: sql.NullBool{Valid: false}},
		{id: 5, match: sql.NullBool{Bool: false, Valid: true}},
		{id: 6, match: sql.NullBool{Bool: false, Valid: true}},
	}, got)
}

func TestSubordinateArrayEqualsAnyExpressionNullSemanticsVectorized(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `SET distsql = always`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `SET vectorize = on`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, vals INT[])`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO t VALUES
			(1, ARRAY[20]),
			(2, ARRAY[NULL, 30]),
			(3, ARRAY[NULL, 20]),
			(4, NULL),
			(5, ARRAY[]::INT[]),
			(6, ARRAY[30])
	`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `SELECT id, $1::INT = ANY(vals) FROM t ORDER BY id`, 20)
	require.NoError(t, err)
	defer rows.Close()

	type result struct {
		id    int
		match sql.NullBool
	}
	var got []result
	for rows.Next() {
		var r result
		require.NoError(t, rows.Scan(&r.id, &r.match))
		got = append(got, r)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []result{
		{id: 1, match: sql.NullBool{Bool: true, Valid: true}},
		{id: 2, match: sql.NullBool{Valid: false}},
		{id: 3, match: sql.NullBool{Bool: true, Valid: true}},
		{id: 4, match: sql.NullBool{Valid: false}},
		{id: 5, match: sql.NullBool{Bool: false, Valid: true}},
		{id: 6, match: sql.NullBool{Bool: false, Valid: true}},
	}, got)
}

func TestSubordinateArrayEqualsAnyNullFilterVectorized(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `SET distsql = always`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `SET vectorize = on`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, vals INT[])`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO t VALUES
			(1, ARRAY[20]),
			(2, ARRAY[NULL, 30]),
			(3, ARRAY[NULL, 20]),
			(4, NULL),
			(5, ARRAY[]::INT[]),
			(6, ARRAY[30])
	`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `SELECT id FROM t WHERE $1::INT = ANY(vals) ORDER BY id`, 20)
	require.NoError(t, err)
	defer rows.Close()

	var got []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		got = append(got, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{1, 3}, got)
}

func TestSubordinateArrayEqualsAnyWithResidualFilter(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, vals INT[])`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO t VALUES
			(1, ARRAY[20]),
			(2, ARRAY[30]),
			(3, ARRAY[20, 40]),
			(4, ARRAY[20, 50]),
			(5, NULL)
	`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `SELECT id FROM t WHERE $1::INT = ANY(vals) AND id > 2 ORDER BY id DESC`, 20)
	require.NoError(t, err)
	defer rows.Close()

	var got []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		got = append(got, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{4, 3}, got)
}

func TestSubordinateArrayEqualsAnyProjectionMaterializesArrayVectorized(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `SET distsql = always`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `SET vectorize = on`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, vals INT[])`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO t VALUES
			(1, ARRAY[10, 20]),
			(2, ARRAY[30]),
			(3, ARRAY[20, 40]),
			(4, NULL)
	`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `SELECT vals FROM t WHERE $1::INT = ANY(vals) ORDER BY id DESC`, 20)
	require.NoError(t, err)
	defer rows.Close()

	var got [][]int64
	for rows.Next() {
		var vals []int64
		require.NoError(t, rows.Scan(pq.Array(&vals)))
		got = append(got, vals)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, [][]int64{{20, 40}, {10, 20}}, got)
}

func TestSubordinateArrayEqualsAnyUnsupportedExprFallback(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, vals INT[])`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO t VALUES
			(1, ARRAY[10, 20]),
			(2, ARRAY[20, 40]),
			(3, ARRAY[10]),
			(4, NULL)
	`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `SELECT id FROM t WHERE id * 10 = ANY(vals) ORDER BY id`)
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

func TestSubordinateArrayEqualsAnyUnsupportedExprFallbackVectorized(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `SET distsql = always`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `SET vectorize = on`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, vals INT[])`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO t VALUES
			(1, ARRAY[10, 20]),
			(2, ARRAY[20, 40]),
			(3, ARRAY[10]),
			(4, NULL)
	`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `SELECT id FROM t WHERE id * 10 = ANY(vals) ORDER BY id`)
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
