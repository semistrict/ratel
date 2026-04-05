package inproc_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/testutils/inproc"
	"github.com/stretchr/testify/require"
)

// benchCluster creates a single-node cluster for benchmarking.
// The cluster is created once per top-level benchmark function.
func benchCluster(b *testing.B) (*inproc.Cluster, context.Context) {
	b.Helper()
	c := inproc.StartCluster(b, 1)
	b.Cleanup(func() { c.Stop() })
	return c, context.Background()
}

// BenchmarkUDF runs all UDF benchmarks against a shared cluster.
func BenchmarkUDF(b *testing.B) {
	c, ctx := benchCluster(b)
	db := c.ServerConn(0)

	// Set up tables used by multiple sub-benchmarks.
	_, err := db.ExecContext(ctx, `CREATE TABLE bench_lookup (id INT PRIMARY KEY, val INT)`)
	require.NoError(b, err)
	_, err = db.ExecContext(ctx, `INSERT INTO bench_lookup VALUES (1, 100), (2, 200), (3, 300)`)
	require.NoError(b, err)

	_, err = db.ExecContext(ctx, `CREATE TABLE bench_labels (id INT PRIMARY KEY, label STRING)`)
	require.NoError(b, err)
	_, err = db.ExecContext(ctx, `INSERT INTO bench_labels VALUES (1, 'alpha'), (2, 'beta'), (3, 'gamma')`)
	require.NoError(b, err)

	_, err = db.ExecContext(ctx, `CREATE TABLE bench_items (id INT PRIMARY KEY, label_id INT)`)
	require.NoError(b, err)
	for i := 1; i <= 100; i++ {
		_, err = db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO bench_items VALUES (%d, %d)`, i, (i%3)+1))
		require.NoError(b, err)
	}

	// Create UDFs.
	_, err = db.ExecContext(ctx, `
		CREATE FUNCTION bench_double(x INT) RETURNS INT LANGUAGE javascript AS $$
			return x * 2;
		$$
	`)
	require.NoError(b, err)

	// TODO(ramon): Enable sql`` benchmarks once InternalExecutor adapter
	// for SQLExecutor is wired through the pgwire evaluation path.

	// Warm up.
	var intResult int
	var strResult string
	db.QueryRowContext(ctx, `SELECT bench_double(1)`).Scan(&intResult)

	// --- Benchmarks ---

	b.Run("BaselineSQL", func(b *testing.B) {
		// Plain SQL query, no UDF. Measures pgwire + parse + plan + execute.
		for i := 0; i < b.N; i++ {
			err := db.QueryRowContext(ctx, `SELECT val FROM bench_lookup WHERE id = $1`, (i%3)+1).Scan(&intResult)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("PureComputation", func(b *testing.B) {
		// UDF with no SQL callback. Measures SQL overhead + V8 call overhead.
		for i := 0; i < b.N; i++ {
			err := db.QueryRowContext(ctx, `SELECT bench_double($1)`, i).Scan(&intResult)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("BaselineScan/100rows", func(b *testing.B) {
		// Scan 100 rows with no UDF.
		for i := 0; i < b.N; i++ {
			rows, err := db.QueryContext(ctx, `SELECT id, label_id FROM bench_items`)
			if err != nil {
				b.Fatal(err)
			}
			n := 0
			for rows.Next() {
				var id, labelID int
				if err := rows.Scan(&id, &labelID); err != nil {
					b.Fatal(err)
				}
				n++
			}
			rows.Close()
			if n != 100 {
				b.Fatalf("expected 100 rows, got %d", n)
			}
		}
	})

	b.Run("UDFScan/100rows", func(b *testing.B) {
		// Scan 100 rows with pure-computation UDF (no sql`` callback).
		for i := 0; i < b.N; i++ {
			rows, err := db.QueryContext(ctx, `SELECT bench_double(label_id) FROM bench_items`)
			if err != nil {
				b.Fatal(err)
			}
			n := 0
			for rows.Next() {
				if err := rows.Scan(&intResult); err != nil {
					b.Fatal(err)
				}
				n++
			}
			rows.Close()
			if n != 100 {
				b.Fatalf("expected 100 rows, got %d", n)
			}
		}
	})

	b.Run("BaselineJoin/100rows", func(b *testing.B) {
		// Join 100 rows (no UDF). Comparison for sql`` callback pattern.
		for i := 0; i < b.N; i++ {
			rows, err := db.QueryContext(ctx,
				`SELECT l.label FROM bench_items i JOIN bench_labels l ON i.label_id = l.id`)
			if err != nil {
				b.Fatal(err)
			}
			n := 0
			for rows.Next() {
				if err := rows.Scan(&strResult); err != nil {
					b.Fatal(err)
				}
				n++
			}
			rows.Close()
			if n != 100 {
				b.Fatalf("expected 100 rows, got %d", n)
			}
		}
	})

	_ = strResult
}
