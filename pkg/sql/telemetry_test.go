// Copyright 2020 The Cockroach Authors.
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
	gosql "database/sql"
	"net/url"
	"testing"

	"github.com/semistrict/ratel/pkg/base"
	"github.com/semistrict/ratel/pkg/sql/catalog/catconstants"
	"github.com/semistrict/ratel/pkg/sql/sqltestutils"
	"github.com/semistrict/ratel/pkg/testutils/serverutils"
	"github.com/semistrict/ratel/pkg/testutils/skip"
	"github.com/semistrict/ratel/pkg/testutils/sqlutils"
	"github.com/semistrict/ratel/pkg/util/leaktest"
	"github.com/semistrict/ratel/pkg/util/log"
	"github.com/stretchr/testify/require"
)

func TestTelemetry(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	skip.UnderRace(t, "takes >1min under race")

	sqltestutils.TelemetryTest(
		t,
		[]base.TestServerArgs{{}},
	)
}

func TestTelemetryRecordCockroachShell(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	cluster := serverutils.StartNewTestCluster(
		t,
		1,
		base.TestClusterArgs{},
	)
	defer cluster.Stopper().Stop(context.Background())

	pgUrl, cleanupFn := sqlutils.PGUrl(
		t,
		cluster.Server(0).ServingSQLAddr(),
		"TestTelemetryRecordCockroachShell",
		url.User("root"),
	)
	defer cleanupFn()
	q := pgUrl.Query()

	q.Add("application_name", catconstants.ReportableAppNamePrefix+catconstants.InternalSQLAppName)
	pgUrl.RawQuery = q.Encode()

	db, err := gosql.Open("postgres", pgUrl.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var appName string
	err = db.QueryRow("SHOW application_name").Scan(&appName)
	require.NoError(t, err)
	require.Equal(t, catconstants.ReportableAppNamePrefix+catconstants.InternalSQLAppName, appName)

	var counter int
	err = db.QueryRow(
		"SELECT usage_count FROM crdb_internal.feature_usage WHERE feature_name = 'sql.connection.cockroach_cli'",
	).Scan(&counter)
	require.NoError(t, err)
	require.Equal(t, 1, counter)

}
