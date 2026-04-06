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

	"github.com/cockroachdb/cockroach/pkg/clusterversion"
	"github.com/cockroachdb/cockroach/pkg/jobs"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/migration"
	"github.com/cockroachdb/cockroach/pkg/security"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/systemschema"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
)

const addVolatilityToWasmFunctions = `
ALTER TABLE system.wasm_functions
ADD COLUMN IF NOT EXISTS volatility STRING NOT NULL DEFAULT 'immutable' FAMILY "primary"
`

func wasmFunctionsVolatilityMigration(
	ctx context.Context, cs clusterversion.ClusterVersion, d migration.TenantDeps, _ *jobs.Job,
) error {
	op := operation{
		name:           "add-wasm-functions-volatility-col",
		schemaList:     []string{"volatility"},
		query:          addVolatilityToWasmFunctions,
		schemaExistsFn: hasColumn,
	}
	if err := migrateTable(ctx, cs, d, op, keys.WasmFunctionsTableID, systemschema.WasmFunctionsTable); err != nil {
		return err
	}
	_, err := d.InternalExecutor.ExecEx(
		ctx,
		"backfill-wasm-functions-volatility",
		nil, /* txn */
		sessiondata.InternalExecutorOverride{User: security.NodeUserName()},
		`UPDATE system.wasm_functions
		 SET volatility = 'stable'
		 WHERE octet_length(wasm_module) = 0
		   AND volatility = 'immutable'`,
	)
	return err
}
