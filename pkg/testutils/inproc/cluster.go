// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package inproc

import (
	"context"
	"fmt"
	"math/rand"
	"sync/atomic"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/server"
	"github.com/cockroachdb/cockroach/pkg/testutils/testcluster"
)

// nextClusterAddrBase assigns disjoint port ranges to successive
// StartCluster invocations so tests running in the same process don't
// share addresses (which can leave stale gRPC connection state).
var nextClusterAddrBase atomic.Uint64

// Cluster is a TestCluster wrapper that uses in-memory networking
// (via Registry) instead of real TCP. It is designed for use inside
// a synctest bubble where all I/O must be virtualized.
type Cluster struct {
	*testcluster.TestCluster
	Registry       *Registry
	addrs          []string
	rpcListeners   []*Listener
	stickyRegistry server.StickyVFSRegistry
	clusterBase    uint64
	nextNodeIdx    int
	clusterArgs    base.TestClusterArgs
}

// StartCluster creates and starts a multi-node in-process cluster
// with in-memory networking. All gRPC connections between nodes go
// through the Registry's in-memory transport. HTTP and SQL listeners
// are also in-memory (SQL shares the RPC listener via cmux).
func StartCluster(t testing.TB, nodes int, extraArgs ...func(*base.TestClusterArgs)) *Cluster {
	registry := NewRegistry()
	stickyRegistry := server.NewStickyVFSRegistry()
	clusterBase := nextClusterAddrBase.Add(100)
	if clusterBase == 100 {
		clusterBase = 30000
		nextClusterAddrBase.Store(clusterBase)
	}

	clusterArgs := base.TestClusterArgs{
		ReplicationMode:   base.ReplicationManual,
		ServerArgsPerNode: make(map[int]base.TestServerArgs),
	}

	for _, fn := range extraArgs {
		fn(&clusterArgs)
	}

	addrs := make([]string, nodes)
	rpcListeners := make([]*Listener, nodes)

	for i := 0; i < nodes; i++ {
		rpcAddr := fmt.Sprintf("127.0.0.1:%d", clusterBase+uint64(i))
		httpAddr := fmt.Sprintf("127.0.0.1:%d", clusterBase+1000+uint64(i))
		addrs[i] = rpcAddr

		rpcListener := registry.Register(rpcAddr)
		rpcListeners[i] = rpcListener
		httpListener := NewListener(httpAddr)

		// Start from ServerArgs defaults, then overlay per-node overrides.
		args := clusterArgs.ServerArgs
		if perNode, ok := clusterArgs.ServerArgsPerNode[i]; ok {
			args = perNode
		}
		args.Insecure = true
		args.Addr = rpcAddr
		// Route SQL client connections through the in-memory registry.
		args.SQLDialFunc = registry.SQLDialFunc()
		// Install the in-memory listener via the TestServerArgs.Listener
		// field. testcluster.StartTestCluster propagates this into
		// TestingKnobs.RPCListener automatically.
		args.Listener = rpcListener
		// Do NOT set args.SQLAddr — this keeps SplitListenSQL=false so
		// SQL shares the RPC listener via cmux (already in-memory).
		args.StoreSpecs = []base.StoreSpec{
			{InMemory: true, StickyVFSID: fmt.Sprintf("inproc-%d", i)},
		}
		args.Locality = roachpb.Locality{
			Tiers: []roachpb.Tier{
				{Key: "region", Value: "test"},
				{Key: "dc", Value: fmt.Sprintf("dc%d", i+1)},
			},
		}

		// Disable the replica scanner and all background queues. Their
		// timers can otherwise survive past cluster shutdown and produce
		// "reset of synctest timer from outside bubble" panics on later
		// tests.
		storeKnobs, _ := args.Knobs.Store.(*kvserver.StoreTestingKnobs)
		if storeKnobs == nil {
			storeKnobs = &kvserver.StoreTestingKnobs{}
		}
		storeKnobs.DisableScanner = true
		storeKnobs.DisableGCQueue = true
		storeKnobs.DisableConsistencyQueue = true
		storeKnobs.DisableRaftLogQueue = true
		storeKnobs.DisableRaftSnapshotQueue = true
		storeKnobs.DisableReplicaGCQueue = true
		storeKnobs.DisableTimeSeriesMaintenanceQueue = true
		storeKnobs.DisableLoadBasedSplitting = true
		storeKnobs.DisableReplicaRebalancing = true
		// Disable range log writes to avoid a deadlock where the SQL
		// INSERT INTO system.rangelog inside ChangeReplicas/Split
		// transactions hangs waiting for its own Raft proposal.
		storeKnobs.DisableRangeLogWrite = true
		args.Knobs.Store = storeKnobs

		serverKnobs := args.Knobs.Server
		if serverKnobs == nil {
			serverKnobs = &server.TestingKnobs{}
		}
		tk := serverKnobs.(*server.TestingKnobs)
		tk.HTTPListener = httpListener
		tk.ShareRPCListenSQL = true
		tk.StickyVFSRegistry = stickyRegistry
		// The goschedstats runnable-count callback ticker runs in a goroutine
		// started at package init, outside any synctest bubble, and would
		// otherwise send on bubble-scoped admission control channels.
		tk.DisableRunnableCountCallbacks = true
		tk.ContextTestingKnobs.DialerFunc = registry.DialerFuncFor(rpcAddr)
		args.Knobs.Server = tk

		clusterArgs.ServerArgsPerNode[i] = args
	}

	tc := testcluster.StartTestCluster(t, nodes, clusterArgs)
	return &Cluster{
		TestCluster:    tc,
		Registry:       registry,
		addrs:          addrs,
		rpcListeners:   rpcListeners,
		stickyRegistry: stickyRegistry,
		clusterBase:    clusterBase,
		nextNodeIdx:    nodes,
		clusterArgs:    clusterArgs,
	}
}

// StopNode stops a node. The node can be restarted with RestartNode.
func (c *Cluster) StopNode(nodeIdx int) {
	c.StopServer(nodeIdx)
}

// RestartNode restarts a previously stopped node, re-opening its
// in-memory listeners.
func (c *Cluster) RestartNode(t testing.TB, nodeIdx int) {
	if err := c.RestartNodeE(nodeIdx); err != nil {
		t.Fatal(err)
	}
}

// RestartNodeE restarts a previously stopped node, re-opening its
// in-memory listeners.
func (c *Cluster) RestartNodeE(nodeIdx int) error {
	if err := c.rpcListeners[nodeIdx].Reopen(); err != nil {
		return err
	}
	return c.RestartServer(nodeIdx)
}

// PartitionNode blocks all new inbound connections to the given node.
func (c *Cluster) PartitionNode(nodeIdx int) {
	c.Registry.Block(c.addrs[nodeIdx])
}

// HealPartition restores connectivity to a previously partitioned node.
func (c *Cluster) HealPartition(nodeIdx int) {
	c.Registry.Unblock(c.addrs[nodeIdx])
}

// PartitionLink blocks traffic from srcNodeIdx to dstNodeIdx and tears down
// any existing connections on that directed link.
func (c *Cluster) PartitionLink(srcNodeIdx, dstNodeIdx int) {
	c.Registry.BlockLink(c.addrs[srcNodeIdx], c.addrs[dstNodeIdx])
}

// HealLink restores a previously blocked directed link.
func (c *Cluster) HealLink(srcNodeIdx, dstNodeIdx int) {
	c.Registry.UnblockLink(c.addrs[srcNodeIdx], c.addrs[dstNodeIdx])
}

// PartitionNodeGroups blocks all directed traffic between distinct groups while
// preserving connectivity within each group.
func (c *Cluster) PartitionNodeGroups(groups [][]int) {
	addrs := make([][]string, len(groups))
	for i := range groups {
		addrs[i] = make([]string, len(groups[i]))
		for j, nodeIdx := range groups[i] {
			addrs[i][j] = c.addrs[nodeIdx]
		}
	}
	c.Registry.PartitionGroups(addrs)
}

// PartitionRandomHalves applies the Jepsen "parts" topology to the selected
// nodes using the provided RNG.
func (c *Cluster) PartitionRandomHalves(nodeIdxs []int, rng *rand.Rand) {
	c.Registry.PartitionGrudge(RandomHalvesGrudge(c.nodeAddrs(nodeIdxs), rng))
}

// PartitionMajoritiesRing applies the Jepsen "majority-ring" topology to the
// selected nodes in the order provided.
func (c *Cluster) PartitionMajoritiesRing(nodeIdxs []int) {
	c.Registry.PartitionGrudge(MajoritiesRingGrudge(c.nodeAddrs(nodeIdxs)))
}

// HealAllLinks removes all directed link partitions while leaving whole-node
// partitions untouched.
func (c *Cluster) HealAllLinks() {
	c.Registry.HealAllLinks()
}

func (c *Cluster) nodeAddrs(nodeIdxs []int) []string {
	addrs := make([]string, len(nodeIdxs))
	for i, nodeIdx := range nodeIdxs {
		addrs[i] = c.addrs[nodeIdx]
	}
	return addrs
}

// NodeAddr returns the in-memory address for the given node index.
func (c *Cluster) NodeAddr(nodeIdx int) string {
	return c.addrs[nodeIdx]
}

// AddNode dynamically adds a new node to a running cluster with in-memory
// networking. The new node joins through the first node's RPC address.
func (c *Cluster) AddNode(t testing.TB) int {
	i := c.nextNodeIdx
	c.nextNodeIdx++

	rpcAddr := fmt.Sprintf("127.0.0.1:%d", c.clusterBase+uint64(i))
	httpAddr := fmt.Sprintf("127.0.0.1:%d", c.clusterBase+1000+uint64(i))
	rpcListener := c.Registry.Register(rpcAddr)
	httpListener := NewListener(httpAddr)

	args := c.clusterArgs.ServerArgs
	args.Insecure = true
	args.Addr = rpcAddr
	args.SQLDialFunc = c.Registry.SQLDialFunc()
	args.Listener = rpcListener
	args.StoreSpecs = []base.StoreSpec{
		{InMemory: true, StickyVFSID: fmt.Sprintf("inproc-%d", i)},
	}
	args.Locality = roachpb.Locality{
		Tiers: []roachpb.Tier{
			{Key: "region", Value: "test"},
			{Key: "dc", Value: fmt.Sprintf("dc%d", i+1)},
		},
	}

	storeKnobs, _ := args.Knobs.Store.(*kvserver.StoreTestingKnobs)
	if storeKnobs == nil {
		storeKnobs = &kvserver.StoreTestingKnobs{}
	} else {
		clone := *storeKnobs
		storeKnobs = &clone
	}
	storeKnobs.DisableScanner = true
	storeKnobs.DisableGCQueue = true
	storeKnobs.DisableConsistencyQueue = true
	storeKnobs.DisableRaftLogQueue = true
	storeKnobs.DisableRaftSnapshotQueue = true
	storeKnobs.DisableReplicaGCQueue = true
	storeKnobs.DisableTimeSeriesMaintenanceQueue = true
	storeKnobs.DisableLoadBasedSplitting = true
	storeKnobs.DisableReplicaRebalancing = true
	storeKnobs.DisableRangeLogWrite = true
	args.Knobs.Store = storeKnobs

	serverKnobs := args.Knobs.Server
	if serverKnobs == nil {
		serverKnobs = &server.TestingKnobs{}
	}
	tk := serverKnobs.(*server.TestingKnobs)
	tk.HTTPListener = httpListener
	tk.ShareRPCListenSQL = true
	tk.StickyVFSRegistry = c.stickyRegistry
	tk.DisableRunnableCountCallbacks = true
	tk.ContextTestingKnobs.DialerFunc = c.Registry.DialerFuncFor(rpcAddr)
	args.Knobs.Server = tk

	c.addrs = append(c.addrs, rpcAddr)
	c.rpcListeners = append(c.rpcListeners, rpcListener)
	c.AddAndStartServer(t.(*testing.T), args)
	return i
}

// Stop stops the cluster and closes the registry.
func (c *Cluster) Stop() {
	for i := range c.Conns {
		if c.Conns[i] != nil {
			_ = c.Conns[i].Close()
			c.Conns[i] = nil
		}
	}
	c.Stopper().Stop(context.Background())
	c.stickyRegistry.CloseAllEngines()
	c.Registry.Close()
}
