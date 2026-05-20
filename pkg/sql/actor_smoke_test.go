package sql_test

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/upgrade/upgradebase"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
)

func TestActorSmokeScopeSimpleIsolation(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	ctx := context.Background()
	s, db, _ := serverutils.StartServer(t, actorSmokeServerArgs())
	defer s.Stopper().Stop(ctx)

	sqlDB := sqlutils.MakeSQLRunner(db)
	sqlDB.Exec(t, `CREATE TABLE t (id INT PRIMARY KEY, v STRING)`)
	sqlDB.Exec(t, `SET actor_scope = 'alpha'`)
	sqlDB.Exec(t, `INSERT INTO t VALUES (1, 'alpha')`)
	sqlDB.Exec(t, `SET actor_scope = 'beta'`)
	sqlDB.Exec(t, `INSERT INTO t VALUES (1, 'beta')`)
	sqlDB.CheckQueryResults(t, `SELECT v FROM t WHERE id = 1`, [][]string{{"beta"}})
	sqlDB.Exec(t, `RESET actor_scope`)
	sqlDB.CheckQueryResults(t, `SELECT count(*) FROM t`, [][]string{{"0"}})
	sqlDB.Exec(t, `SET actor_scope = 'alpha'`)
	sqlDB.CheckQueryResults(t, `SELECT v FROM t WHERE id = 1`, [][]string{{"alpha"}})
}

func TestActorSmokeTableQualifierSimpleQueries(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	ctx := context.Background()
	s, db, _ := serverutils.StartServer(t, actorSmokeServerArgs())
	defer s.Stopper().Stop(ctx)

	sqlDB := sqlutils.MakeSQLRunner(db)
	sqlDB.Exec(t, `CREATE TABLE t (id INT PRIMARY KEY, v STRING)`)
	sqlDB.Exec(t, `INSERT INTO actor('alpha').t VALUES (1, 'alpha')`)
	sqlDB.Exec(t, `INSERT INTO actor('beta').t VALUES (1, 'beta')`)
	sqlDB.CheckQueryResults(t, `SELECT v FROM actor('alpha').t WHERE id = 1`, [][]string{{"alpha"}})
	sqlDB.CheckQueryResults(t, `SELECT v FROM actor('beta').t WHERE id = 1`, [][]string{{"beta"}})
}

func TestActorSmokeRegistryCreatedOnFirstWrite(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	ctx := context.Background()
	s, db, _ := serverutils.StartServer(t, actorSmokeServerArgs())
	defer s.Stopper().Stop(ctx)

	sqlDB := sqlutils.MakeSQLRunner(db)
	sqlDB.Exec(t, `CREATE TABLE t (id INT PRIMARY KEY)`)
	sqlDB.Exec(t, `SET actor_scope = 'alpha'`)
	sqlDB.Exec(t, `INSERT INTO t VALUES (1)`)

	var actorName string
	var actorHash []byte
	if err := db.QueryRow(`SELECT actor_name, actor_hash FROM system.actors WHERE actor_name = 'alpha'`).Scan(&actorName, &actorHash); err != nil {
		t.Fatal(err)
	}
	if actorName != "alpha" {
		t.Fatalf("expected actor alpha, got %q", actorName)
	}
	expected := keys.ActorHash("alpha")
	if got, want := hex.EncodeToString(actorHash), hex.EncodeToString(expected[:]); got != want {
		t.Fatalf("expected actor hash %s, got %s", want, got)
	}

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM system.actors WHERE actor_name = 'alpha'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one actor row, got %d", count)
	}
}

func actorSmokeServerArgs() base.TestServerArgs {
	return base.TestServerArgs{
		Knobs: base.TestingKnobs{
			UpgradeManager: &upgradebase.TestingKnobs{
				DontUseJobs: true,
				ListBetweenOverride: func(_, _ roachpb.Version) []roachpb.Version {
					return nil
				},
			},
		},
	}
}
