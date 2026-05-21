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
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"capnproto.org/go/capnp/v3/rpc"
	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/server/actorstorage"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/tracing"
	"github.com/cockroachdb/cockroach/pkg/util/tracing/tracingpb"
)

// benchDOEnv holds the shared state for DO storage benchmarks.
type benchDOEnv struct {
	stage   actorstorage.ActorStorage_Stage
	cleanup func()
}

func setupBenchDOEnv(b *testing.B, kvDB *kv.DB) *benchDOEnv {
	b.Helper()

	serverConn, clientConn := net.Pipe()

	storageServer := &actorstorage.StorageServer{
		DB:    kvDB,
		Codec: keys.SystemSQLCodec,
	}

	srvTransport := rpc.NewStreamTransport(serverConn)
	srvRPC := rpc.NewConn(srvTransport, &rpc.Options{
		BootstrapClient: actorstorage.NewClient(storageServer),
	})

	cliTransport := rpc.NewStreamTransport(clientConn)
	cliRPC := rpc.NewConn(cliTransport, nil)

	ctx := context.Background()
	bootstrap := cliRPC.Bootstrap(ctx)
	client := actorstorage.ActorStorage(bootstrap)

	future, release := client.GetStage(ctx, func(p actorstorage.ActorStorage_getStage_Params) error {
		return p.SetStableId("bench-actor-001")
	})
	results, err := future.Struct()
	if err != nil {
		b.Fatal(err)
	}
	stage := results.Stage().AddRef()
	release()

	return &benchDOEnv{
		stage: stage,
		cleanup: func() {
			stage.Release()
			client.Release()
			cliRPC.Close()
			srvRPC.Close()
		},
	}
}

// BenchmarkDOStoragePut benchmarks writing a single key-value pair through
// capnp RPC to Ratel KV.
func BenchmarkDOStoragePut(b *testing.B) {
	defer leaktest.AfterTest(b)()

	s, _, kvDB := serverutils.StartServer(b, base.TestServerArgs{})
	ctx := context.Background()
	defer s.Stopper().Stop(ctx)

	env := setupBenchDOEnv(b, kvDB)
	defer env.cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Appendf(nil, "bench-key-%d", i)
		val := fmt.Appendf(nil, "bench-val-%d", i)
		future, release := actorstorage.ActorStorage_Operations(env.stage).Put(ctx, func(p actorstorage.ActorStorage_Operations_put_Params) error {
			entries, err := p.NewEntries(1)
			if err != nil {
				return err
			}
			if err := entries.At(0).SetKey(key); err != nil {
				return err
			}
			return entries.At(0).SetValue(val)
		})
		if _, err := future.Struct(); err != nil {
			b.Fatal(err)
		}
		release()
	}
}

// BenchmarkDOStorageGet benchmarks reading a single key through capnp RPC.
// Pre-populates the key before the benchmark loop.
func BenchmarkDOStorageGet(b *testing.B) {
	defer leaktest.AfterTest(b)()

	s, _, kvDB := serverutils.StartServer(b, base.TestServerArgs{})
	ctx := context.Background()
	defer s.Stopper().Stop(ctx)

	env := setupBenchDOEnv(b, kvDB)
	defer env.cleanup()

	// Pre-populate.
	future, release := actorstorage.ActorStorage_Operations(env.stage).Put(ctx, func(p actorstorage.ActorStorage_Operations_put_Params) error {
		entries, err := p.NewEntries(1)
		if err != nil {
			return err
		}
		entries.At(0).SetKey([]byte("bench-key"))
		return entries.At(0).SetValue([]byte("bench-value-data-here"))
	})
	if _, err := future.Struct(); err != nil {
		b.Fatal(err)
	}
	release()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		future, release := actorstorage.ActorStorage_Operations(env.stage).Get(ctx, func(p actorstorage.ActorStorage_Operations_get_Params) error {
			return p.SetKey([]byte("bench-key"))
		})
		result, err := future.Struct()
		if err != nil {
			b.Fatal(err)
		}
		val, err := result.Value()
		if err != nil {
			b.Fatal(err)
		}
		if len(val) == 0 {
			b.Fatal("empty value")
		}
		release()
	}
}

// BenchmarkDOStoragePutGet benchmarks a put followed by a get of the same key.
func BenchmarkDOStoragePutGet(b *testing.B) {
	defer leaktest.AfterTest(b)()

	s, _, kvDB := serverutils.StartServer(b, base.TestServerArgs{})
	ctx := context.Background()
	defer s.Stopper().Stop(ctx)

	env := setupBenchDOEnv(b, kvDB)
	defer env.cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Appendf(nil, "bench-rw-%d", i)
		val := fmt.Appendf(nil, "value-%d", i)

		// Put.
		putFuture, putRelease := actorstorage.ActorStorage_Operations(env.stage).Put(ctx, func(p actorstorage.ActorStorage_Operations_put_Params) error {
			entries, err := p.NewEntries(1)
			if err != nil {
				return err
			}
			entries.At(0).SetKey(key)
			return entries.At(0).SetValue(val)
		})
		if _, err := putFuture.Struct(); err != nil {
			b.Fatal(err)
		}
		putRelease()

		// Get.
		getFuture, getRelease := actorstorage.ActorStorage_Operations(env.stage).Get(ctx, func(p actorstorage.ActorStorage_Operations_get_Params) error {
			return p.SetKey(key)
		})
		result, err := getFuture.Struct()
		if err != nil {
			b.Fatal(err)
		}
		v, err := result.Value()
		if err != nil {
			b.Fatal(err)
		}
		if len(v) == 0 {
			b.Fatal("empty value")
		}
		getRelease()
	}
}

// BenchmarkDOStorageDelete benchmarks deleting a single key.
// Pre-populates keys before the benchmark loop.
func BenchmarkDOStorageDelete(b *testing.B) {
	defer leaktest.AfterTest(b)()

	s, _, kvDB := serverutils.StartServer(b, base.TestServerArgs{})
	ctx := context.Background()
	defer s.Stopper().Stop(ctx)

	env := setupBenchDOEnv(b, kvDB)
	defer env.cleanup()

	// Pre-populate all keys.
	for i := 0; i < b.N; i++ {
		key := fmt.Appendf(nil, "del-key-%d", i)
		future, release := actorstorage.ActorStorage_Operations(env.stage).Put(ctx, func(p actorstorage.ActorStorage_Operations_put_Params) error {
			entries, err := p.NewEntries(1)
			if err != nil {
				return err
			}
			entries.At(0).SetKey(key)
			return entries.At(0).SetValue([]byte("x"))
		})
		if _, err := future.Struct(); err != nil {
			b.Fatal(err)
		}
		release()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Appendf(nil, "del-key-%d", i)
		future, release := actorstorage.ActorStorage_Operations(env.stage).Delete(ctx, func(p actorstorage.ActorStorage_Operations_delete_Params) error {
			keys, err := p.NewKeys(1)
			if err != nil {
				return err
			}
			return keys.Set(0, key)
		})
		if _, err := future.Struct(); err != nil {
			b.Fatal(err)
		}
		release()
	}
}

// BenchmarkDOStorageBatchPut benchmarks writing N entries in a single put call.
func BenchmarkDOStorageBatchPut(b *testing.B) {
	defer leaktest.AfterTest(b)()

	for _, batchSize := range []int{1, 10, 50, 128} {
		b.Run(fmt.Sprintf("batch=%d", batchSize), func(b *testing.B) {
			s, _, kvDB := serverutils.StartServer(b, base.TestServerArgs{})
			ctx := context.Background()
			defer s.Stopper().Stop(ctx)

			env := setupBenchDOEnv(b, kvDB)
			defer env.cleanup()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				future, release := actorstorage.ActorStorage_Operations(env.stage).Put(ctx, func(p actorstorage.ActorStorage_Operations_put_Params) error {
					entries, err := p.NewEntries(int32(batchSize))
					if err != nil {
						return err
					}
					for j := 0; j < batchSize; j++ {
						key := fmt.Appendf(nil, "batch-%d-%d", i, j)
						val := fmt.Appendf(nil, "v-%d-%d", i, j)
						entries.At(j).SetKey(key)
						entries.At(j).SetValue(val)
					}
					return nil
				})
				if _, err := future.Struct(); err != nil {
					b.Fatal(err)
				}
				release()
			}
		})
	}
}

// BenchmarkDOStorageGetStage benchmarks acquiring a Stage capability for a new actor.
func BenchmarkDOStorageGetStage(b *testing.B) {
	defer leaktest.AfterTest(b)()

	s, _, kvDB := serverutils.StartServer(b, base.TestServerArgs{})
	ctx := context.Background()
	defer s.Stopper().Stop(ctx)

	serverConn, clientConn := net.Pipe()

	storageServer := &actorstorage.StorageServer{
		DB:    kvDB,
		Codec: keys.SystemSQLCodec,
	}

	srvTransport := rpc.NewStreamTransport(serverConn)
	srvRPC := rpc.NewConn(srvTransport, &rpc.Options{
		BootstrapClient: actorstorage.NewClient(storageServer),
	})

	cliTransport := rpc.NewStreamTransport(clientConn)
	cliRPC := rpc.NewConn(cliTransport, nil)

	bootstrap := cliRPC.Bootstrap(ctx)
	client := actorstorage.ActorStorage(bootstrap)
	defer func() {
		client.Release()
		cliRPC.Close()
		srvRPC.Close()
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stableId := fmt.Sprintf("actor-%d", i)
		future, release := client.GetStage(ctx, func(p actorstorage.ActorStorage_getStage_Params) error {
			return p.SetStableId(stableId)
		})
		results, err := future.Struct()
		if err != nil {
			b.Fatal(err)
		}
		stage := results.Stage().AddRef()
		release()
		stage.Release()
	}
}

// BenchmarkWorkerDOE2E benchmarks the full end-to-end path:
//
//	HTTP request → cmux → reverse proxy → workerd → JS fetch()
//	→ DO stub → ActorCache → capnp TwoPartyClient → socketpair
//	→ Go capnp server → Ratel KV (Pebble) → response back
//
// The worker increments a counter in a Durable Object on each request.
// Each iteration does one HTTP round-trip that triggers a DO storage
// get + put.
func BenchmarkWorkerDOE2E(b *testing.B) {
	if os.Getenv("RATEL_WORKERD_BIN") == "" {
		if p, err := exec.LookPath("workerd"); err == nil {
			os.Setenv("RATEL_WORKERD_BIN", p)
		} else {
			b.Skip("workerd binary not found (set RATEL_WORKERD_BIN)")
		}
	}
	defer leaktest.AfterTest(b)()

	s, _, _ := serverutils.StartServer(b, base.TestServerArgs{})
	ctx := context.Background()
	defer s.Stopper().Stop(ctx)

	adminClient, err := s.GetAdminHTTPClient()
	if err != nil {
		b.Fatal(err)
	}

	bindings := `{"durable_objects": [{"class_name": "Counter"}]}`
	req, err := http.NewRequest(
		http.MethodPut,
		fmt.Sprintf("%s/api/v2/workers/%s/", s.AdminURL(), "bench-counter"),
		strings.NewReader(testWorkerCounter),
	)
	if err != nil {
		b.Fatal(err)
	}
	req.Header.Set("X-Bindings", bindings)
	resp, err := adminClient.Do(req)
	if err != nil {
		b.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b.Fatalf("deploy failed: %d", resp.StatusCode)
	}

	rpcAddr := s.RPCAddr()
	workerURL := fmt.Sprintf("http://%s/workers/bench-counter/", rpcAddr)

	// Wait for workerd to be ready.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		r, rErr := http.Get(workerURL)
		if rErr == nil && r.StatusCode == http.StatusOK {
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
			break
		}
		if rErr == nil {
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Verify it works before benchmarking.
	r, err := http.Get(workerURL)
	if err != nil {
		b.Fatalf("worker not ready: %v", err)
	}
	body, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if r.StatusCode != http.StatusOK {
		b.Fatalf("worker returned %d: %s", r.StatusCode, body)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r, err := http.Get(workerURL)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, r.Body)
		r.Body.Close()
		if r.StatusCode != http.StatusOK {
			b.Fatalf("iteration %d: status %d", i, r.StatusCode)
		}
	}
}

// TestDOStorageTrace runs a DO put+get through capnp RPC with verbose
// tracing enabled, then prints the span tree so you can see where
// time is spent.
func TestDOStorageTrace(t *testing.T) {
	defer leaktest.AfterTest(t)()

	s, _, kvDB := serverutils.StartServer(t, base.TestServerArgs{})
	ctx := context.Background()
	defer s.Stopper().Stop(ctx)

	tr := s.TracerI().(*tracing.Tracer)

	// Create the root recording span first so we can set it as parent
	// on the storage server — this gives us a single trace tree.
	_, rootSpan := tr.StartSpanCtx(ctx, "do.e2e.put+get",
		tracing.WithRecording(tracingpb.RecordingVerbose),
	)

	serverConn, clientConn := net.Pipe()

	storageServer := &actorstorage.StorageServer{
		DB:         kvDB,
		Codec:      keys.SystemSQLCodec,
		Tracer:     tr,
		ParentSpan: rootSpan,
	}

	srvTransport := rpc.NewStreamTransport(serverConn)
	srvRPC := rpc.NewConn(srvTransport, &rpc.Options{
		BootstrapClient: actorstorage.NewClient(storageServer),
	})

	cliTransport := rpc.NewStreamTransport(clientConn)
	cliRPC := rpc.NewConn(cliTransport, nil)

	bootstrap := cliRPC.Bootstrap(ctx)
	client := actorstorage.ActorStorage(bootstrap)
	defer func() {
		client.Release()
		cliRPC.Close()
		srvRPC.Close()
	}()

	// Get a stage.
	future, release := client.GetStage(ctx, func(p actorstorage.ActorStorage_getStage_Params) error {
		return p.SetStableId("trace-actor")
	})
	results, err := future.Struct()
	if err != nil {
		t.Fatal(err)
	}
	stage := results.Stage().AddRef()
	release()
	defer stage.Release()

	// Warm up: one put+get to prime caches.
	warmPut, warmPutR := actorstorage.ActorStorage_Operations(stage).Put(ctx, func(p actorstorage.ActorStorage_Operations_put_Params) error {
		entries, _ := p.NewEntries(1)
		entries.At(0).SetKey([]byte("warmup"))
		entries.At(0).SetValue([]byte("data"))
		return nil
	})
	warmPut.Struct()
	warmPutR()

	warmGet, warmGetR := actorstorage.ActorStorage_Operations(stage).Get(ctx, func(p actorstorage.ActorStorage_Operations_get_Params) error {
		return p.SetKey([]byte("warmup"))
	})
	warmGet.Struct()
	warmGetR()

	// Traced put.
	rootSpan.Record("begin put")
	putFuture, putRelease := actorstorage.ActorStorage_Operations(stage).Put(ctx, func(p actorstorage.ActorStorage_Operations_put_Params) error {
		entries, _ := p.NewEntries(1)
		entries.At(0).SetKey([]byte("traced-key"))
		entries.At(0).SetValue([]byte("traced-value"))
		return nil
	})
	if _, err := putFuture.Struct(); err != nil {
		t.Fatal(err)
	}
	putRelease()
	rootSpan.Record("put complete")

	// Traced get.
	rootSpan.Record("begin get")
	getFuture, getRelease := actorstorage.ActorStorage_Operations(stage).Get(ctx, func(p actorstorage.ActorStorage_Operations_get_Params) error {
		return p.SetKey([]byte("traced-key"))
	})
	getResult, err := getFuture.Struct()
	if err != nil {
		t.Fatal(err)
	}
	val, _ := getResult.Value()
	if string(append([]byte(nil), val...)) != "traced-value" {
		t.Fatalf("expected traced-value, got %q", val)
	}
	getRelease()
	rootSpan.Record("get complete")

	rec := rootSpan.FinishAndGetRecording(tracingpb.RecordingVerbose)
	t.Logf("Trace of DO put+get through capnp RPC:\n%s", rec.String())
}
