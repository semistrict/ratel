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
				fmt.Sprintf(`SELECT j#>>'{needle,tiny}' FROM %s WHERE id = 1`, table))
			require.NoError(b, err)
			b.Cleanup(func() { _ = pathProjection.Close() })

			pathFilterOnly, err := db.PrepareContext(ctx,
				fmt.Sprintf(`SELECT id FROM %s WHERE id = 1 AND j#>>'{needle,tiny}' = 'v'`, table))
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
						fmt.Sprintf(`SELECT j#>>'{needle,-1,tiny}' FROM %s`, table))
					require.NoError(b, err)
					b.Cleanup(func() { _ = pathProjection.Close() })

					pathFilterOnly, err := db.PrepareContext(ctx,
						fmt.Sprintf(`SELECT id FROM %s WHERE j#>>'{needle,-1,tiny}' = 'v'`, table))
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
