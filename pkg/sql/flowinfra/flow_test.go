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

package flowinfra_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/semistrict/ratel/pkg/base"
	"github.com/semistrict/ratel/pkg/kv"
	"github.com/semistrict/ratel/pkg/security"
	"github.com/semistrict/ratel/pkg/settings/cluster"
	"github.com/semistrict/ratel/pkg/sql"
	"github.com/semistrict/ratel/pkg/sql/parser"
	"github.com/semistrict/ratel/pkg/sql/sessiondatapb"
	"github.com/semistrict/ratel/pkg/testutils/serverutils"
	"github.com/semistrict/ratel/pkg/testutils/sqlutils"
	"github.com/semistrict/ratel/pkg/util/leaktest"
	"github.com/semistrict/ratel/pkg/util/log"
)

// BenchmarkFlowSetup sets up a flow for a scan that is dominated by the setup
// cost.
func BenchmarkFlowSetup(b *testing.B) {
	defer leaktest.AfterTest(b)()
	defer log.Scope(b).Close(b)
	ctx := context.Background()

	s, conn, _ := serverutils.StartServer(b, base.TestServerArgs{
		Settings: cluster.MakeTestingClusterSettings(),
	})
	defer s.Stopper().Stop(ctx)

	r := sqlutils.MakeSQLRunner(conn)
	r.Exec(b, "CREATE DATABASE b; CREATE TABLE b.test (k INT);")

	execCfg := s.ExecutorConfig().(sql.ExecutorConfig)
	dsp := execCfg.DistSQLPlanner
	stmt, err := parser.ParseOne("SELECT k FROM b.test WHERE k=1")
	if err != nil {
		b.Fatal(err)
	}
	for _, vectorize := range []bool{true, false} {
		for _, distribute := range []bool{true, false} {
			b.Run(fmt.Sprintf("vectorize=%t/distribute=%t", vectorize, distribute), func(b *testing.B) {
				vectorizeMode := sessiondatapb.VectorizeOff
				if vectorize {
					vectorizeMode = sessiondatapb.VectorizeOn
				}
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					// NB: planner cannot be reset and can only be used for
					// a single statement, so we create a new one on every
					// iteration.
					planner, cleanup := sql.NewInternalPlanner(
						"test",
						kv.NewTxn(ctx, s.DB(), s.NodeID()),
						security.RootUserName(),
						&sql.MemoryMetrics{},
						&execCfg,
						sessiondatapb.SessionData{VectorizeMode: vectorizeMode},
					)
					b.StartTimer()
					err := dsp.Exec(
						ctx,
						planner,
						stmt,
						distribute,
					)
					b.StopTimer()
					if err != nil {
						b.Fatal(err)
					}
					cleanup()
				}
			})
		}
	}
}
