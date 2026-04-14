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

// This file tests Ratel's ephemeral node model: nodes are disposable
// and the cluster survives continuous churn. A chaos monkey kills
// random nodes, an autoscaler replaces them with fresh nodes, and a
// query runner hammers the cluster with bank transfers and actor
// operations the entire time. The invariant is that the bank total
// never changes and actor data is never lost.

import (
	"context"
	gosql "database/sql"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/semistrict/ratel/pkg/testutils/inproc"
	"github.com/stretchr/testify/require"
)

const (
	chaosMinNodes          = 3
	chaosInitialNodes      = 3
	chaosBankAccounts      = 5
	chaosBankInitBalance   = int64(10)
	chaosBankTotal         = chaosBankAccounts * chaosBankInitBalance
	chaosWorkloadDuration  = 90 * time.Second
	chaosKillInterval      = 30 * time.Second
	chaosAutoscaleInterval = 10 * time.Second
	chaosWorkers           = 4
)

// chaosStats tracks events for final reporting.
type chaosStats struct {
	kills       atomic.Int64
	spawns      atomic.Int64
	transfers   atomic.Int64
	reads       atomic.Int64
	actorWrites atomic.Int64
	actorReads  atomic.Int64
	errors      atomic.Int64
}

// TestSyncChaosShutdown is a minimal test for the shutdown mechanism used
// by the chaos test. It starts a goroutine doing repeated SQL queries on
// node 0, then verifies that partition + cancel cleanly unblocks it.
func TestSyncChaosShutdown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := startSyncCluster(t, 3)
		defer stopSyncCluster(c)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		db := c.ServerConn(0)
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)

		_, err := db.ExecContext(ctx, `SET distsql = off`)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `SET vectorize = off`)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `CREATE TABLE shutdown_test (id INT PRIMARY KEY, v INT)`)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `INSERT INTO shutdown_test VALUES (1, 0)`)
		require.NoError(t, err)

		stopCh := make(chan struct{})
		done := make(chan struct{})

		go func() {
			defer close(done)
			for {
				select {
				case <-stopCh:
					return
				default:
				}
				_, err := db.ExecContext(ctx, `UPDATE shutdown_test SET v = v + 1 WHERE id = 1`)
				if err != nil {
					return
				}
				time.Sleep(50 * time.Millisecond)
			}
		}()

		// Let the worker run for a bit.
		time.Sleep(time.Second)

		// Shutdown sequence: signal stop, cancel context, force-close pipes.
		close(stopCh)
		cancel()
		c.PartitionNode(0)

		// Worker should exit promptly.
		<-done

		// Heal so stopSyncCluster can shut down cleanly.
		c.HealPartition(0)
	})
}

// TestSyncChaosBankOnly tests bank transfers on a stable cluster with
// the partition shutdown mechanism. No chaos monkey or autoscaler.
func TestSyncChaosBankOnly(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := startSyncCluster(t, 3)
		defer stopSyncCluster(c)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		db := c.ServerConn(0)
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)

		_, err := db.ExecContext(ctx, `SET distsql = off`)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `SET vectorize = off`)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `CREATE TABLE accounts (id INT PRIMARY KEY, balance BIGINT NOT NULL)`)
		require.NoError(t, err)
		for i := 0; i < chaosBankAccounts; i++ {
			_, err = db.ExecContext(ctx, `INSERT INTO accounts (id, balance) VALUES ($1, $2)`, i, chaosBankInitBalance)
			require.NoError(t, err)
		}

		stopCh := make(chan struct{})
		var stats chaosStats
		var wg sync.WaitGroup

		for w := 0; w < 2; w++ {
			wg.Add(1)
			go func(proc int) {
				defer wg.Done()
				runChaosBankWorker(ctx, stopCh, proc, db, &stats)
			}(w)
		}

		time.Sleep(30 * time.Second)
		close(stopCh)
		cancel()
		c.PartitionNode(0)
		wg.Wait()
		c.HealPartition(0)

		// Verify bank invariant.
		verifyDB := c.ServerConn(0)
		defer func() { _ = verifyDB.Close() }()
		balances, err := readChaosBalances(t.Context(), verifyDB)
		require.NoError(t, err)
		require.Len(t, balances, chaosBankAccounts)
		var total int64
		for _, b := range balances {
			total += b
			require.GreaterOrEqual(t, b, int64(0))
		}
		require.Equal(t, chaosBankTotal, total, "bank total changed: balances=%v", balances)

		t.Logf("bank-only stats: transfers=%d reads=%d errors=%d",
			stats.transfers.Load(), stats.reads.Load(), stats.errors.Load())
	})
}

// TestSyncChaosKillAndAdd tests killing a node, adding a fresh replacement,
// and verifying bank transfers continue correctly.
func TestSyncChaosKillAndAdd(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := startSyncCluster(t, 3)
		defer stopSyncCluster(c)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		db := c.ServerConn(0)
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)

		_, err := db.ExecContext(ctx, `SET distsql = off`)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `SET vectorize = off`)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `CREATE TABLE accounts (id INT PRIMARY KEY, balance BIGINT NOT NULL)`)
		require.NoError(t, err)
		for i := 0; i < chaosBankAccounts; i++ {
			_, err = db.ExecContext(ctx, `INSERT INTO accounts (id, balance) VALUES ($1, $2)`, i, chaosBankInitBalance)
			require.NoError(t, err)
		}

		stopCh := make(chan struct{})
		var stats chaosStats
		var wg sync.WaitGroup

		// Bank workers on node 0.
		for w := 0; w < 2; w++ {
			wg.Add(1)
			go func(proc int) {
				defer wg.Done()
				runChaosBankWorker(ctx, stopCh, proc, db, &stats)
			}(w)
		}

		// Let workload establish.
		time.Sleep(5 * time.Second)

		// Kill node 2, add a replacement.
		c.StopNode(2)
		time.Sleep(2 * time.Second)
		c.AddNode(t)

		// Run more workload.
		time.Sleep(10 * time.Second)

		// Shutdown.
		close(stopCh)
		cancel()
		c.PartitionNode(0)
		wg.Wait()
		c.HealPartition(0)

		verifyDB := c.ServerConn(0)
		defer func() { _ = verifyDB.Close() }()
		balances, err := readChaosBalances(t.Context(), verifyDB)
		require.NoError(t, err)
		var total int64
		for _, b := range balances {
			total += b
		}
		require.Equal(t, chaosBankTotal, total, "bank total changed: balances=%v", balances)

		t.Logf("kill-and-add stats: transfers=%d reads=%d errors=%d",
			stats.transfers.Load(), stats.reads.Load(), stats.errors.Load())
	})
}

// TestSyncRatelChaos is the main chaos test for the ephemeral node model.
// It verifies that a Ratel cluster survives continuous node churn while
// maintaining bank transfer invariants and actor data consistency.
func TestSyncRatelChaos(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		runRatelChaos(t, true)
	})
}

// TestRatelChaos is the non-synctest variant for debugging.
func TestRatelChaos(t *testing.T) {
	runRatelChaos(t, false)
}

func runRatelChaos(t *testing.T, useSynctest bool) {
	t.Helper()

	c := startSyncCluster(t, chaosInitialNodes)
	if useSynctest {
		defer stopSyncCluster(c)
	} else {
		defer c.Stop()
	}

	// Use a cancellable context so in-flight queries abort on shutdown.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Use node 0 for setup — it is never killed.
	// Closed explicitly before wg.Wait() to unblock workers in synctest.
	setupDB := c.ServerConn(0)
	setupDB.SetMaxOpenConns(1)
	setupDB.SetMaxIdleConns(1)

	_, err := setupDB.ExecContext(ctx, `SET distsql = off`)
	require.NoError(t, err)
	_, err = setupDB.ExecContext(ctx, `SET vectorize = off`)
	require.NoError(t, err)

	// Set up bank accounts.
	_, err = setupDB.ExecContext(ctx, `CREATE TABLE accounts (id INT PRIMARY KEY, balance BIGINT NOT NULL)`)
	require.NoError(t, err)
	for i := 0; i < chaosBankAccounts; i++ {
		_, err = setupDB.ExecContext(ctx, `INSERT INTO accounts (id, balance) VALUES ($1, $2)`, i, chaosBankInitBalance)
		require.NoError(t, err)
	}

	// Set up actor table.
	_, err = setupDB.ExecContext(ctx, `CREATE TABLE items (id INT PRIMARY KEY, v STRING)`)
	require.NoError(t, err)

	// Seed actors.
	actors := []string{"alice", "bob", "carol"}
	for i, actor := range actors {
		_, err = setupDB.ExecContext(ctx,
			fmt.Sprintf(`INSERT INTO actor('%s').items VALUES ($1, $2)`, actor),
			i+1, fmt.Sprintf("%s-seed", actor))
		require.NoError(t, err)
	}

	stopCh := make(chan struct{})
	var stats chaosStats
	var wg sync.WaitGroup

	// Track which nodes are alive. Node 0 is never killed.
	// Protected by mu. Maps node index -> true if alive.
	var mu sync.Mutex
	alive := make(map[int]bool)
	for i := 0; i < chaosInitialNodes; i++ {
		alive[i] = true
	}

	// Goroutine 1+2: bank transfer workers (always talk to node 0).
	for w := 0; w < chaosWorkers; w++ {
		wg.Add(1)
		go func(proc int) {
			defer wg.Done()
			runChaosBankWorker(ctx, stopCh, proc, setupDB, &stats)
		}(w)
	}

	// Goroutine 3: actor query worker (always talks to node 0).
	wg.Add(1)
	go func() {
		defer wg.Done()
		runChaosActorWorker(ctx, stopCh, setupDB, actors, &stats)
	}()

	// Goroutine 4: chaos monkey — kills random non-zero nodes.
	wg.Add(1)
	go func() {
		defer wg.Done()
		runChaosMonkey(ctx, stopCh, c, &mu, alive, &stats)
	}()

	// Goroutine 5: autoscaler — adds fresh nodes when count < threshold.
	wg.Add(1)
	go func() {
		defer wg.Done()
		runChaosAutoscaler(t, ctx, stopCh, c, &mu, alive, &stats)
	}()

	time.Sleep(chaosWorkloadDuration)
	close(stopCh)
	cancel()
	// Partition node 0 to force-close all in-memory pipe connections.
	// Without this, workers stuck in SQL queries over in-memory pipes
	// block forever in synctest (which can't advance time past durable
	// I/O waits). Heal immediately after workers drain.
	c.PartitionNode(0)
	wg.Wait()
	c.HealPartition(0)

	// Fresh connection for final verification.
	verifyDB := c.ServerConn(0)
	defer func() { _ = verifyDB.Close() }()
	verifyCtx := t.Context()

	// Final verification: bank invariant.
	balances, err := readChaosBalances(verifyCtx, verifyDB)
	require.NoError(t, err)
	require.Len(t, balances, chaosBankAccounts)
	var total int64
	for _, b := range balances {
		total += b
		require.GreaterOrEqual(t, b, int64(0), "negative balance: %v", balances)
	}
	require.Equal(t, chaosBankTotal, total, "bank total changed: balances=%v", balances)

	// Final verification: actor data still present.
	for _, actor := range actors {
		var count int
		err := verifyDB.QueryRowContext(verifyCtx,
			fmt.Sprintf(`SELECT count(*) FROM actor('%s').items`, actor)).Scan(&count)
		require.NoError(t, err)
		require.GreaterOrEqual(t, count, 1, "actor %s lost all data", actor)
	}

	t.Logf("chaos stats: kills=%d spawns=%d transfers=%d reads=%d actor_writes=%d actor_reads=%d errors=%d",
		stats.kills.Load(), stats.spawns.Load(), stats.transfers.Load(),
		stats.reads.Load(), stats.actorWrites.Load(), stats.actorReads.Load(),
		stats.errors.Load())
}

func runChaosBankWorker(
	ctx context.Context,
	stopCh <-chan struct{},
	proc int,
	db *gosql.DB,
	stats *chaosStats,
) {
	rng := rand.New(rand.NewSource(int64(proc + 1)))
	for {
		select {
		case <-stopCh:
			return
		default:
		}

		if rng.Intn(3) == 0 {
			// Read balances and check invariant inline.
			balances, err := readChaosBalances(ctx, db)
			if err != nil {
				stats.errors.Add(1)
			} else {
				stats.reads.Add(1)
				var sum int64
				for _, b := range balances {
					sum += b
				}
				if sum != chaosBankTotal {
					panic(fmt.Sprintf("bank invariant violated mid-workload: sum=%d want=%d balances=%v",
						sum, chaosBankTotal, balances))
				}
			}
		} else {
			from := rng.Intn(chaosBankAccounts)
			to := rng.Intn(chaosBankAccounts - 1)
			if to >= from {
				to++
			}
			amount := int64(rng.Intn(5) + 1)
			err := runChaosTransfer(ctx, db, from, to, amount)
			if err != nil {
				stats.errors.Add(1)
			} else {
				stats.transfers.Add(1)
			}
		}

		time.Sleep(25 * time.Millisecond)
	}
}

func runChaosTransfer(ctx context.Context, db *gosql.DB, from, to int, amount int64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var fromBal int64
	if err := tx.QueryRowContext(ctx, `SELECT balance FROM accounts WHERE id = $1`, from).Scan(&fromBal); err != nil {
		return err
	}
	if fromBal < amount {
		return nil // skip, insufficient funds
	}

	var toBal int64
	if err := tx.QueryRowContext(ctx, `SELECT balance FROM accounts WHERE id = $1`, to).Scan(&toBal); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE accounts SET balance = $1 WHERE id = $2`, fromBal-amount, from); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE accounts SET balance = $1 WHERE id = $2`, toBal+amount, to); err != nil {
		return err
	}
	return tx.Commit()
}

func readChaosBalances(ctx context.Context, db *gosql.DB) ([]int64, error) {
	rows, err := db.QueryContext(ctx, `SELECT balance FROM accounts ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var balances []int64
	for rows.Next() {
		var b int64
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		balances = append(balances, b)
	}
	return balances, rows.Err()
}

func runChaosActorWorker(
	ctx context.Context,
	stopCh <-chan struct{},
	db *gosql.DB,
	actors []string,
	stats *chaosStats,
) {
	rng := rand.New(rand.NewSource(42))
	nextID := 100

	for {
		select {
		case <-stopCh:
			return
		default:
		}

		actor := actors[rng.Intn(len(actors))]
		if rng.Intn(3) == 0 {
			// Write a new item to a random actor.
			_, err := db.ExecContext(ctx,
				fmt.Sprintf(`INSERT INTO actor('%s').items VALUES ($1, $2)`, actor),
				nextID, fmt.Sprintf("%s-%d", actor, nextID))
			if err != nil {
				if !isRetryable(err) {
					stats.errors.Add(1)
				}
			} else {
				stats.actorWrites.Add(1)
				nextID++
			}
		} else {
			// Read from a random actor.
			var count int
			err := db.QueryRowContext(ctx,
				fmt.Sprintf(`SELECT count(*) FROM actor('%s').items`, actor)).Scan(&count)
			if err != nil {
				if !isRetryable(err) {
					stats.errors.Add(1)
				}
			} else {
				stats.actorReads.Add(1)
			}
		}

		time.Sleep(50 * time.Millisecond)
	}
}

func runChaosMonkey(
	ctx context.Context,
	stopCh <-chan struct{},
	c *inproc.Cluster,
	mu *sync.Mutex,
	alive map[int]bool,
	stats *chaosStats,
) {
	rng := rand.New(rand.NewSource(99))
	ticker := time.NewTicker(chaosKillInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
		}

		// Pick a random alive node that isn't node 0.
		mu.Lock()
		var candidates []int
		for idx, up := range alive {
			if up && idx != 0 {
				candidates = append(candidates, idx)
			}
		}
		mu.Unlock()

		if len(candidates) == 0 {
			continue
		}

		victim := candidates[rng.Intn(len(candidates))]
		c.StopNode(victim)

		mu.Lock()
		alive[victim] = false
		mu.Unlock()

		stats.kills.Add(1)
	}
}

func runChaosAutoscaler(
	t testing.TB,
	ctx context.Context,
	stopCh <-chan struct{},
	c *inproc.Cluster,
	mu *sync.Mutex,
	alive map[int]bool,
	stats *chaosStats,
) {
	ticker := time.NewTicker(chaosAutoscaleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
		}

		mu.Lock()
		liveCount := 0
		for _, up := range alive {
			if up {
				liveCount++
			}
		}
		mu.Unlock()

		if liveCount >= chaosMinNodes {
			continue
		}

		// Add a fresh node.
		newIdx := c.AddNode(t)

		mu.Lock()
		alive[newIdx] = true
		mu.Unlock()

		stats.spawns.Add(1)
	}
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "restart transaction") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "node unavailable") ||
		strings.Contains(msg, "result is ambiguous") ||
		strings.Contains(msg, "breaker open")
}
