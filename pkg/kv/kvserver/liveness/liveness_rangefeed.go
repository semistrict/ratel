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

package liveness

import (
	"context"
	"time"

	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/kv/kvserver/liveness/livenesspb"
	"github.com/semistrict/ratel/pkg/util/log"
	"github.com/semistrict/ratel/pkg/util/stop"
)

// StartLivenessPoller starts a background loop that periodically polls liveness
// records from KV and feeds them into the NodeLiveness cache. This replaces
// gossip-based liveness distribution. A rangefeed would be more efficient
// but cannot operate on the liveness range (which uses expiration-based
// leases incompatible with rangefeeds in CockroachDB 22.1).
func (nl *NodeLiveness) StartLivenessPoller(
	ctx context.Context, stopper *stop.Stopper,
) error {
	return stopper.RunAsyncTask(ctx, "liveness-poller", func(ctx context.Context) {
		const pollInterval = 5 * time.Second
		for {
			if err := nl.pollLivenessFromKV(ctx); err != nil {
				log.Warningf(ctx, "failed to poll liveness from KV: %v", err)
			}
			// Sleep using time.Sleep so synctest can advance fake time.
			// Check quiesce before and after.
			select {
			case <-stopper.ShouldQuiesce():
				return
			default:
			}
			time.Sleep(pollInterval)
			select {
			case <-stopper.ShouldQuiesce():
				return
			default:
			}
		}
	})
}

func (nl *NodeLiveness) pollLivenessFromKV(ctx context.Context) error {
	kvs, err := nl.db.Scan(ctx, keys.NodeLivenessPrefix, keys.NodeLivenessKeyMax, 0)
	if err != nil {
		return err
	}
	for _, kv := range kvs {
		if kv.Value == nil {
			continue
		}
		var l livenesspb.Liveness
		if err := kv.Value.GetProto(&l); err != nil {
			log.Warningf(ctx, "failed to decode liveness record: %v", err)
			continue
		}
		nl.maybeUpdate(ctx, Record{
			Liveness: l,
			raw:      kv.Value.TagAndDataBytes(),
		})
	}
	return nil
}
