// Copyright 2026 The Cockroach Authors
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

package inproc_test

import (
	"context"
	gosql "database/sql"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cockroachdb/cockroach/pkg/testutils/inproc"
	"github.com/stretchr/testify/require"
)

const (
	jepsenSequentialTables  = 4
	jepsenSequentialSubkeys = 4
)

type jepsenSequentialOp struct {
	Proc     int
	Type     string
	Function string
	Value    int64
	Seen     []string
	Err      string
	At       time.Time
}

type jepsenSequentialBadRead struct {
	Value int64
	Seen  []string
}

type jepsenSequentialHistory struct {
	mu  sync.Mutex
	ops []jepsenSequentialOp
}

func (h *jepsenSequentialHistory) Add(op jepsenSequentialOp) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ops = append(h.ops, op)
}

func (h *jepsenSequentialHistory) Snapshot() []jepsenSequentialOp {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]jepsenSequentialOp, len(h.ops))
	copy(out, h.ops)
	return out
}

type jepsenSequentialTrackedKey struct {
	Table string
	Key   string
}

type jepsenSequentialKeyTracker struct {
	mu   sync.Mutex
	keys map[jepsenSequentialTrackedKey]struct{}
}

func composeJepsenSequentialNemeses(
	nemeses ...func(context.Context, <-chan struct{}, *inproc.Cluster, *gosql.DB, *jepsenSequentialKeyTracker),
) func(context.Context, <-chan struct{}, *inproc.Cluster, *gosql.DB, *jepsenSequentialKeyTracker) {
	return func(
		ctx context.Context,
		stopCh <-chan struct{},
		c *inproc.Cluster,
		db *gosql.DB,
		keys *jepsenSequentialKeyTracker,
	) {
		var wg sync.WaitGroup
		for _, nemesis := range nemeses {
			wg.Add(1)
			go func(n func(context.Context, <-chan struct{}, *inproc.Cluster, *gosql.DB, *jepsenSequentialKeyTracker)) {
				defer wg.Done()
				n(ctx, stopCh, c, db, keys)
			}(nemesis)
		}
		wg.Wait()
	}
}

func newJepsenSequentialKeyTracker() *jepsenSequentialKeyTracker {
	return &jepsenSequentialKeyTracker{keys: make(map[jepsenSequentialTrackedKey]struct{})}
}

func (t *jepsenSequentialKeyTracker) Add(key jepsenSequentialTrackedKey) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.keys[key] = struct{}{}
}

func (t *jepsenSequentialKeyTracker) RandomExcluding(
	excluded map[jepsenSequentialTrackedKey]struct{}, rng *rand.Rand,
) (jepsenSequentialTrackedKey, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	filtered := make([]jepsenSequentialTrackedKey, 0, len(t.keys))
	for key := range t.keys {
		if _, skip := excluded[key]; skip {
			continue
		}
		filtered = append(filtered, key)
	}
	if len(filtered) == 0 {
		return jepsenSequentialTrackedKey{}, false
	}
	return filtered[rng.Intn(len(filtered))], true
}

func TestSyncJepsenSequentialSplit(t *testing.T) {
	runSyncTest(t, func(t *testing.T) {
		runJepsenSequentialWorkload(t, runJepsenSequentialSplitNemesis)
	})
}

func TestSyncJepsenSequentialRestart(t *testing.T) {
	runSyncTest(t, func(t *testing.T) {
		runJepsenSequentialWorkload(t, runJepsenSequentialRestartNemesis)
	})
}

func TestSyncJepsenSequentialParts(t *testing.T) {
	runSyncTest(t, func(t *testing.T) {
		runJepsenSequentialWorkload(t, runJepsenSequentialPartsNemesis)
	})
}

func TestSyncJepsenSequentialMajorityRing(t *testing.T) {
	runSyncTest(t, func(t *testing.T) {
		runJepsenSequentialWorkload(t, runJepsenSequentialMajorityRingNemesis)
	})
}

func TestSyncJepsenSequentialPartsRestart(t *testing.T) {
	runSyncTest(t, func(t *testing.T) {
		runJepsenSequentialWorkload(t, composeJepsenSequentialNemeses(
			runJepsenSequentialPartsNemesis,
			runJepsenSequentialRestartNemesis,
		))
	})
}

func TestSyncJepsenSequentialMajorityRingRestart(t *testing.T) {
	runSyncTest(t, func(t *testing.T) {
		runJepsenSequentialWorkload(t, composeJepsenSequentialNemeses(
			runJepsenSequentialMajorityRingNemesis,
			runJepsenSequentialRestartNemesis,
		))
	})
}

func runJepsenSequentialWorkload(
	t *testing.T,
	nemesis func(context.Context, <-chan struct{}, *inproc.Cluster, *gosql.DB, *jepsenSequentialKeyTracker),
) {
	t.Helper()

	c := startSyncCluster(t, 3)
	defer stopSyncCluster(c)

	ctx := t.Context()
	workerDB := c.ServerConn(0)
	defer func() { _ = workerDB.Close() }()
	workerDB.SetMaxOpenConns(1)
	workerDB.SetMaxIdleConns(1)
	configureJepsenBankSession(t, ctx, workerDB)

	nemesisDB := c.ServerConn(0)
	if nemesisDB != workerDB {
		defer func() { _ = nemesisDB.Close() }()
		nemesisDB.SetMaxOpenConns(1)
		nemesisDB.SetMaxIdleConns(1)
		configureJepsenBankSession(t, ctx, nemesisDB)
	}

	setupJepsenSequentialTables(t, ctx, workerDB)
	stopCh := make(chan struct{})
	history := &jepsenSequentialHistory{}
	keys := newJepsenSequentialKeyTracker()
	var wg sync.WaitGroup
	const workers = 2
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(proc int) {
			defer wg.Done()
			runJepsenSequentialWorker(ctx, stopCh, proc, workerDB, history, keys)
		}(i)
	}
	if nemesis != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			nemesis(ctx, stopCh, c, nemesisDB, keys)
		}()
	}
	time.Sleep(jepsenWorkloadDuration)
	close(stopCh)
	wg.Wait()
	time.Sleep(time.Second)
	synctest.Wait()

	badReads := checkJepsenSequential(history.Snapshot())
	require.Empty(t, badReads, "bad Jepsen sequential reads: %+v", badReads)
}

func setupJepsenSequentialTables(t *testing.T, ctx context.Context, db *gosql.DB) {
	t.Helper()

	for _, table := range jepsenSequentialTableNames() {
		_, err := db.ExecContext(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, table))
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE %s (key STRING PRIMARY KEY)`, table))
		require.NoError(t, err)
	}
}

func runJepsenSequentialWorker(
	ctx context.Context,
	stopCh <-chan struct{},
	proc int,
	db *gosql.DB,
	history *jepsenSequentialHistory,
	keys *jepsenSequentialKeyTracker,
) {
	rng := rand.New(rand.NewSource(int64(proc + 101)))
	var next int64 = int64(proc * 100000)

	for {
		select {
		case <-stopCh:
			return
		default:
		}

		value := next
		next++

		if rng.Intn(2) == 0 {
			history.Add(jepsenSequentialOp{
				Proc:     proc,
				Type:     "invoke",
				Function: "write",
				Value:    value,
				At:       time.Now(),
			})
			err := writeJepsenSequentialValue(ctx, db, value, keys)
			op := jepsenSequentialOp{
				Proc:     proc,
				Type:     "ok",
				Function: "write",
				Value:    value,
				At:       time.Now(),
			}
			if err != nil {
				op.Type = "info"
				op.Err = err.Error()
			}
			history.Add(op)
		} else {
			target := value
			if target > 0 {
				target = value - int64(rng.Intn(8))
				if target < 0 {
					target = 0
				}
			}
			history.Add(jepsenSequentialOp{
				Proc:     proc,
				Type:     "invoke",
				Function: "read",
				Value:    target,
				At:       time.Now(),
			})
			seen, err := readJepsenSequentialValue(ctx, db, target)
			op := jepsenSequentialOp{
				Proc:     proc,
				Type:     "ok",
				Function: "read",
				Value:    target,
				Seen:     seen,
				At:       time.Now(),
			}
			if err != nil {
				op.Type = "info"
				op.Err = err.Error()
				op.Seen = nil
			}
			history.Add(op)
		}

		time.Sleep(25 * time.Millisecond)
	}
}

func writeJepsenSequentialValue(
	ctx context.Context, db *gosql.DB, value int64, keys *jepsenSequentialKeyTracker,
) error {
	for _, subkey := range jepsenSequentialSubkeyNames(value) {
		table := jepsenSequentialTableFor(subkey)
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s (key) VALUES ($1)`, table), subkey); err != nil {
			return err
		}
		keys.Add(jepsenSequentialTrackedKey{Table: table, Key: subkey})
	}
	return nil
}

func readJepsenSequentialValue(ctx context.Context, db *gosql.DB, value int64) ([]string, error) {
	subkeys := jepsenSequentialSubkeyNames(value)
	seen := make([]string, 0, len(subkeys))
	for i := len(subkeys) - 1; i >= 0; i-- {
		subkey := subkeys[i]
		table := jepsenSequentialTableFor(subkey)
		var found string
		err := db.QueryRowContext(
			ctx,
			fmt.Sprintf(`SELECT key FROM %s WHERE key = $1`, table),
			subkey,
		).Scan(&found)
		switch err {
		case nil:
			seen = append(seen, found)
		case gosql.ErrNoRows:
			seen = append(seen, "")
		default:
			return nil, err
		}
	}
	return seen, nil
}

func runJepsenSequentialSplitNemesis(
	ctx context.Context,
	stopCh <-chan struct{},
	_ *inproc.Cluster,
	db *gosql.DB,
	keys *jepsenSequentialKeyTracker,
) {
	rng := rand.New(rand.NewSource(701))
	alreadySplit := make(map[jepsenSequentialTrackedKey]struct{})
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			key, ok := keys.RandomExcluding(alreadySplit, rng)
			if !ok {
				continue
			}
			if _, err := db.ExecContext(
				ctx,
				fmt.Sprintf(`ALTER TABLE %s SPLIT AT VALUES ($1)`, key.Table),
				key.Key,
			); err == nil {
				alreadySplit[key] = struct{}{}
			}
		}
	}
}

func runJepsenSequentialRestartNemesis(
	ctx context.Context,
	stopCh <-chan struct{},
	c *inproc.Cluster,
	_ *gosql.DB,
	_ *jepsenSequentialKeyTracker,
) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			c.StopNode(2)
			if !waitOrStop(stopCh, 250*time.Millisecond) {
				return
			}
			_ = c.RestartNodeE(2)
		}
	}
}

func runJepsenSequentialPartsNemesis(
	ctx context.Context,
	stopCh <-chan struct{},
	c *inproc.Cluster,
	_ *gosql.DB,
	_ *jepsenSequentialKeyTracker,
) {
	rng := rand.New(rand.NewSource(1701))
	runTimedLinkPartitionNemesis(stopCh, c, nil, func() {
		c.PartitionRandomHalves([]int{0, 1, 2}, rng)
	})
}

func runJepsenSequentialMajorityRingNemesis(
	ctx context.Context,
	stopCh <-chan struct{},
	c *inproc.Cluster,
	_ *gosql.DB,
	_ *jepsenSequentialKeyTracker,
) {
	runTimedLinkPartitionNemesis(stopCh, c, nil, func() {
		c.PartitionMajoritiesRing([]int{0, 1, 2})
	})
}

func jepsenSequentialTableNames() []string {
	tables := make([]string, 0, jepsenSequentialTables)
	for i := 0; i < jepsenSequentialTables; i++ {
		tables = append(tables, fmt.Sprintf("seq_%d", i))
	}
	return tables
}

func jepsenSequentialSubkeyNames(value int64) []string {
	subkeys := make([]string, 0, jepsenSequentialSubkeys)
	for i := 0; i < jepsenSequentialSubkeys; i++ {
		subkeys = append(subkeys, fmt.Sprintf("%d_%d", value, i))
	}
	return subkeys
}

func jepsenSequentialTableFor(subkey string) string {
	i := 0
	for idx := len(subkey) - 1; idx >= 0; idx-- {
		if subkey[idx] == '_' {
			n, err := strconv.Atoi(subkey[idx+1:])
			if err == nil {
				i = n
			}
			break
		}
	}
	return fmt.Sprintf("seq_%d", i%jepsenSequentialTables)
}

func checkJepsenSequential(history []jepsenSequentialOp) []jepsenSequentialBadRead {
	var bad []jepsenSequentialBadRead
	for _, op := range history {
		if op.Type != "ok" || op.Function != "read" {
			continue
		}
		sawPresent := false
		isBad := false
		for _, value := range op.Seen {
			if value == "" {
				if sawPresent {
					isBad = true
					break
				}
				continue
			}
			sawPresent = true
		}
		if isBad {
			bad = append(bad, jepsenSequentialBadRead{
				Value: op.Value,
				Seen:  op.Seen,
			})
		}
	}
	return bad
}

func TestJepsenSequentialChecker(t *testing.T) {
	tests := []struct {
		name string
		ops  []jepsenSequentialOp
		bad  int
	}{
		{
			name: "all missing",
			ops: []jepsenSequentialOp{{
				Type:     "ok",
				Function: "read",
				Value:    1,
				Seen:     []string{"", "", "", ""},
			}},
			bad: 0,
		},
		{
			name: "all present",
			ops: []jepsenSequentialOp{{
				Type:     "ok",
				Function: "read",
				Value:    2,
				Seen:     []string{"2_3", "2_2", "2_1", "2_0"},
			}},
			bad: 0,
		},
		{
			name: "prefix missing suffix present is valid",
			ops: []jepsenSequentialOp{{
				Type:     "ok",
				Function: "read",
				Value:    3,
				Seen:     []string{"", "", "3_1", "3_0"},
			}},
			bad: 0,
		},
		{
			name: "present then missing is invalid",
			ops: []jepsenSequentialOp{{
				Type:     "ok",
				Function: "read",
				Value:    4,
				Seen:     []string{"4_3", "", "4_1", "4_0"},
			}},
			bad: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bad := checkJepsenSequential(tc.ops)
			require.Len(t, bad, tc.bad)
		})
	}
}
