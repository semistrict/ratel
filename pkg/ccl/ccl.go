// Copyright 2026 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package ccl

// TestingEnableEnterprise is an OSS compatibility hook for tests that are
// shared with CCL builds.
func TestingEnableEnterprise() func() {
	return func() {}
}

// TestingDisableEnterprise is an OSS compatibility hook for tests that are
// shared with CCL builds.
func TestingDisableEnterprise() func() {
	return func() {}
}
