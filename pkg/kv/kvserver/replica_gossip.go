// Copyright 2019 The Cockroach Authors.
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

package kvserver

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/semistrict/ratel/pkg/config"
	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/kv/kvserver/kvserverbase"
	"github.com/semistrict/ratel/pkg/kv/kvserver/uncertainty"
	"github.com/semistrict/ratel/pkg/roachpb"
	"github.com/semistrict/ratel/pkg/storage"
	"github.com/semistrict/ratel/pkg/util/log"
)

func (r *Replica) gossipFirstRange(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gossipFirstRangeLocked(ctx)
}

func (r *Replica) gossipFirstRangeLocked(ctx context.Context) {
	cb := r.store.cfg.FirstRangeCallback
	if cb == nil {
		return
	}
	log.Event(ctx, "notifying first range callback")
	if log.V(1) {
		log.Infof(ctx, "first range callback from store %d, r%d: %s",
			r.store.StoreID(), r.RangeID, r.mu.state.Desc.Replicas())
	}
	cb(r.mu.state.Desc)
}

// shouldGossip returns true if this replica should be broadcasting
// first-range updates. We use the lease to ensure only one node does so.
func (r *Replica) shouldGossip(ctx context.Context) bool {
	return r.OwnsValidLease(ctx, r.store.Clock().NowAsClockTimestamp())
}

// MaybeGossipSystemConfigRaftMuLocked is a no-op. The system config gossip
// trigger has been replaced by the span config infrastructure.
//
// TODO(ajwerner): Remove this in 22.2.
func (r *Replica) MaybeGossipSystemConfigRaftMuLocked(ctx context.Context) error {
	return nil
}

// MaybeGossipSystemConfigIfHaveFailureRaftMuLocked is a no-op. The system
// config gossip trigger has been replaced by the span config infrastructure.
func (r *Replica) MaybeGossipSystemConfigIfHaveFailureRaftMuLocked(ctx context.Context) error {
	return nil
}

// MaybeGossipNodeLivenessRaftMuLocked is a no-op. Node liveness records are
// now distributed via the liveness rangefeed.
func (r *Replica) MaybeGossipNodeLivenessRaftMuLocked(
	ctx context.Context, span roachpb.Span,
) error {
	return nil
}

var errSystemConfigIntent = errors.New("must retry later due to intent on SystemConfigSpan")

// loadSystemConfig scans the system config span and returns the system
// config.
func (r *Replica) loadSystemConfig(ctx context.Context) (*config.SystemConfigEntries, error) {
	ba := roachpb.BatchRequest{}
	ba.ReadConsistency = roachpb.INCONSISTENT
	ba.Timestamp = r.store.Clock().Now()
	ba.Add(&roachpb.ScanRequest{RequestHeader: roachpb.RequestHeaderFromSpan(keys.SystemConfigSpan)})
	// Call evaluateBatch instead of Send to avoid reacquiring latches.
	rec := NewReplicaEvalContext(r, todoSpanSet)
	rw := r.Engine().NewReadOnly(storage.StandardDurability)
	defer rw.Close()

	br, result, pErr := evaluateBatch(
		ctx, kvserverbase.CmdIDKey(""), rw, rec, nil, &ba, uncertainty.Interval{}, true, /* readOnly */
	)
	if pErr != nil {
		return nil, pErr.GoError()
	}
	if intents := result.Local.DetachEncounteredIntents(); len(intents) > 0 {
		// There were intents, so what we read may not be consistent. Attempt
		// to nudge the intents in case they're expired; next time around we'll
		// hopefully have more luck.
		// This is called from handleReadWriteLocalEvalResult (with raftMu
		// locked), so disallow synchronous processing (which blocks that mutex
		// for too long and is a potential deadlock).
		if err := r.store.intentResolver.CleanupIntentsAsync(ctx, intents, false /* allowSync */); err != nil {
			log.Warningf(ctx, "%v", err)
		}
		return nil, errSystemConfigIntent
	}
	kvs := br.Responses[0].GetInner().(*roachpb.ScanResponse).Rows
	sysCfg := &config.SystemConfigEntries{}
	sysCfg.Values = kvs
	return sysCfg, nil
}

// getLeaseForFirstRange tries to obtain a range lease. Only one of the
// replicas should broadcast first-range updates; the bool returned indicates
// whether it's us.
func (r *Replica) getLeaseForFirstRange(ctx context.Context) (bool, *roachpb.Error) {
	if !r.IsInitialized() {
		return false, roachpb.NewErrorf("range not initialized")
	}
	var hasLease bool
	var pErr *roachpb.Error
	if err := r.store.Stopper().RunTask(
		ctx, "storage.Replica: acquiring lease for first range",
		func(ctx context.Context) {
			// Check for or obtain the lease, if none active.
			_, pErr = r.redirectOnOrAcquireLease(ctx)
			hasLease = pErr == nil
			if pErr != nil {
				switch e := pErr.GetDetail().(type) {
				case *roachpb.NotLeaseHolderError:
					// NotLeaseHolderError means there is an active lease, but only if
					// the lease holder is set; otherwise, it's likely a timeout.
					if e.LeaseHolder != nil {
						pErr = nil
					}
				default:
					// Any other error is worth being logged visibly.
					log.Warningf(ctx, "could not acquire lease for first range: %s", pErr)
				}
			}
		}); err != nil {
		pErr = roachpb.NewError(err)
	}
	return hasLease, pErr
}

// maybeGossipFirstRange calls the FirstRangeCallback if this is the first
// range and a range lease can be obtained. The Store calls this periodically
// on first range replicas.
func (r *Replica) maybeGossipFirstRange(ctx context.Context) *roachpb.Error {
	if !r.IsFirstRange() {
		return nil
	}
	if r.store.cfg.FirstRangeCallback == nil {
		return nil
	}

	hasLease, pErr := r.getLeaseForFirstRange(ctx)
	if pErr != nil {
		return pErr
	} else if !hasLease {
		return nil
	}
	r.gossipFirstRange(ctx)
	return nil
}
