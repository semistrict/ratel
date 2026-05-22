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
	"database/sql"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/sql/row"
	"github.com/cockroachdb/cockroach/pkg/testutils/inproc/internal/planassert"
	"github.com/stretchr/testify/require"
)

func measureQueryRowIntPeakHeap(t *testing.T, stmt *sql.Stmt, expected int) uint64 {
	t.Helper()

	var warmup int
	require.NoError(t, stmt.QueryRow().Scan(&warmup))
	require.Equal(t, expected, warmup)

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	var got int
	var peak uint64
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
	require.NoError(t, err)
	require.Equal(t, expected, got)
	return atomic.LoadUint64(&peak)
}

func TestSubordinateJSONLargeRootArrayAggregateFieldDoesNotMaterializeWholeJSON(t *testing.T) {
	for _, vectorizeMode := range []string{"off", "on"} {
		t.Run("vectorize="+vectorizeMode, func(t *testing.T) {
			ctx, c, db := startSubordinateJSONCluster(t, vectorizeMode)
			defer c.Stop()

			doc, expected := makeLargeSubordinateJSONBenchRootArrayAggregateDoc(1 << 20)
			_, err := db.ExecContext(ctx, `CREATE TABLE agg_materialize (id INT PRIMARY KEY, j JSONB)`)
			require.NoError(t, err)
			_, err = db.ExecContext(ctx, `INSERT INTO agg_materialize VALUES (1, $1::JSONB)`, doc)
			require.NoError(t, err)

			var fullMaterializeCalls int64
			prevHook := row.TestingSubordinateJSONFullValueMaterializeHook
			row.TestingSubordinateJSONFullValueMaterializeHook = func() {
				atomic.AddInt64(&fullMaterializeCalls, 1)
			}
			var builderMaterializeCalls int64
			prevBuilderHook := row.TestingSubordinateJSONBuilderMaterializeHook
			row.TestingSubordinateJSONBuilderMaterializeHook = func() {
				atomic.AddInt64(&builderMaterializeCalls, 1)
			}
			var lazyDecodeCalls int64
			prevLazyHook := row.TestingSubordinateJSONLazyRootDecodeHook
			row.TestingSubordinateJSONLazyRootDecodeHook = func() {
				atomic.AddInt64(&lazyDecodeCalls, 1)
			}
			var rootIndexFetchCalls int64
			prevIndexHook := row.TestingSubordinateJSONLazyRootIndexFetchHook
			row.TestingSubordinateJSONLazyRootIndexFetchHook = func() {
				atomic.AddInt64(&rootIndexFetchCalls, 1)
			}
			defer func() {
				row.TestingSubordinateJSONFullValueMaterializeHook = prevHook
				row.TestingSubordinateJSONBuilderMaterializeHook = prevBuilderHook
				row.TestingSubordinateJSONLazyRootDecodeHook = prevLazyHook
				row.TestingSubordinateJSONLazyRootIndexFetchHook = prevIndexHook
			}()

			var got int
			err = db.QueryRowContext(ctx, `
				SELECT COALESCE(sum((elem.value->'test')::INT), 0)::INT
				FROM agg_materialize, LATERAL jsonb_array_elements(j) AS elem(value)
				WHERE id = 1
			`).Scan(&got)
			require.NoError(t, err)
			require.Equal(t, expected, got)
			require.Zero(t, atomic.LoadInt64(&fullMaterializeCalls),
				"aggregating over jsonb_array_elements(j) should stream array elements without materializing the whole JSON subtree")
			require.Zero(t, atomic.LoadInt64(&builderMaterializeCalls),
				"aggregating over jsonb_array_elements(j) and reading elem.value->'test' should not rebuild each element object")
			require.Zero(t, atomic.LoadInt64(&lazyDecodeCalls),
				"aggregating over jsonb_array_elements(j) should not fall back to decoding the full lazy root array")
			require.Zero(t, atomic.LoadInt64(&rootIndexFetchCalls),
				"aggregating over jsonb_array_elements(j) should stream the root array instead of doing per-index lookups")
		})
	}
}

func TestSubordinateJSONLargeRootArrayAggregateFieldAnalyzeMemoryRoughlyConstant(t *testing.T) {
	for _, vectorizeMode := range []string{"off", "on"} {
		t.Run("vectorize="+vectorizeMode, func(t *testing.T) {
			var smallMem, largeMem int64
			for _, tc := range []struct {
				name        string
				targetBytes int
			}{
				{name: "1MiB", targetBytes: 1 << 20},
				{name: "8MiB", targetBytes: 8 << 20},
			} {
				t.Run(tc.name, func(t *testing.T) {
					ctx, c, db := startSubordinateJSONCluster(t, vectorizeMode)
					defer c.Stop()

					doc, expected := makeLargeSubordinateJSONBenchRootArrayAggregateDoc(tc.targetBytes)
					_, err := db.ExecContext(ctx, `CREATE TABLE agg_mem (id INT PRIMARY KEY, j JSONB)`)
					require.NoError(t, err)
					_, err = db.ExecContext(ctx, `INSERT INTO agg_mem VALUES (1, $1::JSONB)`, doc)
					require.NoError(t, err)

					query := `SELECT COALESCE(sum((elem.value->'test')::INT), 0)::INT
						FROM agg_mem, LATERAL jsonb_array_elements(j) AS elem(value)
						WHERE id = 1`
					var got int
					require.NoError(t, db.QueryRowContext(ctx, query).Scan(&got))
					require.Equal(t, expected, got)

					plan := planassert.AnalyzeVerbose(t, ctx, db, query)
					mem := planassert.MaximumMemoryUsageBytes(t, plan)
					planassert.UsesAtMostMaximumMemoryUsage(t, plan, 512<<10 /* 512 KiB */)
					if tc.targetBytes == 1<<20 {
						smallMem = mem
					} else {
						largeMem = mem
					}
				})
			}
			require.LessOrEqual(t, largeMem, smallMem*2,
				"aggregating over jsonb_array_elements(j) should keep EXPLAIN ANALYZE max memory roughly constant as the array grows")
		})
	}
}
