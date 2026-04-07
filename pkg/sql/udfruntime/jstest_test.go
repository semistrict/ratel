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
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package udfruntime

import (
	"fmt"
	"os"
	"strings"
	"testing"

	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/require"
	v8 "github.com/tommie/v8go"
)

// TestPool tests the V8 pool state machine in isolation — no Registry,
// no SQL, just V8 + pool.js + an invoke function.
func TestPool(t *testing.T) {
	tests := []struct {
		name   string
		jsFile string
		inputs []int64
		want   []int64
	}{
		{
			name:   "sync_double",
			jsFile: "testdata/sync_double.js",
			inputs: []int64{1, 2, 3, 10, -5},
			want:   []int64{2, 4, 6, 20, -10},
		},
		{
			name:   "async_resolve",
			jsFile: "testdata/async_resolve.js",
			inputs: []int64{1, 2, 3},
			want:   []int64{2, 3, 4},
		},
		{
			name:   "mixed",
			jsFile: "testdata/mixed.js",
			// x>5 → Promise.resolve(x*10), else x*2
			inputs: []int64{1, 3, 6, 10},
			want:   []int64{2, 6, 60, 100},
		},
		{
			name:   "async_chain",
			jsFile: "testdata/async_chain.js",
			// (x+1)*2 - 1
			inputs: []int64{0, 5, 10},
			want:   []int64{1, 11, 21},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := runPoolTest(t, tt.jsFile, tt.inputs)
			require.Equal(t, tt.want, results)
		})
	}
}

// TestPool_Error tests that JS errors are propagated through the pool.
func TestPool_Error(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	loadFile(t, ctx, "testdata/error.js")
	_, err := ctx.RunScript(poolJS, "pool.js")
	require.NoError(t, err)

	// Submit with a negative value that triggers the error.
	argsJSON := `[[-3]]`
	strVal, err := v8.NewValue(iso, argsJSON)
	require.NoError(t, err)
	_, err = ctx.Global().MethodCall("__pool_submit", strVal)
	require.NoError(t, err)

	val, err := ctx.Global().MethodCall("__pool_collect")
	require.NoError(t, err)
	require.False(t, val.IsNull())

	// Parse and verify the error tuple.
	var entries [][]jsoniter.RawMessage
	require.NoError(t, jsoniter.UnmarshalFromString(val.String(), &entries))
	require.Len(t, entries, 1)
	// Error tuple has 3 elements: [idx, null, "error message"]
	require.Len(t, entries[0], 3)

	var errMsg string
	require.NoError(t, jsoniter.Unmarshal(entries[0][2], &errMsg))
	require.Contains(t, errMsg, "negative input: -3")
}

// TestPool_Ordering verifies that results maintain correct index ordering
// even when promises resolve out of order.
func TestPool_Ordering(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	// Mixed sync/async — sync results arrive immediately, async after pump.
	_, err := ctx.RunScript(`function invoke(x) {
		return x % 2 === 0 ? x * 10 : Promise.resolve(x * 100);
	}`, "ordering.js")
	require.NoError(t, err)

	_, err = ctx.RunScript(poolJS, "pool.js")
	require.NoError(t, err)

	// Submit 8 values: evens are sync, odds are async.
	argsJSON := `[[0],[1],[2],[3],[4],[5],[6],[7]]`
	strVal, err := v8.NewValue(iso, argsJSON)
	require.NoError(t, err)
	_, err = ctx.Global().MethodCall("__pool_submit", strVal)
	require.NoError(t, err)

	// First collect: should get the sync results (evens).
	val, err := ctx.Global().MethodCall("__pool_collect")
	require.NoError(t, err)
	require.False(t, val.IsNull())

	results := make(map[int]int64)
	parseResults(t, val.String(), results)

	// Pump microtasks to resolve the async promises.
	ctx.PerformMicrotaskCheckpoint()

	// Second collect: should get the async results (odds).
	val, err = ctx.Global().MethodCall("__pool_collect")
	require.NoError(t, err)
	if !val.IsNull() {
		parseResults(t, val.String(), results)
	}

	// Verify all 8 results are correct.
	require.Len(t, results, 8)
	for i := 0; i < 8; i++ {
		if i%2 == 0 {
			require.Equal(t, int64(i*10), results[i], "idx %d", i)
		} else {
			require.Equal(t, int64(i*100), results[i], "idx %d", i)
		}
	}
}

// TestPool_Reset verifies that reset clears all state.
func TestPool_Reset(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	_, err := ctx.RunScript(`function invoke(x) { return x; }`, "identity.js")
	require.NoError(t, err)
	_, err = ctx.RunScript(poolJS, "pool.js")
	require.NoError(t, err)

	// Submit, collect, reset, submit again — indices should restart at 0.
	strVal, err := v8.NewValue(iso, `[[10]]`)
	require.NoError(t, err)
	_, err = ctx.Global().MethodCall("__pool_submit", strVal)
	require.NoError(t, err)

	val, err := ctx.Global().MethodCall("__pool_collect")
	require.NoError(t, err)
	require.False(t, val.IsNull())
	require.Contains(t, val.String(), "[0,10]")

	// Reset.
	_, err = ctx.Global().MethodCall("__pool_reset")
	require.NoError(t, err)

	// Submit again — idx should be 0 again.
	strVal, err = v8.NewValue(iso, `[[20]]`)
	require.NoError(t, err)
	_, err = ctx.Global().MethodCall("__pool_submit", strVal)
	require.NoError(t, err)

	val, err = ctx.Global().MethodCall("__pool_collect")
	require.NoError(t, err)
	require.False(t, val.IsNull())
	require.Contains(t, val.String(), "[0,20]")
}

// runPoolTest loads a JS file, installs the pool, submits inputs, pumps
// microtasks, collects results, and returns them in order.
func runPoolTest(t *testing.T, jsFile string, inputs []int64) []int64 {
	t.Helper()

	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	// Load the invoke function.
	loadFile(t, ctx, jsFile)

	// Load the pool.
	_, err := ctx.RunScript(poolJS, "pool.js")
	require.NoError(t, err)

	// Build args JSON: [[x0],[x1],...]
	var b strings.Builder
	b.WriteByte('[')
	for i, x := range inputs {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "[%d]", x)
	}
	b.WriteByte(']')

	strVal, err := v8.NewValue(iso, b.String())
	require.NoError(t, err)

	_, err = ctx.Global().MethodCall("__pool_submit", strVal)
	require.NoError(t, err)

	// Collect loop with microtask pumping.
	resultMap := make(map[int]int64)
	for len(resultMap) < len(inputs) {
		ctx.PerformMicrotaskCheckpoint()

		val, err := ctx.Global().MethodCall("__pool_collect")
		require.NoError(t, err)
		if val.IsNull() {
			continue
		}
		parseResults(t, val.String(), resultMap)
	}

	// Convert map to ordered slice.
	out := make([]int64, len(inputs))
	for i := range out {
		v, ok := resultMap[i]
		require.True(t, ok, "missing result for idx %d", i)
		out[i] = v
	}
	return out
}

func loadFile(t *testing.T, ctx *v8.Context, path string) {
	t.Helper()
	src, err := os.ReadFile(path)
	require.NoError(t, err)
	_, err = ctx.RunScript(string(src), path)
	require.NoError(t, err)
}

func parseResults(t *testing.T, jsonStr string, into map[int]int64) {
	t.Helper()
	var entries [][]jsoniter.RawMessage
	require.NoError(t, jsoniter.UnmarshalFromString(jsonStr, &entries))
	for _, entry := range entries {
		require.True(t, len(entry) >= 2, "entry too short: %s", string(entry[0]))
		var idx int
		require.NoError(t, jsoniter.Unmarshal(entry[0], &idx))
		// Check for error.
		if len(entry) >= 3 {
			var errMsg string
			require.NoError(t, jsoniter.Unmarshal(entry[2], &errMsg))
			t.Fatalf("unexpected error at idx %d: %s", idx, errMsg)
		}
		var val float64
		require.NoError(t, jsoniter.Unmarshal(entry[1], &val))
		into[idx] = int64(val)
	}
}
