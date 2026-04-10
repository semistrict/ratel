// Copyright 2025 The Cockroach Authors.
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

package inproc

import (
	"context"
	"fmt"
	"math/rand"
	"sync/atomic"
	"testing"

	"github.com/semistrict/ratel/pkg/base"
	"github.com/semistrict/ratel/pkg/kv/kvserver"
	"github.com/semistrict/ratel/pkg/roachpb"
	"github.com/semistrict/ratel/pkg/server"
	"github.com/semistrict/ratel/pkg/settings/cluster"
	"github.com/semistrict/ratel/pkg/sql/contention"
	"github.com/semistrict/ratel/pkg/testutils/testcluster"
)

var nextClusterAddrBase atomic.Uint64

// Cluster is a TestCluster wrapper that uses in-memory networking
// (via Registry) instead of real TCP. It is designed for use inside
// a synctest bubble where all I/O must be virtualized.
type Cluster struct {
	*testcluster.TestCluster
	Registry       *Registry
	addrs          []string
	rpcListeners   []*Listener
	stickyRegistry server.StickyInMemEnginesRegistry
}

// StartCluster creates and starts a multi-node in-process cluster
// with in-memory networking and storage. All gRPC connections between
// nodes go through the Registry's in-memory transport. HTTP and SQL
// listeners are also in-memory (SQL shares the RPC listener via cmux).
//
// The cluster uses StickyEngineRegistry and ReusableListeners to
// support node restarts.
func StartCluster(t testing.TB, nodes int, extraArgs ...func(*base.TestClusterArgs)) *Cluster {
	registry := NewRegistry()
	stickyRegistry := server.NewStickyInMemEnginesRegistry()
	clusterBase := nextClusterAddrBase.Add(100)
	if clusterBase == 100 {
		clusterBase = 30000
		nextClusterAddrBase.Store(clusterBase)
	}

	clusterArgs := base.TestClusterArgs{
		ReplicationMode:   base.ReplicationManual,
		ReusableListeners: true,
		ServerArgsPerNode: make(map[int]base.TestServerArgs),
	}

	for _, fn := range extraArgs {
		fn(&clusterArgs)
	}

	if clusterArgs.ServerArgs.Settings == nil {
		clusterArgs.ServerArgs.Settings = cluster.MakeTestingClusterSettings()
	}
	contention.TxnIDResolutionInterval.Override(
		context.Background(), &clusterArgs.ServerArgs.Settings.SV, 0,
	)

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
		args.SQLDialFunc = registry.SQLDialFunc()
		args.Listener = rpcListener
		args.StoreSpecs = []base.StoreSpec{
			{InMemory: true, StickyInMemoryEngineID: fmt.Sprintf("inproc-%d", i)},
		}
		args.Locality = roachpb.Locality{
			Tiers: []roachpb.Tier{
				{Key: "region", Value: "test"},
				{Key: "dc", Value: fmt.Sprintf("dc%d", i+1)},
			},
		}

		// Disable range log writes to avoid a deadlock where the SQL
		// INSERT INTO system.rangelog inside ChangeReplicas/Split
		// transactions hangs waiting for its own Raft proposal.
		storeKnobs := args.Knobs.Store
		if storeKnobs == nil {
			storeKnobs = &kvserver.StoreTestingKnobs{}
		}
		storeTestingKnobs := storeKnobs.(*kvserver.StoreTestingKnobs)
		storeTestingKnobs.DisableRangeLogWrite = true
		storeTestingKnobs.DisablePeriodicGossips = true
		storeTestingKnobs.DisableRangefeedUpdater = true
		storeTestingKnobs.DisableStoreRebalancer = true
		storeTestingKnobs.DisableScanner = true
		args.Knobs.Store = storeKnobs

		serverKnobs := args.Knobs.Server
		if serverKnobs == nil {
			serverKnobs = &server.TestingKnobs{}
		}
		tk := serverKnobs.(*server.TestingKnobs)
		tk.RPCListener = rpcListener
		tk.HTTPListener = httpListener
		tk.ShareRPCListenSQL = true
		tk.StickyEngineRegistry = stickyRegistry
		tk.DisableAuthSessionPurge = true
		tk.DisableNodeStatusWrite = true
		tk.DisableEnvironmentSample = true
		tk.DisableReplicationReporter = true
		tk.DisableProtectedTSProvider = true
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
	// Re-open the in-memory RPC listener so the restarted node can
	// accept connections on the same address.
	c.rpcListeners[nodeIdx].Reset()
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

// Stop stops the cluster and closes the registry.
func (c *Cluster) Stop() {
	for i := range c.Conns {
		if c.Conns[i] != nil {
			_ = c.Conns[i].Close()
			c.Conns[i] = nil
		}
	}
	c.TestCluster.StopServers(context.Background())
	c.Stopper().Stop(context.Background())
	c.stickyRegistry.CloseAllStickyInMemEngines()
	c.Registry.Close()
}
