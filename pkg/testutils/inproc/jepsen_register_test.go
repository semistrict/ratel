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
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/anishathalye/porcupine"
	"github.com/semistrict/ratel/pkg/testutils/inproc"
	"github.com/stretchr/testify/require"
)

const jepsenRegisterKeys = 6

type jepsenRegisterInput struct {
	Key      int64
	Op       string
	Value    int64
	Expected int64
}

type jepsenRegisterOutput struct {
	Value int64
	Ok    bool
}

type jepsenRegisterNemesisFunc func(
	ctx context.Context,
	stopCh <-chan struct{},
	c *inproc.Cluster,
	db *gosql.DB,
	history *jepsenRegisterHistory,
)

type jepsenRegisterHistory struct {
	mu         sync.Mutex
	ops        []porcupine.Operation
	infos      []string
	nextOpID   int64
	nextSeqNum int64
}

func (h *jepsenRegisterHistory) Begin(clientID int, input jepsenRegisterInput) jepsenRegisterPending {
	return jepsenRegisterPending{
		history:  h,
		clientID: clientID,
		id:       int(atomic.AddInt64(&h.nextOpID, 1)),
		input:    input,
		call:     atomic.AddInt64(&h.nextSeqNum, 1),
	}
}

func (h *jepsenRegisterHistory) AddInfo(format string, args ...interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.infos = append(h.infos, fmt.Sprintf(format, args...))
}

func (h *jepsenRegisterHistory) Snapshot() ([]porcupine.Operation, []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ops := make([]porcupine.Operation, len(h.ops))
	copy(ops, h.ops)
	infos := make([]string, len(h.infos))
	copy(infos, h.infos)
	return ops, infos
}

type jepsenRegisterPending struct {
	history  *jepsenRegisterHistory
	clientID int
	id       int
	input    jepsenRegisterInput
	call     int64
}

func (p jepsenRegisterPending) Finish(output jepsenRegisterOutput) {
	p.history.mu.Lock()
	defer p.history.mu.Unlock()
	p.history.ops = append(p.history.ops, porcupine.Operation{
		ClientId: p.clientID,
		Input:    p.input,
		Call:     p.call,
		Output:   output,
		Return:   atomic.AddInt64(&p.history.nextSeqNum, 1),
		Metadata: p.id,
	})
}

func TestSyncJepsenRegister(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		runJepsenRegisterWorkload(t, nil)
	})
}

func TestSyncJepsenRegisterRestart(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		runJepsenRegisterWorkload(t, runJepsenRegisterRestartNemesis)
	})
}

func TestSyncJepsenRegisterSplit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		runJepsenRegisterWorkload(t, runJepsenRegisterSplitNemesis)
	})
}

func TestSyncJepsenRegisterPartition(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		runJepsenRegisterWorkload(t, runJepsenRegisterPartitionNemesis)
	})
}

func runJepsenRegisterWorkload(t *testing.T, nemesis jepsenRegisterNemesisFunc) {
	t.Helper()

	c := startSyncCluster(t, 3)
	defer stopSyncCluster(c)

	ctx := t.Context()
	setupDB := c.ServerConn(0)
	setupDB.SetMaxOpenConns(1)
	setupDB.SetMaxIdleConns(1)
	configureJepsenBankSession(t, ctx, setupDB)
	setupJepsenRegisterTable(t, ctx, setupDB)
	nemesisDB := c.ServerConn(0)
	nemesisDB.SetMaxOpenConns(1)
	nemesisDB.SetMaxIdleConns(1)
	configureJepsenBankSession(t, ctx, nemesisDB)

	history := &jepsenRegisterHistory{}
	const workers = 3
	opsPerWorker := 20
	if nemesis != nil {
		opsPerWorker = 80
	}
	stopCh := make(chan struct{})
	var workerWG sync.WaitGroup
	for i := 0; i < workers; i++ {
		workerWG.Add(1)
		go func(proc int) {
			defer workerWG.Done()
			db := c.ServerConn(proc % 3)
			db.SetMaxOpenConns(1)
			db.SetMaxIdleConns(1)
			configureJepsenBankSession(t, ctx, db)
			runJepsenRegisterWorker(ctx, proc, db, history, opsPerWorker)
		}(i)
	}

	var nemesisWG sync.WaitGroup
	if nemesis != nil {
		nemesisWG.Add(1)
		go func() {
			defer nemesisWG.Done()
			nemesis(ctx, stopCh, c, nemesisDB, history)
		}()
	}

	workerWG.Wait()
	close(stopCh)
	nemesisWG.Wait()

	ops, infos := history.Snapshot()
	require.NotEmpty(t, ops)
	if nemesis == nil {
		require.Empty(t, infos, "unexpected register workload infos: %+v", infos)
	}

	res := porcupine.CheckOperationsTimeout(jepsenRegisterModel(), ops, time.Second)
	require.Equal(t, porcupine.Ok, res)
}

func setupJepsenRegisterTable(t *testing.T, ctx context.Context, db *gosql.DB) {
	t.Helper()

	_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS test`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE TABLE test (id INT PRIMARY KEY, val INT NOT NULL)`)
	require.NoError(t, err)
	for i := 0; i < jepsenRegisterKeys; i++ {
		_, err = db.ExecContext(ctx, `INSERT INTO test (id, val) VALUES ($1, 0)`, i)
		require.NoError(t, err)
	}
}

func runJepsenRegisterWorker(
	ctx context.Context,
	proc int,
	db *gosql.DB,
	history *jepsenRegisterHistory,
	ops int,
) {
	rng := rand.New(rand.NewSource(int64(proc + 801)))

	for i := 0; i < ops; i++ {
		key := int64(proc % jepsenRegisterKeys)
		opType := rng.Intn(4)

		switch opType {
		case 0:
			input := jepsenRegisterInput{Key: key, Op: "read"}
			pending := history.Begin(proc, input)
			value, err := readJepsenRegister(ctx, db, key)
			if err != nil {
				history.AddInfo("proc=%d op=read key=%d err=%v", proc, key, err)
				break
			}
			pending.Finish(jepsenRegisterOutput{Value: value})
		case 1:
			value := int64(rng.Intn(5))
			input := jepsenRegisterInput{Key: key, Op: "write", Value: value}
			pending := history.Begin(proc, input)
			if err := writeJepsenRegister(ctx, db, key, value); err != nil {
				history.AddInfo("proc=%d op=write key=%d value=%d err=%v", proc, key, value, err)
				break
			}
			pending.Finish(jepsenRegisterOutput{Ok: true})
		default:
			expected := int64(rng.Intn(5))
			value := int64(rng.Intn(5))
			input := jepsenRegisterInput{Key: key, Op: "cas", Expected: expected, Value: value}
			pending := history.Begin(proc, input)
			ok, err := casJepsenRegister(ctx, db, key, expected, value)
			if err != nil {
				history.AddInfo(
					"proc=%d op=cas key=%d expected=%d value=%d err=%v",
					proc, key, expected, value, err,
				)
				break
			}
			pending.Finish(jepsenRegisterOutput{Ok: ok})
		}

		time.Sleep(25 * time.Millisecond)
	}
}

func runJepsenRegisterRestartNemesis(
	ctx context.Context,
	stopCh <-chan struct{},
	c *inproc.Cluster,
	_ *gosql.DB,
	history *jepsenRegisterHistory,
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
			if err := c.RestartNodeE(2); err != nil {
				history.AddInfo("nemesis=restart node=2 err=%v", err)
			}
		}
	}
}

func runJepsenRegisterSplitNemesis(
	ctx context.Context,
	stopCh <-chan struct{},
	_ *inproc.Cluster,
	db *gosql.DB,
	history *jepsenRegisterHistory,
) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	var nextKey int64
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			key := nextKey % jepsenRegisterKeys
			nextKey++
			if _, err := db.ExecContext(ctx, `ALTER TABLE test SPLIT AT VALUES ($1)`, key); err != nil {
				history.AddInfo("nemesis=split key=%d err=%v", key, err)
			}
		}
	}
}

func runJepsenRegisterPartitionNemesis(
	ctx context.Context,
	stopCh <-chan struct{},
	c *inproc.Cluster,
	_ *gosql.DB,
	history *jepsenRegisterHistory,
) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	healed := true
	heal := func() {
		c.HealLink(2, 0)
		c.HealLink(0, 2)
		c.HealLink(2, 1)
		c.HealLink(1, 2)
		healed = true
	}
	defer func() {
		if !healed {
			heal()
		}
	}()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			history.AddInfo("nemesis=partition isolate-node-2")
			c.PartitionLink(2, 0)
			c.PartitionLink(0, 2)
			c.PartitionLink(2, 1)
			c.PartitionLink(1, 2)
			healed = false
			select {
			case <-stopCh:
				heal()
				return
			case <-time.After(500 * time.Millisecond):
				heal()
			}
		}
	}
}

func readJepsenRegister(ctx context.Context, db *gosql.DB, key int64) (int64, error) {
	var value int64
	err := db.QueryRowContext(ctx, `SELECT val FROM test WHERE id = $1`, key).Scan(&value)
	return value, err
}

func writeJepsenRegister(ctx context.Context, db *gosql.DB, key, value int64) error {
	_, err := db.ExecContext(ctx, `UPDATE test SET val = $2 WHERE id = $1`, key, value)
	return err
}

func casJepsenRegister(ctx context.Context, db *gosql.DB, key, expected, value int64) (bool, error) {
	res, err := db.ExecContext(
		ctx,
		`UPDATE test SET val = $3 WHERE id = $1 AND val = $2`,
		key, expected, value,
	)
	if err != nil {
		return false, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows == 1, nil
}

func jepsenRegisterModel() porcupine.Model {
	return porcupine.Model{
		Partition: func(history []porcupine.Operation) [][]porcupine.Operation {
			partitions := make(map[int64][]porcupine.Operation)
			var keys []int64
			seen := make(map[int64]struct{})
			for _, op := range history {
				key := op.Input.(jepsenRegisterInput).Key
				partitions[key] = append(partitions[key], op)
				if _, ok := seen[key]; !ok {
					seen[key] = struct{}{}
					keys = append(keys, key)
				}
			}
			out := make([][]porcupine.Operation, 0, len(keys))
			for _, key := range keys {
				out = append(out, partitions[key])
			}
			return out
		},
		Init: func() interface{} {
			return int64(0)
		},
		Step: func(state interface{}, input interface{}, output interface{}) (bool, interface{}) {
			current := state.(int64)
			in := input.(jepsenRegisterInput)
			out := output.(jepsenRegisterOutput)
			switch in.Op {
			case "read":
				return out.Value == current, current
			case "write":
				return out.Ok, in.Value
			case "cas":
				if current == in.Expected {
					return out.Ok, in.Value
				}
				return !out.Ok, current
			default:
				return false, current
			}
		},
		DescribeOperation: func(input interface{}, output interface{}) string {
			in := input.(jepsenRegisterInput)
			out := output.(jepsenRegisterOutput)
			switch in.Op {
			case "read":
				return fmt.Sprintf("read(%d) -> %d", in.Key, out.Value)
			case "write":
				return fmt.Sprintf("write(%d, %d)", in.Key, in.Value)
			case "cas":
				result := "fail"
				if out.Ok {
					result = "ok"
				}
				return fmt.Sprintf("cas(%d, %d, %d) -> %s", in.Key, in.Expected, in.Value, result)
			default:
				return "<invalid>"
			}
		},
	}
}

func TestJepsenRegisterModel(t *testing.T) {
	model := jepsenRegisterModel()

	ok := porcupine.CheckOperations(model, []porcupine.Operation{
		{
			ClientId: 0,
			Input:    jepsenRegisterInput{Key: 0, Op: "write", Value: 2},
			Call:     1,
			Output:   jepsenRegisterOutput{Ok: true},
			Return:   4,
		},
		{
			ClientId: 1,
			Input:    jepsenRegisterInput{Key: 0, Op: "read"},
			Call:     2,
			Output:   jepsenRegisterOutput{Value: 2},
			Return:   3,
		},
		{
			ClientId: 2,
			Input:    jepsenRegisterInput{Key: 1, Op: "cas", Expected: 0, Value: 4},
			Call:     2,
			Output:   jepsenRegisterOutput{Ok: true},
			Return:   3,
		},
	})
	require.True(t, ok)

	ok = porcupine.CheckOperations(model, []porcupine.Operation{
		{
			ClientId: 0,
			Input:    jepsenRegisterInput{Key: 0, Op: "write", Value: 2},
			Call:     1,
			Output:   jepsenRegisterOutput{Ok: true},
			Return:   6,
		},
		{
			ClientId: 1,
			Input:    jepsenRegisterInput{Key: 0, Op: "read"},
			Call:     2,
			Output:   jepsenRegisterOutput{Value: 2},
			Return:   3,
		},
		{
			ClientId: 2,
			Input:    jepsenRegisterInput{Key: 0, Op: "read"},
			Call:     4,
			Output:   jepsenRegisterOutput{Value: 0},
			Return:   5,
		},
	})
	require.False(t, ok)
}
