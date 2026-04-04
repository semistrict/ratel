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
}

// NewRegistry creates a new in-memory network registry.
func NewRegistry() *Registry {
	return &Registry{
		listeners: make(map[string]*Listener),
		blocked:   make(map[string]bool),
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
	r.mu.Lock()
	if r.blocked[addr] {
		r.mu.Unlock()
		return nil, errors.Newf("connection to %q refused: node is partitioned", addr)
	}
	l, ok := r.listeners[addr]
	r.mu.Unlock()
	if !ok {
		return nil, errors.Newf("no listener registered for %q", addr)
	}
	conn, err := l.Dial(ctx)
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

// Block prevents new connections to the given address, simulating a
// network partition. Existing connections are not affected.
func (r *Registry) Block(addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.blocked[addr] = true
}

// Unblock re-allows connections to the given address, healing a
// simulated partition.
func (r *Registry) Unblock(addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.blocked, addr)
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
	r.mu.Unlock()
	for _, l := range listeners {
		l.Close()
	}
}
