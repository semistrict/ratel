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
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func makeLargeSubordinateJSONBenchDoc(targetBytes int) string {
	var b strings.Builder
	b.Grow(targetBytes + targetBytes/8)
	b.WriteString(`{"needle":{"tiny":"v"},"junk":{`)

	chunk := strings.Repeat("x", 240)
	for i := 0; b.Len() < targetBytes; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `"k%06d":"%s"`, i, chunk)
	}

	b.WriteString(`}}`)
	return b.String()
}

func makeLargeSubordinateJSONBenchNegativeIndexDoc(targetBytes int) string {
	var b strings.Builder
	b.Grow(targetBytes + targetBytes/8)
	b.WriteString(`{"needle":[{"tiny":"v"}],"junk":{`)

	chunk := strings.Repeat("x", 240)
	for i := 0; b.Len() < targetBytes; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `"k%06d":"%s"`, i, chunk)
	}

	b.WriteString(`}}`)
	return b.String()
}

func makeLargeSubordinateJSONBenchRootArrayIndexDoc(targetBytes int) string {
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

func makeLargeSubordinateJSONBenchRootObjectKeyDoc(targetBytes int) string {
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

func makeLargeSubordinateJSONBenchRootArrayAggregateDoc(targetBytes int) (string, int) {
	var b strings.Builder
	b.Grow(targetBytes + targetBytes/8)
	b.WriteByte('[')

	chunk := strings.Repeat("x", 240)
	count := 0
	for b.Len() < targetBytes {
		if count > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"test":1,"junk":"%s","i":%d}`, chunk, count)
		count++
	}

	b.WriteByte(']')
	return b.String(), count
}

func setupSubordinateJSONBenchTable(
	b *testing.B, ctx context.Context, db *sql.DB, table string, targetBytes int,
) int {
	b.Helper()

	doc := makeLargeSubordinateJSONBenchDoc(targetBytes)

	_, err := db.ExecContext(ctx, `SET distsql = always`)
	require.NoError(b, err)
	_, err = db.ExecContext(ctx, `SET vectorize = on`)
	require.NoError(b, err)
	_, err = db.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE %s (id INT PRIMARY KEY, j JSONB)`, table))
	require.NoError(b, err)
	_, err = db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s VALUES (1, $1::JSONB)`, table), doc)
	require.NoError(b, err)

	return len(doc)
}

func setupSubordinateJSONBenchTableWithDoc(
	b *testing.B, ctx context.Context, db *sql.DB, table string, doc string, vectorize string,
) int {
	b.Helper()

	_, err := db.ExecContext(ctx, `SET distsql = always`)
	require.NoError(b, err)
	_, err = db.ExecContext(ctx, fmt.Sprintf(`SET vectorize = %s`, vectorize))
	require.NoError(b, err)
	_, err = db.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE %s (id INT PRIMARY KEY, j JSONB)`, table))
	require.NoError(b, err)
	_, err = db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s VALUES (1, $1::JSONB)`, table), doc)
	require.NoError(b, err)

	return len(doc)
}

func setupSubordinateJSONBenchTableWithRows(
	b *testing.B, ctx context.Context, db *sql.DB, table string, doc string, vectorize string, rows int,
) int {
	b.Helper()

	_, err := db.ExecContext(ctx, `SET distsql = always`)
	require.NoError(b, err)
	_, err = db.ExecContext(ctx, fmt.Sprintf(`SET vectorize = %s`, vectorize))
	require.NoError(b, err)
	_, err = db.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE %s (id INT PRIMARY KEY, j JSONB)`, table))
	require.NoError(b, err)
	for i := 1; i <= rows; i++ {
		_, err = db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s VALUES ($1, $2::JSONB)`, table), i, doc)
		require.NoError(b, err)
	}

	return len(doc)
}

func benchmarkSubordinateJSONQueryRowString(
	b *testing.B, stmt *sql.Stmt, expected string, bytes int64,
) {
	b.Helper()
	b.ReportAllocs()
	b.SetBytes(bytes)
	var got string
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got = ""
		err := stmt.QueryRow().Scan(&got)
		if err != nil {
			b.Fatal(err)
		}
		if got != expected {
			b.Fatalf("unexpected result %q", got)
		}
	}
}

func benchmarkSubordinateJSONQueryRowStringPeakHeap(
	b *testing.B, stmt *sql.Stmt, expected string, bytes int64,
) {
	b.Helper()
	b.ReportAllocs()
	b.SetBytes(bytes)
	// This is a process-wide HeapAlloc sample taken around the query execution.
	// It is useful for tracking churn, but it is not the same signal as EXPLAIN
	// ANALYZE's maximum memory usage, which better captures retained query memory.

	var before runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	var got string
	var peak uint64
	for i := 0; i < b.N; i++ {
		got = ""
		done := make(chan struct{})
		go func(base uint64) {
			ticker := time.NewTicker(100 * time.Microsecond)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					var ms runtime.MemStats
					runtime.ReadMemStats(&ms)
					if ms.HeapAlloc > base {
						delta := ms.HeapAlloc - base
						for {
							prev := atomic.LoadUint64(&peak)
							if delta <= prev || atomic.CompareAndSwapUint64(&peak, prev, delta) {
								break
							}
						}
					}
				}
			}
		}(before.HeapAlloc)
		err := stmt.QueryRow().Scan(&got)
		close(done)
		if err != nil {
			b.Fatal(err)
		}
		if got != expected {
			b.Fatalf("unexpected result %q", got)
		}
	}
	b.ReportMetric(float64(atomic.LoadUint64(&peak)), "peak_heap_B")
}

func benchmarkSubordinateJSONQueryRowInt(
	b *testing.B, stmt *sql.Stmt, expected int, bytes int64,
) {
	b.Helper()
	b.ReportAllocs()
	b.SetBytes(bytes)
	var got int
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got = 0
		err := stmt.QueryRow().Scan(&got)
		if err != nil {
			b.Fatal(err)
		}
		if got != expected {
			b.Fatalf("unexpected result %d", got)
		}
	}
}

func benchmarkSubordinateJSONQueryRowIntPeakHeap(
	b *testing.B, stmt *sql.Stmt, expected int, bytes int64,
) {
	b.Helper()
	b.ReportAllocs()
	b.SetBytes(bytes)
	// This is a process-wide HeapAlloc sample taken around the query execution.
	// It is useful for tracking churn, but it is not the same signal as EXPLAIN
	// ANALYZE's maximum memory usage, which better captures retained query memory.

	var before runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	var got int
	var peak uint64
	for i := 0; i < b.N; i++ {
		got = 0
		done := make(chan struct{})
		go func(base uint64) {
			ticker := time.NewTicker(100 * time.Microsecond)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					var ms runtime.MemStats
					runtime.ReadMemStats(&ms)
					if ms.HeapAlloc > base {
						delta := ms.HeapAlloc - base
						for {
							prev := atomic.LoadUint64(&peak)
							if delta <= prev || atomic.CompareAndSwapUint64(&peak, prev, delta) {
								break
							}
						}
					}
				}
			}
		}(before.HeapAlloc)
		err := stmt.QueryRow().Scan(&got)
		close(done)
		if err != nil {
			b.Fatal(err)
		}
		if got != expected {
			b.Fatalf("unexpected result %d", got)
		}
	}
	b.ReportMetric(float64(atomic.LoadUint64(&peak)), "peak_heap_B")
}

func benchmarkSubordinateJSONExecPeakHeap(
	b *testing.B,
	stmt *sql.Stmt,
	resetStmt *sql.Stmt,
	execArgs []any,
	resetArgs []any,
	bytes int64,
) {
	b.Helper()
	b.ReportAllocs()
	b.SetBytes(bytes)
	// This is a process-wide HeapAlloc sample taken around the statement
	// execution. It is useful for tracking churn, not retained query memory.

	var peak uint64
	for i := 0; i < b.N; i++ {
		runtime.GC()
		var before runtime.MemStats
		runtime.ReadMemStats(&before)

		done := make(chan struct{})
		go func(base uint64) {
			ticker := time.NewTicker(100 * time.Microsecond)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					var ms runtime.MemStats
					runtime.ReadMemStats(&ms)
					if ms.HeapAlloc > base {
						delta := ms.HeapAlloc - base
						for {
							prev := atomic.LoadUint64(&peak)
							if delta <= prev || atomic.CompareAndSwapUint64(&peak, prev, delta) {
								break
							}
						}
					}
				}
			}
		}(before.HeapAlloc)

		res, err := stmt.Exec(execArgs...)
		close(done)
		if err != nil {
			b.Fatal(err)
		}
		rows, err := res.RowsAffected()
		if err != nil {
			b.Fatal(err)
		}
		if rows != 1 {
			b.Fatalf("unexpected rows affected %d", rows)
		}

		b.StopTimer()
		_, err = resetStmt.Exec(resetArgs...)
		if err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
	}
	b.ReportMetric(float64(atomic.LoadUint64(&peak)), "peak_heap_B")
}

// BenchmarkSubordinateJSONLargeRow compares tiny scan-local JSON reads against
// full JSON projection as row size grows. This is intended to show whether
// path-local reads stay allocation-bounded relative to row size; it does not
// claim that a 1 GiB row is safe in the default test environment.
func BenchmarkSubordinateJSONLargeRow(b *testing.B) {
	for _, tc := range []struct {
		name        string
		targetBytes int
	}{
		{name: "64KiB", targetBytes: 64 << 10},
		{name: "1MiB", targetBytes: 1 << 20},
		{name: "8MiB", targetBytes: 8 << 20},
	} {
		b.Run(tc.name, func(b *testing.B) {
			c, ctx := benchCluster(b)
			db := c.ServerConn(0)
			table := fmt.Sprintf("bench_json_%d", tc.targetBytes)
			docLen := setupSubordinateJSONBenchTable(b, ctx, db, table, tc.targetBytes)

			pathProjection, err := db.PrepareContext(ctx,
				fmt.Sprintf(`SELECT j->'needle'->>'tiny' FROM %s WHERE id = 1`, table))
			require.NoError(b, err)
			b.Cleanup(func() { _ = pathProjection.Close() })

			pathFilterOnly, err := db.PrepareContext(ctx,
				fmt.Sprintf(`SELECT id FROM %s WHERE id = 1 AND j->'needle'->>'tiny' = 'v'`, table))
			require.NoError(b, err)
			b.Cleanup(func() { _ = pathFilterOnly.Close() })

			fullProjection, err := db.PrepareContext(ctx,
				fmt.Sprintf(`SELECT j::STRING FROM %s WHERE id = 1`, table))
			require.NoError(b, err)
			b.Cleanup(func() { _ = fullProjection.Close() })
			var fullProjectionExpected string
			require.NoError(b, fullProjection.QueryRow().Scan(&fullProjectionExpected))

			b.Run("PathProjection", func(b *testing.B) {
				benchmarkSubordinateJSONQueryRowString(b, pathProjection, "v", int64(docLen))
			})
			b.Run("PathFilterOnly", func(b *testing.B) {
				benchmarkSubordinateJSONQueryRowInt(b, pathFilterOnly, 1, int64(docLen))
			})
			b.Run("FullProjection", func(b *testing.B) {
				benchmarkSubordinateJSONQueryRowString(b, fullProjection, fullProjectionExpected, int64(docLen))
			})
		})
	}
}

// BenchmarkSubordinateJSONLargeRowFullScan exercises the vectorized full-scan
// path, including a negative-index lookup that depends on ancestor container
// metadata arriving before descendants in scan order.
func BenchmarkSubordinateJSONLargeRowFullScan(b *testing.B) {
	for _, tc := range []struct {
		name        string
		targetBytes int
	}{
		{name: "1MiB", targetBytes: 1 << 20},
		{name: "8MiB", targetBytes: 8 << 20},
	} {
		b.Run(tc.name, func(b *testing.B) {
			for _, vectorize := range []string{"on", "off"} {
				b.Run("vectorize="+vectorize, func(b *testing.B) {
					c, ctx := benchCluster(b)
					db := c.ServerConn(0)
					table := fmt.Sprintf("bench_json_fullscan_%d_%s", tc.targetBytes, vectorize)
					doc := makeLargeSubordinateJSONBenchNegativeIndexDoc(tc.targetBytes)
					docLen := setupSubordinateJSONBenchTableWithDoc(b, ctx, db, table, doc, vectorize)

					pathProjection, err := db.PrepareContext(ctx,
						fmt.Sprintf(`SELECT j->'needle'->-1->>'tiny' FROM %s`, table))
					require.NoError(b, err)
					b.Cleanup(func() { _ = pathProjection.Close() })

					pathFilterOnly, err := db.PrepareContext(ctx,
						fmt.Sprintf(`SELECT id FROM %s WHERE j->'needle'->-1->>'tiny' = 'v'`, table))
					require.NoError(b, err)
					b.Cleanup(func() { _ = pathFilterOnly.Close() })

					b.Run("PathProjection", func(b *testing.B) {
						benchmarkSubordinateJSONQueryRowString(b, pathProjection, "v", int64(docLen))
					})
					b.Run("PathFilterOnly", func(b *testing.B) {
						benchmarkSubordinateJSONQueryRowInt(b, pathFilterOnly, 1, int64(docLen))
					})
					b.Run("PathFilterOnlyPeakHeap", func(b *testing.B) {
						benchmarkSubordinateJSONQueryRowIntPeakHeap(b, pathFilterOnly, 1, int64(docLen))
					})
				})
			}
		})
	}
}

// BenchmarkSubordinateJSONLargeRootArrayIndex tracks a fixed array-index path
// read over a root JSON array with many elements. This is the direct benchmark
// for queries of the form j->10->>'test'.
func BenchmarkSubordinateJSONLargeRootArrayIndex(b *testing.B) {
	for _, tc := range []struct {
		name        string
		targetBytes int
	}{
		{name: "1MiB", targetBytes: 1 << 20},
		{name: "8MiB", targetBytes: 8 << 20},
	} {
		b.Run(tc.name, func(b *testing.B) {
			c, ctx := benchCluster(b)
			db := c.ServerConn(0)
			table := fmt.Sprintf("bench_json_root_array_%d", tc.targetBytes)
			doc := makeLargeSubordinateJSONBenchRootArrayIndexDoc(tc.targetBytes)
			docLen := setupSubordinateJSONBenchTableWithDoc(b, ctx, db, table, doc, "on")

			pathProjection, err := db.PrepareContext(ctx,
				fmt.Sprintf(`SELECT j->10->>'test' FROM %s WHERE id = 1`, table))
			require.NoError(b, err)
			b.Cleanup(func() { _ = pathProjection.Close() })

			pathFilterOnly, err := db.PrepareContext(ctx,
				fmt.Sprintf(`SELECT id FROM %s WHERE id = 1 AND j->10->>'test' = 'v'`, table))
			require.NoError(b, err)
			b.Cleanup(func() { _ = pathFilterOnly.Close() })

			b.Run("PathProjection", func(b *testing.B) {
				benchmarkSubordinateJSONQueryRowString(b, pathProjection, "v", int64(docLen))
			})
			b.Run("PathProjectionPeakHeap", func(b *testing.B) {
				benchmarkSubordinateJSONQueryRowStringPeakHeap(b, pathProjection, "v", int64(docLen))
			})
			b.Run("PathFilterOnly", func(b *testing.B) {
				benchmarkSubordinateJSONQueryRowInt(b, pathFilterOnly, 1, int64(docLen))
			})
			b.Run("PathFilterOnlyPeakHeap", func(b *testing.B) {
				benchmarkSubordinateJSONQueryRowIntPeakHeap(b, pathFilterOnly, 1, int64(docLen))
			})
		})
	}
}

// BenchmarkSubordinateJSONLargeRootArrayAggregateField tracks aggregation over
// a huge root JSON array of objects using jsonb_array_elements, i.e. queries of
// the form SELECT sum((elem.value->'test')::INT) FROM t, LATERAL
// jsonb_array_elements(j) AS elem(value). The PeakHeap variant samples
// process-wide HeapAlloc; retained query memory is covered by
// TestSubordinateJSONLargeRootArrayAggregateFieldAnalyzeMemoryRoughlyConstant.
func BenchmarkSubordinateJSONLargeRootArrayAggregateField(b *testing.B) {
	for _, tc := range []struct {
		name        string
		targetBytes int
	}{
		{name: "1MiB", targetBytes: 1 << 20},
		{name: "8MiB", targetBytes: 8 << 20},
	} {
		b.Run(tc.name, func(b *testing.B) {
			for _, vectorize := range []string{"on", "off"} {
				b.Run("vectorize="+vectorize, func(b *testing.B) {
					c, ctx := benchCluster(b)
					db := c.ServerConn(0)
					table := fmt.Sprintf("bench_json_root_array_agg_%d_%s", tc.targetBytes, vectorize)
					doc, expected := makeLargeSubordinateJSONBenchRootArrayAggregateDoc(tc.targetBytes)
					docLen := setupSubordinateJSONBenchTableWithDoc(b, ctx, db, table, doc, vectorize)

					sumStmt, err := db.PrepareContext(ctx, fmt.Sprintf(`
						SELECT COALESCE(sum((elem.value->'test')::INT), 0)::INT
						FROM %s, LATERAL jsonb_array_elements(j) AS elem(value)
						WHERE id = 1
					`, table))
					require.NoError(b, err)
					b.Cleanup(func() { _ = sumStmt.Close() })

					b.Run("SumField", func(b *testing.B) {
						benchmarkSubordinateJSONQueryRowInt(b, sumStmt, expected, int64(docLen))
					})
					b.Run("SumFieldPeakHeap", func(b *testing.B) {
						benchmarkSubordinateJSONQueryRowIntPeakHeap(b, sumStmt, expected, int64(docLen))
					})
				})
			}
		})
	}
}

// BenchmarkSubordinateJSONLargeRootArrayAppendOneElement tracks appending one
// JSON array element to a huge existing root array. The reset statement runs
// outside the measured region so peak heap reflects the append itself.
func BenchmarkSubordinateJSONLargeRootArrayAppendOneElement(b *testing.B) {
	for _, tc := range []struct {
		name        string
		targetBytes int
	}{
		{name: "1MiB", targetBytes: 1 << 20},
		{name: "8MiB", targetBytes: 8 << 20},
	} {
		b.Run(tc.name, func(b *testing.B) {
			c, ctx := benchCluster(b)
			db := c.ServerConn(0)
			table := fmt.Sprintf("bench_json_root_array_append_%d", tc.targetBytes)
			doc := makeLargeSubordinateJSONBenchRootArrayIndexDoc(tc.targetBytes)
			docLen := setupSubordinateJSONBenchTableWithDoc(b, ctx, db, table, doc, "on")

			appendStmt, err := db.PrepareContext(ctx,
				fmt.Sprintf(`UPDATE %s SET j = j || $1::JSONB WHERE id = 1`, table))
			require.NoError(b, err)
			b.Cleanup(func() { _ = appendStmt.Close() })

			resetStmt, err := db.PrepareContext(ctx,
				fmt.Sprintf(`UPDATE %s SET j = $1::JSONB WHERE id = 1`, table))
			require.NoError(b, err)
			b.Cleanup(func() { _ = resetStmt.Close() })

			appendArg := `[{"test":"appended"}]`
			resetArg := doc

			// Sanity-check the update shape once before measuring.
			_, err = appendStmt.ExecContext(ctx, appendArg)
			require.NoError(b, err)
			_, err = resetStmt.ExecContext(ctx, resetArg)
			require.NoError(b, err)

			b.Run("AppendOneElementPeakHeap", func(b *testing.B) {
				benchmarkSubordinateJSONExecPeakHeap(
					b,
					appendStmt,
					resetStmt,
					[]any{appendArg},
					[]any{resetArg},
					int64(docLen),
				)
			})
		})
	}
}

// BenchmarkSubordinateJSONLargeRootArrayDeleteLastElement tracks deleting the
// last element from a huge existing root array. The reset statement runs
// outside the measured region so peak heap reflects the delete itself.
func BenchmarkSubordinateJSONLargeRootArrayDeleteLastElement(b *testing.B) {
	for _, tc := range []struct {
		name        string
		targetBytes int
	}{
		{name: "1MiB", targetBytes: 1 << 20},
		{name: "8MiB", targetBytes: 8 << 20},
	} {
		b.Run(tc.name, func(b *testing.B) {
			c, ctx := benchCluster(b)
			db := c.ServerConn(0)
			table := fmt.Sprintf("bench_json_root_array_delete_%d", tc.targetBytes)
			doc := makeLargeSubordinateJSONBenchRootArrayIndexDoc(tc.targetBytes)
			docLen := setupSubordinateJSONBenchTableWithDoc(b, ctx, db, table, doc, "on")

			deleteStmt, err := db.PrepareContext(ctx,
				fmt.Sprintf(`UPDATE %s SET j = j - (jsonb_array_length(j) - 1) WHERE id = 1`, table))
			require.NoError(b, err)
			b.Cleanup(func() { _ = deleteStmt.Close() })

			resetStmt, err := db.PrepareContext(ctx,
				fmt.Sprintf(`UPDATE %s SET j = $1::JSONB WHERE id = 1`, table))
			require.NoError(b, err)
			b.Cleanup(func() { _ = resetStmt.Close() })

			// Sanity-check the update shape once before measuring.
			_, err = deleteStmt.ExecContext(ctx)
			require.NoError(b, err)
			_, err = resetStmt.ExecContext(ctx, doc)
			require.NoError(b, err)

			b.Run("DeleteLastElementPeakHeap", func(b *testing.B) {
				benchmarkSubordinateJSONExecPeakHeap(
					b,
					deleteStmt,
					resetStmt,
					nil,
					[]any{doc},
					int64(docLen),
				)
			})
		})
	}
}

// BenchmarkSubordinateJSONLargeRootArrayUpdateElement tracks updating one
// existing nested field inside a huge existing root array.
func BenchmarkSubordinateJSONLargeRootArrayUpdateElement(b *testing.B) {
	for _, tc := range []struct {
		name        string
		targetBytes int
	}{
		{name: "1MiB", targetBytes: 1 << 20},
		{name: "8MiB", targetBytes: 8 << 20},
	} {
		b.Run(tc.name, func(b *testing.B) {
			c, ctx := benchCluster(b)
			db := c.ServerConn(0)
			table := fmt.Sprintf("bench_json_root_array_update_%d", tc.targetBytes)
			doc := makeLargeSubordinateJSONBenchRootArrayIndexDoc(tc.targetBytes)
			docLen := setupSubordinateJSONBenchTableWithDoc(b, ctx, db, table, doc, "on")

			updateStmt, err := db.PrepareContext(ctx,
				fmt.Sprintf(`UPDATE %s SET j = jsonb_set(j, '{10,test}', '"updated"'::JSONB, false) WHERE id = 1`, table))
			require.NoError(b, err)
			b.Cleanup(func() { _ = updateStmt.Close() })

			resetStmt, err := db.PrepareContext(ctx,
				fmt.Sprintf(`UPDATE %s SET j = $1::JSONB WHERE id = 1`, table))
			require.NoError(b, err)
			b.Cleanup(func() { _ = resetStmt.Close() })

			_, err = updateStmt.ExecContext(ctx)
			require.NoError(b, err)
			_, err = resetStmt.ExecContext(ctx, doc)
			require.NoError(b, err)

			b.Run("UpdateElementPeakHeap", func(b *testing.B) {
				benchmarkSubordinateJSONExecPeakHeap(
					b,
					updateStmt,
					resetStmt,
					nil,
					[]any{doc},
					int64(docLen),
				)
			})
		})
	}
}

// BenchmarkSubordinateJSONLargeRootObjectKey tracks a fixed root-object key
// read over a huge JSON object with many sibling keys.
func BenchmarkSubordinateJSONLargeRootObjectKey(b *testing.B) {
	for _, tc := range []struct {
		name        string
		targetBytes int
	}{
		{name: "1MiB", targetBytes: 1 << 20},
		{name: "8MiB", targetBytes: 8 << 20},
	} {
		b.Run(tc.name, func(b *testing.B) {
			c, ctx := benchCluster(b)
			db := c.ServerConn(0)
			table := fmt.Sprintf("bench_json_root_object_%d", tc.targetBytes)
			doc := makeLargeSubordinateJSONBenchRootObjectKeyDoc(tc.targetBytes)
			docLen := setupSubordinateJSONBenchTableWithDoc(b, ctx, db, table, doc, "on")

			pathProjection, err := db.PrepareContext(ctx,
				fmt.Sprintf(`SELECT j->>'test' FROM %s WHERE id = 1`, table))
			require.NoError(b, err)
			b.Cleanup(func() { _ = pathProjection.Close() })

			pathFilterOnly, err := db.PrepareContext(ctx,
				fmt.Sprintf(`SELECT id FROM %s WHERE id = 1 AND j->>'test' = 'v'`, table))
			require.NoError(b, err)
			b.Cleanup(func() { _ = pathFilterOnly.Close() })

			b.Run("PathProjection", func(b *testing.B) {
				benchmarkSubordinateJSONQueryRowString(b, pathProjection, "v", int64(docLen))
			})
			b.Run("PathProjectionPeakHeap", func(b *testing.B) {
				benchmarkSubordinateJSONQueryRowStringPeakHeap(b, pathProjection, "v", int64(docLen))
			})
			b.Run("PathFilterOnly", func(b *testing.B) {
				benchmarkSubordinateJSONQueryRowInt(b, pathFilterOnly, 1, int64(docLen))
			})
			b.Run("PathFilterOnlyPeakHeap", func(b *testing.B) {
				benchmarkSubordinateJSONQueryRowIntPeakHeap(b, pathFilterOnly, 1, int64(docLen))
			})
		})
	}
}

// BenchmarkSubordinateJSONLargeRootObjectAppendOneKey tracks adding one
// top-level key to a huge existing root object.
func BenchmarkSubordinateJSONLargeRootObjectAppendOneKey(b *testing.B) {
	for _, tc := range []struct {
		name        string
		targetBytes int
	}{
		{name: "1MiB", targetBytes: 1 << 20},
		{name: "8MiB", targetBytes: 8 << 20},
	} {
		b.Run(tc.name, func(b *testing.B) {
			c, ctx := benchCluster(b)
			db := c.ServerConn(0)
			table := fmt.Sprintf("bench_json_root_object_append_%d", tc.targetBytes)
			doc := makeLargeSubordinateJSONBenchRootObjectKeyDoc(tc.targetBytes)
			docLen := setupSubordinateJSONBenchTableWithDoc(b, ctx, db, table, doc, "on")

			appendStmt, err := db.PrepareContext(ctx,
				fmt.Sprintf(`UPDATE %s SET j = j || $1::JSONB WHERE id = 1`, table))
			require.NoError(b, err)
			b.Cleanup(func() { _ = appendStmt.Close() })

			resetStmt, err := db.PrepareContext(ctx,
				fmt.Sprintf(`UPDATE %s SET j = $1::JSONB WHERE id = 1`, table))
			require.NoError(b, err)
			b.Cleanup(func() { _ = resetStmt.Close() })

			appendArg := `{"appended":"v"}`
			resetArg := doc

			_, err = appendStmt.ExecContext(ctx, appendArg)
			require.NoError(b, err)
			_, err = resetStmt.ExecContext(ctx, resetArg)
			require.NoError(b, err)

			b.Run("AppendOneKeyPeakHeap", func(b *testing.B) {
				benchmarkSubordinateJSONExecPeakHeap(
					b,
					appendStmt,
					resetStmt,
					[]any{appendArg},
					[]any{resetArg},
					int64(docLen),
				)
			})
		})
	}
}

// BenchmarkSubordinateJSONLargeRootObjectDeleteOneKey tracks deleting one
// top-level key from a huge existing root object.
func BenchmarkSubordinateJSONLargeRootObjectDeleteOneKey(b *testing.B) {
	for _, tc := range []struct {
		name        string
		targetBytes int
	}{
		{name: "1MiB", targetBytes: 1 << 20},
		{name: "8MiB", targetBytes: 8 << 20},
	} {
		b.Run(tc.name, func(b *testing.B) {
			c, ctx := benchCluster(b)
			db := c.ServerConn(0)
			table := fmt.Sprintf("bench_json_root_object_delete_%d", tc.targetBytes)
			doc := makeLargeSubordinateJSONBenchRootObjectKeyDoc(tc.targetBytes)
			docLen := setupSubordinateJSONBenchTableWithDoc(b, ctx, db, table, doc, "on")

			deleteStmt, err := db.PrepareContext(ctx,
				fmt.Sprintf(`UPDATE %s SET j = j - 'tail_delete' WHERE id = 1`, table))
			require.NoError(b, err)
			b.Cleanup(func() { _ = deleteStmt.Close() })

			resetStmt, err := db.PrepareContext(ctx,
				fmt.Sprintf(`UPDATE %s SET j = $1::JSONB WHERE id = 1`, table))
			require.NoError(b, err)
			b.Cleanup(func() { _ = resetStmt.Close() })

			_, err = deleteStmt.ExecContext(ctx)
			require.NoError(b, err)
			_, err = resetStmt.ExecContext(ctx, doc)
			require.NoError(b, err)

			b.Run("DeleteOneKeyPeakHeap", func(b *testing.B) {
				benchmarkSubordinateJSONExecPeakHeap(
					b,
					deleteStmt,
					resetStmt,
					nil,
					[]any{doc},
					int64(docLen),
				)
			})
		})
	}
}

// BenchmarkSubordinateJSONLargeRootObjectUpdateKey tracks updating one existing
// top-level key inside a huge existing root object.
func BenchmarkSubordinateJSONLargeRootObjectUpdateKey(b *testing.B) {
	for _, tc := range []struct {
		name        string
		targetBytes int
	}{
		{name: "1MiB", targetBytes: 1 << 20},
		{name: "8MiB", targetBytes: 8 << 20},
	} {
		b.Run(tc.name, func(b *testing.B) {
			c, ctx := benchCluster(b)
			db := c.ServerConn(0)
			table := fmt.Sprintf("bench_json_root_object_update_%d", tc.targetBytes)
			doc := makeLargeSubordinateJSONBenchRootObjectKeyDoc(tc.targetBytes)
			docLen := setupSubordinateJSONBenchTableWithDoc(b, ctx, db, table, doc, "on")

			updateStmt, err := db.PrepareContext(ctx,
				fmt.Sprintf(`UPDATE %s SET j = jsonb_set(j, '{test}', '"updated"'::JSONB, false) WHERE id = 1`, table))
			require.NoError(b, err)
			b.Cleanup(func() { _ = updateStmt.Close() })

			resetStmt, err := db.PrepareContext(ctx,
				fmt.Sprintf(`UPDATE %s SET j = $1::JSONB WHERE id = 1`, table))
			require.NoError(b, err)
			b.Cleanup(func() { _ = resetStmt.Close() })

			_, err = updateStmt.ExecContext(ctx)
			require.NoError(b, err)
			_, err = resetStmt.ExecContext(ctx, doc)
			require.NoError(b, err)

			b.Run("UpdateKeyPeakHeap", func(b *testing.B) {
				benchmarkSubordinateJSONExecPeakHeap(
					b,
					updateStmt,
					resetStmt,
					nil,
					[]any{doc},
					int64(docLen),
				)
			})
		})
	}
}

// BenchmarkSubordinateJSONSkippedShapes tracks shapes that the tiny-read
// row-head optimization originally skipped so we can widen support with a
// concrete baseline for each one.
func BenchmarkSubordinateJSONSkippedShapes(b *testing.B) {
	for _, tc := range []struct {
		name        string
		targetBytes int
	}{
		{name: "1MiB", targetBytes: 1 << 20},
		{name: "8MiB", targetBytes: 8 << 20},
	} {
		b.Run(tc.name, func(b *testing.B) {
			for _, vectorize := range []string{"on", "off"} {
				b.Run("vectorize="+vectorize, func(b *testing.B) {
					doc := makeLargeSubordinateJSONBenchNegativeIndexDoc(tc.targetBytes)

					b.Run("ReversePathFilterOnly", func(b *testing.B) {
						c, ctx := benchCluster(b)
						db := c.ServerConn(0)
						table := fmt.Sprintf("bench_json_reverse_%d_%s", tc.targetBytes, vectorize)
						docLen := setupSubordinateJSONBenchTableWithRows(b, ctx, db, table, doc, vectorize, 2)

						stmt, err := db.PrepareContext(ctx,
							fmt.Sprintf(`SELECT id FROM %s WHERE j->'needle'->-1->>'tiny' = 'v' ORDER BY id DESC LIMIT 1`, table))
						require.NoError(b, err)
						b.Cleanup(func() { _ = stmt.Close() })

						benchmarkSubordinateJSONQueryRowInt(b, stmt, 2, int64(docLen))
					})

					b.Run("ReversePathFilterOnlyPeakHeap", func(b *testing.B) {
						c, ctx := benchCluster(b)
						db := c.ServerConn(0)
						table := fmt.Sprintf("bench_json_reverse_heap_%d_%s", tc.targetBytes, vectorize)
						docLen := setupSubordinateJSONBenchTableWithRows(b, ctx, db, table, doc, vectorize, 2)

						stmt, err := db.PrepareContext(ctx,
							fmt.Sprintf(`SELECT id FROM %s WHERE j->'needle'->-1->>'tiny' = 'v' ORDER BY id DESC LIMIT 1`, table))
						require.NoError(b, err)
						b.Cleanup(func() { _ = stmt.Close() })

						benchmarkSubordinateJSONQueryRowIntPeakHeap(b, stmt, 2, int64(docLen))
					})

					b.Run("ExistsFilterOnly", func(b *testing.B) {
						c, ctx := benchCluster(b)
						db := c.ServerConn(0)
						table := fmt.Sprintf("bench_json_exists_%d_%s", tc.targetBytes, vectorize)
						docLen := setupSubordinateJSONBenchTableWithDoc(b, ctx, db, table, doc, vectorize)

						stmt, err := db.PrepareContext(ctx,
							fmt.Sprintf(`SELECT id FROM %s WHERE j ? 'needle'`, table))
						require.NoError(b, err)
						b.Cleanup(func() { _ = stmt.Close() })

						benchmarkSubordinateJSONQueryRowInt(b, stmt, 1, int64(docLen))
					})

					b.Run("ExistsFilterOnlyPeakHeap", func(b *testing.B) {
						c, ctx := benchCluster(b)
						db := c.ServerConn(0)
						table := fmt.Sprintf("bench_json_exists_heap_%d_%s", tc.targetBytes, vectorize)
						docLen := setupSubordinateJSONBenchTableWithDoc(b, ctx, db, table, doc, vectorize)

						stmt, err := db.PrepareContext(ctx,
							fmt.Sprintf(`SELECT id FROM %s WHERE j ? 'needle'`, table))
						require.NoError(b, err)
						b.Cleanup(func() { _ = stmt.Close() })

						benchmarkSubordinateJSONQueryRowIntPeakHeap(b, stmt, 1, int64(docLen))
					})

				})
			}
		})
	}
}
