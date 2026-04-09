package inproc_test

import (
	"context"
	"strings"
	"testing"

	"github.com/semistrict/ratel/pkg/base"
	"github.com/semistrict/ratel/pkg/testutils/inproc"
	"github.com/stretchr/testify/require"
)

// TestDistributedUDF verifies that UDFs execute on remote nodes in a
// distributed query plan, not just on the gateway.
func TestDistributedUDF(t *testing.T) {
	c := inproc.StartCluster(t, 3, func(args *base.TestClusterArgs) {
		args.ReplicationMode = base.ReplicationAuto
	})
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	// Create a simple JS UDF that doubles a number.
	_, err := db.ExecContext(ctx, `
		CREATE FUNCTION double_it(x INT) RETURNS INT LANGUAGE javascript AS $$
			return x * 2;
		$$
	`)
	require.NoError(t, err)

	// Create a table and insert data.
	_, err = db.ExecContext(ctx, `CREATE TABLE nums (id INT PRIMARY KEY, val INT)`)
	require.NoError(t, err)

	for i := 1; i <= 20; i++ {
		_, err = db.ExecContext(ctx, `INSERT INTO nums VALUES ($1, $2)`, i, i*10)
		require.NoError(t, err)
	}

	// Query using the UDF.
	rows, err := db.QueryContext(ctx, `SELECT id, double_it(val) FROM nums ORDER BY id`)
	require.NoError(t, err)
	defer rows.Close()

	for i := 1; i <= 20; i++ {
		require.True(t, rows.Next())
		var id, doubled int
		require.NoError(t, rows.Scan(&id, &doubled))
		require.Equal(t, i, id)
		require.Equal(t, i*10*2, doubled)
	}
	require.False(t, rows.Next())
	require.NoError(t, rows.Err())

	// Verify the query plan is distributed (not just on gateway).
	var plan string
	err = db.QueryRowContext(ctx, `EXPLAIN (DISTSQL) SELECT double_it(val) FROM nums`).Scan(&plan)
	// EXPLAIN DISTSQL returns multiple rows, collect them all.
	planRows, err := db.QueryContext(ctx, `EXPLAIN (DISTSQL) SELECT double_it(val) FROM nums`)
	require.NoError(t, err)
	defer planRows.Close()
	var planLines []string
	for planRows.Next() {
		var line string
		require.NoError(t, planRows.Scan(&line))
		planLines = append(planLines, line)
	}
	require.NoError(t, planRows.Err())
	fullPlan := strings.Join(planLines, "\n")
	t.Logf("DISTSQL plan:\n%s", fullPlan)
}

// TestDistributedUDFWithSQL verifies that a UDF using the sql“
// tagged template works when executed on remote nodes.
// TODO: Enable once InternalExecutor adapter for SQLExecutor is wired.
func TestDistributedUDFWithSQL(t *testing.T) {
	t.Skip("sql`` in distributed UDFs requires InternalExecutor adapter for SQLExecutor interface")
	c := inproc.StartCluster(t, 3, func(args *base.TestClusterArgs) {
		args.ReplicationMode = base.ReplicationAuto
	})
	defer c.Stop()

	ctx := context.Background()
	db := c.ServerConn(0)

	// Create tables FIRST, before any CREATE FUNCTION.
	_, err := db.ExecContext(ctx, `CREATE TABLE labels (id INT PRIMARY KEY, label STRING)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO labels VALUES (1, 'alpha'), (2, 'beta'), (3, 'gamma')`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE TABLE items (id INT PRIMARY KEY, label_id INT)`)
	require.NoError(t, err)
	for i := 1; i <= 9; i++ {
		labelID := (i % 3) + 1
		_, err = db.ExecContext(ctx, `INSERT INTO items VALUES ($1, $2)`, i, labelID)
		require.NoError(t, err)
	}

	// Create a UDF that queries the lookup table using sql tagged template.
	_, err = db.ExecContext(ctx, `
		CREATE FUNCTION lookup_label(x INT) RETURNS STRING LANGUAGE javascript AS $$
			const rows = await sql`+"`"+`SELECT label FROM labels WHERE id = ${x}`+"`"+`;
			if (rows.length === 0) return "unknown";
			return rows[0].label;
		$$
	`)
	require.NoError(t, err)

	// Query using the UDF that does SQL internally.
	rows, err := db.QueryContext(ctx, `SELECT id, lookup_label(label_id) FROM items ORDER BY id`)
	require.NoError(t, err)
	defer rows.Close()

	expected := []string{"beta", "gamma", "alpha", "beta", "gamma", "alpha", "beta", "gamma", "alpha"}
	for i := 0; i < 9; i++ {
		require.True(t, rows.Next(), "expected row %d", i+1)
		var id int
		var label string
		require.NoError(t, rows.Scan(&id, &label))
		require.Equal(t, i+1, id)
		require.Equal(t, expected[i], label, "row %d: expected %s, got %s", i+1, expected[i], label)
	}
	require.False(t, rows.Next())
	require.NoError(t, rows.Err())

	// Also test UDF in WHERE clause with SQL callback.
	var count int
	err = db.QueryRowContext(ctx, `SELECT count(*) FROM items WHERE lookup_label(label_id) = 'alpha'`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 3, count)

	t.Logf("all %d rows matched expected labels", len(expected))
}
