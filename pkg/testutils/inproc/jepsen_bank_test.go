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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/testutils/inproc"
	"github.com/stretchr/testify/require"
)

const (
	jepsenBankAccounts     = 5
	jepsenBankTotal        = int64(50)
	jepsenWorkloadDuration = 5 * time.Second
)

type jepsenBankTransfer struct {
	From   int64
	To     int64
	Amount int64
}

type jepsenBankOp struct {
	Proc     int
	Type     string
	Function string
	Balances []int64
	Transfer jepsenBankTransfer
	Err      string
	At       time.Time
}

type jepsenBankBadRead struct {
	Kind     string
	Expected int64
	Found    int64
	Balances []int64
}

type jepsenBankNemesisFunc func(
	ctx context.Context,
	stopCh <-chan struct{},
	c *inproc.Cluster,
	db *gosql.DB,
	history *jepsenBankHistory,
	keys *jepsenKeyTracker,
)

type jepsenBankHistory struct {
	mu  sync.Mutex
	ops []jepsenBankOp
}

func composeJepsenBankNemeses(nemeses ...jepsenBankNemesisFunc) jepsenBankNemesisFunc {
	return func(
		ctx context.Context,
		stopCh <-chan struct{},
		c *inproc.Cluster,
		db *gosql.DB,
		history *jepsenBankHistory,
		keys *jepsenKeyTracker,
	) {
		var wg sync.WaitGroup
		for _, nemesis := range nemeses {
			wg.Add(1)
			go func(n jepsenBankNemesisFunc) {
				defer wg.Done()
				n(ctx, stopCh, c, db, history, keys)
			}(nemesis)
		}
		wg.Wait()
	}
}

func (h *jepsenBankHistory) Add(op jepsenBankOp) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ops = append(h.ops, op)
}

func (h *jepsenBankHistory) Snapshot() []jepsenBankOp {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]jepsenBankOp, len(h.ops))
	copy(out, h.ops)
	return out
}

type jepsenKeyTracker struct {
	mu   sync.Mutex
	keys map[int64]struct{}
}

func newJepsenKeyTracker() *jepsenKeyTracker {
	return &jepsenKeyTracker{keys: make(map[int64]struct{})}
}

func (t *jepsenKeyTracker) Add(keys ...int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, key := range keys {
		t.keys[key] = struct{}{}
	}
}

func (t *jepsenKeyTracker) Random(rng *rand.Rand) (int64, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return randomTrackedKey(t.keys, rng)
}

func (t *jepsenKeyTracker) RandomExcluding(excluded map[int64]struct{}, rng *rand.Rand) (int64, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	filtered := make(map[int64]struct{}, len(t.keys))
	for key := range t.keys {
		if _, skip := excluded[key]; !skip {
			filtered[key] = struct{}{}
		}
	}
	return randomTrackedKey(filtered, rng)
}

func randomTrackedKey(keys map[int64]struct{}, rng *rand.Rand) (int64, bool) {
	if len(keys) == 0 {
		return 0, false
	}
	i := rng.Intn(len(keys))
	for key := range keys {
		if i == 0 {
			return key, true
		}
		i--
	}
	return 0, false
}

func TestSyncJepsenBankSplit(t *testing.T) {
	runSyncTest(t, func(t *testing.T) {
		runJepsenBankWorkload(t, runJepsenBankSplitNemesis)
	})
}

func TestSyncJepsenBankRestart(t *testing.T) {
	runSyncTest(t, func(t *testing.T) {
		runJepsenBankWorkload(t, runJepsenBankRestartNemesis)
	})
}

func TestSyncJepsenBankPartition(t *testing.T) {
	runSyncTest(t, func(t *testing.T) {
		runJepsenBankWorkload(t, runJepsenBankPartitionNemesis)
	})
}

func TestSyncJepsenBankParts(t *testing.T) {
	runSyncTest(t, func(t *testing.T) {
		runJepsenBankWorkload(t, runJepsenBankPartsNemesis)
	})
}

func TestSyncJepsenBankMajorityRing(t *testing.T) {
	runSyncTest(t, func(t *testing.T) {
		runJepsenBankWorkload(t, runJepsenBankMajorityRingNemesis)
	})
}

func TestSyncJepsenBankPartsRestart(t *testing.T) {
	runSyncTest(t, func(t *testing.T) {
		runJepsenBankWorkload(t, composeJepsenBankNemeses(
			runJepsenBankPartsNemesis,
			runJepsenBankRestartNemesis,
		))
	})
}

func TestSyncJepsenBankMajorityRingRestart(t *testing.T) {
	runSyncTest(t, func(t *testing.T) {
		runJepsenBankWorkload(t, composeJepsenBankNemeses(
			runJepsenBankMajorityRingNemesis,
			runJepsenBankRestartNemesis,
		))
	})
}

func runJepsenBankWorkload(t *testing.T, nemesis jepsenBankNemesisFunc) {
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

	setupJepsenBankAccounts(t, ctx, workerDB)
	stopCh := make(chan struct{})
	history := &jepsenBankHistory{}
	keys := newJepsenKeyTracker()
	var wg sync.WaitGroup
	const workers = 2
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(proc int) {
			defer wg.Done()
			runJepsenBankWorker(ctx, stopCh, proc, workerDB, history, keys)
		}(i)
	}
	if nemesis != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			nemesis(ctx, stopCh, c, nemesisDB, history, keys)
		}()
	}
	time.Sleep(jepsenWorkloadDuration)
	close(stopCh)
	wg.Wait()

	finalBalances, err := readJepsenBankBalances(ctx, workerDB)
	require.NoError(t, err)
	history.Add(jepsenBankOp{
		Proc:     -1,
		Type:     "ok",
		Function: "read",
		Balances: finalBalances,
		At:       time.Now(),
	})

	badReads := checkJepsenBankReads(history.Snapshot(), jepsenBankAccounts, jepsenBankTotal)
	require.Empty(t, badReads, "bad Jepsen bank reads: %+v", badReads)
}

func configureJepsenBankSession(t *testing.T, ctx context.Context, db *gosql.DB) {
	t.Helper()

	_, err := db.ExecContext(ctx, `SET distsql = off`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `SET vectorize = off`)
	require.NoError(t, err)
}

func setupJepsenBankAccounts(t *testing.T, ctx context.Context, db *gosql.DB) {
	t.Helper()

	_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS accounts`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
CREATE TABLE accounts (
	id INT PRIMARY KEY,
	balance BIGINT NOT NULL
)`)
	require.NoError(t, err)

	for i := 0; i < jepsenBankAccounts; i++ {
		_, err = db.ExecContext(ctx, `INSERT INTO accounts (id, balance) VALUES ($1, $2)`, i, 10)
		require.NoError(t, err)
	}
}

func runJepsenBankWorker(
	ctx context.Context,
	stopCh <-chan struct{},
	proc int,
	db *gosql.DB,
	history *jepsenBankHistory,
	keys *jepsenKeyTracker,
) {
	rng := rand.New(rand.NewSource(int64(proc + 1)))

	for {
		select {
		case <-stopCh:
			return
		default:
		}

		if rng.Intn(2) == 0 {
			balances, err := readJepsenBankBalances(ctx, db)
			op := jepsenBankOp{
				Proc:     proc,
				Function: "read",
				At:       time.Now(),
			}
			if err != nil {
				op.Type = "info"
				op.Err = err.Error()
			} else {
				op.Type = "ok"
				op.Balances = balances
			}
			history.Add(op)
		} else {
			transfer := randomJepsenBankTransfer(rng)

			err := runJepsenBankTransfer(ctx, db, transfer)
			op := jepsenBankOp{
				Proc:     proc,
				Function: "transfer",
				Transfer: transfer,
				At:       time.Now(),
			}
			switch {
			case err == nil:
				op.Type = "ok"
				keys.Add(transfer.From, transfer.To)
			case strings.HasPrefix(err.Error(), "negative:"):
				op.Type = "fail"
				op.Err = err.Error()
			default:
				op.Type = "info"
				op.Err = err.Error()
			}
			history.Add(op)
		}

		time.Sleep(25 * time.Millisecond)
	}
}

func randomJepsenBankTransfer(rng *rand.Rand) jepsenBankTransfer {
	from := rng.Intn(jepsenBankAccounts)
	to := rng.Intn(jepsenBankAccounts - 1)
	if to >= from {
		to++
	}
	return jepsenBankTransfer{
		From:   int64(from),
		To:     int64(to),
		Amount: int64(rng.Intn(5) + 1),
	}
}

func runJepsenBankSplitNemesis(
	ctx context.Context,
	stopCh <-chan struct{},
	_ *inproc.Cluster,
	db *gosql.DB,
	history *jepsenBankHistory,
	keys *jepsenKeyTracker,
) {
	rng := rand.New(rand.NewSource(99))
	alreadySplit := make(map[int64]struct{})
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			op := jepsenBankOp{
				Proc:     -1,
				Type:     "info",
				Function: "split",
				At:       time.Now(),
			}
			key, ok := keys.RandomExcluding(alreadySplit, rng)
			if !ok {
				op.Err = "nothing-to-split"
				history.Add(op)
				continue
			}
			_, err := db.ExecContext(ctx, `ALTER TABLE accounts SPLIT AT VALUES ($1)`, key)
			if err != nil {
				op.Err = err.Error()
			} else {
				alreadySplit[key] = struct{}{}
				op.Err = fmt.Sprintf("split:%d", key)
			}
			history.Add(op)
		}
	}
}

func runJepsenBankRestartNemesis(
	ctx context.Context,
	stopCh <-chan struct{},
	c *inproc.Cluster,
	_ *gosql.DB,
	history *jepsenBankHistory,
	_ *jepsenKeyTracker,
) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			op := jepsenBankOp{
				Proc:     -1,
				Type:     "info",
				Function: "restart",
				At:       time.Now(),
			}

			c.StopNode(2)
			if !waitOrStop(stopCh, 250*time.Millisecond) {
				history.Add(op)
				return
			}
			if err := c.RestartNodeE(2); err != nil {
				op.Err = err.Error()
			} else {
				op.Err = "restarted:2"
			}
			history.Add(op)
		}
	}
}

func runJepsenBankPartitionNemesis(
	ctx context.Context,
	stopCh <-chan struct{},
	c *inproc.Cluster,
	_ *gosql.DB,
	history *jepsenBankHistory,
	_ *jepsenKeyTracker,
) {
	runTimedLinkPartitionNemesis(stopCh, c, func() {
		history.Add(jepsenBankOp{
			Proc:     -1,
			Type:     "info",
			Function: "partition",
			Err:      "isolate-node-2",
			At:       time.Now(),
		})
	}, func() {
		c.PartitionNodeGroups([][]int{{0, 1}, {2}})
	})
}

func runJepsenBankPartsNemesis(
	ctx context.Context,
	stopCh <-chan struct{},
	c *inproc.Cluster,
	_ *gosql.DB,
	history *jepsenBankHistory,
	_ *jepsenKeyTracker,
) {
	rng := rand.New(rand.NewSource(1234))
	runTimedLinkPartitionNemesis(stopCh, c, func() {
		history.Add(jepsenBankOp{
			Proc:     -1,
			Type:     "info",
			Function: "partition",
			Err:      "parts",
			At:       time.Now(),
		})
	}, func() {
		c.PartitionRandomHalves([]int{0, 1, 2}, rng)
	})
}

func runJepsenBankMajorityRingNemesis(
	ctx context.Context,
	stopCh <-chan struct{},
	c *inproc.Cluster,
	_ *gosql.DB,
	history *jepsenBankHistory,
	_ *jepsenKeyTracker,
) {
	runTimedLinkPartitionNemesis(stopCh, c, func() {
		history.Add(jepsenBankOp{
			Proc:     -1,
			Type:     "info",
			Function: "partition",
			Err:      "majority-ring",
			At:       time.Now(),
		})
	}, func() {
		c.PartitionMajoritiesRing([]int{0, 1, 2})
	})
}

func readJepsenBankBalances(ctx context.Context, db *gosql.DB) ([]int64, error) {
	rows, err := db.QueryContext(ctx, `SELECT balance FROM accounts ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var balances []int64
	for rows.Next() {
		var balance int64
		if err := rows.Scan(&balance); err != nil {
			return nil, err
		}
		balances = append(balances, balance)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return balances, nil
}

func runJepsenBankTransfer(ctx context.Context, db *gosql.DB, transfer jepsenBankTransfer) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var fromBalance int64
	if err := tx.QueryRowContext(
		ctx, `SELECT balance FROM accounts WHERE id = $1`, transfer.From,
	).Scan(&fromBalance); err != nil {
		return err
	}
	var toBalance int64
	if err := tx.QueryRowContext(
		ctx, `SELECT balance FROM accounts WHERE id = $1`, transfer.To,
	).Scan(&toBalance); err != nil {
		return err
	}

	fromBalance -= transfer.Amount
	toBalance += transfer.Amount
	if fromBalance < 0 {
		return fmt.Errorf("negative:%d:%d", transfer.From, fromBalance)
	}
	if toBalance < 0 {
		return fmt.Errorf("negative:%d:%d", transfer.To, toBalance)
	}

	if _, err := tx.ExecContext(
		ctx, `UPDATE accounts SET balance = $1 WHERE id = $2`, fromBalance, transfer.From,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(
		ctx, `UPDATE accounts SET balance = $1 WHERE id = $2`, toBalance, transfer.To,
	); err != nil {
		return err
	}

	return tx.Commit()
}

func checkJepsenBankReads(history []jepsenBankOp, accounts int, total int64) []jepsenBankBadRead {
	var badReads []jepsenBankBadRead
	for _, op := range history {
		if op.Type != "ok" || op.Function != "read" {
			continue
		}
		if len(op.Balances) != accounts {
			badReads = append(badReads, jepsenBankBadRead{
				Kind:     "wrong-n",
				Expected: int64(accounts),
				Found:    int64(len(op.Balances)),
				Balances: op.Balances,
			})
			continue
		}

		var sum int64
		negative := false
		for _, balance := range op.Balances {
			sum += balance
			if balance < 0 {
				negative = true
			}
		}

		if sum != total {
			badReads = append(badReads, jepsenBankBadRead{
				Kind:     "wrong-total",
				Expected: total,
				Found:    sum,
				Balances: op.Balances,
			})
			continue
		}

		if negative {
			badReads = append(badReads, jepsenBankBadRead{
				Kind:     "negative-value",
				Balances: op.Balances,
			})
		}
	}
	return badReads
}

func TestJepsenBankChecker(t *testing.T) {
	tests := []struct {
		name      string
		history   []jepsenBankOp
		wantKinds []string
	}{
		{
			name: "valid read",
			history: []jepsenBankOp{{
				Type:     "ok",
				Function: "read",
				Balances: []int64{10, 10, 10, 10, 10},
			}},
		},
		{
			name: "wrong account count",
			history: []jepsenBankOp{{
				Type:     "ok",
				Function: "read",
				Balances: []int64{10, 10, 10},
			}},
			wantKinds: []string{"wrong-n"},
		},
		{
			name: "wrong total",
			history: []jepsenBankOp{{
				Type:     "ok",
				Function: "read",
				Balances: []int64{10, 10, 10, 10, 9},
			}},
			wantKinds: []string{"wrong-total"},
		},
		{
			name: "negative balance",
			history: []jepsenBankOp{{
				Type:     "ok",
				Function: "read",
				Balances: []int64{10, 10, 10, 25, -5},
			}},
			wantKinds: []string{"negative-value"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			badReads := checkJepsenBankReads(tc.history, jepsenBankAccounts, jepsenBankTotal)
			require.Len(t, badReads, len(tc.wantKinds))
			for i, kind := range tc.wantKinds {
				require.Equal(t, kind, badReads[i].Kind)
			}
		})
	}
}
