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

	"github.com/semistrict/ratel/pkg/security"
	"github.com/semistrict/ratel/pkg/sql/catalog/colinfo"
	"github.com/semistrict/ratel/pkg/sql/pgwire/pgcode"
	"github.com/semistrict/ratel/pkg/sql/pgwire/pgerror"
	"github.com/semistrict/ratel/pkg/sql/sem/tree"
	"github.com/semistrict/ratel/pkg/sql/sessiondata"
)

type dropWasmFunctionNode struct {
	n *tree.DropFunction
}

func (p *planner) DropFunction(
	ctx context.Context, n *tree.DropFunction,
) (planNode, error) {
	return &dropWasmFunctionNode{n: n}, nil
}

func (n *dropWasmFunctionNode) startExec(params runParams) error {
	ctx := params.ctx
	p := params.p

	hasAdmin, err := p.HasAdminRole(ctx)
	if err != nil {
		return err
	}
	if !hasAdmin {
		return pgerror.Newf(pgcode.InsufficientPrivilege,
			"only users with the admin role can DROP FUNCTION")
	}

	funcName := string(n.n.Name)

	registry := p.execCfg.UDFRegistry
	if registry == nil {
		return fmt.Errorf("UDF runtime not available on this node")
	}

	// Check if function exists locally.
	_, _, ok := registry.GetSignature(funcName)
	if !ok {
		if n.n.IfExists {
			return nil
		}
		return pgerror.Newf(pgcode.UndefinedFunction,
			"function %s does not exist", funcName)
	}

	// Deregister from UDF runtime and FunDefs.
	registry.Deregister(funcName)
	tree.UnregisterFunction(funcName)

	// Delete from system.wasm_functions.
	_, err = p.execCfg.InternalExecutor.ExecEx(
		ctx,
		"drop-wasm-function",
		p.Txn(),
		sessiondata.InternalExecutorOverride{User: security.RootUserName()},
		`DELETE FROM system.wasm_functions WHERE function_name = $1`,
		funcName,
	)
	if err != nil {
		return fmt.Errorf("deleting function from system table: %w", err)
	}

	return nil
}

func (n *dropWasmFunctionNode) Next(runParams) (bool, error) { return false, nil }
func (n *dropWasmFunctionNode) Values() tree.Datums          { return nil }
func (n *dropWasmFunctionNode) Close(context.Context)        {}

var _ planNode = &dropWasmFunctionNode{}

func (*dropWasmFunctionNode) columns() colinfo.ResultColumns { return nil }
