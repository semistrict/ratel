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

package server

import (
	"hash/fnv"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"

	"github.com/cockroachdb/cockroach/pkg/gossip"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/liveness"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/rpc"
	"github.com/cockroachdb/cockroach/pkg/util"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
)

// workerRouter routes worker HTTP requests to the correct node in a
// multi-node cluster. For workers with Durable Objects, all requests
// are consistently routed to the same node (via rendezvous hashing on
// worker name) to ensure single-writer semantics for ActorCache.
// Stateless workers are served locally.
type workerRouter struct {
	localNodeID roachpb.NodeID
	gossip      *gossip.Gossip
	nl          *liveness.NodeLiveness
	rpcCtx      *rpc.Context

	mu struct {
		syncutil.RWMutex
		proxies map[util.UnresolvedAddr]*httputil.ReverseProxy
	}
}

func newWorkerRouter(
	localNodeID roachpb.NodeID,
	g *gossip.Gossip,
	nl *liveness.NodeLiveness,
	rpcCtx *rpc.Context,
) *workerRouter {
	r := &workerRouter{
		localNodeID: localNodeID,
		gossip:      g,
		nl:          nl,
		rpcCtx:      rpcCtx,
	}
	r.mu.proxies = make(map[util.UnresolvedAddr]*httputil.ReverseProxy)
	return r
}

// routeRequest checks whether a worker request should be forwarded to
// another node. Returns true if the request was forwarded. Returns false
// if the request should be handled locally.
//
// The X-Ratel-Forwarded header prevents forwarding loops: if set, the
// request is always handled locally.
func (wr *workerRouter) routeRequest(
	w http.ResponseWriter, r *http.Request, workerName string, hasDOs bool,
) bool {
	// Already forwarded — serve locally to break loops.
	if r.Header.Get("X-Ratel-Forwarded") != "" {
		return false
	}

	// Stateless workers are served locally.
	if !hasDOs {
		return false
	}

	target := wr.pickNode(workerName)
	if target == wr.localNodeID {
		return false
	}

	addr, err := wr.gossip.GetNodeIDHTTPAddress(target)
	if err != nil {
		log.Warningf(r.Context(), "cannot resolve HTTP address for n%d: %v", target, err)
		return false
	}

	proxy := wr.getOrCreateProxy(*addr)
	r.Header.Set("X-Ratel-Forwarded", "1")
	proxy.ServeHTTP(w, r)
	return true
}

// pickNode uses rendezvous hashing to consistently map a worker name
// to a live node. The selected node changes only when nodes join or leave.
func (wr *workerRouter) pickNode(workerName string) roachpb.NodeID {
	liveMap := wr.nl.GetIsLiveMap()

	nodes := make([]roachpb.NodeID, 0, len(liveMap))
	for id, entry := range liveMap {
		if entry.IsLive {
			nodes = append(nodes, id)
		}
	}
	if len(nodes) == 0 {
		return wr.localNodeID
	}

	// Sort for determinism.
	sort.Slice(nodes, func(i, j int) bool { return nodes[i] < nodes[j] })

	// Rendezvous hashing: pick the node with the highest hash for this key.
	var best roachpb.NodeID
	var bestHash uint64
	for _, id := range nodes {
		h := rendezvousHash(workerName, id)
		if h > bestHash || best == 0 {
			bestHash = h
			best = id
		}
	}
	return best
}

func rendezvousHash(key string, nodeID roachpb.NodeID) uint64 {
	h := fnv.New64a()
	h.Write([]byte(key))
	var buf [8]byte
	buf[0] = byte(nodeID >> 24)
	buf[1] = byte(nodeID >> 16)
	buf[2] = byte(nodeID >> 8)
	buf[3] = byte(nodeID)
	h.Write(buf[:4])
	return h.Sum64()
}

func (wr *workerRouter) getOrCreateProxy(addr util.UnresolvedAddr) *httputil.ReverseProxy {
	wr.mu.RLock()
	if p, ok := wr.mu.proxies[addr]; ok {
		wr.mu.RUnlock()
		return p
	}
	wr.mu.RUnlock()

	wr.mu.Lock()
	defer wr.mu.Unlock()

	// Double-check after acquiring write lock.
	if p, ok := wr.mu.proxies[addr]; ok {
		return p
	}

	u := url.URL{
		Scheme: "http",
		Host:   addr.AddressField,
	}
	proxy := httputil.NewSingleHostReverseProxy(&u)

	if wr.rpcCtx != nil {
		if httpClient, err := wr.rpcCtx.GetHTTPClient(); err == nil {
			proxy.Transport = httpClient.Transport
		}
	}

	wr.mu.proxies[addr] = proxy
	return proxy
}
