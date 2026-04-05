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

	"github.com/semistrict/ratel/pkg/security"
	"github.com/semistrict/ratel/pkg/sql/sem/tree"
	"github.com/semistrict/ratel/pkg/sql/sessiondata"
	"github.com/semistrict/ratel/pkg/sql/types"
	"github.com/semistrict/ratel/pkg/sql/wasmruntime"
	"github.com/semistrict/ratel/pkg/util/log"
)

// InitWasmFunctionResolver sets the tree.UDFResolver hook to lazy-load
// WASM functions from the system.wasm_functions table on cache miss.
func InitWasmFunctionResolver(execCfg *ExecutorConfig) {
	tree.UDFResolver = func(name string) *tree.FunctionDefinition {
		return resolveWasmFunction(execCfg, name)
	}
}

func resolveWasmFunction(execCfg *ExecutorConfig, name string) *tree.FunctionDefinition {
	ctx := context.Background()
	registry := execCfg.WasmRegistry
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
		log.Warningf(ctx, "error resolving WASM function %q: %v", name, err)
		return nil
	}
	if row == nil {
		return nil
	}

	// Extract columns.
	wasmModule := []byte(tree.MustBeDBytes(row[0]))
	argTypesBytes := []byte(tree.MustBeDBytes(row[1]))
	retTypeBytes := []byte(tree.MustBeDBytes(row[2]))

	if len(retTypeBytes) == 0 {
		log.Warningf(ctx, "WASM function %q has empty return type", name)
		return nil
	}

	// Decode arg types.
	var paramValTypes []wasmruntime.ValType
	var sqlArgTypes []*types.T
	for _, b := range argTypesBytes {
		vt := wasmruntime.ValType(b)
		paramValTypes = append(paramValTypes, vt)
		sqlType, err := wasmruntime.ValTypeToSQLType(vt)
		if err != nil {
			log.Warningf(ctx, "WASM function %q has unsupported arg type: %v", name, err)
			return nil
		}
		sqlArgTypes = append(sqlArgTypes, sqlType)
	}

	// Decode return type.
	retValType := wasmruntime.ValType(retTypeBytes[0])
	retType, err := wasmruntime.ValTypeToSQLType(retValType)
	if err != nil {
		log.Warningf(ctx, "WASM function %q has unsupported return type: %v", name, err)
		return nil
	}

	// Compile and register in the local WASM runtime.
	err = registry.CompileAndRegister(ctx, name, wasmModule, "invoke",
		paramValTypes, retValType, 0)
	if err != nil {
		log.Warningf(ctx, "error compiling WASM function %q: %v", name, err)
		return nil
	}

	// Register the SQL function definition.
	if err := registerWasmFunDef(registry, name, sqlArgTypes, retType, tree.VolatilityImmutable); err != nil {
		log.Warningf(ctx, "error registering WASM function %q: %v", name, err)
		return nil
	}

	// Return the now-registered definition.
	def, ok := tree.FunDefs[name]
	if !ok {
		log.Warningf(ctx, "WASM function %q registered but not found in FunDefs", name)
		return nil
	}
	return def
}

