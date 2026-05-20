// Copyright 2022 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package kvstreamer_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/kv/kvclient/kvstreamer"
	"github.com/cockroachdb/cockroach/pkg/sql"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/randutil"
	"github.com/cockroachdb/cockroach/pkg/util/tracing/tracingpb"
	"github.com/dustin/go-humanize"
	"github.com/stretchr/testify/require"
)

// TestLargeKeys verifies that the Streamer successfully completes the queries
// when the keys to lookup (i.e. the enqueued requests themselves) as well as
// the looked up rows are large. It additionally ensures that the Streamer
// issues a reasonable number of the KV request.
//
// Additionally, this test verifies that the streamer is, in fact, used in all
// cases when we expect.
func TestLargeKeys(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	testCases := []struct {
		name  string
		query string
	}{
		{
			name:  "index join, no ordering",
			query: "SELECT * FROM foo@foo_attribute_idx WHERE attribute=1",
		},
		{
			name:  "index join, with ordering",
			query: "SELECT * FROM foo@foo_attribute_idx ORDER BY attribute",
		},
		{
			name:  "lookup join, no ordering",
			query: "SELECT * FROM bar INNER LOOKUP JOIN foo ON lookup_blob = pk_blob",
		},
		{
			name:  "lookup join, with ordering",
			query: "SELECT * FROM bar INNER LOOKUP JOIN foo ON lookup_blob = pk_blob ORDER BY pk_blob",
		},
	}

	rng, _ := randutil.NewTestRand()
	recCh := make(chan tracingpb.Recording, 1)
	// We want to capture the trace of the query so that we can count how many
	// KV requests the Streamer issued.
	s, db, _ := serverutils.StartServer(t, base.TestServerArgs{
		Knobs: base.TestingKnobs{
			SQLExecutor: &sql.ExecutorTestingKnobs{
				WithStatementTrace: func(trace tracingpb.Recording, stmt string) {
					for _, tc := range testCases {
						if tc.query == stmt {
							recCh <- trace
						}
					}
				},
			},
		},
	})
	ctx := context.Background()
	defer s.Stopper().Stop(ctx)

	// We will lower the distsql_workmem limit so that we can operate with
	// smaller blobs.
	//
	// Our target input batch buffer size is about 8.3KiB. This number is the
	// minimum we can assume and is derived as 100KiB / 12 which comes from the
	// join reader. That processor bumps the memory limit to be at least 100KiB
	// and uses 1/12 of the limit to buffer its input row batch.
	//
	// The vectorized index join, however, uses 1/8th of the limit for the input
	// batch buffer size, so we get to 8.3KiB x 8 which is about 67KiB.
	_, err := db.Exec("SET distsql_workmem='67KiB'")
	require.NoError(t, err)
	// To improve the test coverage, occasionally lower the maximum number of
	// concurrent requests.
	if rng.Float64() < 0.25 {
		_, err = db.Exec("SET CLUSTER SETTING kv.streamer.concurrency_limit = $1", rng.Intn(10)+1)
		require.NoError(t, err)
	}
	// Workmem limit will be set up in such a manner that the input row buffer
	// size is about 8.3KiB, so we have several interesting options for the blob
	// size:
	// - around 2000 is interesting because multiple requests are enqueued at
	// same time and there might be parallelism going on within the Streamer.
	// - 6000 is interesting because it doesn't exceed the buffer size, yet two
	// rows with such blobs do exceed it. The index joiners are expected to to
	// process each row on its own.
	// - 10000 is interesting because a single row already exceeds the buffer
	// size.
	for _, pkBlobSize := range []int{1000 + rng.Intn(2000), 6000, 10000} {
		for _, onlyLarge := range []bool{false, true} {
			_, err = db.Exec("DROP TABLE IF EXISTS foo")
			require.NoError(t, err)
			_, err = db.Exec("DROP TABLE IF EXISTS bar")
			require.NoError(t, err)
			_, err = db.Exec(`
				CREATE TABLE foo (
					pk_blob STRING PRIMARY KEY, attribute INT8, extra INT8, blob STRING,
					INDEX (attribute)
				);
			`)
			require.NoError(t, err)
			_, err = db.Exec("CREATE TABLE bar (lookup_blob STRING PRIMARY KEY)")
			require.NoError(t, err)

			numRows := rng.Intn(10) + 10
			for i := 0; i < numRows; i++ {
				letter := string(byte('a') + byte(i))
				valueSize := pkBlobSize
				if !onlyLarge && rng.Float64() < 0.5 {
					valueSize = rng.Intn(10) + 1
				}
				blobSize := int(float64(valueSize)*5.0*rng.Float64()) + 1
				_, err = db.Exec("INSERT INTO foo SELECT repeat($1, $2), 1, 1, repeat($1, $3);", letter, valueSize, blobSize)
				require.NoError(t, err)
				_, err = db.Exec("INSERT INTO bar SELECT repeat($1, $2);", letter, valueSize)
				require.NoError(t, err)
			}

			for _, newRangeProbability := range []float64{0, rng.Float64()} {
				for i := 1; i < numRows; i++ {
					if rng.Float64() < newRangeProbability {
						letter := string(byte('a') + byte(i))
						_, err = db.Exec("ALTER TABLE foo SPLIT AT VALUES ($1);", letter)
						require.NoError(t, err)
					}
				}
				_, err = db.Exec("SELECT count(*) FROM foo")
				require.NoError(t, err)

				for _, tc := range testCases {
					vectorizeModes := []string{"on", "off"}
					if strings.Contains(tc.name, "lookup") {
						vectorizeModes = []string{"on"}
					}
					for _, vectorizeMode := range vectorizeModes {
						_, err = db.Exec("SET vectorize = " + vectorizeMode)
						require.NoError(t, err)
						t.Run(fmt.Sprintf(
							"%s/size=%s/onlyLarge=%t/numRows=%d/newRangeProb=%.2f/vec=%s",
							tc.name, humanize.Bytes(uint64(pkBlobSize)),
							onlyLarge, numRows, newRangeProbability, vectorizeMode,
						), func(t *testing.T) {
							_, err = db.Exec(tc.query)
							if err != nil {
								<-recCh
								t.Fatal(err)
							}
							tr := <-recCh
							var numStreamerRequests int
							for _, rs := range tr {
								if rs.Operation == kvstreamer.AsyncRequestOp {
									numStreamerRequests++
								}
							}
							require.Greater(t, 4*numRows, numStreamerRequests)
							require.Greater(t, numStreamerRequests, 0)
						})
					}
				}
			}
		}
	}
}
