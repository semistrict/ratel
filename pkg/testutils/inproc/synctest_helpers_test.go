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

package inproc_test

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/semistrict/ratel/pkg/base"
	"github.com/semistrict/ratel/pkg/testutils/inproc"
)

func startSyncCluster(
	t *testing.T, nodes int, extraArgs ...func(*base.TestClusterArgs),
) *inproc.Cluster {
	t.Helper()
	return inproc.StartCluster(t, nodes, extraArgs...)
}

func stopSyncCluster(c *inproc.Cluster) {
	c.Stop()
	// Flush lingering fake-time timers in the just-stopped cluster before the
	// next synctest bubble starts. Without this, long-lived background timers
	// can wake in a later test and trip cross-bubble panics.
	time.Sleep(24 * time.Hour)
	synctest.Wait()
}

func waitOrStop(stopCh <-chan struct{}, d time.Duration) bool {
	select {
	case <-stopCh:
		return false
	case <-time.After(d):
	}

	select {
	case <-stopCh:
		return false
	default:
		return true
	}
}
