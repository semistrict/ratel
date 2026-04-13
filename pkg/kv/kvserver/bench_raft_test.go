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

package kvserver_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/semistrict/ratel/pkg/base"
	"github.com/semistrict/ratel/pkg/kv"
	"github.com/semistrict/ratel/pkg/testutils/serverutils"
	"github.com/semistrict/ratel/pkg/util/log"
)

// BenchmarkSingleNodePut benchmarks a single non-transactional Put through
// the full KV path on a single-node in-memory cluster: DistSender → local
// RPC → replica concurrency → Raft propose → Raft apply → Pebble write.
func BenchmarkSingleNodePut(b *testing.B) {
	defer log.Scope(b).Close(b)
	ctx := context.Background()

	s, _, db := serverutils.StartServer(b, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)

	// Warm up: write one key to acquire the range lease.
	if err := db.Put(ctx, "warmup", "v"); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Appendf(nil, "bench-put-%08d", i)
		if err := db.Put(ctx, key, "value"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSingleNodeGet benchmarks a single non-transactional Get.
func BenchmarkSingleNodeGet(b *testing.B) {
	defer log.Scope(b).Close(b)
	ctx := context.Background()

	s, _, db := serverutils.StartServer(b, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)

	if err := db.Put(ctx, "bench-get-key", "bench-value"); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kv, err := db.Get(ctx, "bench-get-key")
		if err != nil {
			b.Fatal(err)
		}
		if kv.Value == nil {
			b.Fatal("nil value")
		}
	}
}

// BenchmarkSingleNodePutGet benchmarks a Put followed by a Get of the same key.
func BenchmarkSingleNodePutGet(b *testing.B) {
	defer log.Scope(b).Close(b)
	ctx := context.Background()

	s, _, db := serverutils.StartServer(b, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)

	if err := db.Put(ctx, "warmup", "v"); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Appendf(nil, "bench-pg-%08d", i)
		if err := db.Put(ctx, key, "value"); err != nil {
			b.Fatal(err)
		}
		kv, err := db.Get(ctx, key)
		if err != nil {
			b.Fatal(err)
		}
		if kv.Value == nil {
			b.Fatal("nil value")
		}
	}
}

// BenchmarkSingleNodeBatchPut benchmarks batch writes of N keys in one call.
func BenchmarkSingleNodeBatchPut(b *testing.B) {
	defer log.Scope(b).Close(b)
	ctx := context.Background()

	for _, batchSize := range []int{1, 10, 50} {
		b.Run(fmt.Sprintf("batch=%d", batchSize), func(b *testing.B) {
			s, _, db := serverutils.StartServer(b, base.TestServerArgs{})
			defer s.Stopper().Stop(ctx)

			if err := db.Put(ctx, "warmup", "v"); err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				batch := &kv.Batch{}
				for j := 0; j < batchSize; j++ {
					key := fmt.Appendf(nil, "bench-bp-%08d-%04d", i, j)
					batch.Put(key, "value")
				}
				if err := db.Run(ctx, batch); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkSingleNodeDel benchmarks a single non-transactional delete.
func BenchmarkSingleNodeDel(b *testing.B) {
	defer log.Scope(b).Close(b)
	ctx := context.Background()

	s, _, db := serverutils.StartServer(b, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)

	// Pre-populate.
	for i := 0; i < b.N; i++ {
		key := fmt.Appendf(nil, "bench-del-%08d", i)
		if err := db.Put(ctx, key, "v"); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Appendf(nil, "bench-del-%08d", i)
		if err := db.Del(ctx, key); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSingleNode1PCTxn benchmarks a 1PC transaction with a single Put.
func BenchmarkSingleNode1PCTxn(b *testing.B) {
	defer log.Scope(b).Close(b)
	ctx := context.Background()

	s, _, db := serverutils.StartServer(b, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)

	if err := db.Put(ctx, "warmup", "v"); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := db.Txn(ctx, func(ctx context.Context, txn *kv.Txn) error {
			key := fmt.Appendf(nil, "bench-txn-%08d", i)
			return txn.Put(ctx, key, "value")
		}); err != nil {
			b.Fatal(err)
		}
	}
}
