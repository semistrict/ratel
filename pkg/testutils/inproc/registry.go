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
	"net"
	"sync"

	"github.com/cockroachdb/errors"
)

// Registry is an in-memory network that maps addresses to Listeners.
// It provides a Dial function that routes connections through the
// appropriate in-memory listener, and supports partitioning nodes
// by blocking their addresses.
type Registry struct {
	mu        sync.Mutex
	listeners map[string]*Listener
	blocked   map[string]bool
	links     map[string]map[string]bool
}

// NewRegistry creates a new in-memory network registry.
func NewRegistry() *Registry {
	return &Registry{
		listeners: make(map[string]*Listener),
		blocked:   make(map[string]bool),
		links:     make(map[string]map[string]bool),
	}
}

// Register creates and registers a new in-memory listener for the given
// address. Panics if the address is already registered.
func (r *Registry) Register(addr string) *Listener {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.listeners[addr]; ok {
		panic(errors.Newf("address %q already registered", addr))
	}
	l := NewListener(addr)
	r.listeners[addr] = l
	return l
}

// Dial connects to the listener at the given address. Returns an error
// if the address is not registered, is blocked (partitioned), or the
// context is canceled.
func (r *Registry) Dial(ctx context.Context, addr string) (net.Conn, error) {
	return r.DialFrom(ctx, "", addr)
}

// DialFrom connects to the listener at the given address on behalf of a
// logical source address.
func (r *Registry) DialFrom(ctx context.Context, source, addr string) (net.Conn, error) {
	r.mu.Lock()
	if r.blocked[addr] || r.links[source][addr] {
		r.mu.Unlock()
		return nil, errors.Newf("connection to %q refused: node is partitioned", addr)
	}
	l, ok := r.listeners[addr]
	r.mu.Unlock()
	if !ok {
		return nil, errors.Newf("no listener registered for %q", addr)
	}
	conn, err := l.DialWithSource(ctx, source)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// DialerFunc returns a function compatible with rpc.ContextTestingKnobs.DialerFunc
// that routes all gRPC connections through this registry.
func (r *Registry) DialerFunc() func(ctx context.Context, addr string) (net.Conn, error) {
	return r.Dial
}

// DialerFuncFor returns a dialer that records a logical source address for
// directed-link partitioning.
func (r *Registry) DialerFuncFor(source string) func(ctx context.Context, addr string) (net.Conn, error) {
	return func(ctx context.Context, addr string) (net.Conn, error) {
		return r.DialFrom(ctx, source, addr)
	}
}

// SQLDialFunc returns a function compatible with base.TestServerArgs.SQLDialFunc
// that routes SQL (pgwire) connections through this registry.
func (r *Registry) SQLDialFunc() func(network, addr string) (net.Conn, error) {
	return func(network, addr string) (net.Conn, error) {
		return r.Dial(context.Background(), addr)
	}
}

// Block prevents new connections to the given address, simulating a
// network partition. Existing connections are not affected.
func (r *Registry) Block(addr string) {
	r.mu.Lock()
	l := r.listeners[addr]
	defer r.mu.Unlock()
	r.blocked[addr] = true
	if l != nil {
		l.CloseActiveConns()
	}
}

// Unblock re-allows connections to the given address, healing a
// simulated partition.
func (r *Registry) Unblock(addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.blocked, addr)
}

// BlockLink prevents new connections from source to addr and tears down any
// existing directed connections on that link.
func (r *Registry) BlockLink(source, addr string) {
	r.mu.Lock()
	l := r.listeners[addr]
	if r.links[source] == nil {
		r.links[source] = make(map[string]bool)
	}
	r.links[source][addr] = true
	r.mu.Unlock()
	if l != nil {
		l.CloseActiveConnsFrom(source)
	}
}

// UnblockLink heals a previously blocked directed link.
func (r *Registry) UnblockLink(source, addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if links := r.links[source]; links != nil {
		delete(links, addr)
		if len(links) == 0 {
			delete(r.links, source)
		}
	}
}

// Unregister removes a listener from the registry and closes it.
func (r *Registry) Unregister(addr string) {
	r.mu.Lock()
	l, ok := r.listeners[addr]
	if ok {
		delete(r.listeners, addr)
	}
	r.mu.Unlock()
	if ok {
		l.Close()
	}
}

// Close closes all listeners and clears the registry.
func (r *Registry) Close() {
	r.mu.Lock()
	listeners := r.listeners
	r.listeners = make(map[string]*Listener)
	r.blocked = make(map[string]bool)
	r.links = make(map[string]map[string]bool)
	r.mu.Unlock()
	for _, l := range listeners {
		l.Close()
	}
}
