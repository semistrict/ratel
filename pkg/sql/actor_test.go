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

package sql_test

import (
	"context"
	"database/sql"
	"encoding/hex"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/semistrict/ratel/pkg/base"
	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/kv"
	"github.com/semistrict/ratel/pkg/kv/kvserver/kvserverbase"
	"github.com/semistrict/ratel/pkg/roachpb"
	"github.com/semistrict/ratel/pkg/testutils/serverutils"
	"github.com/semistrict/ratel/pkg/testutils/sqlutils"
	"github.com/semistrict/ratel/pkg/testutils/testcluster"
	"github.com/semistrict/ratel/pkg/util/hlc"
	"github.com/semistrict/ratel/pkg/util/leaktest"
	"github.com/semistrict/ratel/pkg/util/log"
)

func TestActorScopeSimpleIsolation(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	s, db, _ := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(context.Background())
	defer db.Close()

	sqlDB := sqlutils.MakeSQLRunner(db)
	sqlDB.Exec(t, `CREATE TABLE t (id INT PRIMARY KEY, v STRING)`)

	sqlDB.Exec(t, `INSERT INTO t VALUES (1, 'base')`)
	sqlDB.CheckQueryResults(t, `SELECT * FROM t`, [][]string{{"1", "base"}})

	sqlDB.Exec(t, `SET actor_scope = 'alpha'`)
	sqlDB.Exec(t, `INSERT INTO t VALUES (1, 'actor-alpha')`)
	sqlDB.CheckQueryResults(t, `SHOW actor_scope`, [][]string{{"alpha"}})
	sqlDB.CheckQueryResults(t, `SELECT * FROM t`, [][]string{{"1", "actor-alpha"}})

	sqlDB.Exec(t, `SET actor_scope = 'beta'`)
	sqlDB.CheckQueryResults(t, `SHOW actor_scope`, [][]string{{"beta"}})
	sqlDB.CheckQueryResults(t, `SELECT * FROM t`, [][]string{})
	sqlDB.Exec(t, `INSERT INTO t VALUES (1, 'actor-beta')`)
	sqlDB.CheckQueryResults(t, `SELECT * FROM t`, [][]string{{"1", "actor-beta"}})

	sqlDB.Exec(t, `SET actor_scope = ''`)
	sqlDB.CheckQueryResults(t, `SHOW actor_scope`, [][]string{{""}})
	sqlDB.CheckQueryResults(t, `SELECT * FROM t`, [][]string{{"1", "base"}})
}

func TestActorTableQualifierSimpleQueries(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	s, db, _ := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(context.Background())
	defer db.Close()

	sqlDB := sqlutils.MakeSQLRunner(db)
	sqlDB.Exec(t, `CREATE TABLE t (id INT PRIMARY KEY, v STRING)`)
	sqlDB.Exec(t, `INSERT INTO t VALUES (1, 'base')`)

	sqlDB.Exec(t, `SET actor_scope = 'alpha'`)
	sqlDB.Exec(t, `INSERT INTO t VALUES (1, 'actor-alpha')`)

	sqlDB.Exec(t, `SET actor_scope = 'beta'`)
	sqlDB.Exec(t, `INSERT INTO t VALUES (1, 'actor-beta')`)

	// Use actor('name').table syntax (no session scope).
	sqlDB.Exec(t, `SET actor_scope = ''`)
	sqlDB.CheckQueryResults(t, `SELECT * FROM t`, [][]string{{"1", "base"}})
	sqlDB.CheckQueryResults(t, `SELECT * FROM actor('alpha').t`, [][]string{{"1", "actor-alpha"}})
	sqlDB.CheckQueryResults(t, `SELECT * FROM actor('beta').t`, [][]string{{"1", "actor-beta"}})
	sqlDB.CheckQueryResults(t, `SELECT * FROM actor('alpha').t WHERE id = 1`, [][]string{{"1", "actor-alpha"}})
	alphaHash := keys.ActorHash("alpha")
	sqlDB.CheckQueryResults(t, `SELECT encode(actor_id('alpha'), 'hex')`, [][]string{{hex.EncodeToString(alphaHash[:])}})
}

func TestActorCrossActorTransaction(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	s, db, _ := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(context.Background())
	defer db.Close()

	sqlDB := sqlutils.MakeSQLRunner(db)
	sqlDB.Exec(t, `CREATE TABLE t (id INT PRIMARY KEY, v STRING)`)

	// Seed data into two actors using actor().table syntax.
	sqlDB.Exec(t, `INSERT INTO actor('alice').t VALUES (1, 'alice-v1')`)
	sqlDB.Exec(t, `INSERT INTO actor('bob').t VALUES (1, 'bob-v1')`)

	// Single-actor UPDATE using actor().table syntax.
	sqlDB.Exec(t, `UPDATE actor('alice').t SET v = 'alice-v2' WHERE id = 1`)
	sqlDB.CheckQueryResults(t, `SELECT v FROM actor('alice').t WHERE id = 1`, [][]string{{"alice-v2"}})

	sqlDB.Exec(t, `UPDATE actor('bob').t SET v = 'bob-v2' WHERE id = 1`)
	sqlDB.CheckQueryResults(t, `SELECT v FROM actor('bob').t WHERE id = 1`, [][]string{{"bob-v2"}})

	// Cross-actor transaction touching both actors. Both actors are in
	// different ranges (sticky-split), so this exercises 2PC.
	sqlDB.Exec(t, `BEGIN`)
	sqlDB.Exec(t, `UPDATE actor('alice').t SET v = 'alice-v3' WHERE id = 1`)
	sqlDB.Exec(t, `UPDATE actor('bob').t SET v = 'bob-v3' WHERE id = 1`)
	sqlDB.Exec(t, `COMMIT`)

	// Verify both actors see committed data.
	sqlDB.CheckQueryResults(t, `SELECT v FROM actor('alice').t WHERE id = 1`, [][]string{{"alice-v3"}})
	sqlDB.CheckQueryResults(t, `SELECT v FROM actor('bob').t WHERE id = 1`, [][]string{{"bob-v3"}})
}

func TestActorDeletion(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	s, db, _ := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(context.Background())
	defer db.Close()

	sqlDB := sqlutils.MakeSQLRunner(db)
	sqlDB.Exec(t, `CREATE TABLE t (id INT PRIMARY KEY, v STRING)`)

	// Create actor and write data using actor().table syntax.
	sqlDB.Exec(t, `INSERT INTO actor('doomed').t VALUES (1, 'doomed-data')`)
	sqlDB.Exec(t, `INSERT INTO actor('doomed').t VALUES (2, 'doomed-data-2')`)

	// Verify data exists.
	sqlDB.CheckQueryResults(t, `SELECT count(*) FROM actor('doomed').t`, [][]string{{"2"}})
	sqlDB.CheckQueryResults(t, `SELECT count(*) FROM system.actors WHERE actor_name = 'doomed'`, [][]string{{"1"}})

	// Delete the actor.
	sqlDB.Exec(t, `SELECT crdb_internal.delete_actor('doomed')`)

	// Verify data is gone.
	sqlDB.CheckQueryResults(t, `SELECT count(*) FROM actor('doomed').t`, [][]string{{"0"}})
	sqlDB.CheckQueryResults(t, `SELECT count(*) FROM system.actors WHERE actor_name = 'doomed'`, [][]string{{"0"}})
}

func TestActorExplainShowsActorIdentity(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	s, db, _ := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(context.Background())
	defer db.Close()

	sqlDB := sqlutils.MakeSQLRunner(db)
	sqlDB.Exec(t, `CREATE TABLE t (id INT PRIMARY KEY, v STRING)`)

	// EXPLAIN with session actor_scope should show actor in scan.
	sqlDB.Exec(t, `SET actor_scope = 'alpha'`)
	rows := sqlDB.QueryStr(t, `EXPLAIN SELECT * FROM t WHERE id = 1`)
	found := false
	for _, row := range rows {
		for _, col := range row {
			if strings.Contains(col, "actor: alpha") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("EXPLAIN output should contain 'actor: alpha', got: %v", rows)
	}

	// EXPLAIN with actor().table syntax should also show actor.
	sqlDB.Exec(t, `SET actor_scope = ''`)
	rows = sqlDB.QueryStr(t, `EXPLAIN SELECT * FROM actor('beta').t WHERE id = 1`)
	found = false
	for _, row := range rows {
		for _, col := range row {
			if strings.Contains(col, "actor: beta") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("EXPLAIN output should contain 'actor: beta', got: %v", rows)
	}
}

func TestActorRegistryCreatedOnFirstWrite(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	s, db, _ := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(context.Background())
	defer db.Close()

	sqlDB := sqlutils.MakeSQLRunner(db)
	sqlDB.Exec(t, `CREATE TABLE t (id INT PRIMARY KEY, v STRING)`)
	sqlDB.CheckQueryResults(t, `SELECT count(*) FROM system.actors`, [][]string{{"0"}})

	sqlDB.Exec(t, `INSERT INTO actor('alpha').t VALUES (1, 'actor-alpha')`)

	alphaHash := keys.ActorHash("alpha")
	sqlDB.CheckQueryResults(t,
		`SELECT tenant_id, actor_name, encode(actor_hash, 'hex') FROM system.actors`,
		[][]string{{"1", "alpha", hex.EncodeToString(alphaHash[:])}},
	)
}

func TestActorRegistryConcurrentCreateSameActor(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	tc := testcluster.StartTestCluster(t, 2, base.TestClusterArgs{})
	defer tc.Stopper().Stop(context.Background())

	db1 := tc.ServerConn(0)
	defer db1.Close()
	db2 := tc.ServerConn(1)
	defer db2.Close()

	sqlutils.MakeSQLRunner(db1).Exec(t, `CREATE TABLE t (id INT PRIMARY KEY, v STRING)`)

	var start sync.WaitGroup
	start.Add(1)
	var wg sync.WaitGroup
	errCh := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			db := db1
			if i%2 == 1 {
				db = db2
			}
			start.Wait()
			if _, err := db.Exec(`SET actor_scope = 'alpha'`); err != nil {
				errCh <- err
				return
			}
			if _, err := db.Exec(`INSERT INTO t VALUES ($1, $2)`, i+1, "v"+strconv.Itoa(i+1)); err != nil {
				errCh <- err
				return
			}
			if _, err := db.Exec(`SET actor_scope = ''`); err != nil {
				errCh <- err
				return
			}
		}(i)
	}
	start.Done()
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent actor insert failed: %+v", err)
		}
	}

	sqlDB := sqlutils.MakeSQLRunner(db1)
	sqlDB.CheckQueryResults(t, `SELECT count(*) FROM system.actors WHERE actor_name = 'alpha'`, [][]string{{"1"}})
	sqlDB.CheckQueryResults(t, `SELECT count(*) FROM actor('alpha').t`, [][]string{{"8"}})
}

func TestActorScopeDistSQLIndexJoin(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	tc := testcluster.StartTestCluster(t, 2, base.TestClusterArgs{})
	defer tc.Stopper().Stop(context.Background())

	db := tc.ServerConn(0)
	defer db.Close()
	sqlDB := sqlutils.MakeSQLRunner(db)

	sqlDB.Exec(t, `CREATE TABLE t (id INT PRIMARY KEY, v INT, w STRING, INDEX by_v (v))`)
	sqlDB.Exec(t, `INSERT INTO t VALUES (2, 1, 'base')`)
	sqlDB.Exec(t, `INSERT INTO actor('alpha').t VALUES (1, 1, 'actor-alpha')`)

	assertActorIndexHasKVs(t, tc.Server(0).DB(), tableIDForName(t, db, "t"), 1 /* primary */, "alpha")
	assertActorIndexHasKVs(t, tc.Server(0).DB(), tableIDForName(t, db, "t"), 2 /* secondary */, "alpha")

	// Session-scope queries on the secondary index.
	sqlDB.Exec(t, `SET actor_scope = 'alpha'`)
	sqlDB.CheckQueryResults(t, `SELECT id FROM t@by_v`, [][]string{{"1"}})
	sqlDB.CheckQueryResults(t, `SELECT id FROM t@by_v WHERE v = 1`, [][]string{{"1"}})
	sqlDB.Exec(t, `SET actor_scope = ''`)

	// DistSQL with actor().table syntax.
	sqlDB.Exec(t, `SET distsql = always`)
	sqlDB.CheckQueryResults(t, `SELECT id FROM actor('alpha').t@by_v WHERE v = 1`, [][]string{{"1"}})

	tableID := lookupTableID(t, db, "t")
	moveActorIndexRangeToNode(t, tc, tableID, 1 /* primary index */, "alpha", 1 /* node */)
	moveActorIndexRangeToNode(t, tc, tableID, 2 /* secondary index */, "alpha", 1 /* node */)

	sqlDB.CheckQueryResults(t, `SELECT id FROM actor('alpha').t@by_v WHERE v = 1`, [][]string{{"1"}})
	sqlDB.CheckQueryResults(t, `SELECT w FROM actor('alpha').t@by_v WHERE v = 1`, [][]string{{"actor-alpha"}})
}

func TestActorRangeRejectsInteriorSplitAndCrossActorMerge(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	s, db, _ := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(context.Background())
	defer db.Close()

	sqlDB := sqlutils.MakeSQLRunner(db)
	sqlDB.Exec(t, `CREATE TABLE t (id INT PRIMARY KEY, v STRING)`)
	sqlDB.Exec(t, `INSERT INTO actor('alpha').t VALUES (1, 'actor-alpha')`)

	tableID := lookupTableID(t, db, "t")
	codec := keys.MakeActorSQLCodec(keys.SystemSQLCodec, "alpha")
	actorPrefix := roachpb.Key(codec.TenantPrefix())
	interiorSplitKey := roachpb.Key(codec.TablePrefix(uint32(tableID)))

	err := s.DB().AdminSplit(context.Background(), interiorSplitKey, hlc.MaxTimestamp)
	if err == nil || !strings.Contains(err.Error(), "cannot split actor range at interior key") {
		t.Fatalf("expected interior actor split rejection, got %+v", err)
	}

	err = s.DB().AdminMerge(context.Background(), actorPrefix)
	if err == nil || !strings.Contains(err.Error(), "cannot merge across actor boundaries") {
		t.Fatalf("expected cross-actor merge rejection, got %+v", err)
	}
}

func TestActorMaxSizeRejectsWrites(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	s, db, _ := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(context.Background())
	defer db.Close()

	sqlDB := sqlutils.MakeSQLRunner(db)
	sqlDB.Exec(t, `CREATE TABLE t (id INT PRIMARY KEY, v STRING)`)
	sqlDB.Exec(t, `INSERT INTO actor('alpha').t VALUES (1, 'seed')`)

	kvserverbase.ActorMaxSize.Override(
		context.Background(), &s.ClusterSettings().SV, 1<<10, /* 1 KiB */
	)

	// The second actor-scoped insert should be rejected by the KV backpressure
	// check because the actor range already exceeds the 1 KiB limit.
	_, err := db.Exec(`INSERT INTO actor('alpha').t VALUES (2, repeat('x', 4096))`)
	if err == nil {
		t.Fatal("expected actor size limit error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot exceed kv.actor.max_size") {
		t.Fatalf("unexpected error: %+v", err)
	}

	// Read-back the actor data to confirm the seed row is still there
	// and the rejected row is not.
	sqlDB.CheckQueryResults(t, `SELECT id FROM actor('alpha').t ORDER BY id`, [][]string{{"1"}})
}

func TestActorMutualExclusion(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	s, db, _ := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(context.Background())
	defer db.Close()

	sqlDB := sqlutils.MakeSQLRunner(db)
	sqlDB.Exec(t, `CREATE TABLE t (id INT PRIMARY KEY, v STRING)`)

	// Using actor().table when actor_scope is set should error.
	sqlDB.Exec(t, `SET actor_scope = 'alpha'`)
	sqlDB.ExpectErr(t,
		"cannot use actor\\(\\) table qualifier when actor_scope is set",
		`SELECT * FROM actor('beta').t`,
	)
	sqlDB.Exec(t, `SET actor_scope = ''`)
}

func lookupTableID(t *testing.T, db *sql.DB, tableName string) int {
	t.Helper()
	var tableID int
	if err := db.QueryRow(
		`SELECT table_id FROM crdb_internal.tables WHERE name = $1 AND database_name = current_database()`,
		tableName,
	).Scan(&tableID); err != nil {
		t.Fatalf("lookup table id for %s: %+v", tableName, err)
	}
	return tableID
}

func tableIDForName(t *testing.T, db *sql.DB, tableName string) int {
	t.Helper()
	return lookupTableID(t, db, tableName)
}

func assertActorIndexHasKVs(t *testing.T, kvDB *kv.DB, tableID int, indexID int, actorName string) {
	t.Helper()
	codec := keys.MakeActorSQLCodec(keys.SystemSQLCodec, actorName)
	prefix := roachpb.Key(codec.IndexPrefix(uint32(tableID), uint32(indexID)))
	kvs, err := kvDB.Scan(context.Background(), prefix, prefix.PrefixEnd(), 1)
	if err != nil {
		t.Fatalf("scan actor index prefix %s: %+v", prefix, err)
	}
	if len(kvs) == 0 {
		t.Fatalf("expected actor index prefix %s to contain KVs", prefix)
	}
}

func moveActorIndexRangeToNode(
	t *testing.T,
	tc *testcluster.TestCluster,
	tableID int,
	indexID int,
	actorName string,
	serverIdx int,
) {
	t.Helper()
	codec := keys.MakeActorSQLCodec(keys.SystemSQLCodec, actorName)
	rangeKey := roachpb.Key(codec.TenantPrefix())
	rangeDesc := tc.LookupRangeOrFatal(t, rangeKey)
	target := tc.Target(serverIdx)
	if _, ok := rangeDesc.GetReplicaDescriptor(target.StoreID); !ok {
		var err error
		rangeDesc, err = tc.AddVoters(rangeKey, target)
		if err != nil {
			t.Fatalf("add voter for %s: %+v", rangeKey, err)
		}
	}
	if err := tc.TransferRangeLease(rangeDesc, target); err != nil {
		t.Fatalf("transfer lease for %s: %+v", rangeKey, err)
	}
}
