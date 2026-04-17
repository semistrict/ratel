// Copyright 2021 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package goschedstats

import (
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/stretchr/testify/require"
)

func TestNumRunnableGoroutines(t *testing.T) {
	// numRunnableGoroutines historically reached into the Go runtime via
	// //go:linkname to return the true runnable count.  Go 1.23+ forbids
	// reaching into runtime internals and there is no public runtime/metrics
	// equivalent, so the stub always reports zero and this behavioural test is
	// no longer meaningful.
	t.Skip("numRunnableGoroutines is a stub under modern Go (see runtime_modern.go)")
}

type testTimeTicker struct {
	numResets         int
	lastResetDuration time.Duration
}

func (t *testTimeTicker) Reset(d time.Duration) {
	t.numResets++
	t.lastResetDuration = d
}

func TestSchedStatsTicker(t *testing.T) {
	runnable := 0
	numRunnable := func() (numRunnable int, numProcs int) {
		return runnable, 1
	}
	var callbackSamplePeriod time.Duration
	var numCallbacks int
	cb := func(numRunnable int, numProcs int, samplePeriod time.Duration) {
		require.Equal(t, runnable, numRunnable)
		require.Equal(t, 1, numProcs)
		callbackSamplePeriod = samplePeriod
		numCallbacks++
	}
	cbs := []callbackWithID{{cb, 0}}
	now := timeutil.UnixEpoch
	startTime := now
	sst := schedStatsTicker{
		lastTime:              now,
		curPeriod:             samplePeriodShort,
		numRunnableGoroutines: numRunnable,
	}
	tt := testTimeTicker{}
	// Tick every 1ms until the reportingPeriod has elapsed.
	for i := 1; ; i++ {
		now = now.Add(samplePeriodShort)
		sst.getStatsOnTick(now, cbs, &tt)
		if now.Sub(startTime) <= reportingPeriod {
			// No reset of the time ticker.
			require.Equal(t, 0, tt.numResets)
			// Each tick causes a callback.
			require.Equal(t, i, numCallbacks)
			require.Equal(t, samplePeriodShort, callbackSamplePeriod)
		} else {
			break
		}
	}
	// Since underloaded, the time ticker is reset to samplePeriodLong, and this
	// period is provided to the latest callback.
	require.Equal(t, 1, tt.numResets)
	require.Equal(t, samplePeriodLong, tt.lastResetDuration)
	require.Equal(t, samplePeriodLong, callbackSamplePeriod)
	// Increase load so no longer underloaded.
	runnable = 2
	startTime = now
	tt.numResets = 0
	for i := 1; ; i++ {
		now = now.Add(samplePeriodLong)
		sst.getStatsOnTick(now, cbs, &tt)
		if now.Sub(startTime) <= reportingPeriod {
			// No reset of the time ticker.
			require.Equal(t, 0, tt.numResets)
			// Each tick causes a callback.
			require.Equal(t, samplePeriodLong, callbackSamplePeriod)
		} else {
			break
		}
	}
	// No longer underloaded, so the time ticker is reset to samplePeriodShort,
	// and this period is provided to the latest callback.
	require.Equal(t, 1, tt.numResets)
	require.Equal(t, samplePeriodShort, tt.lastResetDuration)
	require.Equal(t, samplePeriodShort, callbackSamplePeriod)
}
