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
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tabledesc_test

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/clusterversion"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/desctestutils"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/internal/validate"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/tabledesc"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/protoutil"
	"github.com/stretchr/testify/require"
)

func TestValidateAllowsExistingUserTablesWithMultipleFamilies(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()

	s, sqlDB, kvDB := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)

	sqlRunner := sqlutils.MakeSQLRunner(sqlDB)
	sqlRunner.Exec(t, `CREATE DATABASE d`)
	sqlRunner.Exec(t, `CREATE TABLE d.t (a INT PRIMARY KEY, b INT)`)

	baseDesc := desctestutils.TestingGetPublicTableDescriptor(kvDB, keys.SystemSQLCodec, "d", "t")
	descProto := protoutil.Clone(baseDesc.TableDesc()).(*descpb.TableDescriptor)
	descProto.Families = []descpb.ColumnFamilyDescriptor{
		{
			ID:              0,
			Name:            "primary",
			ColumnNames:     []string{"a"},
			ColumnIDs:       []descpb.ColumnID{1},
			DefaultColumnID: 1,
		},
		{
			ID:              1,
			Name:            "f1",
			ColumnNames:     []string{"b"},
			ColumnIDs:       []descpb.ColumnID{2},
			DefaultColumnID: 2,
		},
	}
	descProto.NextFamilyID = 2
	desc := tabledesc.NewBuilder(descProto).BuildImmutableTable()
	require.NoError(t, validate.Self(clusterversion.TestingClusterVersion, desc))
}
