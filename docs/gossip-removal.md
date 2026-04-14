# Gossip Protocol Removal

## Motivation

CockroachDB's gossip protocol assumes long-lived nodes with stable addresses.
Ratel's ephemeral node model — where nodes are disposable and the cluster
survives continuous churn — is fundamentally broken by gossip:

- Dead node descriptors persist for 2 hours (TTL). Every alive node tries
  reconnecting to every dead address it ever learned about.
- In synctest, gossip reconnection burns fake time so aggressively that a
  5-minute chaos test cannot complete.
- The gossip overlay network is architecturally wrong for a world where nodes
  come and go freely.

## What Was Removed

**24 files deleted** from `pkg/gossip/`:

- `client.go`, `server.go` — gossip RPC client and server
- `infostore.go` — gossip key-value store
- `keys.go` — gossip key constructors (`MakeNodeIDKey`, `MakeStoreDescKey`, etc.)
- `node_set.go`, `util.go` — connection management
- `info.go`, `doc.go` — types and documentation
- `simulation/network.go` — gossip network simulator
- All corresponding `*_test.go` files

**Other deleted files:**

- `pkg/cmd/gossipsim/main.go` — standalone gossip simulator binary
- `pkg/cmd/roachtest/tests/gossip.go` — roachtest gossip-specific tests
- `pkg/kv/kvserver/gossip_test.go` — kvserver gossip integration tests
- `pkg/testutils/gossiputil/store_gossiper.go` — test helper for gossip-based store descriptors

**Retained stubs** (for protobuf compatibility):

- `pkg/gossip/gossip.go` — empty `Gossip` struct so `*gossip.Gossip` compiles
- `pkg/gossip/status.go` — `SafeFormat` methods for protobuf types defined in `gossip.pb.go`
- `pkg/gossip/gossip.pb.go` — generated protobuf (unchanged, needed by `serverpb`)

## Replacement Architecture

### Node Descriptors: KV-backed NodeDescStore

Each node writes its `roachpb.NodeDescriptor` to a system KV key under
`NodeDescriptorPrefix` (added to `pkg/keys/constants.go`). The
`nodedescstore.Store` (`pkg/kv/kvclient/nodedescstore/store.go`) maintains an
in-memory cache populated via KV scans.

**Used by:** `DistSender` (for routing RPCs to nodes), `NodeDialer` (address
resolution), status server (listing nodes), `crdb_internal` tables.

**In tests:** `SharedNodeDescStore` (`pkg/server/testing_knobs.go`) provides a
simple in-memory map shared across all nodes in a `TestCluster`. The
`lazyNodeDescStore` wrapper (`pkg/server/server.go:1015`) tries the real KV
store first, then falls back to the shared store.

### First Range: LocalFirstRangeProvider

`kvcoord.LocalFirstRangeProvider` (`pkg/kv/kvclient/kvcoord/`) is set by the
node that holds range 1's lease. It calls registered callbacks when the first
range descriptor changes.

**Bootstrap sequence:** At cold start, `ratel join` reads peer addresses from S3
node discovery. The node connects to peers, joins the cluster, and the first
range provider is populated by the range 1 leaseholder.

**In tests:** `SharedFirstRangeProvider` (`pkg/server/testing_knobs.go`) is
shared across all nodes in a `TestCluster`.

### Liveness: KV Polling

`NodeLiveness.StartLivenessPoller()` (`pkg/kv/kvserver/liveness/liveness.go`)
replaces gossip-based liveness distribution. Each node periodically scans the
liveness range (`keys.NodeLivenessSpan`) and calls `maybeUpdate()` on changes.

Previously, the liveness range leaseholder gossipped all liveness records to
every node. Now each node reads them directly from KV.

### Store Descriptors

`Store.GossipStore()` and `Store.asyncGossipStore()` are no-ops. Store
descriptors are published via the node descriptor store and KV. `StorePool`
receives store descriptor updates via callbacks instead of gossip subscriptions.

### System Config

The system config gossip trigger was already disabled
(`DisableSystemConfigGossipTrigger` in upstream 22.1). System config changes
propagate via the existing span config infrastructure and rangefeed-backed
watchers.

### DistSQL Version

Gossip-based DistSQL version broadcasting removed from `pkg/sql/distsql/server.go`.
Ratel runs a homogeneous binary — all nodes are the same version.

### Statement Diagnostics

The gossip-based push notification in
`pkg/sql/stmtdiagnostics/statement_diagnostics.go` was removed. The existing
`poll` goroutine (which already polled the diagnostics table) is sufficient.

## Key Changes by Package

### pkg/server/

- `server.go`: Removed `gossip.New()` construction, `s.gossip` field,
  `gossip.Start()` call. Added `lazyNodeDescStore`, wiring for
  `SharedNodeDescStore` and `SharedFirstRangeProvider` from testing knobs.
- `node.go`: Removed `n.gossip` field, gossip-based `SetNodeDescriptor`,
  periodic re-gossip. Node descriptors written to KV via `nodedescstore`.
- `testing_knobs.go`: Added `SharedNodeDescStore`, `SharedFirstRangeProvider`
  types and fields on `TestingKnobs`.
- `testserver.go`: `Gossip()` returns nil. Added `NodeDescStoreI()`,
  `FirstRangeProviderI()`.
- `status.go`: `Gossip()` RPC returns empty `InfoStatus` for backward compat.
- `workerd_router.go`: Uses `nodedescstore` for address resolution instead of
  gossip.

### pkg/kv/kvserver/

- `store.go`: Removed `Gossip *gossip.Gossip` from `StoreConfig`. Gossip-based
  store publishing replaced with no-ops. `systemGossipUpdate` callback retained
  (name kept for clarity) but receives system config from span config
  infrastructure, not gossip.
- `replica_gossip.go`: `gossipFirstRange` writes to `FirstRangeCallback` instead
  of gossip. `MaybeGossipNodeLivenessRaftMuLocked` is a no-op.
  `MaybeGossipSystemConfig*` methods are no-ops.
- `store_pool.go`: Accepts store descriptor callbacks instead of `*gossip.Gossip`.
- `stores.go`: Removed gossip-based node descriptor registration.
- `liveness/liveness.go`: Removed `gossip` field. `StartLivenessPoller()` scans
  liveness range via KV.

### pkg/kv/kvclient/kvcoord/

- `dist_sender.go`: `DistSenderConfig` uses `NodeDescStore` interface and
  `FirstRangeProvider` interface instead of `*gossip.Gossip`.

### pkg/sql/

- `distsql/server.go`: Removed gossip-based version broadcasting.
- `distsql_physical_planner.go`: Removed gossip-based draining/version checks.
- `crdb_internal.go`: Node/store info tables read from `nodedescstore` instead
  of gossip.
- `stmtdiagnostics/statement_diagnostics.go`: Removed gossip notification,
  relies on polling.
- `exec_util.go`, `execinfra/server_config.go`: Removed `*gossip.Gossip` fields.

### pkg/testutils/

- `testcluster/testcluster.go`: Injects `SharedNodeDescStore` and
  `SharedFirstRangeProvider` into all test cluster nodes. Shares
  `cluster.Settings` across nodes.
- `localtestcluster/local_test_cluster.go`: Added `simpleNodeDescStore` for
  single-node test clusters.
- `gossiputil/store_gossiper.go`: Deleted.
- `serverutils/test_server_shim.go`: Added `NodeDescStoreI()`,
  `FirstRangeProviderI()` to `TestServerInterface`.

### pkg/keys/

- `constants.go`: Added `NodeDescriptorPrefix`, `StoreDescriptorPrefix`.
- `keys.go`: Added key encoding functions for node/store descriptors.
- `spans.go`: Added `NodeDescriptorSpan`, `StoreDescriptorSpan`.

## Test Changes

~30 test files updated. Common patterns:

1. **Replace `makeGossip()` / gossip setup** with mock `NodeDescStore` +
   `FirstRangeProvider` implementations.
2. **Replace `g.AddInfoProto(gossip.MakeNodeIDKey(...))` node registration**
   with `store.AddNode(desc)` on a mock store.
3. **Replace `gossip.AddressResolver(g)`** with `store.AddressResolver()`.
4. **Replace `gossiputil.NewStoreGossiper(g)`** with direct store descriptor
   callbacks on `StorePool`.
5. **Delete gossip-only tests** that verified gossip propagation behavior
   (irrelevant without gossip).

## Net Impact

- **115 files changed**, 1,775 insertions, 11,955 deletions (~10k net lines removed)
- **24 files deleted** (gossip implementation + tests + simulator)
- `pkg/gossip/` reduced from ~5,000 lines of Go to ~100 lines of stubs + generated protobuf
