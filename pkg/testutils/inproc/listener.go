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
)

// Listener is an in-memory net.Listener that uses net.Pipe() to create
// paired connections. It is suitable for use inside a synctest bubble
// where real TCP is not permitted.
//
// The listener supports being closed and re-opened via Reset(), which
// is needed for node restarts in test clusters.
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

// Close closes the listener. It can be re-opened via Reset().
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

// Reset re-opens a closed listener so it can accept connections again.
// This is used when restarting a node in a test cluster.
func (l *Listener) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ch = make(chan net.Conn)
	l.closed = make(chan struct{})
	l.pairs = make(map[*connPair]struct{})
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
