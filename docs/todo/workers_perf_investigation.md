# Workers Platform Performance Investigation

TODO: Investigate the allocation and CPU hotspots identified below.

## Benchmarks (2026-04-12, Apple Silicon, single-node in-memory)

### Full E2E: HTTP → workerd → DO → capnp RPC → Ratel KV

```
BenchmarkWorkerDOE2E-15    25406    472582 ns/op    44758 B/op    124 allocs/op
```

472 us per request through: HTTP client → cmux → reverse proxy → workerd →
JS fetch() → DO stub → ActorCache → capnp TwoPartyClient → socketpair →
Go capnp server → Ratel KV (get + put) → response back.

### DO Storage via capnp RPC (no workerd)

```
BenchmarkDOStoragePut-15                       38637     32491 ns/op    16090 B/op     164 allocs/op
BenchmarkDOStorageGet-15                       62515     21049 ns/op     9517 B/op     124 allocs/op
BenchmarkDOStoragePutGet-15                    20710     59236 ns/op    25845 B/op     291 allocs/op
BenchmarkDOStorageDelete-15                    32866     35373 ns/op    15425 B/op     163 allocs/op
BenchmarkDOStorageBatchPut/batch=1-15          37008     31979 ns/op    15886 B/op     164 allocs/op
BenchmarkDOStorageBatchPut/batch=10-15         25426     58073 ns/op    45698 B/op     340 allocs/op
BenchmarkDOStorageBatchPut/batch=50-15          8881    123796 ns/op   178038 B/op    1045 allocs/op
BenchmarkDOStorageBatchPut/batch=128-15         3742    270368 ns/op   438766 B/op    2388 allocs/op
BenchmarkDOStorageGetStage-15                  58329     22873 ns/op    10378 B/op     169 allocs/op
```

### Raw KV (no capnp, no workerd)

```
BenchmarkSingleNodePut-15         131222     45112 ns/op    19556 B/op      70 allocs/op
BenchmarkSingleNodeGet-15        1120722      5268 ns/op     3344 B/op      29 allocs/op
BenchmarkSingleNodePutGet-15      112651     53564 ns/op    21800 B/op      98 allocs/op
BenchmarkSingleNodeDel-15         126798     47909 ns/op    17773 B/op      65 allocs/op
BenchmarkSingleNode1PCTxn-15       39770    146907 ns/op    72516 B/op     356 allocs/op
```

### Overhead breakdown

| Layer                        | Put (us) | Get (us) |
|------------------------------|----------|----------|
| Raw KV                       | 45       | 5.3      |
| capnp RPC overhead           | -13 (*)  | 16       |
| DO Storage (capnp + KV)      | 32       | 21       |
| workerd + HTTP + JS overhead | ~413     | —        |
| Full E2E (put+get)           | 473      | —        |

(*) DO Put is faster than raw KV Put because the capnp benchmark uses
`kv.Batch.Run` (batched path) while the raw benchmark uses `db.Put`
(unbatched path with extra overhead). The true capnp overhead for Put
is ~5 us based on trace data.

## Trace: DO Put+Get through capnp RPC

```
 0.000ms  do.e2e.put+get
 0.633ms    do.storage.put (warmup — includes lease acquisition)
 1.253ms    do.storage.get (warmup)
 1.347ms    begin put
 1.352ms      do.storage.put
 1.353ms        dist sender send
 1.367ms          Internal/Batch (local gRPC)
 1.373ms            store_send: executing Put
 1.378ms            concurrency: sequencing
 1.380ms            concurrency: acquiring latches
 1.387ms            replica_write: executing batch
 1.397ms            replica_evaluate: evaluated Put (+0.010ms)
 1.406ms            replica_raft: proposing command (+0.009ms)
 1.412ms              local proposal
 1.420ms              raft: applying command (+0.008ms)
 1.440ms    put complete (total: 0.093ms)
 1.440ms    begin get
 1.445ms      do.storage.get
 1.445ms        dist sender send
 1.458ms          Internal/Batch (local gRPC)
 1.465ms            store_send: executing Get
 1.469ms            concurrency: sequencing
 1.476ms            replica_read: executing batch
 1.491ms            replica_evaluate: evaluated Get (+0.015ms)
 1.494ms            read completed
 1.505ms    get complete (total: 0.065ms)
```

### Per-component cost (post-warmup, single node)

| Component                 | Put (us) | Get (us) |
|---------------------------|----------|----------|
| capnp → do.storage entry  | 5        | 5        |
| dist sender routing       | 14       | 13       |
| concurrency (latches)     | 7        | 7        |
| evaluate command          | 10       | 15       |
| Raft propose + apply      | 23       | —        |
| return to caller          | 20       | 11       |

## CPU Profile (top application functions, excluding runtime)

```
flat%  function
1.47%  pebble.(*Iterator).Close
0.48%  storage.EngineKeyCompare
0.41%  arenaskl.(*Skiplist).findSpliceForLevel       (memtable insert)
0.15%  pebble.(*Iterator).constructPointIter
0.12%  storage.(*pebbleIterator).destroy
0.10%  Replica.executeReadOnlyBatch
0.10%  storage.mvccGet
```

65% of total CPU is goroutine scheduling overhead (pthread_cond_signal/wait).

## Allocation Profile

### TODO: Investigate TrackRaftProtos (29% of all bytes allocated)

`kvserver.TrackRaftProtos.func1` allocates 6.01 GB (29% of all bytes) and
5.8M objects across the benchmark run. This is a debug/testing function that
wraps every Raft proto to track which types flow through the system. It
should be disabled or compiled out in non-test builds.

Location: `pkg/kv/kvserver/track_raft_protos.go` (or similar).

### TODO: Investigate context.WithValue (18% of all allocations)

`context.WithValue` is the #1 allocator by count: 21.5M allocations (18%).
Every KV operation wraps context multiple times as it propagates through
the stack (tracing spans, log tags, admission control, etc.). Consider:
- Reducing the number of context wraps per operation
- Using a struct-of-fields context wrapper instead of chained WithValue
- Amortizing context construction across batch operations

### TODO: Investigate BatchRequest.CreateReply (7% of allocations)

8.3M allocations constructing response protos. Consider pooling.

### Top allocators by bytes

```
  GB   %     function
6.01  29%    TrackRaftProtos.func1              (debug proto tracking)
2.74  13%    kv.(*DB).Get                       (Get request construction)
1.71   8%    pebble.(*memFile).Write             (Pebble WAL writes)
0.96   5%    context.WithValue                  (context propagation)
0.70   3%    Replica.Send                       (replica request handling)
0.69   3%    Replica.requestToProposal          (Raft proposal construction)
0.68   3%    grpcTransport.sendBatch            (local gRPC transport)
0.59   3%    BatchRequest.CreateReply           (response proto alloc)
0.55   3%    spanlatch.allocGuardAndLatches      (latch alloc)
0.52   3%    kv.(*DB).Put                       (Put request construction)
```

### Top allocators by count

```
 count     %     function
 21.5M   18%     context.WithValue
  8.3M    7%     BatchRequest.CreateReply
  5.9M    5%     pebble.mergingIter.init
  5.8M    5%     TrackRaftProtos.func1
  3.3M    3%     RequestUnion.MustSetInner
  3.1M    3%     NewReplicaSlice
  3.0M    3%     OptimizeReplicaOrder
  2.9M    2%     spanlatch.allocGuardAndLatches
  2.9M    2%     kv.(*Batch).growReqs
  2.9M    2%     rangecache.getCachedRLocked
```
