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
)

// Listener is an in-memory net.Listener that uses net.Pipe() to create
// paired connections. It is suitable for use inside a synctest bubble
// where real TCP is not permitted.
type Listener struct {
	addr   memAddr
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
	select {
	case conn := <-l.ch:
		return conn, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

// Close closes the listener.
func (l *Listener) Close() error {
	select {
	case <-l.closed:
		// Already closed.
	default:
		close(l.closed)
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
	server, client := net.Pipe()
	select {
	case l.ch <- server:
		return client, nil
	case <-l.closed:
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
