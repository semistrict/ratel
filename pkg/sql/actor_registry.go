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
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package sql

import (
	"bytes"
	"context"

	"github.com/cockroachdb/errors"
	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/kv"
	"github.com/semistrict/ratel/pkg/security"
	"github.com/semistrict/ratel/pkg/sql/pgwire/pgcode"
	"github.com/semistrict/ratel/pkg/sql/pgwire/pgerror"
	"github.com/semistrict/ratel/pkg/sql/sem/tree"
	"github.com/semistrict/ratel/pkg/sql/sessiondata"
)

// ensureActorExists makes actor creation a cluster-global, idempotent
// initialization step performed before actor-scoped writes emit KV. The row in
// system.actors is the single source of truth for actor existence, so
// concurrent first writes on different nodes converge on exactly one logical
// creation.
//
// The function is structured as a fast-path check followed by a slow-path
// creation. The fast path reads system.actors without acquiring any locks; if
// the actor already exists and its hash matches, we return immediately. Only
// first-write paths hit the slow path which inserts the registry row.
//
// No dedicated range is created per actor. Multiple small actors share ranges.
// The split queue handles splitting at actor boundaries when a range grows
// large enough.
func (p *planner) ensureActorExists(ctx context.Context, actorName string) error {
	if actorName == "" {
		return nil
	}

	actorHash := keys.ActorHash(actorName)
	tenantID := p.ExecCfg().Codec.TenantID()
	ie := p.actorRegistryInternalExecutor(ctx)

	// Fast path: actor already registered.
	row, err := ie.QueryRowEx(
		ctx,
		"check-actor-exists",
		nil, // outside any txn — read-only point lookup
		sessiondata.InternalExecutorOverride{
			User:     security.RootUserName(),
			Database: "system",
		},
		`SELECT actor_hash FROM system.actors WHERE tenant_id = $1 AND actor_name = $2`,
		tenantID.ToUint64(),
		actorName,
	)
	if err != nil {
		return err
	}
	if row != nil {
		storedHash := []byte(*row[0].(*tree.DBytes))
		if !bytes.Equal(storedHash, actorHash[:]) {
			return errors.AssertionFailedf("actor %q hash mismatch in registry", actorName)
		}
		return nil
	}

	// Slow path: first write for this actor. Register it.
	if err := p.ExecCfg().DB.Txn(ctx, func(ctx context.Context, txn *kv.Txn) error {
		if _, err := ie.ExecEx(
			ctx,
			"ensure-actor-exists",
			txn,
			sessiondata.InternalExecutorOverride{
				User:     security.RootUserName(),
				Database: "system",
			},
			`INSERT INTO system.actors (tenant_id, actor_name, actor_hash)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (tenant_id, actor_name) DO NOTHING`,
			tenantID.ToUint64(),
			actorName,
			actorHash[:],
		); err != nil {
			if pgerror.GetPGCode(err) == pgcode.UniqueViolation {
				return pgerror.Newf(
					pgcode.UniqueViolation,
					"actor %q collides with an existing actor hash in tenant %d",
					actorName,
					tenantID.ToUint64(),
				)
			}
			return err
		}

		verifyRow, err := ie.QueryRowEx(
			ctx,
			"load-actor",
			txn,
			sessiondata.InternalExecutorOverride{
				User:     security.RootUserName(),
				Database: "system",
			},
			`SELECT actor_hash FROM system.actors WHERE tenant_id = $1 AND actor_name = $2`,
			tenantID.ToUint64(),
			actorName,
		)
		if err != nil {
			return err
		}
		if verifyRow == nil {
			return errors.AssertionFailedf("actor %q not found after creation", actorName)
		}
		storedHash := []byte(*verifyRow[0].(*tree.DBytes))
		if !bytes.Equal(storedHash, actorHash[:]) {
			return errors.AssertionFailedf("actor %q hash mismatch in registry", actorName)
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (p *planner) actorRegistryInternalExecutor(ctx context.Context) *InternalExecutor {
	sd := p.SessionData().Clone()
	sd.ActorScope = ""
	return p.ExecCfg().InternalExecutorFactory(ctx, sd).(*InternalExecutor)
}
