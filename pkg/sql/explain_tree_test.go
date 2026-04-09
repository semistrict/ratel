// Copyright 2018 The Cockroach Authors.
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

package sql

import (
	"context"
	"testing"

	"github.com/cockroachdb/datadriven"
	"github.com/semistrict/ratel/pkg/base"
	"github.com/semistrict/ratel/pkg/kv"
	"github.com/semistrict/ratel/pkg/security"
	"github.com/semistrict/ratel/pkg/sql/execstats"
	"github.com/semistrict/ratel/pkg/sql/opt/exec/explain"
	"github.com/semistrict/ratel/pkg/sql/parser"
	"github.com/semistrict/ratel/pkg/sql/sessiondatapb"
	"github.com/semistrict/ratel/pkg/sql/sessionphase"
	"github.com/semistrict/ratel/pkg/testutils"
	"github.com/semistrict/ratel/pkg/testutils/serverutils"
	"github.com/semistrict/ratel/pkg/testutils/sqlutils"
	"github.com/semistrict/ratel/pkg/util/leaktest"
	"github.com/semistrict/ratel/pkg/util/log"
	yaml "gopkg.in/yaml.v2"
)

func TestPlanToTreeAndPlanToString(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	ctx := context.Background()
	s, sqlDB, db := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)

	execCfg := s.ExecutorConfig().(ExecutorConfig)
	r := sqlutils.MakeSQLRunner(sqlDB)
	r.Exec(t, `
		SET CLUSTER SETTING sql.stats.automatic_collection.enabled = false;
		CREATE DATABASE t;
		USE t;
	`)

	datadriven.RunTest(t, testutils.TestDataPath(t, "explain_tree"), func(t *testing.T, d *datadriven.TestData) string {
		switch d.Cmd {
		case "exec":
			r.Exec(t, d.Input)
			return ""

		case "plan-string", "plan-tree":
			stmt, err := parser.ParseOne(d.Input)
			if err != nil {
				t.Fatal(err)
			}

			internalPlanner, cleanup := NewInternalPlanner(
				"test",
				kv.NewTxn(ctx, db, s.NodeID()),
				security.RootUserName(),
				&MemoryMetrics{},
				&execCfg,
				sessiondatapb.SessionData{},
			)
			defer cleanup()
			p := internalPlanner.(*planner)

			ih := &p.instrumentation
			ih.codec = execCfg.Codec
			ih.collectBundle = true
			ih.savePlanForStats = true

			p.stmt = makeStatement(stmt, ClusterWideID{})
			if err := p.makeOptimizerPlan(ctx); err != nil {
				t.Fatal(err)
			}
			p.curPlan.flags.Set(planFlagExecDone)
			p.curPlan.close(ctx)
			if d.Cmd == "plan-string" {
				ob := ih.emitExplainAnalyzePlanToOutputBuilder(
					explain.Flags{Verbose: true, ShowTypes: true},
					sessionphase.NewTimes(),
					&execstats.QueryLevelStats{},
				)
				return ob.BuildString()
			}
			treeYaml, err := yaml.Marshal(ih.PlanForStats(ctx))
			if err != nil {
				t.Fatal(err)
			}
			return string(treeYaml)

		default:
			t.Fatalf("unsupported command %s", d.Cmd)
			return ""
		}
	})
}
