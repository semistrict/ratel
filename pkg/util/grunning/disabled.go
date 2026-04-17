// Copyright 2022 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package grunning

// grunningnanos used to linkname into a patched upstream runtime symbol
// (runtime.grunningnanos) provided only by the forked CRDB Go toolchain.
// Upstream Go does not export that symbol, and stdlib linkname restrictions
// in go1.23+ forbid us from reaching into the runtime anyway, so report 0
// and flag the facility as unsupported.
func grunningnanos() int64 { return 0 }

func supported() bool { return false }
