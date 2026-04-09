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
	"testing"

	"github.com/cockroachdb/cockroach/pkg/testutils/inproc"
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
