// Copyright 2026 The Ratel Authors.
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

package inproc_test

import (
	"context"
	"regexp"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/testutils/inproc"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// TestSubordinateArrayInsertSelect verifies that inserting a row with an
// array column and selecting it back returns the correct values. This
// exercises the full subordinate key write + read path.
func TestSubordinateArrayInsertSelect(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, tags TEXT[])`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `INSERT INTO t VALUES (1, ARRAY['a','b','c'])`)
	require.NoError(t, err)

	var tags []string
	err = db.QueryRowContext(ctx, `SELECT tags FROM t WHERE id = 1`).Scan(pq.Array(&tags))
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b", "c"}, tags)
}

// TestSubordinateArrayNull verifies that NULL arrays round-trip correctly.
func TestSubordinateArrayNull(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, vals INT[])`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `INSERT INTO t VALUES (1, NULL)`)
	require.NoError(t, err)

	var isNull bool
	err = db.QueryRowContext(ctx, `SELECT vals IS NULL FROM t WHERE id = 1`).Scan(&isNull)
	require.NoError(t, err)
	require.True(t, isNull)
}

// TestSubordinateArrayEmpty verifies that empty arrays round-trip correctly
// and are distinct from NULL.
func TestSubordinateArrayEmpty(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, vals INT[])`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `INSERT INTO t VALUES (1, ARRAY[]::INT[])`)
	require.NoError(t, err)

	var isNull bool
	err = db.QueryRowContext(ctx, `SELECT vals IS NULL FROM t WHERE id = 1`).Scan(&isNull)
	require.NoError(t, err)
	require.False(t, isNull, "empty array should not be NULL")

	var vals []int64
	err = db.QueryRowContext(ctx, `SELECT vals FROM t WHERE id = 1`).Scan(pq.Array(&vals))
	require.NoError(t, err)
	require.Equal(t, []int64{}, vals)
}

// TestSubordinateArrayUpdate verifies that updating an array column
// correctly replaces old subordinate keys.
func TestSubordinateArrayUpdate(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, vals INT[])`)
	require.NoError(t, err)

	// Insert initial array.
	_, err = db.ExecContext(ctx, `INSERT INTO t VALUES (1, ARRAY[10, 20, 30])`)
	require.NoError(t, err)

	// Grow: append an element.
	_, err = db.ExecContext(ctx, `UPDATE t SET vals = array_append(vals, 40) WHERE id = 1`)
	require.NoError(t, err)
	var vals []int64
	err = db.QueryRowContext(ctx, `SELECT vals FROM t WHERE id = 1`).Scan(pq.Array(&vals))
	require.NoError(t, err)
	require.Equal(t, []int64{10, 20, 30, 40}, vals)

	// Shrink: replace with shorter array.
	_, err = db.ExecContext(ctx, `UPDATE t SET vals = ARRAY[99] WHERE id = 1`)
	require.NoError(t, err)
	err = db.QueryRowContext(ctx, `SELECT vals FROM t WHERE id = 1`).Scan(pq.Array(&vals))
	require.NoError(t, err)
	require.Equal(t, []int64{99}, vals)

	// Replace entirely.
	_, err = db.ExecContext(ctx, `UPDATE t SET vals = ARRAY[5, 6] WHERE id = 1`)
	require.NoError(t, err)
	err = db.QueryRowContext(ctx, `SELECT vals FROM t WHERE id = 1`).Scan(pq.Array(&vals))
	require.NoError(t, err)
	require.Equal(t, []int64{5, 6}, vals)
}

// TestSubordinateArrayDelete verifies that deleting a row also removes
// all subordinate keys (no leaked KVs).
func TestSubordinateArrayDelete(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, vals INT[])`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `INSERT INTO t VALUES (1, ARRAY[10, 20, 30])`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `DELETE FROM t WHERE id = 1`)
	require.NoError(t, err)

	var count int
	err = db.QueryRowContext(ctx, `SELECT count(*) FROM t`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	// Re-insert to verify no stale subordinate keys interfere.
	_, err = db.ExecContext(ctx, `INSERT INTO t VALUES (1, ARRAY[77])`)
	require.NoError(t, err)
	var vals []int64
	err = db.QueryRowContext(ctx, `SELECT vals FROM t WHERE id = 1`).Scan(pq.Array(&vals))
	require.NoError(t, err)
	require.Equal(t, []int64{77}, vals)
}

// TestSubordinateArrayLarge verifies correct handling of large arrays.
func TestSubordinateArrayLarge(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, vals INT[])`)
	require.NoError(t, err)

	// Build a large array via generate_series.
	const n = 1000
	_, err = db.ExecContext(ctx, `INSERT INTO t SELECT 1, array_agg(g) FROM generate_series(1, $1) AS g`, n)
	require.NoError(t, err)

	var vals []int64
	err = db.QueryRowContext(ctx, `SELECT vals FROM t WHERE id = 1`).Scan(pq.Array(&vals))
	require.NoError(t, err)
	require.Equal(t, n, len(vals))
	for i := 0; i < n; i++ {
		require.Equal(t, int64(i+1), vals[i])
	}
}

// TestSubordinateArrayReverseScanOrder verifies that reverse scans preserve the
// logical element order inside each array datum.
func TestSubordinateArrayReverseScanOrder(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, vals INT[])`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `INSERT INTO t VALUES (1, ARRAY[10, 20, 30]), (2, ARRAY[40, 50])`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `SELECT vals FROM t ORDER BY id DESC`)
	require.NoError(t, err)
	defer rows.Close()

	var got [][]int64
	for rows.Next() {
		var vals []int64
		require.NoError(t, rows.Scan(pq.Array(&vals)))
		got = append(got, vals)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, [][]int64{{40, 50}, {10, 20, 30}}, got)
}

func TestSubordinateArrayReverseScanOrderVectorized(t *testing.T) {
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
	_, err = db.ExecContext(ctx, `INSERT INTO t VALUES (1, ARRAY[10, 20, 30]), (2, ARRAY[40, 50])`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `SELECT vals FROM t ORDER BY id DESC`)
	require.NoError(t, err)
	defer rows.Close()

	var got [][]int64
	for rows.Next() {
		var vals []int64
		require.NoError(t, rows.Scan(pq.Array(&vals)))
		got = append(got, vals)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, [][]int64{{40, 50}, {10, 20, 30}}, got)
}

// TestSubordinateArrayRespectsMaxRowSize verifies that arrays encoded as
// subordinate KV entries are still subject to row-size guardrails.
func TestSubordinateArrayRespectsMaxRowSize(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `SET CLUSTER SETTING sql.guardrails.max_row_size_err = '1KiB'`)
	require.NoError(t, err)
	defer func() {
		_, resetErr := db.ExecContext(ctx, `SET CLUSTER SETTING sql.guardrails.max_row_size_err = DEFAULT`)
		require.NoError(t, resetErr)
	}()

	_, err = db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, vals INT[])`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `INSERT INTO t SELECT 1, array_agg(g) FROM generate_series(1, 500) AS g`)
	require.Error(t, err)
	require.Regexp(t, regexp.MustCompile(`row larger than max row size`), err.Error())
}

// TestSubordinateArrayMixedColumns verifies that a table with both scalar
// and array columns works correctly.
func TestSubordinateArrayMixedColumns(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `CREATE TABLE t (
		id INT PRIMARY KEY,
		name TEXT,
		tags TEXT[],
		score INT
	)`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `INSERT INTO t VALUES (1, 'alice', ARRAY['go','rust'], 100)`)
	require.NoError(t, err)

	var name string
	var tags []string
	var score int
	err = db.QueryRowContext(ctx, `SELECT name, tags, score FROM t WHERE id = 1`).Scan(&name, pq.Array(&tags), &score)
	require.NoError(t, err)
	require.Equal(t, "alice", name)
	require.Equal(t, []string{"go", "rust"}, tags)
	require.Equal(t, 100, score)
}

// TestSubordinateArrayDropAddColumn verifies that dropping an array column
// and re-adding one with the same name does not resurrect old subordinate
// key data. The new column gets a new column ID, so old subordinate keys
// (keyed by the old column ID) must not be visible.
func TestSubordinateArrayDropAddColumn(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, tags TEXT[])`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `INSERT INTO t VALUES (1, ARRAY['old_a','old_b'])`)
	require.NoError(t, err)

	// Drop the column and add a new one with the same name.
	_, err = db.ExecContext(ctx, `ALTER TABLE t DROP COLUMN tags`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `ALTER TABLE t ADD COLUMN tags TEXT[]`)
	require.NoError(t, err)

	// The new column should be NULL (not the old data).
	var isNull bool
	err = db.QueryRowContext(ctx, `SELECT tags IS NULL FROM t WHERE id = 1`).Scan(&isNull)
	require.NoError(t, err)
	require.True(t, isNull, "re-added column should be NULL, not old data")

	// Insert new data and verify it works.
	_, err = db.ExecContext(ctx, `UPDATE t SET tags = ARRAY['new_x'] WHERE id = 1`)
	require.NoError(t, err)

	var tags []string
	err = db.QueryRowContext(ctx, `SELECT tags FROM t WHERE id = 1`).Scan(pq.Array(&tags))
	require.NoError(t, err)
	require.Equal(t, []string{"new_x"}, tags)
}

// TestSubordinateArrayInvertedIndex verifies that inverted indexes on array
// columns work correctly with subordinate key encoding. The inverted index
// is built from the DArray datum, not from subordinate keys directly.
func TestSubordinateArrayInvertedIndex(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `CREATE TABLE t (
		id INT PRIMARY KEY,
		tags TEXT[],
		INVERTED INDEX idx_tags (tags)
	)`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `INSERT INTO t VALUES
		(1, ARRAY['go','rust','c']),
		(2, ARRAY['python','go']),
		(3, ARRAY['rust','zig']),
		(4, ARRAY['c','c++'])
	`)
	require.NoError(t, err)

	// @> (contains): rows whose tags contain ARRAY['go']
	rows, err := db.QueryContext(ctx, `SELECT id FROM t WHERE tags @> ARRAY['go'] ORDER BY id`)
	require.NoError(t, err)
	var ids []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		ids = append(ids, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{1, 2}, ids)

	// @> with multiple elements
	var id int
	err = db.QueryRowContext(ctx, `SELECT id FROM t WHERE tags @> ARRAY['go','rust']`).Scan(&id)
	require.NoError(t, err)
	require.Equal(t, 1, id)

	// <@ (contained by)
	rows, err = db.QueryContext(ctx, `SELECT id FROM t WHERE tags <@ ARRAY['go','rust','c','zig'] ORDER BY id`)
	require.NoError(t, err)
	ids = nil
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		ids = append(ids, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{1, 3}, ids)
}

// TestSubordinateArrayANY verifies that the ANY/ALL operators work correctly
// with arrays stored as subordinate keys.
func TestSubordinateArrayANY(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, vals INT[])`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `INSERT INTO t VALUES
		(1, ARRAY[10, 20, 30]),
		(2, ARRAY[20, 40]),
		(3, ARRAY[50])
	`)
	require.NoError(t, err)

	// = ANY: rows where vals contains 20
	rows, err := db.QueryContext(ctx, `SELECT id FROM t WHERE 20 = ANY(vals) ORDER BY id`)
	require.NoError(t, err)
	var ids []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		ids = append(ids, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{1, 2}, ids)

	// < ALL: rows where all elements are > 15
	rows, err = db.QueryContext(ctx, `SELECT id FROM t WHERE 15 < ALL(vals) ORDER BY id`)
	require.NoError(t, err)
	ids = nil
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		ids = append(ids, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{2, 3}, ids)
}

// TestSubordinateArrayElementAccess verifies that array element access (arr[i])
// and array functions work correctly.
func TestSubordinateArrayElementAccess(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, vals TEXT[])`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `INSERT INTO t VALUES (1, ARRAY['a','b','c','d'])`)
	require.NoError(t, err)

	// Element access (1-indexed in SQL).
	var elem string
	err = db.QueryRowContext(ctx, `SELECT vals[2] FROM t WHERE id = 1`).Scan(&elem)
	require.NoError(t, err)
	require.Equal(t, "b", elem)

	// array_length
	var length int
	err = db.QueryRowContext(ctx, `SELECT array_length(vals, 1) FROM t WHERE id = 1`).Scan(&length)
	require.NoError(t, err)
	require.Equal(t, 4, length)

	// array_position
	var pos int
	err = db.QueryRowContext(ctx, `SELECT array_position(vals, 'c') FROM t WHERE id = 1`).Scan(&pos)
	require.NoError(t, err)
	require.Equal(t, 3, pos)

	// array_remove
	var vals []string
	err = db.QueryRowContext(ctx, `SELECT array_remove(vals, 'b') FROM t WHERE id = 1`).Scan(pq.Array(&vals))
	require.NoError(t, err)
	require.Equal(t, []string{"a", "c", "d"}, vals)

	// array_cat
	err = db.QueryRowContext(ctx, `SELECT array_cat(vals, ARRAY['e','f']) FROM t WHERE id = 1`).Scan(pq.Array(&vals))
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b", "c", "d", "e", "f"}, vals)

	// unnest
	rows, err := db.QueryContext(ctx, `SELECT unnest(vals) FROM t WHERE id = 1`)
	require.NoError(t, err)
	var elems []string
	for rows.Next() {
		var e string
		require.NoError(t, rows.Scan(&e))
		elems = append(elems, e)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []string{"a", "b", "c", "d"}, elems)
}

// TestSubordinateArrayAggregation verifies that array_agg and other
// aggregate functions work with subordinate key arrays.
func TestSubordinateArrayAggregation(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `CREATE TABLE items (id INT PRIMARY KEY, category TEXT, name TEXT)`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `INSERT INTO items VALUES
		(1, 'fruit', 'apple'),
		(2, 'fruit', 'banana'),
		(3, 'veg', 'carrot'),
		(4, 'veg', 'daikon'),
		(5, 'fruit', 'elderberry')
	`)
	require.NoError(t, err)

	// array_agg grouped by category
	rows, err := db.QueryContext(ctx,
		`SELECT category, array_agg(name ORDER BY name) FROM items GROUP BY category ORDER BY category`)
	require.NoError(t, err)

	type row struct {
		category string
		names    []string
	}
	var results []row
	for rows.Next() {
		var r row
		require.NoError(t, rows.Scan(&r.category, pq.Array(&r.names)))
		results = append(results, r)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []row{
		{"fruit", []string{"apple", "banana", "elderberry"}},
		{"veg", []string{"carrot", "daikon"}},
	}, results)
}

// TestSubordinateArrayMultipleRows verifies correct behavior when scanning
// multiple rows with array columns — each row's subordinate keys must be
// correctly grouped.
func TestSubordinateArrayMultipleRows(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, vals INT[])`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `INSERT INTO t VALUES
		(1, ARRAY[10, 20]),
		(2, ARRAY[30]),
		(3, ARRAY[40, 50, 60]),
		(4, NULL),
		(5, ARRAY[]::INT[])
	`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `SELECT id, vals FROM t ORDER BY id`)
	require.NoError(t, err)

	type row struct {
		id   int
		vals []int64
		null bool
	}
	var results []row
	for rows.Next() {
		var r row
		var scanVals *[]int64
		scanVals = new([]int64)
		var rawVals interface{}
		require.NoError(t, rows.Scan(&r.id, &rawVals))
		if rawVals == nil {
			r.null = true
		} else {
			// Re-query to get the actual array values for this row.
			err := db.QueryRowContext(ctx, `SELECT vals FROM t WHERE id = $1`, r.id).Scan(pq.Array(scanVals))
			require.NoError(t, err)
			r.vals = *scanVals
		}
		results = append(results, r)
	}
	require.NoError(t, rows.Err())

	require.Equal(t, 5, len(results))
	require.Equal(t, []int64{10, 20}, results[0].vals)
	require.Equal(t, []int64{30}, results[1].vals)
	require.Equal(t, []int64{40, 50, 60}, results[2].vals)
	require.True(t, results[3].null)
	// Row 5 has empty array — scanned as nil by the raw interface scan.
	// This is expected since pq doesn't distinguish empty array from NULL
	// in the raw interface{} scan path.
}

// TestSubordinateArrayJoin verifies that joins involving tables with
// array columns work correctly.
func TestSubordinateArrayJoin(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `CREATE TABLE users (id INT PRIMARY KEY, name TEXT, tags TEXT[])`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE TABLE posts (id INT PRIMARY KEY, user_id INT, title TEXT)`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `INSERT INTO users VALUES
		(1, 'alice', ARRAY['admin','editor']),
		(2, 'bob', ARRAY['viewer'])
	`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO posts VALUES
		(1, 1, 'Hello'),
		(2, 2, 'World'),
		(3, 1, 'Again')
	`)
	require.NoError(t, err)

	// Join and read array column.
	rows, err := db.QueryContext(ctx,
		`SELECT p.title, u.tags FROM posts p JOIN users u ON p.user_id = u.id WHERE 'admin' = ANY(u.tags) ORDER BY p.id`)
	require.NoError(t, err)

	type result struct {
		title string
		tags  []string
	}
	var results []result
	for rows.Next() {
		var r result
		require.NoError(t, rows.Scan(&r.title, pq.Array(&r.tags)))
		results = append(results, r)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []result{
		{"Hello", []string{"admin", "editor"}},
		{"Again", []string{"admin", "editor"}},
	}, results)
}

func TestSubordinateArrayJoinReverseOrder(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `CREATE TABLE users (id INT PRIMARY KEY, name TEXT, tags TEXT[])`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE TABLE posts (id INT PRIMARY KEY, user_id INT, title TEXT)`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `INSERT INTO users VALUES
		(1, 'alice', ARRAY['admin','editor']),
		(2, 'bob', ARRAY['viewer','author'])
	`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO posts VALUES
		(1, 1, 'Hello'),
		(2, 2, 'World'),
		(3, 1, 'Again'),
		(4, 2, 'More')
	`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `
		SELECT p.title, u.tags
		FROM users u
		JOIN posts p ON p.user_id = u.id
		ORDER BY u.id DESC, p.id DESC
	`)
	require.NoError(t, err)
	defer rows.Close()

	type result struct {
		title string
		tags  []string
	}
	var results []result
	for rows.Next() {
		var r result
		require.NoError(t, rows.Scan(&r.title, pq.Array(&r.tags)))
		results = append(results, r)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []result{
		{"More", []string{"viewer", "author"}},
		{"World", []string{"viewer", "author"}},
		{"Again", []string{"admin", "editor"}},
		{"Hello", []string{"admin", "editor"}},
	}, results)
}

func TestSubordinateArrayJoinReverseOrderVectorized(t *testing.T) {
	c := inproc.StartCluster(t, 1)
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	_, err := db.ExecContext(ctx, `SET distsql = always`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `SET vectorize = on`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE TABLE users (id INT PRIMARY KEY, name TEXT, tags TEXT[])`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE TABLE posts (id INT PRIMARY KEY, user_id INT, title TEXT)`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `INSERT INTO users VALUES
		(1, 'alice', ARRAY['admin','editor']),
		(2, 'bob', ARRAY['viewer','author'])
	`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO posts VALUES
		(1, 1, 'Hello'),
		(2, 2, 'World'),
		(3, 1, 'Again'),
		(4, 2, 'More')
	`)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `
		SELECT p.title, u.tags
		FROM users u
		JOIN posts p ON p.user_id = u.id
		ORDER BY u.id DESC, p.id DESC
	`)
	require.NoError(t, err)
	defer rows.Close()

	type result struct {
		title string
		tags  []string
	}
	var results []result
	for rows.Next() {
		var r result
		require.NoError(t, rows.Scan(&r.title, pq.Array(&r.tags)))
		results = append(results, r)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []result{
		{"More", []string{"viewer", "author"}},
		{"World", []string{"viewer", "author"}},
		{"Again", []string{"admin", "editor"}},
		{"Hello", []string{"admin", "editor"}},
	}, results)
}
