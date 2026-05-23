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
	"testing"

	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

func TestValidateActorSQLRejectsDDL(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	for _, stmt := range []string{
		`CREATE TABLE messages (id INT PRIMARY KEY)`,
		`CREATE TABLE messages AS SELECT 1`,
		`CREATE INDEX messages_idx ON messages (id)`,
		`ALTER TABLE messages ADD COLUMN body STRING`,
		`DROP TABLE messages`,
		`TRUNCATE messages`,
		`ANALYZE messages`,
		`SELECT 1; CREATE TABLE messages (id INT PRIMARY KEY)`,
	} {
		err := ValidateActorSQL(stmt)
		require.Error(t, err, stmt)
		require.Contains(t, err.Error(), "DDL is not allowed")
	}
}

func TestValidateActorSQLAllowsNonDDL(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	for _, stmt := range []string{
		`SELECT 1`,
		`SELECT * FROM messages WHERE room = $1`,
		`INSERT INTO messages (room, body) VALUES ($1, $2)`,
		`UPDATE messages SET body = $1 WHERE id = $2`,
		`DELETE FROM messages WHERE id = $1`,
	} {
		require.NoError(t, ValidateActorSQL(stmt), stmt)
	}
}

func TestValidateActorSQLScopeRequiresActorPlaceholder(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	for _, stmt := range []string{
		`SELECT * FROM system.ratel_chat_messages`,
		`SELECT * FROM system.ratel_chat_messages WHERE actor_id = 'lobby'`,
		`INSERT INTO system.ratel_chat_messages (actor_id, timestamp) VALUES ('lobby', $1)`,
	} {
		require.Error(t, ValidateActorSQLScope(stmt), stmt)
	}

	for _, stmt := range []string{
		`SELECT * FROM system.ratel_chat_messages WHERE actor_id = $1`,
		`INSERT INTO system.ratel_chat_messages (actor_id, timestamp) VALUES ($1, $2)`,
		`DELETE FROM system.ratel_chat_messages WHERE actor_id = $1 AND timestamp = $2`,
	} {
		require.NoError(t, ValidateActorSQLScope(stmt), stmt)
	}
}
