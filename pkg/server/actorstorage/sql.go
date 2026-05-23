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

package actorstorage

import (
	"strings"

	"github.com/cockroachdb/cockroach/pkg/sql/parser"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/errors"
)

// ValidateActorSQL rejects statements that are not safe to execute in a Durable
// Object actor scope. Actor-scoped SQL uses pre-created per-actor tables; schema
// changes must be handled out of band.
func ValidateActorSQL(sql string) error {
	stmts, err := parser.Parse(sql)
	if err != nil {
		return err
	}
	for _, stmt := range stmts {
		if tree.CanModifySchema(stmt.AST) {
			return errors.Newf("DDL is not allowed in Durable Object actor scope: %s", strings.ToUpper(stmt.AST.StatementTag()))
		}
	}
	return nil
}

// ValidateActorSQLScope checks the convention used by Ratel's actor SQL
// service: $1 is reserved for the actor scope and must appear in the query.
func ValidateActorSQLScope(sql string) error {
	normalized := strings.Join(strings.Fields(strings.ToLower(sql)), " ")
	scopedPredicate := strings.Contains(normalized, "actor_id = $1")
	scopedInsert := strings.Contains(normalized, "(actor_id,") &&
		(strings.Contains(normalized, "values ($1,") || strings.Contains(normalized, "values($1,"))
	if !scopedPredicate && !scopedInsert {
		return errors.New("actor SQL must bind actor scope with $1")
	}
	return nil
}
