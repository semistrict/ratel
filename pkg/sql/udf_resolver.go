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

package sql

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/security"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/sql/udfruntime"
	"github.com/cockroachdb/cockroach/pkg/util/log"
)

// InitWasmFunctionResolver sets the tree.UDFResolver hook to lazy-load
// UDF functions from the system.wasm_functions table on cache miss.
func InitWasmFunctionResolver(execCfg *ExecutorConfig) {
	tree.UDFResolver = func(name string) *tree.FunctionDefinition {
		return resolveWasmFunction(execCfg, name)
	}
}

func resolveWasmFunction(execCfg *ExecutorConfig, name string) *tree.FunctionDefinition {
	ctx := context.Background()
	registry := execCfg.UDFRegistry
	if registry == nil {
		return nil
	}

	// Query system.wasm_functions for this function name.
	row, err := execCfg.InternalExecutor.QueryRowEx(
		ctx,
		"resolve-wasm-function",
		nil, // no txn needed for reads
		sessiondata.InternalExecutorOverride{User: security.RootUserName()},
		`SELECT wasm_module, arg_types, return_type, wat_source
		 FROM system.wasm_functions
		 WHERE function_name = $1
		 LIMIT 1`,
		name,
	)
	if err != nil {
		log.Warningf(ctx, "error resolving UDF function %q: %v", name, err)
		return nil
	}
	if row == nil {
		return nil
	}

	// Extract columns.
	argTypesBytes := []byte(tree.MustBeDBytes(row[1]))
	retTypeBytes := []byte(tree.MustBeDBytes(row[2]))
	watSource := string(tree.MustBeDString(row[3]))

	if len(retTypeBytes) == 0 {
		log.Warningf(ctx, "UDF function %q has empty return type", name)
		return nil
	}

	// Decode arg types.
	var paramValTypes []udfruntime.ValType
	var sqlArgTypes []*types.T
	for _, b := range argTypesBytes {
		vt := udfruntime.ValType(b)
		paramValTypes = append(paramValTypes, vt)
		sqlType, err := udfruntime.ValTypeToSQLType(vt)
		if err != nil {
			log.Warningf(ctx, "UDF function %q has unsupported arg type: %v", name, err)
			return nil
		}
		sqlArgTypes = append(sqlArgTypes, sqlType)
	}

	// Decode return type.
	retValType := udfruntime.ValType(retTypeBytes[0])
	retType, err := udfruntime.ValTypeToSQLType(retValType)
	if err != nil {
		log.Warningf(ctx, "UDF function %q has unsupported return type: %v", name, err)
		return nil
	}

	// JavaScript function: source is in watSource field.
	err = registry.CompileAndRegisterJS(name, watSource,
		paramValTypes, retValType, 0)
	if err != nil {
		log.Warningf(ctx, "error compiling JavaScript function %q: %v", name, err)
		return nil
	}

	// Register the SQL function definition.
	if err := registerUDFFunDef(registry, name, sqlArgTypes, retType, tree.VolatilityImmutable); err != nil {
		log.Warningf(ctx, "error registering UDF function %q: %v", name, err)
		return nil
	}

	// Return the now-registered definition.
	def, ok := tree.FunDefs[name]
	if !ok {
		log.Warningf(ctx, "UDF function %q registered but not found in FunDefs", name)
		return nil
	}
	return def
}
