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

// Package planassert provides reusable EXPLAIN-plan readers and assertions for
// inproc integration tests.
package planassert

import (
	"context"
	"database/sql"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var kvRowsReadRE = regexp.MustCompile(`KV rows read: ([\d,]+)`)
var maxMemoryUsageRE = regexp.MustCompile(`maximum memory usage: ([\d.]+) ([KMG]?i?B)`)

// VecVerbose returns the EXPLAIN (VEC, VERBOSE) output joined into one string.
func VecVerbose(t testing.TB, ctx context.Context, db *sql.DB, query string) string {
	t.Helper()
	rows, err := db.QueryContext(ctx, `EXPLAIN (VEC, VERBOSE) `+query)
	require.NoError(t, err)
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line string
		require.NoError(t, rows.Scan(&line))
		lines = append(lines, line)
	}
	require.NoError(t, rows.Err())
	return strings.Join(lines, "\n")
}

// DistSQLJSON returns the EXPLAIN (DISTSQL, JSON) output.
func DistSQLJSON(t testing.TB, ctx context.Context, db *sql.DB, query string) string {
	t.Helper()
	var plan string
	err := db.QueryRowContext(ctx, `EXPLAIN (DISTSQL, JSON) `+query).Scan(&plan)
	require.NoError(t, err)
	return plan
}

// DistSQLVerbose returns the EXPLAIN (DISTSQL) output joined into one string.
func DistSQLVerbose(t testing.TB, ctx context.Context, db *sql.DB, query string) string {
	t.Helper()
	rows, err := db.QueryContext(ctx, `EXPLAIN (DISTSQL) `+query)
	require.NoError(t, err)
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line string
		require.NoError(t, rows.Scan(&line))
		lines = append(lines, line)
	}
	require.NoError(t, rows.Err())
	return strings.Join(lines, "\n")
}

// AnalyzeVerbose returns the EXPLAIN ANALYZE (VERBOSE) output joined into one string.
func AnalyzeVerbose(t testing.TB, ctx context.Context, db *sql.DB, query string) string {
	t.Helper()
	rows, err := db.QueryContext(ctx, `EXPLAIN ANALYZE (VERBOSE) `+query)
	require.NoError(t, err)
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line string
		require.NoError(t, rows.Scan(&line))
		lines = append(lines, line)
	}
	require.NoError(t, rows.Err())
	return strings.Join(lines, "\n")
}

// Contains asserts that the plan output contains all requested fragments.
func Contains(t testing.TB, plan string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		require.Contains(t, plan, fragment)
	}
}

// NotContains asserts that the plan output omits all requested fragments.
func NotContains(t testing.TB, plan string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		require.NotContains(t, plan, fragment)
	}
}

// UsesColBatchScan asserts that the plan stays in the native vectorized scan.
func UsesColBatchScan(t testing.TB, plan string) {
	t.Helper()
	Contains(t, plan, "*colfetcher.ColBatchScan")
	NotContains(t, plan, "*rowexec.tableReader")
}

// UsesFullDistribution asserts that the plan is distributed across the cluster.
func UsesFullDistribution(t testing.TB, plan string) {
	t.Helper()
	Contains(t, plan, "distribution: full")
}

// KVRowsRead extracts the "KV rows read" counter from EXPLAIN ANALYZE (VERBOSE).
func KVRowsRead(t testing.TB, plan string) int {
	t.Helper()
	matches := kvRowsReadRE.FindStringSubmatch(plan)
	require.Len(t, matches, 2, "missing KV rows read in plan:\n%s", plan)
	n, err := strconv.Atoi(strings.ReplaceAll(matches[1], ",", ""))
	require.NoError(t, err)
	return n
}

// UsesAtMostKVRowsRead asserts that EXPLAIN ANALYZE reported a bounded number
// of KV rows read for the statement under test.
func UsesAtMostKVRowsRead(t testing.TB, plan string, max int) {
	t.Helper()
	require.LessOrEqual(t, KVRowsRead(t, plan), max, plan)
}

// MaximumMemoryUsageBytes extracts the EXPLAIN ANALYZE maximum memory usage.
func MaximumMemoryUsageBytes(t testing.TB, plan string) int64 {
	t.Helper()
	matches := maxMemoryUsageRE.FindStringSubmatch(plan)
	require.Len(t, matches, 3, "missing maximum memory usage in plan:\n%s", plan)
	value, err := strconv.ParseFloat(matches[1], 64)
	require.NoError(t, err)

	var multiplier float64 = 1
	switch matches[2] {
	case "B":
		multiplier = 1
	case "KiB":
		multiplier = 1 << 10
	case "MiB":
		multiplier = 1 << 20
	case "GiB":
		multiplier = 1 << 30
	default:
		require.FailNowf(t, "unknown memory unit", "unit=%s plan=\n%s", matches[2], plan)
	}
	return int64(value * multiplier)
}

// UsesAtMostMaximumMemoryUsage asserts that EXPLAIN ANALYZE reported bounded
// maximum memory usage for the statement under test.
func UsesAtMostMaximumMemoryUsage(t testing.TB, plan string, maxBytes int64) {
	t.Helper()
	require.LessOrEqual(t, MaximumMemoryUsageBytes(t, plan), maxBytes, plan)
}
