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

package inproc

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegistryBlockLink(t *testing.T) {
	r := NewRegistry()
	defer r.Close()

	l := r.Register("n2")
	ctx := context.Background()

	acceptOne := func() <-chan net.Conn {
		ch := make(chan net.Conn, 1)
		go func() {
			conn, err := l.Accept()
			require.NoError(t, err)
			ch <- conn
		}()
		return ch
	}

	serverACh := acceptOne()
	aToB, err := r.DialFrom(ctx, "n0", "n2")
	require.NoError(t, err)
	defer aToB.Close()
	serverA := <-serverACh
	defer serverA.Close()

	serverCCh := acceptOne()
	cToB, err := r.DialFrom(ctx, "n1", "n2")
	require.NoError(t, err)
	defer cToB.Close()
	serverC := <-serverCCh
	defer serverC.Close()

	r.BlockLink("n0", "n2")

	_, err = r.DialFrom(ctx, "n0", "n2")
	require.Error(t, err)

	serverStillOpenCh := acceptOne()
	stillOpen, err := r.DialFrom(ctx, "n1", "n2")
	require.NoError(t, err)
	defer stillOpen.Close()
	serverStillOpen := <-serverStillOpenCh
	defer serverStillOpen.Close()

	_, err = aToB.Write([]byte("x"))
	require.Error(t, err)

	r.UnblockLink("n0", "n2")

	serverHealedCh := acceptOne()
	healed, err := r.DialFrom(ctx, "n0", "n2")
	require.NoError(t, err)
	defer healed.Close()
	serverHealed := <-serverHealedCh
	defer serverHealed.Close()
}
