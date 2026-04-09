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
	"math/rand"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/semistrict/ratel/pkg/testutils/inproc"
	"github.com/stretchr/testify/require"
)

type jepsenSetsOp struct {
	Proc     int
	Type     string
	Function string
	Value    int64
	Values   []int64
	Err      string
	At       time.Time
}

type jepsenSetsBadRead struct {
	Duplicates []int64
	Lost       []int64
	Unexpected []int64
	Revived    []int64
}

type jepsenSetsHistory struct {
	mu  sync.Mutex
	ops []jepsenSetsOp
}

func (h *jepsenSetsHistory) Add(op jepsenSetsOp) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ops = append(h.ops, op)
}

func (h *jepsenSetsHistory) Snapshot() []jepsenSetsOp {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]jepsenSetsOp, len(h.ops))
	copy(out, h.ops)
	return out
}

func TestSyncJepsenSetsSplit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		runJepsenSetsWorkload(t, runJepsenSetsSplitNemesis)
	})
}

func TestSyncJepsenSetsRestart(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		runJepsenSetsWorkload(t, runJepsenSetsRestartNemesis)
	})
}

func runJepsenSetsWorkload(t *testing.T, nemesis jepsenBankNemesisFunc) {
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

	setupJepsenSetsTable(t, ctx, workerDB)
	stopCh := make(chan struct{})
	history := &jepsenSetsHistory{}
	keys := newJepsenKeyTracker()
	var wg sync.WaitGroup
	const workers = 2
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(proc int) {
			defer wg.Done()
			runJepsenSetsWorker(ctx, stopCh, proc, workerDB, history, keys)
		}(i)
	}
	if nemesis != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			nemesis(ctx, stopCh, c, nemesisDB, nil, keys)
		}()
	}
	time.Sleep(jepsenWorkloadDuration)
	close(stopCh)
	wg.Wait()

	finalValues, err := readJepsenSetsValues(ctx, workerDB)
	require.NoError(t, err)
	history.Add(jepsenSetsOp{
		Proc:     -1,
		Type:     "ok",
		Function: "read",
		Values:   finalValues,
		At:       time.Now(),
	})

	badRead, ok := checkJepsenSets(history.Snapshot())
	require.False(t, ok, "bad Jepsen sets read: %+v", badRead)
}

func setupJepsenSetsTable(t *testing.T, ctx context.Context, db *gosql.DB) {
	t.Helper()

	_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS jepsen_set`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `CREATE TABLE jepsen_set (id INT PRIMARY KEY)`)
	require.NoError(t, err)
}

func runJepsenSetsWorker(
	ctx context.Context,
	stopCh <-chan struct{},
	proc int,
	db *gosql.DB,
	history *jepsenSetsHistory,
	keys *jepsenKeyTracker,
) {
	var next int64 = int64(proc * 100000)

	for {
		select {
		case <-stopCh:
			return
		default:
		}

		value := next
		next++

		history.Add(jepsenSetsOp{
			Proc:     proc,
			Type:     "invoke",
			Function: "add",
			Value:    value,
			At:       time.Now(),
		})

		_, err := db.ExecContext(ctx, `INSERT INTO jepsen_set (id) VALUES ($1)`, value)
		op := jepsenSetsOp{
			Proc:     proc,
			Function: "add",
			Value:    value,
			At:       time.Now(),
		}
		if err != nil {
			op.Type = "info"
			op.Err = err.Error()
		} else {
			op.Type = "ok"
			keys.Add(value)
		}
		history.Add(op)

		time.Sleep(25 * time.Millisecond)
	}
}

func runJepsenSetsSplitNemesis(
	ctx context.Context,
	stopCh <-chan struct{},
	_ *inproc.Cluster,
	db *gosql.DB,
	_ *jepsenBankHistory,
	keys *jepsenKeyTracker,
) {
	rng := rand.New(rand.NewSource(199))
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
			if _, err := db.ExecContext(ctx, `ALTER TABLE jepsen_set SPLIT AT VALUES ($1)`, key); err == nil {
				alreadySplit[key] = struct{}{}
			}
		}
	}
}

func runJepsenSetsRestartNemesis(
	ctx context.Context,
	stopCh <-chan struct{},
	c *inproc.Cluster,
	_ *gosql.DB,
	_ *jepsenBankHistory,
	_ *jepsenKeyTracker,
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

func readJepsenSetsValues(ctx context.Context, db *gosql.DB) ([]int64, error) {
	rows, err := db.QueryContext(ctx, `SELECT id FROM jepsen_set ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var values []int64
	for rows.Next() {
		var value int64
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func checkJepsenSets(history []jepsenSetsOp) (jepsenSetsBadRead, bool) {
	var attempts []int64
	added := make(map[int64]struct{})
	failed := make(map[int64]struct{})
	uncertain := make(map[int64]struct{})
	var finalRead []int64

	for _, op := range history {
		if op.Function != "add" {
			if op.Type == "ok" && op.Function == "read" {
				finalRead = op.Values
			}
			continue
		}
		switch op.Type {
		case "invoke":
			attempts = append(attempts, op.Value)
		case "ok":
			added[op.Value] = struct{}{}
		case "fail":
			failed[op.Value] = struct{}{}
		case "info":
			uncertain[op.Value] = struct{}{}
		}
	}

	if finalRead == nil {
		return jepsenSetsBadRead{}, true
	}

	attempted := make(map[int64]struct{}, len(attempts))
	for _, value := range attempts {
		attempted[value] = struct{}{}
	}

	var duplicates []int64
	for value, freq := range frequencies(finalRead) {
		if freq > 1 {
			duplicates = append(duplicates, value)
		}
	}
	slices.Sort(duplicates)

	finalSet := make(map[int64]struct{}, len(finalRead))
	for _, value := range finalRead {
		finalSet[value] = struct{}{}
	}

	lost := setDifferenceMap(added, finalSet)
	unexpected := setDifferenceMap(finalSet, attempted)
	revived := setIntersectionMap(finalSet, failed)
	_ = setIntersectionMap(finalSet, uncertain)

	return jepsenSetsBadRead{
		Duplicates: duplicates,
		Lost:       lost,
		Unexpected: unexpected,
		Revived:    revived,
	}, len(duplicates) > 0 || len(lost) > 0 || len(unexpected) > 0 || len(revived) > 0
}

func frequencies(values []int64) map[int64]int {
	out := make(map[int64]int, len(values))
	for _, value := range values {
		out[value]++
	}
	return out
}

func setDifferenceMap(left, right map[int64]struct{}) []int64 {
	var out []int64
	for value := range left {
		if _, ok := right[value]; !ok {
			out = append(out, value)
		}
	}
	slices.Sort(out)
	return out
}

func setIntersectionMap(left, right map[int64]struct{}) []int64 {
	var out []int64
	for value := range left {
		if _, ok := right[value]; ok {
			out = append(out, value)
		}
	}
	slices.Sort(out)
	return out
}

func TestJepsenSetsChecker(t *testing.T) {
	tests := []struct {
		name string
		ops  []jepsenSetsOp
		want jepsenSetsBadRead
		bad  bool
	}{
		{
			name: "valid read",
			ops: []jepsenSetsOp{
				{Type: "invoke", Function: "add", Value: 1},
				{Type: "ok", Function: "add", Value: 1},
				{Type: "invoke", Function: "add", Value: 2},
				{Type: "ok", Function: "add", Value: 2},
				{Type: "ok", Function: "read", Values: []int64{1, 2}},
			},
		},
		{
			name: "duplicates",
			ops: []jepsenSetsOp{
				{Type: "invoke", Function: "add", Value: 1},
				{Type: "ok", Function: "add", Value: 1},
				{Type: "ok", Function: "read", Values: []int64{1, 1}},
			},
			want: jepsenSetsBadRead{Duplicates: []int64{1}},
			bad:  true,
		},
		{
			name: "lost value",
			ops: []jepsenSetsOp{
				{Type: "invoke", Function: "add", Value: 1},
				{Type: "ok", Function: "add", Value: 1},
				{Type: "ok", Function: "read", Values: []int64{}},
			},
			want: jepsenSetsBadRead{Lost: []int64{1}},
			bad:  true,
		},
		{
			name: "unexpected value",
			ops: []jepsenSetsOp{
				{Type: "invoke", Function: "add", Value: 1},
				{Type: "ok", Function: "read", Values: []int64{2}},
			},
			want: jepsenSetsBadRead{Unexpected: []int64{2}},
			bad:  true,
		},
		{
			name: "revived failed write",
			ops: []jepsenSetsOp{
				{Type: "invoke", Function: "add", Value: 1},
				{Type: "fail", Function: "add", Value: 1},
				{Type: "ok", Function: "read", Values: []int64{1}},
			},
			want: jepsenSetsBadRead{Revived: []int64{1}},
			bad:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, bad := checkJepsenSets(tc.ops)
			require.Equal(t, tc.bad, bad)
			require.Equal(t, tc.want, got)
		})
	}
}
