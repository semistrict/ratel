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

package udfruntime

import (
	"context"

	v8 "github.com/tommie/v8go"
)

// TxnContext is a V8 context scoped to a single database transaction.
// It is created once at transaction start and reused for all UDF calls
// within that transaction. Function definitions are evaluated lazily
// on first call and cached for the transaction's lifetime.
//
// TxnContext is not safe for concurrent use. The caller must hold
// Registry.execMu when calling methods on it.
type TxnContext struct {
	reg       *Registry
	v8ctx     *v8.Context
	executor  SQLExecutor
	goCtx     context.Context
	txn       interface{}
	override  interface{}
	setupDone map[string]bool
}

// NewTxnContext creates a V8 context bound to the given transaction.
// The sql“ tagged template is always available and uses async execution
// (returns Promises, SQL runs in goroutines).
func (r *Registry) NewTxnContext(
	executor SQLExecutor,
	goCtx context.Context,
	txn interface{},
	override interface{},
) *TxnContext {
	r.execMu.Lock()
	defer r.execMu.Unlock()

	global := v8.NewObjectTemplate(r.iso)
	global.Set("sql", r.sqlTemplate)
	v8ctx := v8.NewContext(r.iso, global)

	return &TxnContext{
		reg:       r,
		v8ctx:     v8ctx,
		executor:  executor,
		goCtx:     goCtx,
		txn:       txn,
		override:  override,
		setupDone: make(map[string]bool),
	}
}

// Close releases the V8 context. Must be called when the transaction ends.
func (tc *TxnContext) Close() {
	tc.reg.execMu.Lock()
	defer tc.reg.execMu.Unlock()
	tc.v8ctx.Close()
}
