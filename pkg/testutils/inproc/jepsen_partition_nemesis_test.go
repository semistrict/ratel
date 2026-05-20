// Copyright 2026 The Cockroach Authors
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
	"time"

	"github.com/cockroachdb/cockroach/pkg/testutils/inproc"
)

func runTimedLinkPartitionNemesis(
	stopCh <-chan struct{},
	c *inproc.Cluster,
	beforePartition func(),
	apply func(),
) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	healed := true
	heal := func() {
		c.HealAllLinks()
		healed = true
	}
	defer func() {
		if !healed {
			heal()
		}
	}()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			if beforePartition != nil {
				beforePartition()
			}
			apply()
			healed = false
			select {
			case <-stopCh:
				heal()
				return
			case <-time.After(500 * time.Millisecond):
				heal()
			}
		}
	}
}
