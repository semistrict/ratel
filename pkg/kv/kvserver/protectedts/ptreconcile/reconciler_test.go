// Copyright 2020 The Cockroach Authors.
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

package ptreconcile_test

import (
	"context"
	"testing"
	"time"

	"github.com/semistrict/ratel/pkg/base"
	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/kv"
	"github.com/semistrict/ratel/pkg/kv/kvserver/protectedts"
	"github.com/semistrict/ratel/pkg/kv/kvserver/protectedts/ptpb"
	"github.com/semistrict/ratel/pkg/kv/kvserver/protectedts/ptreconcile"
	"github.com/semistrict/ratel/pkg/roachpb"
	"github.com/semistrict/ratel/pkg/settings/cluster"
	"github.com/semistrict/ratel/pkg/sql"
	"github.com/semistrict/ratel/pkg/testutils"
	"github.com/semistrict/ratel/pkg/testutils/testcluster"
	"github.com/semistrict/ratel/pkg/util/leaktest"
	"github.com/semistrict/ratel/pkg/util/syncutil"
	"github.com/semistrict/ratel/pkg/util/uuid"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/require"
)

func TestReconciler(t *testing.T) {
	defer leaktest.AfterTest(t)()

	ctx := context.Background()
	testutils.RunTrueAndFalse(t, "reconciler", func(t *testing.T, withDeprecatedSpans bool) {
		tc := testcluster.StartTestCluster(t, 1, base.TestClusterArgs{
			ServerArgs: base.TestServerArgs{
				Knobs: base.TestingKnobs{
					ProtectedTS: &protectedts.TestingKnobs{DisableProtectedTimestampForMultiTenant: withDeprecatedSpans},
				},
			},
		})
		defer tc.Stopper().Stop(ctx)

		// Now I want to create some artifacts that should get reconciled away and
		// then make sure that they do and others which should not do not.
		s0 := tc.Server(0)
		ptp := s0.ExecutorConfig().(sql.ExecutorConfig).ProtectedTimestampProvider

		settings := cluster.MakeTestingClusterSettings()
		const testTaskType = "foo"
		var state = struct {
			mu       syncutil.Mutex
			toRemove map[string]struct{}
		}{}
		state.toRemove = map[string]struct{}{}
		r := ptreconcile.New(settings, s0.DB(), ptp,
			ptreconcile.StatusFuncs{
				testTaskType: func(
					ctx context.Context, txn *kv.Txn, meta []byte,
				) (shouldRemove bool, err error) {
					state.mu.Lock()
					defer state.mu.Unlock()
					_, shouldRemove = state.toRemove[string(meta)]
					return shouldRemove, nil
				},
			})
		require.NoError(t, r.StartReconciler(ctx, s0.Stopper()))
		recMeta := "a"
		rec1 := ptpb.Record{
			ID:        uuid.MakeV4().GetBytes(),
			Timestamp: s0.Clock().Now(),
			Mode:      ptpb.PROTECT_AFTER,
			MetaType:  testTaskType,
			Meta:      []byte(recMeta),
		}
		if withDeprecatedSpans {
			rec1.DeprecatedSpans = []roachpb.Span{{Key: keys.MinKey, EndKey: keys.MaxKey}}
		} else {
			rec1.Target = ptpb.MakeClusterTarget()
		}
		require.NoError(t, s0.DB().Txn(ctx, func(ctx context.Context, txn *kv.Txn) error {
			return ptp.Protect(ctx, txn, &rec1)
		}))

		t.Run("update settings", func(t *testing.T) {
			ptreconcile.ReconcileInterval.Override(ctx, &settings.SV, time.Millisecond)
			testutils.SucceedsSoon(t, func() error {
				require.Equal(t, int64(0), r.Metrics().RecordsRemoved.Count())
				require.Equal(t, int64(0), r.Metrics().ReconciliationErrors.Count())
				if processed := r.Metrics().RecordsProcessed.Count(); processed < 1 {
					return errors.Errorf("expected processed to be at least 1, got %d", processed)
				}
				return nil
			})
		})
		t.Run("reconcile", func(t *testing.T) {
			state.mu.Lock()
			state.toRemove[recMeta] = struct{}{}
			state.mu.Unlock()

			ptreconcile.ReconcileInterval.Override(ctx, &settings.SV, time.Millisecond)
			testutils.SucceedsSoon(t, func() error {
				require.Equal(t, int64(0), r.Metrics().ReconciliationErrors.Count())
				if removed := r.Metrics().RecordsRemoved.Count(); removed != 1 {
					return errors.Errorf("expected processed to be 1, got %d", removed)
				}
				return nil
			})
			require.Regexp(t, protectedts.ErrNotExists, s0.DB().Txn(ctx, func(ctx context.Context, txn *kv.Txn) error {
				_, err := ptp.GetRecord(ctx, txn, rec1.ID.GetUUID())
				return err
			}))
		})
	})
}
