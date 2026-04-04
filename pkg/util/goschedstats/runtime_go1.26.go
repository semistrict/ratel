// Copyright 2024 Oxide Computer Company
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
//
// Go 1.26 restricts go:linkname access to runtime internals. This file
// provides a safe alternative using public APIs.

//go:build gc && go1.26

package goschedstats

import "runtime"

func numRunnableGoroutines() (numRunnable int, numProcs int) {
	numProcs = runtime.GOMAXPROCS(0)
	// runtime.NumGoroutine includes all goroutines (runnable + running +
	// blocked). This is a conservative overcount compared to the original
	// implementation which read per-P run queues directly via go:linkname.
	// For admission control purposes, overcounting is safe (it biases toward
	// throttling rather than overloading).
	numRunnable = runtime.NumGoroutine()
	return numRunnable, numProcs
}
