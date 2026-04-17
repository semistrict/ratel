// Copyright 2026 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

//go:build gc && go1.20
// +build gc,go1.20

package goschedstats

import "runtime"

// numRunnableGoroutines is a stub for Go 1.20+. The original implementation
// accessed unexported runtime internals via //go:linkname, which newer Go
// versions restrict. We return (0, GOMAXPROCS) so callers that compute a ratio
// see a "no pressure" signal rather than misreporting.
func numRunnableGoroutines() (numRunnable int, numProcs int) {
	return 0, runtime.GOMAXPROCS(0)
}
