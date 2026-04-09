// Copyright 2022 The Cockroach Authors.
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

package sql_test

import (
	"context"
	gosql "database/sql"
	"fmt"
	"testing"

	"github.com/cockroachdb/errors"
	pbtypes "github.com/gogo/protobuf/types"
	"github.com/lib/pq"
	"github.com/semistrict/ratel/pkg/base"
	"github.com/semistrict/ratel/pkg/jobs"
	"github.com/semistrict/ratel/pkg/jobs/jobspb"
	"github.com/semistrict/ratel/pkg/jobs/jobstest"
	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/kv"
	"github.com/semistrict/ratel/pkg/sql"
	"github.com/semistrict/ratel/pkg/sql/catalog/catpb"
	"github.com/semistrict/ratel/pkg/sql/catalog/descpb"
	"github.com/semistrict/ratel/pkg/sql/catalog/descs"
	"github.com/semistrict/ratel/pkg/sql/catalog/desctestutils"
	"github.com/semistrict/ratel/pkg/sql/catalog/tabledesc"
	"github.com/semistrict/ratel/pkg/sql/sem/tree"
	"github.com/semistrict/ratel/pkg/sql/sqlutil"
	"github.com/semistrict/ratel/pkg/testutils/serverutils"
	"github.com/semistrict/ratel/pkg/util/leaktest"
	"github.com/semistrict/ratel/pkg/util/log"
	"github.com/semistrict/ratel/pkg/util/timeutil"
	"github.com/stretchr/testify/require"
)

func TestValidateTTLScheduledJobs(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	ctx := context.Background()

	testCases := []struct {
		desc          string
		setup         func(t *testing.T, sqlDB *gosql.DB, kvDB *kv.DB, s serverutils.TestServerInterface, tableDesc *tabledesc.Mutable, scheduleID int64)
		expectedErrRe func(tableID descpb.ID, scheduleID int64) string
	}{
		{
			desc: "not pointing at a valid scheduled job",
			setup: func(t *testing.T, sqlDB *gosql.DB, kvDB *kv.DB, s serverutils.TestServerInterface, tableDesc *tabledesc.Mutable, scheduleID int64) {
				tableDesc.RowLevelTTL.ScheduleID = 0
				require.NoError(t, sql.TestingDescsTxn(ctx, s, func(ctx context.Context, txn *kv.Txn, col *descs.Collection) error {
					return col.WriteDesc(ctx, false /* kvBatch */, tableDesc, txn)
				}))
			},
			expectedErrRe: func(tableID descpb.ID, scheduleID int64) string {
				return fmt.Sprintf(`table id %d maps to a non-existent schedule id 0`, tableID)
			},
		},
		{
			desc: "scheduled job points at an different table",
			setup: func(t *testing.T, sqlDB *gosql.DB, kvDB *kv.DB, s serverutils.TestServerInterface, tableDesc *tabledesc.Mutable, scheduleID int64) {
				ie := s.InternalExecutor().(sqlutil.InternalExecutor)
				require.NoError(t, kvDB.Txn(ctx, func(ctx context.Context, txn *kv.Txn) error {
					sj, err := jobs.LoadScheduledJob(
						ctx,
						jobstest.NewJobSchedulerTestEnv(
							jobstest.UseSystemTables,
							timeutil.Now(),
							tree.ScheduledBackupExecutor,
						),
						scheduleID,
						ie,
						txn,
					)
					if err != nil {
						return err
					}
					var args catpb.ScheduledRowLevelTTLArgs
					if err := pbtypes.UnmarshalAny(sj.ExecutionArgs().Args, &args); err != nil {
						return err
					}
					args.TableID = 0
					any, err := pbtypes.MarshalAny(&args)
					if err != nil {
						return err
					}
					sj.SetExecutionDetails(sj.ExecutorType(), jobspb.ExecutionArguments{Args: any})
					return sj.Update(ctx, ie, txn)
				}))
			},
			expectedErrRe: func(tableID descpb.ID, scheduleID int64) string {
				return fmt.Sprintf(`schedule id %d points to table id 0 instead of table id %d`, scheduleID, tableID)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			s, sqlDB, kvDB := serverutils.StartServer(t, base.TestServerArgs{})
			defer s.Stopper().Stop(ctx)

			_, err := sqlDB.Exec(`CREATE TABLE t () WITH (ttl_expire_after = '10 mins')`)
			require.NoError(t, err)

			tableDesc := desctestutils.TestingGetMutableExistingTableDescriptor(kvDB, keys.SystemSQLCodec, "defaultdb", "t")
			require.NotNil(t, tableDesc.GetRowLevelTTL())
			scheduleID := tableDesc.GetRowLevelTTL().ScheduleID

			tc.setup(t, sqlDB, kvDB, s, tableDesc, scheduleID)

			_, err = sqlDB.Exec(`SELECT crdb_internal.validate_ttl_scheduled_jobs()`)
			require.Error(t, err)
			require.Regexp(t, tc.expectedErrRe(tableDesc.GetID(), scheduleID), err)
			var pgxErr *pq.Error
			require.True(t, errors.As(err, &pgxErr))
			require.Regexp(
				t,
				fmt.Sprintf(`use crdb_internal.repair_ttl_table_scheduled_job\(%d\) to repair the missing job`, tableDesc.GetID()),
				pgxErr.Hint,
			)

			// Repair and check jobs are valid.
			_, err = sqlDB.Exec(`DROP SCHEDULE $1`, scheduleID)
			require.NoError(t, err)
			_, err = sqlDB.Exec(`SELECT crdb_internal.repair_ttl_table_scheduled_job($1)`, tableDesc.GetID())
			require.NoError(t, err)
			_, err = sqlDB.Exec(`SELECT crdb_internal.validate_ttl_scheduled_jobs()`)
			require.NoError(t, err)
		})
	}
}
