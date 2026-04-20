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
	"net"
	"sync"
)

// Listener is an in-memory net.Listener that uses net.Pipe() to create
// paired connections. It is suitable for use inside a synctest bubble
// where real TCP is not permitted.
//
// The listener supports being closed and re-opened via Reopen(), which
// is needed for node restarts in test clusters.
type Listener struct {
	addr memAddr

	mu     sync.Mutex
	ch     chan net.Conn
	closed chan struct{}
}

// NewListener creates a new in-memory listener with the given address string.
func NewListener(addr string) *Listener {
	return &Listener{
		addr:   memAddr(addr),
		ch:     make(chan net.Conn),
		closed: make(chan struct{}),
	}
}

// Accept waits for and returns the next connection to the listener.
func (l *Listener) Accept() (net.Conn, error) {
	l.mu.Lock()
	ch := l.ch
	closed := l.closed
	l.mu.Unlock()

	select {
	case conn := <-ch:
		return conn, nil
	case <-closed:
		return nil, net.ErrClosed
	}
}

// Close closes the listener. It can be re-opened via Reopen().
func (l *Listener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	select {
	case <-l.closed:
		// Already closed.
	default:
		close(l.closed)
	}
	return nil
}

// Reopen re-opens a closed listener so it can accept connections again.
// This is used when restarting a node in a test cluster.
func (l *Listener) Reopen() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ch = make(chan net.Conn)
	l.closed = make(chan struct{})
	return nil
}

// Addr returns the listener's network address.
func (l *Listener) Addr() net.Addr {
	return l.addr
}

// Dial creates a new in-memory connection to this listener. The server
// side is delivered via Accept(). Returns an error if the listener is
// closed or the context is canceled.
func (l *Listener) Dial(ctx context.Context) (net.Conn, error) {
	l.mu.Lock()
	ch := l.ch
	closed := l.closed
	l.mu.Unlock()

	server, client := net.Pipe()
	select {
	case ch <- server:
		return client, nil
	case <-closed:
		_ = server.Close()
		_ = client.Close()
		return nil, net.ErrClosed
	case <-ctx.Done():
		_ = server.Close()
		_ = client.Close()
		return nil, ctx.Err()
	}
}

// memAddr implements net.Addr for in-memory addresses.
type memAddr string

func (a memAddr) Network() string { return "inproc" }
func (a memAddr) String() string  { return string(a) }
