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

package row

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/desctestutils"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/tabledesc"
	"github.com/cockroachdb/cockroach/pkg/sql/rowinfra"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/protoutil"
	"github.com/stretchr/testify/require"
)

func TestFetcherIgnoresProjectedArrayColumnButStillGroupsSubordinateKeys(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()

	s, sqlDB, kvDB := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)

	sqlRunner := sqlutils.MakeSQLRunner(sqlDB)
	sqlRunner.Exec(t, `CREATE DATABASE d`)
	sqlRunner.Exec(t, `CREATE TABLE d.t (k INT PRIMARY KEY, a INT[])`)
	sqlRunner.Exec(t, `INSERT INTO d.t VALUES (1, ARRAY[10, 20]), (2, ARRAY[30])`)

	tableDesc := desctestutils.TestingGetPublicTableDescriptor(
		kvDB, keys.SystemSQLCodec, "d", "t",
	)
	rf := initFetcher(t, initFetcherArgs{
		tableDesc: tableDesc,
		indexIdx:  0,
		columns:   []int{0},
	}, false /* reverseScan */, &tree.DatumAlloc{}, nil /* memMon */)

	require.NoError(t, rf.StartScan(
		ctx,
		kv.NewTxn(ctx, kvDB, 0),
		roachpb.Spans{tableDesc.IndexSpan(keys.SystemSQLCodec, tableDesc.GetPrimaryIndexID())},
		rowinfra.NoBytesLimit,
		rowinfra.NoRowLimit,
		false, /* traceKV */
		false, /* forceProductionKVBatchSize */
	))

	var got []string
	for {
		row, err := rf.NextRowDecoded(ctx)
		require.NoError(t, err)
		if row == nil {
			break
		}
		require.Len(t, row, 1)
		got = append(got, row[0].String())
	}

	require.Equal(t, []string{"1", "2"}, got)
}

func TestDeleteRemovesSubordinateKeysForDroppedArrayColumns(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()

	s, sqlDB, kvDB := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)

	sqlRunner := sqlutils.MakeSQLRunner(sqlDB)
	sqlRunner.Exec(t, `CREATE DATABASE d`)
	sqlRunner.Exec(t, `CREATE TABLE d.t (k INT PRIMARY KEY, a INT[])`)
	sqlRunner.Exec(t, `INSERT INTO d.t VALUES (1, ARRAY[10, 20])`)

	originalDesc := desctestutils.TestingGetPublicTableDescriptor(
		kvDB, keys.SystemSQLCodec, "d", "t",
	)
	descProto := protoutil.Clone(originalDesc.TableDesc()).(*descpb.TableDescriptor)
	descProto.Columns = descProto.Columns[:1]
	descProto.Families = []descpb.ColumnFamilyDescriptor{{
		ID:              0,
		Name:            "primary",
		ColumnNames:     []string{"k"},
		ColumnIDs:       []descpb.ColumnID{1},
		DefaultColumnID: 1,
	}}
	descProto.PrimaryIndex.StoreColumnIDs = nil
	descProto.PrimaryIndex.StoreColumnNames = nil
	currentDesc := tabledesc.NewBuilder(descProto).BuildImmutableTable()

	st := cluster.MakeTestingClusterSettings()
	rd := MakeDeleter(keys.SystemSQLCodec, currentDesc, nil /* requestedCols */, &st.SV, false /* internal */, nil /* metrics */)

	b := &kv.Batch{}
	require.NoError(t, rd.DeleteRow(
		ctx, b, []tree.Datum{tree.NewDInt(1)}, PartialIndexUpdateHelper{}, false, /* traceKV */
	))
	require.NoError(t, kvDB.Run(ctx, b))

	span := originalDesc.IndexSpan(keys.SystemSQLCodec, originalDesc.GetPrimaryIndexID())
	kvs, err := kvDB.Scan(ctx, span.Key, span.EndKey, 0)
	require.NoError(t, err)
	require.Len(t, kvs, 0)
}
