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

package jobs

import (
	"context"
	"fmt"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/semistrict/ratel/pkg/base"
	"github.com/semistrict/ratel/pkg/jobs/jobspb"
	"github.com/semistrict/ratel/pkg/jobs/jobstest"
	"github.com/semistrict/ratel/pkg/kv"
	"github.com/semistrict/ratel/pkg/scheduledjobs"
	"github.com/semistrict/ratel/pkg/security"
	"github.com/semistrict/ratel/pkg/settings"
	"github.com/semistrict/ratel/pkg/sql/sem/tree"
	"github.com/semistrict/ratel/pkg/sql/sessiondata"
	"github.com/semistrict/ratel/pkg/sql/sqlutil"
	"github.com/semistrict/ratel/pkg/testutils/serverutils"
	"github.com/semistrict/ratel/pkg/testutils/sqlutils"
	"github.com/semistrict/ratel/pkg/util/timeutil"
	"github.com/stretchr/testify/require"
)

type execSchedulesFn func(ctx context.Context, maxSchedules int64) error
type testHelper struct {
	env           *jobstest.JobSchedulerTestEnv
	server        serverutils.TestServerInterface
	execSchedules execSchedulesFn
	cfg           *scheduledjobs.JobExecutionConfig
	sqlDB         *sqlutils.SQLRunner
}

// newTestHelper creates and initializes appropriate state for a test,
// returning testHelper as well as a cleanup function.
// This test helper does not use system tables for jobs and scheduled jobs.
// It creates separate tables for the test, that are then dropped when cleanup
// function executes.  Because of this, the execution of job scheduler daemon
// is disabled by this test helper.
// If you want to run daemon, invoke it directly.
//
// The testHelper will accelerate the adoption and cancellation loops inside of
// the registry.
func newTestHelper(t *testing.T) (*testHelper, func()) {
	return newTestHelperForTables(t, jobstest.UseTestTables, nil)
}

func newTestHelperWithServerArgs(
	t *testing.T, argsFn func(args *base.TestServerArgs),
) (*testHelper, func()) {
	return newTestHelperForTables(t, jobstest.UseTestTables, argsFn)
}

func newTestHelperForTables(
	t *testing.T, envTableType jobstest.EnvTablesType, argsFn func(args *base.TestServerArgs),
) (*testHelper, func()) {
	var execSchedules execSchedulesFn

	// Setup test scheduled jobs table.
	env := jobstest.NewJobSchedulerTestEnv(envTableType, timeutil.Now())
	knobs := &TestingKnobs{
		JobSchedulerEnv: env,
		TakeOverJobsScheduling: func(daemon func(ctx context.Context, maxSchedules int64) error) {
			execSchedules = daemon
		},
	}

	args := base.TestServerArgs{
		Knobs: base.TestingKnobs{JobsTestingKnobs: knobs},
	}
	if argsFn != nil {
		argsFn(&args)
	}

	s, db, kvDB := serverutils.StartServer(t, args)

	sqlDB := sqlutils.MakeSQLRunner(db)

	if envTableType == jobstest.UseTestTables {
		sqlDB.Exec(t, jobstest.GetScheduledJobsTableSchema(env))
		sqlDB.Exec(t, jobstest.GetJobsTableSchema(env))
	}

	restoreRegistry := settings.TestingSaveRegistry()
	return &testHelper{
			env:    env,
			server: s,
			cfg: &scheduledjobs.JobExecutionConfig{
				Settings:         s.ClusterSettings(),
				InternalExecutor: s.InternalExecutor().(sqlutil.InternalExecutor),
				DB:               kvDB,
				TestingKnobs:     knobs,
			},
			sqlDB:         sqlDB,
			execSchedules: execSchedules,
		}, func() {
			if envTableType == jobstest.UseTestTables {
				sqlDB.Exec(t, "DROP TABLE "+env.SystemJobsTableName())
				sqlDB.Exec(t, "DROP TABLE "+env.ScheduledJobsTableName())
			}
			s.Stopper().Stop(context.Background())
			restoreRegistry()
		}
}

// newScheduledJob is a helper to create scheduled job with helper environment.
func (h *testHelper) newScheduledJob(t *testing.T, scheduleLabel, sql string) *ScheduledJob {
	j := NewScheduledJob(h.env)
	j.SetScheduleLabel(scheduleLabel)
	j.SetOwner(security.TestUserName())
	any, err := types.MarshalAny(&jobspb.SqlStatementExecutionArg{Statement: sql})
	require.NoError(t, err)
	j.SetExecutionDetails(InlineExecutorName, jobspb.ExecutionArguments{Args: any})
	return j
}

// newScheduledJobForExecutor is a helper to create scheduled job for the specified
// executor and its args.
func (h *testHelper) newScheduledJobForExecutor(
	scheduleLabel, executorName string, executorArgs *types.Any,
) *ScheduledJob {
	j := NewScheduledJob(h.env)
	j.SetScheduleLabel(scheduleLabel)
	j.SetOwner(security.TestUserName())
	j.SetExecutionDetails(executorName, jobspb.ExecutionArguments{Args: executorArgs})
	return j
}

// loadSchedule loads  all columns for the specified scheduled job.
func (h *testHelper) loadSchedule(t *testing.T, id int64) *ScheduledJob {
	j := NewScheduledJob(h.env)
	row, cols, err := h.cfg.InternalExecutor.QueryRowExWithCols(
		context.Background(), "sched-load", nil,
		sessiondata.InternalExecutorOverride{User: security.RootUserName()},
		fmt.Sprintf(
			"SELECT * FROM %s WHERE schedule_id = %d",
			h.env.ScheduledJobsTableName(), id),
	)
	require.NoError(t, err)
	require.NotNil(t, row)
	require.NoError(t, j.InitFromDatums(row, cols))
	return j
}

// registerScopedScheduledJobExecutor registers executor under the name,
// and returns a function which, when invoked, de-registers this executor.
func registerScopedScheduledJobExecutor(name string, ex ScheduledJobExecutor) func() {
	RegisterScheduledJobExecutorFactory(
		name,
		func() (ScheduledJobExecutor, error) {
			return ex, nil
		})
	return func() {
		executorRegistry.Lock()
		defer executorRegistry.Unlock()
		delete(executorRegistry.factories, name)
		delete(executorRegistry.executors, name)
	}
}

// addFakeJob adds a fake job associated with the specified scheduleID.
// Returns the id of the newly created job.
func addFakeJob(
	t *testing.T, h *testHelper, scheduleID int64, status Status, txn *kv.Txn,
) jobspb.JobID {
	payload := []byte("fake payload")
	datums, err := h.cfg.InternalExecutor.QueryRowEx(context.Background(), "fake-job", txn,
		sessiondata.InternalExecutorOverride{User: security.RootUserName()},
		fmt.Sprintf(`
INSERT INTO %s (created_by_type, created_by_id, status, payload)
VALUES ($1, $2, $3, $4)
RETURNING id`,
			h.env.SystemJobsTableName(),
		),
		CreatedByScheduledJobs, scheduleID, status, payload,
	)
	require.NoError(t, err)
	require.NotNil(t, datums)
	return jobspb.JobID(tree.MustBeDInt(datums[0]))
}
