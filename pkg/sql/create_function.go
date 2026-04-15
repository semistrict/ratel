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
	"fmt"
	"strings"

	"github.com/semistrict/ratel/pkg/security"
	"github.com/semistrict/ratel/pkg/sql/catalog/colinfo"
	"github.com/semistrict/ratel/pkg/sql/pgwire/pgcode"
	"github.com/semistrict/ratel/pkg/sql/pgwire/pgerror"
	"github.com/semistrict/ratel/pkg/sql/sem/tree"
	"github.com/semistrict/ratel/pkg/sql/sessiondata"
	"github.com/semistrict/ratel/pkg/sql/types"
	"github.com/semistrict/ratel/pkg/sql/udfruntime"
)

type createWasmFunctionNode struct {
	n *tree.CreateFunction
}

func (p *planner) CreateFunction(
	ctx context.Context, n *tree.CreateFunction,
) (planNode, error) {
	return &createWasmFunctionNode{n: n}, nil
}

func (n *createWasmFunctionNode) startExec(params runParams) error {
	ctx := params.ctx
	p := params.p

	// Require admin role to create functions.
	hasAdmin, err := p.HasAdminRole(ctx)
	if err != nil {
		return err
	}
	if !hasAdmin {
		return pgerror.Newf(pgcode.InsufficientPrivilege,
			"only users with the admin role can CREATE FUNCTION")
	}

	// Resolve parameter types.
	var paramValTypes []udfruntime.ValType
	var sqlArgTypes []*types.T
	for i, param := range n.n.Params {
		t, err := tree.ResolveType(ctx, param.Type, p.semaCtx.TypeResolver)
		if err != nil {
			return fmt.Errorf("resolving parameter %d type: %w", i, err)
		}
		sqlArgTypes = append(sqlArgTypes, t)
		vt, err := udfruntime.SQLTypeToValType(t)
		if err != nil {
			return err
		}
		paramValTypes = append(paramValTypes, vt)
	}

	// Resolve return type.
	retType, err := tree.ResolveType(ctx, n.n.ReturnType, p.semaCtx.TypeResolver)
	if err != nil {
		return fmt.Errorf("resolving return type: %w", err)
	}
	retValType, err := udfruntime.SQLTypeToValType(retType)
	if err != nil {
		return err
	}

	// Get the UDF registry from server config.
	registry := p.execCfg.UDFRegistry
	if registry == nil {
		return fmt.Errorf("UDF runtime not available on this node")
	}

	funcName := string(n.n.Name)
	lang := n.n.Language

	switch lang {
	case "javascript":
		// JavaScript UDFs always have access to sql``, so they can read
		// the database. IMMUTABLE is not safe — the optimizer would fold
		// the function at plan time, executing SQL during planning.
		if n.n.Volatility == tree.VolatilityImmutable {
			return pgerror.Newf(pgcode.InvalidFunctionDefinition,
				"JavaScript functions cannot be IMMUTABLE because they have access to sql`...`; use STABLE or VOLATILE")
		}
		// Default to STABLE if no volatility specified.
		if n.n.Volatility == 0 {
			n.n.Volatility = tree.VolatilityStable
		}

		// Persist and execute the wrapped source so bare-body CREATE FUNCTION
		// remains reloadable and retains the historical async semantics.
		jsBody := prepareJavaScriptBody(n.n.Params, n.n.Body)

		err = registry.CompileAndRegisterJS(funcName, jsBody,
			paramValTypes, retValType, 0)
		if err != nil {
			return err
		}

		// Build and register the SQL function definition.
		if err := registerUDFFunDef(registry, funcName, sqlArgTypes, retType, n.n.Volatility); err != nil {
			registry.Deregister(funcName)
			return err
		}

		// Persist to system.wasm_functions so other nodes can discover it.
		argTypesBytes := encodeArgTypes(sqlArgTypes)
		retTypeBytes := []byte{byte(retValType)}

		_, err = p.execCfg.InternalExecutor.ExecEx(
			ctx,
			"create-js-function",
			p.Txn(),
			sessiondata.InternalExecutorOverride{User: security.RootUserName()},
			`UPSERT INTO system.wasm_functions
			(database_id, schema_id, function_name, arg_types, return_type,
			 wasm_module, wat_source, export_name, owner, volatility)
			VALUES (0, 0, $1, $2, $3, $4, $5, 'invoke', $6, $7)`,
			funcName,
			argTypesBytes,
			retTypeBytes,
			[]byte{},
			jsBody,
			p.User().Normalized(),
			persistedUDFVolatility(n.n.Volatility),
		)
		if err != nil {
			tree.UnregisterFunction(funcName)
			registry.Deregister(funcName)
			return fmt.Errorf("persisting JavaScript function: %w", err)
		}

	default:
		return fmt.Errorf("unsupported language %q; supported: javascript", lang)
	}

	return nil
}

// registerUDFFunDef creates a FunctionDefinition from a compiled UDF
// function and registers it in the global FunDefs map.
func registerUDFFunDef(
	registry *udfruntime.Registry,
	funcName string,
	sqlArgTypes []*types.T,
	retType *types.T,
	volatility tree.Volatility,
) error {
	fn, err := registry.MakeFn(funcName)
	if err != nil {
		return err
	}

	argTypeList := make(tree.ArgTypes, len(sqlArgTypes))
	for i, t := range sqlArgTypes {
		argTypeList[i].Name = fmt.Sprintf("arg%d", i+1)
		argTypeList[i].Typ = t
	}

	overload := tree.Overload{
		Types:            argTypeList,
		ReturnType:       tree.FixedReturnType(retType),
		Fn:               fn,
		Volatility:       volatility,
		Info:             fmt.Sprintf("User-defined function %s", funcName),
		DistsqlBlocklist: false,
	}

	def := tree.NewFunctionDefinition(
		funcName,
		&tree.FunctionProperties{
			Category:                "User-defined functions",
			AvailableOnPublicSchema: true,
		},
		[]tree.Overload{overload},
	)

	tree.RegisterFunction(funcName, def)
	return nil
}

func encodeArgTypes(types []*types.T) []byte {
	result := make([]byte, len(types))
	for i, t := range types {
		vt, _ := udfruntime.SQLTypeToValType(t)
		result[i] = byte(vt)
	}
	return result
}

func prepareJavaScriptBody(params []tree.FuncParam, body string) string {
	// Backwards compatibility: callers have historically provided a bare body,
	// and those were always wrapped as async functions.
	if strings.Contains(body, "function invoke") {
		return body
	}

	paramNames := make([]string, len(params))
	for i, param := range params {
		if param.Name != "" {
			paramNames[i] = param.Name
		} else {
			paramNames[i] = fmt.Sprintf("$%d", i+1)
		}
	}
	return fmt.Sprintf("async function invoke(%s) {\n%s\n}",
		strings.Join(paramNames, ", "), body)
}

func persistedUDFVolatility(volatility tree.Volatility) string {
	if volatility == 0 {
		return tree.VolatilityImmutable.String()
	}
	return volatility.String()
}

func parsePersistedUDFVolatility(language udfruntime.Language, volatility string) tree.Volatility {
	switch volatility {
	case tree.VolatilityImmutable.String():
		return tree.VolatilityImmutable
	case tree.VolatilityStable.String():
		return tree.VolatilityStable
	case tree.VolatilityVolatile.String():
		return tree.VolatilityVolatile
	case "":
		if language == udfruntime.LangJavaScript {
			return tree.VolatilityStable
		}
	}
	return tree.VolatilityImmutable
}

func (n *createWasmFunctionNode) Next(runParams) (bool, error) { return false, nil }
func (n *createWasmFunctionNode) Values() tree.Datums          { return nil }
func (n *createWasmFunctionNode) Close(context.Context)        {}

var _ planNode = &createWasmFunctionNode{}

func (*createWasmFunctionNode) columns() colinfo.ResultColumns { return nil }
