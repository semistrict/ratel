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

package inproc_test

import (
	"context"
	gosql "database/sql"
	"fmt"
	"math/rand"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/semistrict/ratel/pkg/testutils/inproc"
	"github.com/stretchr/testify/require"
)

const jepsenCommentsTables = 4

type jepsenCommentsOp struct {
	Proc     int
	Type     string
	Function string
	ID       int64
	Seen     []int64
	Err      string
	At       time.Time
}

type jepsenCommentsBadRead struct {
	Seen    []int64
	Missing []int64
}

type jepsenCommentsHistory struct {
	mu  sync.Mutex
	ops []jepsenCommentsOp
}

func (h *jepsenCommentsHistory) Add(op jepsenCommentsOp) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ops = append(h.ops, op)
}

func (h *jepsenCommentsHistory) Snapshot() []jepsenCommentsOp {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]jepsenCommentsOp, len(h.ops))
	copy(out, h.ops)
	return out
}

func TestSyncJepsenCommentsSplit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		runJepsenCommentsWorkload(t, runJepsenCommentsSplitNemesis)
	})
}

func TestSyncJepsenCommentsRestart(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		runJepsenCommentsWorkload(t, runJepsenCommentsRestartNemesis)
	})
}

func runJepsenCommentsWorkload(
	t *testing.T, nemesis func(context.Context, <-chan struct{}, *inproc.Cluster, *gosql.DB, *jepsenKeyTracker),
) {
	t.Helper()

	c := startSyncCluster(t, 3)
	defer stopSyncCluster(c)

	ctx := t.Context()
	workerDB := c.ServerConn(0)
	workerDB.SetMaxOpenConns(1)
	workerDB.SetMaxIdleConns(1)
	configureJepsenBankSession(t, ctx, workerDB)

	nemesisDB := c.ServerConn(0)
	nemesisDB.SetMaxOpenConns(1)
	nemesisDB.SetMaxIdleConns(1)
	configureJepsenBankSession(t, ctx, nemesisDB)

	setupJepsenCommentsTables(t, ctx, workerDB)
	stopCh := make(chan struct{})
	history := &jepsenCommentsHistory{}
	keys := newJepsenKeyTracker()
	var wg sync.WaitGroup
	const workers = 2
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(proc int) {
			defer wg.Done()
			runJepsenCommentsWorker(ctx, stopCh, proc, workerDB, history, keys)
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

	finalRead, err := readJepsenComments(ctx, workerDB)
	require.NoError(t, err)
	history.Add(jepsenCommentsOp{
		Proc:     -1,
		Type:     "ok",
		Function: "read",
		Seen:     finalRead,
		At:       time.Now(),
	})

	badReads := checkJepsenComments(history.Snapshot())
	require.Empty(t, badReads, "bad Jepsen comments reads: %+v", badReads)
}

func setupJepsenCommentsTables(t *testing.T, ctx context.Context, db *gosql.DB) {
	t.Helper()

	for _, table := range jepsenCommentsTableNames() {
		_, err := db.ExecContext(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, table))
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE %s (id INT PRIMARY KEY, key INT NOT NULL)`, table))
		require.NoError(t, err)
	}
}

func runJepsenCommentsWorker(
	ctx context.Context,
	stopCh <-chan struct{},
	proc int,
	db *gosql.DB,
	history *jepsenCommentsHistory,
	keys *jepsenKeyTracker,
) {
	rng := rand.New(rand.NewSource(int64(proc + 401)))
	var next int64 = int64(proc * 100000)

	for {
		select {
		case <-stopCh:
			return
		default:
		}

		if rng.Intn(2) == 0 {
			id := next
			next++
			history.Add(jepsenCommentsOp{
				Proc:     proc,
				Type:     "invoke",
				Function: "write",
				ID:       id,
				At:       time.Now(),
			})
			err := writeJepsenComments(ctx, db, id)
			op := jepsenCommentsOp{
				Proc:     proc,
				Type:     "ok",
				Function: "write",
				ID:       id,
				At:       time.Now(),
			}
			if err != nil {
				op.Type = "info"
				op.Err = err.Error()
			} else {
				keys.Add(id)
			}
			history.Add(op)
		} else {
			history.Add(jepsenCommentsOp{
				Proc:     proc,
				Type:     "invoke",
				Function: "read",
				At:       time.Now(),
			})
			seen, err := readJepsenComments(ctx, db)
			op := jepsenCommentsOp{
				Proc:     proc,
				Type:     "ok",
				Function: "read",
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

func writeJepsenComments(ctx context.Context, db *gosql.DB, id int64) error {
	table := jepsenCommentsTableFor(id)
	_, err := db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s (id, key) VALUES ($1, 0)`, table), id)
	return err
}

func readJepsenComments(ctx context.Context, db *gosql.DB) ([]int64, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var seen []int64
	for _, table := range jepsenCommentsTableNames() {
		rows, err := tx.QueryContext(ctx, fmt.Sprintf(`SELECT id FROM %s WHERE key = 0`, table))
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return nil, err
			}
			seen = append(seen, id)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}

	slices.Sort(seen)
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return seen, nil
}

func runJepsenCommentsSplitNemesis(
	ctx context.Context, stopCh <-chan struct{}, _ *inproc.Cluster, db *gosql.DB, keys *jepsenKeyTracker,
) {
	rng := rand.New(rand.NewSource(911))
	alreadySplit := make(map[int64]struct{})
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
			table := jepsenCommentsTableFor(key)
			if _, err := db.ExecContext(
				ctx,
				fmt.Sprintf(`ALTER TABLE %s SPLIT AT VALUES ($1)`, table),
				key,
			); err == nil {
				alreadySplit[key] = struct{}{}
			}
		}
	}
}

func runJepsenCommentsRestartNemesis(
	ctx context.Context, stopCh <-chan struct{}, c *inproc.Cluster, _ *gosql.DB, _ *jepsenKeyTracker,
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

func jepsenCommentsTableNames() []string {
	tables := make([]string, 0, jepsenCommentsTables)
	for i := 0; i < jepsenCommentsTables; i++ {
		tables = append(tables, fmt.Sprintf("comment_%d", i))
	}
	return tables
}

func jepsenCommentsTableFor(id int64) string {
	return fmt.Sprintf("comment_%d", int(id%jepsenCommentsTables))
}

func checkJepsenComments(history []jepsenCommentsOp) []jepsenCommentsBadRead {
	completed := make(map[int64]struct{})
	expected := make(map[int64]map[int64]struct{})
	var bad []jepsenCommentsBadRead

	for _, op := range history {
		switch {
		case op.Function == "write" && op.Type == "invoke":
			prior := make(map[int64]struct{}, len(completed))
			for id := range completed {
				prior[id] = struct{}{}
			}
			expected[op.ID] = prior
		case op.Function == "write" && op.Type == "ok":
			completed[op.ID] = struct{}{}
		case op.Function == "read" && op.Type == "ok":
			seenSet := make(map[int64]struct{}, len(op.Seen))
			for _, id := range op.Seen {
				seenSet[id] = struct{}{}
			}

			expectedSeen := make(map[int64]struct{})
			for _, id := range op.Seen {
				for prior := range expected[id] {
					expectedSeen[prior] = struct{}{}
				}
			}

			var missing []int64
			for id := range expectedSeen {
				if _, ok := seenSet[id]; !ok {
					missing = append(missing, id)
				}
			}
			if len(missing) > 0 {
				slices.Sort(missing)
				bad = append(bad, jepsenCommentsBadRead{
					Seen:    slices.Clone(op.Seen),
					Missing: missing,
				})
			}
		}
	}

	return bad
}

func TestJepsenCommentsChecker(t *testing.T) {
	tests := []struct {
		name string
		ops  []jepsenCommentsOp
		bad  int
	}{
		{
			name: "read sees all predecessors",
			ops: []jepsenCommentsOp{
				{Type: "invoke", Function: "write", ID: 1},
				{Type: "ok", Function: "write", ID: 1},
				{Type: "invoke", Function: "write", ID: 2},
				{Type: "ok", Function: "write", ID: 2},
				{Type: "ok", Function: "read", Seen: []int64{1, 2}},
			},
			bad: 0,
		},
		{
			name: "read misses predecessor",
			ops: []jepsenCommentsOp{
				{Type: "invoke", Function: "write", ID: 1},
				{Type: "ok", Function: "write", ID: 1},
				{Type: "invoke", Function: "write", ID: 2},
				{Type: "ok", Function: "write", ID: 2},
				{Type: "ok", Function: "read", Seen: []int64{2}},
			},
			bad: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bad := checkJepsenComments(tc.ops)
			require.Len(t, bad, tc.bad)
		})
	}
}
