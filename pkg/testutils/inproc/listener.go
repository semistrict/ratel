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
// The listener tracks active connection pairs so they can be closed when the
// listener closes or when directed network partitions cut a logical source.
// It also supports being closed and re-opened via Reopen(), which is needed
// for node restarts in test clusters.
type Listener struct {
	addr memAddr

	mu     sync.Mutex
	ch     chan net.Conn
	closed chan struct{}
	pairs  map[*connPair]struct{}
}

type connPair struct {
	listener *Listener
	source   string
	server   net.Conn
	client   net.Conn
	once     sync.Once
}

// NewListener creates a new in-memory listener with the given address string.
func NewListener(addr string) *Listener {
	return &Listener{
		addr:   memAddr(addr),
		ch:     make(chan net.Conn),
		closed: make(chan struct{}),
		pairs:  make(map[*connPair]struct{}),
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

// Close closes the listener and all active connections. Closing the
// connections unblocks any gRPC goroutines reading/writing on the
// pipes, which is necessary for synctest to detect quiescence. It can
// be re-opened via Reopen().
func (l *Listener) Close() error {
	l.mu.Lock()
	pairs := l.snapshotPairsLocked()
	select {
	case <-l.closed:
		// Already closed.
	default:
		close(l.closed)
	}
	l.mu.Unlock()
	for _, pair := range pairs {
		pair.close()
	}
	return nil
}

// Reopen re-opens a closed listener so it can accept connections again.
// This is used when restarting a node in a test cluster.
func (l *Listener) Reopen() error {
	l.mu.Lock()
	pairs := l.snapshotPairsLocked()
	select {
	case <-l.closed:
	default:
		close(l.closed)
	}
	l.ch = make(chan net.Conn)
	l.closed = make(chan struct{})
	l.pairs = make(map[*connPair]struct{})
	l.mu.Unlock()
	for _, pair := range pairs {
		pair.close()
	}
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
	return l.DialWithSource(ctx, "")
}

// DialWithSource is like Dial but records the logical source address so the
// registry can later cut only specific directed links.
func (l *Listener) DialWithSource(ctx context.Context, source string) (net.Conn, error) {
	l.mu.Lock()
	ch := l.ch
	closed := l.closed
	l.mu.Unlock()

	server, client := net.Pipe()
	pair := &connPair{listener: l, source: source, server: server, client: client}
	wrappedServer := &trackedConn{Conn: server, pair: pair}
	wrappedClient := &trackedConn{Conn: client, pair: pair}

	l.mu.Lock()
	select {
	case <-closed:
		l.mu.Unlock()
		pair.close()
		return nil, net.ErrClosed
	default:
		l.pairs[pair] = struct{}{}
	}
	l.mu.Unlock()

	select {
	case ch <- wrappedServer:
		return wrappedClient, nil
	case <-closed:
		pair.close()
		return nil, net.ErrClosed
	case <-ctx.Done():
		pair.close()
		return nil, ctx.Err()
	}
}

func (l *Listener) CloseActiveConns() {
	l.mu.Lock()
	pairs := l.snapshotPairsLocked()
	l.mu.Unlock()
	for _, pair := range pairs {
		pair.close()
	}
}

func (l *Listener) CloseActiveConnsFrom(source string) {
	l.mu.Lock()
	pairs := l.snapshotPairsLocked()
	l.mu.Unlock()
	for _, pair := range pairs {
		if pair.source == source {
			pair.close()
		}
	}
}

func (l *Listener) snapshotPairsLocked() []*connPair {
	pairs := make([]*connPair, 0, len(l.pairs))
	for pair := range l.pairs {
		pairs = append(pairs, pair)
	}
	return pairs
}

func (l *Listener) removePair(pair *connPair) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.pairs, pair)
}

func (p *connPair) close() {
	p.once.Do(func() {
		_ = p.server.Close()
		_ = p.client.Close()
		p.listener.removePair(p)
	})
}

type trackedConn struct {
	net.Conn
	pair *connPair
}

func (c *trackedConn) Close() error {
	c.pair.close()
	return nil
}

// memAddr implements net.Addr for in-memory addresses.
type memAddr string

func (a memAddr) Network() string { return "inproc" }
func (a memAddr) String() string  { return string(a) }
