// Copyright 2024 Oxide Computer Company
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

package migrations

import (
	"context"

	"github.com/semistrict/ratel/pkg/clusterversion"
	"github.com/semistrict/ratel/pkg/jobs"
	"github.com/semistrict/ratel/pkg/migration"
	"github.com/semistrict/ratel/pkg/sql/catalog/systemschema"
)

// wasmFunctionsTableMigration creates the system.wasm_functions table.
func wasmFunctionsTableMigration(
	ctx context.Context, _ clusterversion.ClusterVersion, d migration.TenantDeps, _ *jobs.Job,
) error {
	return createSystemTable(
		ctx, d.DB, d.Codec, systemschema.WasmFunctionsTable,
	)
}
