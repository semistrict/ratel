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

package importer_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/settings/cluster"
	"github.com/semistrict/ratel/pkg/sql/catalog"
	"github.com/semistrict/ratel/pkg/sql/catalog/bootstrap"
	"github.com/semistrict/ratel/pkg/sql/catalog/descpb"
	"github.com/semistrict/ratel/pkg/sql/importer"
	"github.com/semistrict/ratel/pkg/sql/parser"
	"github.com/semistrict/ratel/pkg/sql/row"
	"github.com/semistrict/ratel/pkg/sql/sem/tree"
	"github.com/semistrict/ratel/pkg/sql/sessiondata"
	"github.com/semistrict/ratel/pkg/testutils/skip"
	"github.com/semistrict/ratel/pkg/util/ctxgroup"
	"github.com/semistrict/ratel/pkg/util/timeutil"
	"github.com/semistrict/ratel/pkg/workload"
	"github.com/semistrict/ratel/pkg/workload/tpcc"
)

// BenchmarkConvertToKVs/tpcc/warehouses=1-8         1        3558824936 ns/op          22.46 MB/s
func BenchmarkConvertToKVs(b *testing.B) {
	skip.UnderShort(b, "skipping long benchmark")

	tpccGen := tpcc.FromWarehouses(1)
	b.Run(`tpcc/warehouses=1`, func(b *testing.B) {
		benchmarkConvertToKVs(b, tpccGen)
	})
}

func toTableDescriptor(
	t workload.Table, tableID descpb.ID, ts time.Time,
) (catalog.TableDescriptor, error) {
	ctx := context.Background()
	semaCtx := tree.MakeSemaContext()
	stmt, err := parser.ParseOne(fmt.Sprintf(`CREATE TABLE "%s" %s`, t.Name, t.Schema))
	if err != nil {
		return nil, err
	}
	createTable, ok := stmt.AST.(*tree.CreateTable)
	if !ok {
		return nil, errors.Errorf("expected *tree.CreateTable got %T", stmt)
	}
	// We need to assign a valid parent database ID to the table descriptor, but
	// the value itself doesn't matter, so we arbitrarily pick the system database
	// ID because we know it's valid.
	parentID := descpb.ID(keys.SystemDatabaseID)
	testSettings := cluster.MakeTestingClusterSettings()
	tableDesc, err := importer.MakeTestingSimpleTableDescriptor(
		ctx, &semaCtx, testSettings, createTable, parentID, keys.PublicSchemaID, tableID, importer.NoFKs, ts.UnixNano())
	if err != nil {
		return nil, err
	}
	return tableDesc.ImmutableCopy().(catalog.TableDescriptor), nil
}

func benchmarkConvertToKVs(b *testing.B, g workload.Generator) {
	ctx := context.Background()
	tableID := descpb.ID(bootstrap.TestingUserDescID(0))
	ts := timeutil.Now()

	var bytes int64
	b.ResetTimer()
	for _, t := range g.Tables() {
		tableDesc, err := toTableDescriptor(t, tableID, ts)
		if err != nil {
			b.Fatal(err)
		}

		kvCh := make(chan row.KVBatch)
		g := ctxgroup.WithContext(ctx)
		g.GoCtx(func(ctx context.Context) error {
			defer close(kvCh)
			wc := importer.NewWorkloadKVConverter(
				0, tableDesc, t.InitialRows, 0, t.InitialRows.NumBatches, kvCh)
			evalCtx := &tree.EvalContext{
				SessionDataStack: sessiondata.NewStack(&sessiondata.SessionData{}),
				Codec:            keys.SystemSQLCodec,
			}
			semaCtx := tree.MakeSemaContext()
			return wc.Worker(ctx, evalCtx, &semaCtx)
		})
		for kvBatch := range kvCh {
			for i := range kvBatch.KVs {
				kv := &kvBatch.KVs[i]
				bytes += int64(len(kv.Key) + len(kv.Value.RawBytes))
			}
		}
		if err := g.Wait(); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	b.SetBytes(bytes)
}
