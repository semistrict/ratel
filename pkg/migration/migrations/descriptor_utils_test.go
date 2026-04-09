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

package migrations_test

import (
	"context"
	"testing"

	"github.com/semistrict/ratel/pkg/base"
	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/migration/migrations"
	"github.com/semistrict/ratel/pkg/security"
	"github.com/semistrict/ratel/pkg/sql/catalog"
	"github.com/semistrict/ratel/pkg/sql/catalog/catpb"
	"github.com/semistrict/ratel/pkg/sql/catalog/descpb"
	"github.com/semistrict/ratel/pkg/sql/catalog/tabledesc"
	"github.com/semistrict/ratel/pkg/sql/privilege"
	"github.com/semistrict/ratel/pkg/sql/types"
	"github.com/semistrict/ratel/pkg/testutils/sqlutils"
	"github.com/semistrict/ratel/pkg/testutils/testcluster"
	"github.com/semistrict/ratel/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

func TestCreateSystemTable(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()

	fakeTableSchema := `CREATE TABLE public.fake_table (
	id UUID NOT NULL,
	CONSTRAINT "primary" PRIMARY KEY (id ASC)
)`
	fakeTable := descpb.TableDescriptor{
		Name:                    "fake_table",
		ParentID:                keys.SystemDatabaseID,
		UnexposedParentSchemaID: keys.PublicSchemaID,
		Columns: []descpb.ColumnDescriptor{
			{Name: "id", ID: 1, Type: types.Uuid, Nullable: false},
		},
		NextColumnID: 2,
		Families: []descpb.ColumnFamilyDescriptor{
			{
				Name:            "primary",
				ID:              0,
				ColumnNames:     []string{"id", "secret", "expiration"},
				ColumnIDs:       []descpb.ColumnID{1, 2, 3},
				DefaultColumnID: 0,
			},
		},
		NextFamilyID: 1,
		PrimaryIndex: descpb.IndexDescriptor{
			Name:           tabledesc.LegacyPrimaryKeyIndexName,
			ID:             1,
			Unique:         true,
			KeyColumnNames: []string{"id"},
			KeyColumnDirections: []descpb.IndexDescriptor_Direction{
				descpb.IndexDescriptor_ASC,
			},
			KeyColumnIDs: []descpb.ColumnID{1},
		},
		NextIndexID: 2,
		Privileges: catpb.NewCustomSuperuserPrivilegeDescriptor(
			privilege.ReadData,
			security.NodeUserName(),
		),
	}

	table := tabledesc.NewBuilder(&fakeTable).BuildCreatedMutable().(catalog.TableDescriptor)

	tc := testcluster.StartTestCluster(t, 1, base.TestClusterArgs{})
	defer tc.Stopper().Stop(ctx)

	sqlDB := sqlutils.MakeSQLRunner(tc.ServerConn(0))

	// Verify that the keys were not written.
	checkEntries := func(t *testing.T) [][]string {
		return sqlDB.QueryStr(t, `
SELECT *
  FROM system.namespace
 WHERE "parentID" = $1 AND "parentSchemaID" = $2 AND name = $3`,
			table.GetParentID(), table.GetParentSchemaID(), table.GetName())
	}
	require.Len(t, checkEntries(t), 0)
	require.NoError(t, migrations.CreateSystemTable(
		ctx, tc.Server(0).DB(), keys.SystemSQLCodec, table,
	))
	require.Len(t, checkEntries(t), 1)
	sqlDB.CheckQueryResults(t,
		"SELECT create_statement FROM [SHOW CREATE TABLE system.fake_table]",
		[][]string{{fakeTableSchema}})

	// Make sure it's idempotent.
	require.NoError(t, migrations.CreateSystemTable(
		ctx, tc.Server(0).DB(), keys.SystemSQLCodec, table,
	))
	require.Len(t, checkEntries(t), 1)

}
