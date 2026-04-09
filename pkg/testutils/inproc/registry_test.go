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
	"math/rand"
	"net"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"
)

func TestRegistryBlockLink(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := NewRegistry()
		defer r.Close()

		l := r.Register("n2")
		ctx := context.Background()
		var acceptWG sync.WaitGroup

		acceptOne := func() <-chan net.Conn {
			ch := make(chan net.Conn, 1)
			acceptWG.Add(1)
			go func() {
				defer acceptWG.Done()
				conn, err := l.Accept()
				require.NoError(t, err)
				ch <- conn
			}()
			return ch
		}
		defer func() {
			r.Close()
			synctest.Wait()
			acceptWG.Wait()
		}()

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
	})
}

func TestRegistryPartitionGroups(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := NewRegistry()
		defer r.Close()

		r.Register("n0")
		l1 := r.Register("n1")
		l2 := r.Register("n2")
		ctx := context.Background()

		acceptOne := func(l *Listener) <-chan net.Conn {
			ch := make(chan net.Conn, 1)
			go func() {
				conn, err := l.Accept()
				require.NoError(t, err)
				ch <- conn
			}()
			return ch
		}

		serverCrossCh := acceptOne(l2)
		crossConn, err := r.DialFrom(ctx, "n0", "n2")
		require.NoError(t, err)
		defer crossConn.Close()
		serverCross := <-serverCrossCh
		defer serverCross.Close()

		r.PartitionGroups([][]string{{"n0", "n1"}, {"n2"}})

		serverIntraCh := acceptOne(l1)
		intraConn, err := r.DialFrom(ctx, "n0", "n1")
		require.NoError(t, err)
		defer intraConn.Close()
		serverIntra := <-serverIntraCh
		defer serverIntra.Close()

		_, err = r.DialFrom(ctx, "n0", "n2")
		require.Error(t, err)
		_, err = r.DialFrom(ctx, "n2", "n0")
		require.Error(t, err)
		_, err = crossConn.Write([]byte("x"))
		require.Error(t, err)

		r.HealAllLinks()

		serverHealedCh := acceptOne(l2)
		healedConn, err := r.DialFrom(ctx, "n0", "n2")
		require.NoError(t, err)
		defer healedConn.Close()
		serverHealed := <-serverHealedCh
		defer serverHealed.Close()
	})
}

func TestRegistryPartitionGrudgeClosesSpecifiedLinks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := NewRegistry()
		defer r.Close()

		l1 := r.Register("n1")
		l2 := r.Register("n2")
		ctx := context.Background()

		acceptOne := func(l *Listener) <-chan net.Conn {
			ch := make(chan net.Conn, 1)
			go func() {
				conn, err := l.Accept()
				require.NoError(t, err)
				ch <- conn
			}()
			return ch
		}

		server12Ch := acceptOne(l2)
		conn12, err := r.DialFrom(ctx, "n1", "n2")
		require.NoError(t, err)
		defer conn12.Close()
		server12 := <-server12Ch
		defer server12.Close()

		server01Ch := acceptOne(l1)
		conn01, err := r.DialFrom(ctx, "n0", "n1")
		require.NoError(t, err)
		defer conn01.Close()
		server01 := <-server01Ch
		defer server01.Close()

		server02Ch := acceptOne(l2)
		conn02, err := r.DialFrom(ctx, "n0", "n2")
		require.NoError(t, err)
		defer conn02.Close()
		server02 := <-server02Ch
		defer server02.Close()

		r.PartitionGrudge(map[string][]string{
			"n0": {"n1", "n2"},
		})

		_, err = conn01.Write([]byte("x"))
		require.Error(t, err)
		_, err = conn02.Write([]byte("x"))
		require.Error(t, err)

		readCh := make(chan error, 1)
		buf := make([]byte, 1)
		go func() {
			_, err := server12.Read(buf)
			readCh <- err
		}()
		_, err = conn12.Write([]byte("x"))
		require.NoError(t, err)
		require.NoError(t, <-readCh)
		require.Equal(t, byte('x'), buf[0])
	})
}

func TestRegistryHealAllLinksPreservesBlockedNodes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := NewRegistry()
		defer r.Close()

		l1 := r.Register("n1")
		r.Register("n2")
		ctx := context.Background()

		server01Ch := make(chan net.Conn, 1)
		go func() {
			conn, err := l1.Accept()
			require.NoError(t, err)
			server01Ch <- conn
		}()

		conn01, err := r.DialFrom(ctx, "n0", "n1")
		require.NoError(t, err)
		defer conn01.Close()
		server01 := <-server01Ch
		defer server01.Close()

		r.Block("n2")
		r.BlockLink("n0", "n1")

		_, err = conn01.Write([]byte("x"))
		require.Error(t, err)
		_, err = r.DialFrom(ctx, "n0", "n2")
		require.Error(t, err)

		r.HealAllLinks()

		serverHealedCh := make(chan net.Conn, 1)
		go func() {
			conn, err := l1.Accept()
			require.NoError(t, err)
			serverHealedCh <- conn
		}()

		healed, err := r.DialFrom(ctx, "n0", "n1")
		require.NoError(t, err)
		defer healed.Close()
		serverHealed := <-serverHealedCh
		defer serverHealed.Close()

		_, err = r.DialFrom(ctx, "n0", "n2")
		require.Error(t, err)
	})
}

func TestRandomHalvesGrudge(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	grudge := RandomHalvesGrudge([]string{"n0", "n1", "n2", "n3"}, rng)
	require.Len(t, grudge, 4)
	for node, blocked := range grudge {
		require.Len(t, blocked, 2, node)
	}
}

func TestMajoritiesRingGrudge(t *testing.T) {
	grudge := MajoritiesRingGrudge([]string{"n0", "n1", "n2", "n3", "n4"})
	require.ElementsMatch(t, []string{"n0", "n4"}, grudge["n2"])
	require.ElementsMatch(t, []string{"n0", "n1"}, grudge["n3"])
	require.ElementsMatch(t, []string{"n1", "n2"}, grudge["n4"])
	require.ElementsMatch(t, []string{"n2", "n3"}, grudge["n0"])
	require.ElementsMatch(t, []string{"n3", "n4"}, grudge["n1"])
}
