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

package colbuilder

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/semistrict/ratel/pkg/base"
	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/kv"
	"github.com/semistrict/ratel/pkg/roachpb"
	"github.com/semistrict/ratel/pkg/settings/cluster"
	"github.com/semistrict/ratel/pkg/sql/catalog/descpb"
	"github.com/semistrict/ratel/pkg/sql/catalog/desctestutils"
	"github.com/semistrict/ratel/pkg/sql/colexec"
	"github.com/semistrict/ratel/pkg/sql/colexec/colexecargs"
	"github.com/semistrict/ratel/pkg/sql/execinfra"
	"github.com/semistrict/ratel/pkg/sql/execinfrapb"
	"github.com/semistrict/ratel/pkg/sql/randgen"
	"github.com/semistrict/ratel/pkg/sql/rowenc"
	"github.com/semistrict/ratel/pkg/sql/sem/tree"
	"github.com/semistrict/ratel/pkg/sql/types"
	"github.com/semistrict/ratel/pkg/testutils/serverutils"
	"github.com/semistrict/ratel/pkg/testutils/sqlutils"
	"github.com/semistrict/ratel/pkg/util/leaktest"
	"github.com/semistrict/ratel/pkg/util/log"
	"github.com/stretchr/testify/require"
)

// TestNewColOperatorExpectedTypeSchema ensures that NewColOperator call
// creates such an operator chain that its output type schema is exactly as the
// processor spec expects.
func TestNewColOperatorExpectedTypeSchema(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	ctx := context.Background()
	s, sqlDB, kvDB := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)

	// We will set up the following chain:
	//
	//   ColBatchScan -> a binary projection operator -> a materializer
	//
	// such that the scan operator reads INT2 type but is expected to output
	// INT4 column, then the projection operator performs a binary operation
	// and returns an INT8 column.
	//
	// The crux of the test is an artificial setup of the table reader spec
	// that forces the planning of a cast operator on top of the scan - if such
	// doesn't occur, then the binary projection operator will panic because
	// it expects an Int32 vector whereas an Int16 vector is provided.

	const numRows = 10
	sqlutils.CreateTable(
		t, sqlDB, "t",
		"k INT2 PRIMARY KEY",
		numRows,
		sqlutils.ToRowFn(sqlutils.RowIdxFn),
	)

	st := cluster.MakeTestingClusterSettings()
	evalCtx := tree.MakeTestingEvalContext(st)
	defer evalCtx.Stop(ctx)
	txn := kv.NewTxn(ctx, s.DB(), s.NodeID())
	flowCtx := &execinfra.FlowCtx{
		EvalCtx: &evalCtx,
		Cfg: &execinfra.ServerConfig{
			Settings: st,
		},
		Txn:    txn,
		NodeID: evalCtx.NodeID,
	}

	streamingMemAcc := evalCtx.Mon.MakeBoundAccount()
	defer streamingMemAcc.Close(ctx)

	desc := desctestutils.TestingGetPublicTableDescriptor(kvDB, keys.SystemSQLCodec, "test", "t")
	var spec descpb.IndexFetchSpec
	if err := rowenc.InitIndexFetchSpec(
		&spec, keys.SystemSQLCodec,
		desc, desc.GetPrimaryIndex(),
		[]descpb.ColumnID{desc.PublicColumns()[0].GetID()},
	); err != nil {
		t.Fatal(err)
	}
	tr := execinfrapb.TableReaderSpec{
		FetchSpec: spec,
		Spans:     make([]roachpb.Span, 1),
	}
	var err error
	tr.Spans[0].Key, err = randgen.TestingMakePrimaryIndexKey(desc, 0)
	if err != nil {
		t.Fatal(err)
	}
	tr.Spans[0].EndKey, err = randgen.TestingMakePrimaryIndexKey(desc, numRows+1)
	if err != nil {
		t.Fatal(err)
	}
	var monitorRegistry colexecargs.MonitorRegistry
	defer monitorRegistry.Close(ctx)
	args := &colexecargs.NewColOperatorArgs{
		Spec: &execinfrapb.ProcessorSpec{
			Core:        execinfrapb.ProcessorCoreUnion{TableReader: &tr},
			ResultTypes: []*types.T{types.Int4},
		},
		StreamingMemAccount: &streamingMemAcc,
		MonitorRegistry:     &monitorRegistry,
	}
	r1, err := NewColOperator(ctx, flowCtx, args)
	require.NoError(t, err)
	defer r1.TestCleanupNoError(t)

	args = &colexecargs.NewColOperatorArgs{
		Spec: &execinfrapb.ProcessorSpec{
			Input:       []execinfrapb.InputSyncSpec{{ColumnTypes: []*types.T{types.Int4}}},
			Core:        execinfrapb.ProcessorCoreUnion{Noop: &execinfrapb.NoopCoreSpec{}},
			Post:        execinfrapb.PostProcessSpec{RenderExprs: []execinfrapb.Expression{{Expr: "@1 - 1"}}},
			ResultTypes: []*types.T{types.Int},
		},
		Inputs:              []colexecargs.OpWithMetaInfo{{Root: r1.Root}},
		StreamingMemAccount: &streamingMemAcc,
		MonitorRegistry:     &monitorRegistry,
	}
	r, err := NewColOperator(ctx, flowCtx, args)
	require.NoError(t, err)
	defer r.TestCleanupNoError(t)

	m := colexec.NewMaterializer(
		flowCtx,
		0, /* processorID */
		r.OpWithMetaInfo,
		[]*types.T{types.Int},
	)

	m.Start(ctx)
	var rowIdx int
	for {
		row, meta := m.Next()
		require.Nil(t, meta)
		if row == nil {
			break
		}
		require.Equal(t, 1, len(row))
		expected := tree.DInt(rowIdx)
		require.True(t, row[0].Datum.Compare(&evalCtx, &expected) == 0)
		rowIdx++
	}
	require.Equal(t, numRows, rowIdx)
}

func TestSupportedNativelyRejectsScanLocalArrayTableReader(t *testing.T) {
	defer leaktest.AfterTest(t)()

	spec := &execinfrapb.ProcessorSpec{
		Core: execinfrapb.ProcessorCoreUnion{
			TableReader: &execinfrapb.TableReaderSpec{
				ArrayEqualsAnyFilter: &execinfrapb.ArrayEqualsAnyFilterSpec{
					ArrayColIdx: 0,
				},
			},
		},
	}

	require.ErrorIs(t, supportedNatively(spec), errTableReaderScanLocalWrap)
}

func TestSupportedNativelyAcceptsScanLocalJSONTableReader(t *testing.T) {
	defer leaktest.AfterTest(t)()

	spec := &execinfrapb.ProcessorSpec{
		Core: execinfrapb.ProcessorCoreUnion{
			TableReader: &execinfrapb.TableReaderSpec{
				JsonExistsFilter: &execinfrapb.JSONExistsFilterSpec{
					SourceColIdx: 0,
					Kind:         1,
					Key:          "a",
				},
				JsonPathCompareFilter: &execinfrapb.JSONPathCompareFilterSpec{
					SourceColIdx: 0,
					Kind:         4,
					Path:         []string{`p:"a"`},
					Mode:         1,
				},
				JsonContainsFilter: &execinfrapb.JSONContainsFilterSpec{
					SourceColIdx: 0,
					Kind:         4,
					Path:         []string{`p:"a"`},
					Right:        execinfrapb.Expression{Expr: `'{"b":[1]}'::JSONB`},
				},
				JsonAccesses: []execinfrapb.JSONAccessSpec{{
					SourceColIdx: 0,
					Kind:         5,
					Path:         []string{`p:"a"`},
				}},
			},
		},
	}

	require.NoError(t, supportedNatively(spec))
}

// BenchmarkRenderPlanning benchmarks how long it takes to run a query with many
// render expressions inside. With small number of rows to read, the overhead of
// allocating the initial vectors for the projection operators dominates.
func BenchmarkRenderPlanning(b *testing.B) {
	defer leaktest.AfterTest(b)()
	defer log.Scope(b).Close(b)

	ctx := context.Background()
	s, db, _ := serverutils.StartServer(b, base.TestServerArgs{SQLMemoryPoolSize: 10 << 30})
	defer s.Stopper().Stop(ctx)

	jsonValue := `'{"string": "string", "int": 123, "null": null, "nested": {"string": "string", "int": 123, "null": null, "nested": {"string": "string", "int": 123, "null": null}}}'`

	sqlDB := sqlutils.MakeSQLRunner(db)
	for _, numRows := range []int{1, 1 << 3, 1 << 6, 1 << 9} {
		sqlDB.Exec(b, "DROP TABLE IF EXISTS bench")
		sqlDB.Exec(b, "CREATE TABLE bench (id INT PRIMARY KEY, state JSONB)")
		sqlDB.Exec(b, fmt.Sprintf(`INSERT INTO bench SELECT i, %s FROM generate_series(1, %d) AS g(i)`, jsonValue, numRows))
		sqlDB.Exec(b, "ANALYZE bench")
		for _, numRenders := range []int{1, 1 << 4, 1 << 8, 1 << 12} {
			var sb strings.Builder
			sb.WriteString("SELECT ")
			for i := 0; i < numRenders; i++ {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(fmt.Sprintf("state->'nested'->>'nested' AS test%d", i+1))
			}
			sb.WriteString(" FROM bench")
			query := sb.String()
			b.Run(fmt.Sprintf("rows=%d/renders=%d", numRows, numRenders), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					sqlDB.Exec(b, query)
				}
			})
		}
	}
}
