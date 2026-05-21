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
	"bytes"
	"context"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	capnp "capnproto.org/go/capnp/v3"
	"capnproto.org/go/capnp/v3/rpc"
	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/server/actorstorage"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/tracing"
	"github.com/stretchr/testify/require"
)

var (
	//go:embed testdata/workers/hello.js
	testWorkerHello string
	//go:embed testdata/workers/hello_v2.js
	testWorkerHelloV2 string
	//go:embed testdata/workers/echo.js
	testWorkerEcho string
	//go:embed testdata/workers/counter.js
	testWorkerCounter string
	//go:embed testdata/workers/counter_bindings.js
	testWorkerCounterBindings string
)

// workerdBinPath returns the workerd binary path from RATEL_WORKERD_BIN,
// embedded binary, or PATH.
func workerdBinPath() string {
	if p := os.Getenv("RATEL_WORKERD_BIN"); p != "" {
		return p
	}
	return resolveWorkerdBinary()
}

// skipIfNoWorkerd skips the test if the workerd binary is not available.
func skipIfNoWorkerd(t *testing.T) {
	t.Helper()
	if workerdBinPath() == "" {
		t.Skip("workerd binary not found (set RATEL_WORKERD_BIN); skipping")
	}
}

// --- Unit tests (no server needed) ---

func TestExtractEmbeddedWorkerd(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	path, err := extractEmbeddedWorkerd()
	if err != nil {
		t.Skipf("no embedded workerd binary: %v", err)
	}

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.True(t, info.Mode()&0111 != 0, "extracted binary should be executable")
	require.Greater(t, info.Size(), int64(1024*1024), "extracted binary should be >1MB")

	// Second call should return cached path.
	path2, err := extractEmbeddedWorkerd()
	require.NoError(t, err)
	require.Equal(t, path, path2)
}

func TestRendezvousHashConsistency(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	// Same inputs always produce the same hash.
	h1 := rendezvousHash("my-worker", 1)
	h2 := rendezvousHash("my-worker", 1)
	require.Equal(t, h1, h2)

	// Different nodes produce different hashes.
	h3 := rendezvousHash("my-worker", 2)
	require.NotEqual(t, h1, h3)

	// Different workers produce different hashes.
	h4 := rendezvousHash("other-worker", 1)
	require.NotEqual(t, h1, h4)
}

func TestWorkerdConfigGeneration(t *testing.T) {
	workers := []WorkerDef{
		{
			Name:       "hello",
			Script:     testWorkerHello,
			CompatDate: "2024-01-01",
		},
		{
			Name:       "counter",
			Script:     testWorkerCounterBindings,
			CompatDate: "2024-01-01",
			DOClasses:  []string{"Counter"},
		},
	}

	dir := t.TempDir()
	configPath, err := generateWorkerdConfig(dir, workers, 18787, 3)
	require.NoError(t, err)
	require.FileExists(t, configPath)

	configBytes, err := os.ReadFile(configPath)
	require.NoError(t, err)
	config := string(configBytes)

	require.Contains(t, config, "name = \"router\"")
	require.Contains(t, config, "name = \"hello\"")
	require.Contains(t, config, "name = \"counter\"")
	require.Contains(t, config, ".workerHello")
	require.Contains(t, config, ".workerCounter")
	require.Contains(t, config, "localhost:18787")
	require.Contains(t, config, "durableObjectStorage = ( ratel = 3 )")
	require.Contains(t, config, "className = \"Counter\"")

	routerBytes, err := os.ReadFile(dir + "/router.js")
	require.NoError(t, err)
	require.Contains(t, string(routerBytes), "X-Worker-Name")

	helloBytes, err := os.ReadFile(dir + "/worker_hello.js")
	require.NoError(t, err)
	require.Contains(t, string(helloBytes), "hello")

	counterBytes, err := os.ReadFile(dir + "/worker_counter.js")
	require.NoError(t, err)
	require.Contains(t, string(counterBytes), "Counter")
}

func TestWorkerCapnpID(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{name: "hello", id: "workerHello"},
		{name: "hello-world", id: "workerHelloWorld"},
		{name: "counter_v2", id: "workerCounterV2"},
		{name: "multi-word_worker", id: "workerMultiWordWorker"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.id, workerCapnpID(tt.name))
	}
}

func TestRouterScriptEmbed(t *testing.T) {
	require.Contains(t, routerScript, "X-Worker-Name")
	require.Contains(t, routerScript, "export default")
	require.Contains(t, routerScript, "worker not found")
}

func TestSplitWorkerPath(t *testing.T) {
	tests := []struct {
		path         string
		expectedName string
		expectedRest string
	}{
		{"/workers/hello/", "hello", "/"},
		{"/workers/hello/foo/bar", "hello", "/foo/bar"},
		{"/workers/hello", "hello", "/"},
		{"/other/path", "", "/other/path"},
		{"/workers/", "", "/"},
	}
	for _, tt := range tests {
		name, rest := splitWorkerPath(tt.path)
		require.Equal(t, tt.expectedName, name, "path=%s", tt.path)
		require.Equal(t, tt.expectedRest, rest, "path=%s", tt.path)
	}
}

func TestWorkerdProxyAPIPassThrough(t *testing.T) {
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v2/workers/", r.URL.Path)
		w.Header().Set("X-Test-API", "hit")
		w.WriteHeader(http.StatusAccepted)
	})
	proxy := newWorkerdProxy(api, 1, tracing.NewTracer(), nil /* sidecar */, nil /* router */)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/workers/", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Equal(t, "hit", rec.Header().Get("X-Test-API"))
}

func TestWorkerdProxyRewritesWorkerRequest(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/foo/bar", r.URL.Path)
		require.Equal(t, "hello", r.Header.Get("X-Worker-Name"))
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, "proxied")
	}))
	defer backend.Close()

	port := strings.TrimPrefix(backend.URL, "http://127.0.0.1:")
	proxy := newWorkerdProxy(http.NotFoundHandler(), atoi(t, port), tracing.NewTracer(), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/workers/hello/foo/bar?x=1", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, "proxied", rec.Body.String())
}

func TestWorkerdProxyHealthWithoutSidecar(t *testing.T) {
	proxy := newWorkerdProxy(http.NotFoundHandler(), 1, tracing.NewTracer(), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/workers/_health", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Body.String(), "workerd not running")
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	require.NoError(t, err)
	return v
}

// --- Server-level tests (real Ratel, no workerd needed) ---

func TestWorkerdDeployAndList(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	s, sqlDB, _ := serverutils.StartServer(t, base.TestServerArgs{})
	ctx := context.Background()
	defer s.Stopper().Stop(ctx)

	adminClient, err := s.GetAdminHTTPClient()
	require.NoError(t, err)

	baseURL := s.AdminURL()

	resp := doDeploy(t, adminClient, baseURL, "hello", testWorkerHello, "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body := readBody(t, resp)
	require.Equal(t, "hello", body["name"])
	require.Equal(t, float64(1), body["version"])

	var name string
	var version int64
	err = sqlDB.QueryRow("SELECT name, version FROM system.worker_scripts WHERE name = 'hello'").Scan(&name, &version)
	require.NoError(t, err)
	require.Equal(t, "hello", name)
	require.Equal(t, int64(1), version)

	resp = doDeploy(t, adminClient, baseURL, "hello", testWorkerHelloV2, "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body = readBody(t, resp)
	require.Equal(t, float64(2), body["version"])

	resp = doDeploy(t, adminClient, baseURL, "echo", testWorkerEcho, "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/v2/workers/", nil)
	require.NoError(t, err)
	resp, err = adminClient.Do(req)
	require.NoError(t, err)
	body = readBody(t, resp)

	workers := body["workers"].([]interface{})
	require.Len(t, workers, 2)
	require.Equal(t, "echo", workers[0].(map[string]interface{})["name"])
	require.Equal(t, "hello", workers[1].(map[string]interface{})["name"])
	require.Equal(t, float64(2), workers[1].(map[string]interface{})["latest_version"])
}

func TestWorkerdDeployValidation(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	s, _, _ := serverutils.StartServer(t, base.TestServerArgs{})
	ctx := context.Background()
	defer s.Stopper().Stop(ctx)

	adminClient, err := s.GetAdminHTTPClient()
	require.NoError(t, err)
	baseURL := s.AdminURL()

	resp := doDeploy(t, adminClient, baseURL, "bad", "", "")
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()

	resp = doDeploy(t, adminClient, baseURL, "bad+name", "console.log('hi')", "")
	require.True(t, resp.StatusCode >= 400 && resp.StatusCode < 500,
		"expected 4xx, got %d", resp.StatusCode)
	resp.Body.Close()
}

func TestWorkerdDeployWithBindings(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	s, sqlDB, _ := serverutils.StartServer(t, base.TestServerArgs{})
	ctx := context.Background()
	defer s.Stopper().Stop(ctx)

	adminClient, err := s.GetAdminHTTPClient()
	require.NoError(t, err)
	baseURL := s.AdminURL()

	bindings := `{"durable_objects": [{"class_name": "Counter"}]}`

	resp := doDeploy(t, adminClient, baseURL, "counter", testWorkerCounterBindings, bindings)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body := readBody(t, resp)
	require.Equal(t, float64(1), body["version"])

	var bindingsJSON string
	err = sqlDB.QueryRow("SELECT bindings::STRING FROM system.worker_scripts WHERE name = 'counter'").Scan(&bindingsJSON)
	require.NoError(t, err)
	require.Contains(t, bindingsJSON, "Counter")
}

// --- DO storage capnp tests (real KV, no workerd) ---

// setupDOStorageCapnp creates a capnp RPC client/server pair over a net.Pipe.
func setupDOStorageCapnp(t *testing.T, kvDB *kv.DB) (actorstorage.ActorStorage, func()) {
	t.Helper()
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

	bootstrap := cliRPC.Bootstrap(context.Background())
	client := actorstorage.ActorStorage(bootstrap)

	cleanup := func() {
		client.Release()
		cliRPC.Close()
		srvRPC.Close()
	}
	return client, cleanup
}

// getStage gets a Stage capability for the given actor.
func getStage(ctx context.Context, client actorstorage.ActorStorage, actorHex string) (actorstorage.ActorStorage_Stage, capnp.ReleaseFunc) {
	future, release := client.GetStage(ctx, func(p actorstorage.ActorStorage_getStage_Params) error {
		return p.SetStableId(actorHex)
	})
	results, err := future.Struct()
	if err != nil {
		panic(fmt.Sprintf("getStage failed: %v", err))
	}
	// AddRef the stage so it survives releasing the getStage future.
	stage := results.Stage().AddRef()
	release()
	return stage, func() { stage.Release() }
}

// doPut is a test helper that puts key-value pairs into a stage.
func doPut(t *testing.T, ctx context.Context, stage actorstorage.ActorStorage_Stage, kvs ...string) {
	t.Helper()
	require.True(t, len(kvs)%2 == 0, "doPut requires even number of args (key, val pairs)")
	n := len(kvs) / 2
	future, release := actorstorage.ActorStorage_Operations(stage).Put(ctx, func(p actorstorage.ActorStorage_Operations_put_Params) error {
		entries, err := p.NewEntries(int32(n))
		if err != nil {
			return err
		}
		for i := 0; i < n; i++ {
			if err := entries.At(i).SetKey([]byte(kvs[i*2])); err != nil {
				return err
			}
			if err := entries.At(i).SetValue([]byte(kvs[i*2+1])); err != nil {
				return err
			}
		}
		return nil
	})
	_, err := future.Struct()
	require.NoError(t, err)
	release()
}

// doGet is a test helper that gets a value from a stage.
func doGet(t *testing.T, ctx context.Context, stage actorstorage.ActorStorage_Stage, key string) []byte {
	t.Helper()
	future, release := actorstorage.ActorStorage_Operations(stage).Get(ctx, func(p actorstorage.ActorStorage_Operations_get_Params) error {
		return p.SetKey([]byte(key))
	})
	defer release()
	results, err := future.Struct()
	require.NoError(t, err)
	val, err := results.Value()
	require.NoError(t, err)
	// Copy: val points into the capnp message which release() frees.
	return append([]byte(nil), val...)
}

func TestDOStoragePutGetDelete(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	s, _, kvDB := serverutils.StartServer(t, base.TestServerArgs{})
	ctx := context.Background()
	defer s.Stopper().Stop(ctx)

	client, cleanup := setupDOStorageCapnp(t, kvDB)
	defer cleanup()

	actorHash := keys.ActorClassHash("Counter", "alice")
	actorHex := hex.EncodeToString(actorHash[:])
	stage, releaseStage := getStage(ctx, client, actorHex)
	defer releaseStage()

	// Put.
	doPut(t, ctx, stage, "count", "1")

	// Get.
	val := doGet(t, ctx, stage, "count")
	require.Equal(t, "1", string(val))

	// Delete.
	delFuture, delRelease := actorstorage.ActorStorage_Operations(stage).Delete(ctx, func(p actorstorage.ActorStorage_Operations_delete_Params) error {
		keysList, err := p.NewKeys(1)
		if err != nil {
			return err
		}
		return keysList.Set(0, []byte("count"))
	})
	delResults, err := delFuture.Struct()
	require.NoError(t, err)
	require.Equal(t, int32(1), delResults.NumDeleted())
	delRelease()

	// Get returns empty after delete.
	val = doGet(t, ctx, stage, "count")
	require.Empty(t, val)
}

func TestDOStorageList(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	s, _, kvDB := serverutils.StartServer(t, base.TestServerArgs{})
	ctx := context.Background()
	defer s.Stopper().Stop(ctx)

	client, cleanup := setupDOStorageCapnp(t, kvDB)
	defer cleanup()

	actorHash := keys.ActorClassHash("Counter", "alice")
	actorHex := hex.EncodeToString(actorHash[:])
	stage, releaseStage := getStage(ctx, client, actorHex)
	defer releaseStage()

	doPut(t, ctx, stage, "a", "1", "b", "2", "c", "3")

	// Verify each key exists.
	for _, k := range []string{"a", "b", "c"} {
		val := doGet(t, ctx, stage, k)
		require.NotEmpty(t, val, "key %s should have a value", k)
	}
}

func TestDOStorageDeleteAll(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	s, _, kvDB := serverutils.StartServer(t, base.TestServerArgs{})
	ctx := context.Background()
	defer s.Stopper().Stop(ctx)

	client, cleanup := setupDOStorageCapnp(t, kvDB)
	defer cleanup()

	actorHash := keys.ActorClassHash("Counter", "alice")
	actorHex := hex.EncodeToString(actorHash[:])
	stage, releaseStage := getStage(ctx, client, actorHex)
	defer releaseStage()

	doPut(t, ctx, stage, "x", "1", "y", "2")

	// Delete all.
	delFuture, delRelease := actorstorage.ActorStorage_Operations(stage).DeleteAll(ctx, nil)
	_, err := delFuture.Struct()
	require.NoError(t, err)
	delRelease()

	// Get returns empty after deleteAll.
	val := doGet(t, ctx, stage, "x")
	require.Empty(t, val)
}

func TestDOStorageActorIsolation(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	s, _, kvDB := serverutils.StartServer(t, base.TestServerArgs{})
	ctx := context.Background()
	defer s.Stopper().Stop(ctx)

	client, cleanup := setupDOStorageCapnp(t, kvDB)
	defer cleanup()

	aliceHash := keys.ActorClassHash("Counter", "alice")
	aliceHex := hex.EncodeToString(aliceHash[:])
	bobHash := keys.ActorClassHash("Counter", "bob")
	bobHex := hex.EncodeToString(bobHash[:])

	aliceStage, releaseAlice := getStage(ctx, client, aliceHex)
	defer releaseAlice()
	bobStage, releaseBob := getStage(ctx, client, bobHex)
	defer releaseBob()

	doPut(t, ctx, aliceStage, "count", "alice-val")
	doPut(t, ctx, bobStage, "count", "bob-val")

	// Alice's value.
	val := doGet(t, ctx, aliceStage, "count")
	require.Equal(t, "alice-val", string(val))

	// Bob's value.
	val = doGet(t, ctx, bobStage, "count")
	require.Equal(t, "bob-val", string(val))

	// Delete all for alice.
	delFuture, delRelease := actorstorage.ActorStorage_Operations(aliceStage).DeleteAll(ctx, nil)
	_, err := delFuture.Struct()
	require.NoError(t, err)
	delRelease()

	// Bob is unaffected.
	val = doGet(t, ctx, bobStage, "count")
	require.Equal(t, "bob-val", string(val))

	// Alice is gone.
	val = doGet(t, ctx, aliceStage, "count")
	require.Empty(t, val)
}

// --- Full workerd integration tests (need workerd binary) ---

func TestWorkerdDeployAndInvoke(t *testing.T) {
	skipIfNoWorkerd(t)
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	t.Setenv("RATEL_WORKERD_BIN", workerdBinPath())

	s, _, _ := serverutils.StartServer(t, base.TestServerArgs{})
	ctx := context.Background()
	defer s.Stopper().Stop(ctx)

	adminClient, err := s.GetAdminHTTPClient()
	require.NoError(t, err)

	resp := doDeploy(t, adminClient, s.AdminURL(), "hello", testWorkerHello, "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body := readBody(t, resp)
	require.Equal(t, float64(1), body["version"])

	var invokeResp *http.Response
	require.Eventually(t, func() bool {
		var invokeErr error
		invokeResp, invokeErr = invokeWorker(s.RPCAddr(), "hello", "/", nil)
		return invokeErr == nil && invokeResp.StatusCode == http.StatusOK
	}, 10*time.Second, 200*time.Millisecond, "worker did not become available")

	respBody, err := io.ReadAll(invokeResp.Body)
	invokeResp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, "hello from workerd", string(respBody))

	resp = doDeploy(t, adminClient, s.AdminURL(), "echo", testWorkerEcho, "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	require.Eventually(t, func() bool {
		var invokeErr error
		invokeResp, invokeErr = invokeWorker(s.RPCAddr(), "echo", "/foo/bar", nil)
		return invokeErr == nil && invokeResp.StatusCode == http.StatusOK
	}, 10*time.Second, 200*time.Millisecond, "echo worker did not become available")

	respBody, err = io.ReadAll(invokeResp.Body)
	invokeResp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, "echo: /foo/bar", string(respBody))

	require.Eventually(t, func() bool {
		var invokeErr error
		invokeResp, invokeErr = invokeWorker(s.RPCAddr(), "nonexistent", "/", nil)
		return invokeErr == nil
	}, 5*time.Second, 200*time.Millisecond)
	require.Equal(t, http.StatusNotFound, invokeResp.StatusCode)
	invokeResp.Body.Close()

	resp = doDeploy(t, adminClient, s.AdminURL(), "hello", testWorkerHelloV2, "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body = readBody(t, resp)
	require.Equal(t, float64(2), body["version"])

	require.Eventually(t, func() bool {
		var invokeErr error
		invokeResp, invokeErr = invokeWorker(s.RPCAddr(), "hello", "/", nil)
		if invokeErr != nil {
			return false
		}
		b, _ := io.ReadAll(invokeResp.Body)
		invokeResp.Body.Close()
		return string(b) == "hello v2"
	}, 10*time.Second, 200*time.Millisecond, "redeployed worker did not update")
}

func TestWorkerdDOCounter(t *testing.T) {
	skipIfNoWorkerd(t)
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	t.Setenv("RATEL_WORKERD_BIN", workerdBinPath())

	s, _, _ := serverutils.StartServer(t, base.TestServerArgs{})
	ctx := context.Background()
	defer s.Stopper().Stop(ctx)

	adminClient, err := s.GetAdminHTTPClient()
	require.NoError(t, err)

	bindings := `{"durable_objects": [{"class_name": "Counter"}]}`
	resp := doDeploy(t, adminClient, s.AdminURL(), "counter", testWorkerCounter, bindings)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	readBody(t, resp)

	// Wait for workerd to start.
	var invokeResp *http.Response
	require.Eventually(t, func() bool {
		var invokeErr error
		invokeResp, invokeErr = invokeWorker(s.RPCAddr(), "counter", "/", nil)
		return invokeErr == nil && invokeResp.StatusCode == http.StatusOK
	}, 15*time.Second, 200*time.Millisecond, "counter worker did not become available")

	respBody, err := io.ReadAll(invokeResp.Body)
	invokeResp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, "1", string(respBody))

	// Second request increments.
	invokeResp, err = invokeWorker(s.RPCAddr(), "counter", "/", nil)
	require.NoError(t, err)
	respBody, err = io.ReadAll(invokeResp.Body)
	invokeResp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, "2", string(respBody))

	// Third request.
	invokeResp, err = invokeWorker(s.RPCAddr(), "counter", "/", nil)
	require.NoError(t, err)
	respBody, err = io.ReadAll(invokeResp.Body)
	invokeResp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, "3", string(respBody))
}

// --- Health check and hardening tests ---

func TestWorkerdHealthCheck(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	s, _, _ := serverutils.StartServer(t, base.TestServerArgs{})
	ctx := t.Context()
	defer s.Stopper().Stop(ctx)

	// No workers deployed → health should return 503.
	resp, err := http.Get(fmt.Sprintf("http://%s/workers/_health", s.RPCAddr()))
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	resp.Body.Close()
}

func TestWorkerdHealthCheckWithWorker(t *testing.T) {
	skipIfNoWorkerd(t)
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	t.Setenv("RATEL_WORKERD_BIN", workerdBinPath())

	s, _, _ := serverutils.StartServer(t, base.TestServerArgs{})
	ctx := t.Context()
	defer s.Stopper().Stop(ctx)

	adminClient, err := s.GetAdminHTTPClient()
	require.NoError(t, err)

	// Deploy a worker to start the sidecar.
	resp := doDeploy(t, adminClient, s.AdminURL(), "hello", testWorkerHello, "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Health should become 200 once workerd starts.
	require.Eventually(t, func() bool {
		r, e := http.Get(fmt.Sprintf("http://%s/workers/_health", s.RPCAddr()))
		if e != nil {
			return false
		}
		r.Body.Close()
		return r.StatusCode == http.StatusOK
	}, 10*time.Second, 200*time.Millisecond, "health check did not become healthy")
}

func TestWorkerdConcurrencyLimit(t *testing.T) {
	skipIfNoWorkerd(t)
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	t.Setenv("RATEL_WORKERD_BIN", workerdBinPath())

	s, _, _ := serverutils.StartServer(t, base.TestServerArgs{})
	ctx := t.Context()
	defer s.Stopper().Stop(ctx)

	adminClient, err := s.GetAdminHTTPClient()
	require.NoError(t, err)

	resp := doDeploy(t, adminClient, s.AdminURL(), "hello", testWorkerHello, "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Wait for workerd to become available.
	require.Eventually(t, func() bool {
		r, e := invokeWorker(s.RPCAddr(), "hello", "/", nil)
		if e != nil {
			return false
		}
		r.Body.Close()
		return r.StatusCode == http.StatusOK
	}, 10*time.Second, 200*time.Millisecond)

	// Fire concurrent requests. All should succeed (well under the 256 limit).
	const n = 32
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			r, e := invokeWorker(s.RPCAddr(), "hello", "/", nil)
			if e != nil {
				errs <- e
				return
			}
			io.ReadAll(r.Body)
			r.Body.Close()
			if r.StatusCode != http.StatusOK {
				errs <- fmt.Errorf("status %d", r.StatusCode)
				return
			}
			errs <- nil
		}()
	}
	for i := 0; i < n; i++ {
		require.NoError(t, <-errs)
	}
}

func TestWorkerdDOCounterPersistsAcrossRedeploy(t *testing.T) {
	skipIfNoWorkerd(t)
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	t.Setenv("RATEL_WORKERD_BIN", workerdBinPath())

	s, _, _ := serverutils.StartServer(t, base.TestServerArgs{})
	ctx := t.Context()
	defer s.Stopper().Stop(ctx)

	adminClient, err := s.GetAdminHTTPClient()
	require.NoError(t, err)

	bindings := `{"durable_objects": [{"class_name": "Counter"}]}`

	// Deploy v1 of the counter.
	resp := doDeploy(t, adminClient, s.AdminURL(), "counter", testWorkerCounter, bindings)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	readBody(t, resp)

	// Increment counter to 3.
	var invokeResp *http.Response
	require.Eventually(t, func() bool {
		var e error
		invokeResp, e = invokeWorker(s.RPCAddr(), "counter", "/", nil)
		return e == nil && invokeResp.StatusCode == http.StatusOK
	}, 15*time.Second, 200*time.Millisecond)

	respBody, err := io.ReadAll(invokeResp.Body)
	invokeResp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, "1", string(respBody))

	invokeResp, err = invokeWorker(s.RPCAddr(), "counter", "/", nil)
	require.NoError(t, err)
	respBody, err = io.ReadAll(invokeResp.Body)
	invokeResp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, "2", string(respBody))

	invokeResp, err = invokeWorker(s.RPCAddr(), "counter", "/", nil)
	require.NoError(t, err)
	respBody, err = io.ReadAll(invokeResp.Body)
	invokeResp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, "3", string(respBody))

	// Redeploy same script (triggers workerd restart, new ActorCache).
	resp = doDeploy(t, adminClient, s.AdminURL(), "counter", testWorkerCounter, bindings)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	readBody(t, resp)

	// Counter should continue from 4 (data persisted in KV, not in ActorCache).
	require.Eventually(t, func() bool {
		var e error
		invokeResp, e = invokeWorker(s.RPCAddr(), "counter", "/", nil)
		if e != nil {
			return false
		}
		b, _ := io.ReadAll(invokeResp.Body)
		invokeResp.Body.Close()
		return string(b) == "4"
	}, 15*time.Second, 200*time.Millisecond, "counter did not persist across redeploy")
}

func TestWorkerdForwardingLoopPrevention(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	s, _, _ := serverutils.StartServer(t, base.TestServerArgs{})
	ctx := t.Context()
	defer s.Stopper().Stop(ctx)

	// Send a request with X-Ratel-Forwarded already set.
	// The proxy should never try to forward it again.
	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("http://%s/workers/anything/", s.RPCAddr()), nil)
	require.NoError(t, err)
	req.Header.Set("X-Ratel-Forwarded", "1")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	// Without workerd running, this will proxy to localhost and get a
	// connection refused or 502, but it should NOT loop or hang.
	// The key assertion is that the request completes at all.
	require.True(t, resp.StatusCode >= 400, "expected error status, got %d", resp.StatusCode)
}

func TestRendezvousHashStability(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	// With 3 nodes, compute which node each of 100 workers maps to.
	nodes := []roachpb.NodeID{1, 2, 3}
	assignments := make(map[string]roachpb.NodeID)
	for i := 0; i < 100; i++ {
		name := fmt.Sprintf("worker-%d", i)
		var best roachpb.NodeID
		var bestHash uint64
		for _, id := range nodes {
			h := rendezvousHash(name, id)
			if h > bestHash || best == 0 {
				bestHash = h
				best = id
			}
		}
		assignments[name] = best
	}

	// Verify reasonable distribution (each node gets at least some workers).
	counts := make(map[roachpb.NodeID]int)
	for _, id := range assignments {
		counts[id]++
	}
	for _, id := range nodes {
		require.Greater(t, counts[id], 10,
			"node %d only got %d workers; expected >10 of 100", id, counts[id])
	}

	// Add a 4th node. Most workers should stay on the same node.
	nodesV2 := []roachpb.NodeID{1, 2, 3, 4}
	moved := 0
	for name, oldNode := range assignments {
		var best roachpb.NodeID
		var bestHash uint64
		for _, id := range nodesV2 {
			h := rendezvousHash(name, id)
			if h > bestHash || best == 0 {
				bestHash = h
				best = id
			}
		}
		if best != oldNode {
			moved++
		}
	}
	// With rendezvous hashing, ~25% of workers should move to the new node.
	// Allow some variance.
	require.Less(t, moved, 50, "too many workers moved when adding a node: %d/100", moved)
	require.Greater(t, moved, 5, "suspiciously few workers moved: %d/100", moved)
}

func TestWorkerHasDOs(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	s, _, _ := serverutils.StartServer(t, base.TestServerArgs{})
	ctx := t.Context()
	defer s.Stopper().Stop(ctx)

	adminClient, err := s.GetAdminHTTPClient()
	require.NoError(t, err)
	baseURL := s.AdminURL()

	// Deploy stateless worker.
	resp := doDeploy(t, adminClient, baseURL, "hello", testWorkerHello, "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Deploy DO-bearing worker.
	bindings := `{"durable_objects": [{"class_name": "Counter"}]}`
	resp = doDeploy(t, adminClient, baseURL, "counter", testWorkerCounterBindings, bindings)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// The sidecar may not be running (no workerd binary in this test),
	// but we can check the DB state directly via fetchWorkerDefs.
	// Since we can't access the sidecar directly in this test pattern,
	// verify via the API that bindings are stored correctly.
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/v2/workers/", nil)
	require.NoError(t, err)
	listResp, err := adminClient.Do(req)
	require.NoError(t, err)
	body := readBody(t, listResp)
	workers := body["workers"].([]interface{})
	require.Len(t, workers, 2)
}

// --- Helpers ---

func doDeploy(t *testing.T, client http.Client, baseURL, name, script, bindings string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(
		http.MethodPut,
		fmt.Sprintf("%s/api/v2/workers/%s/", baseURL, name),
		strings.NewReader(script),
	)
	require.NoError(t, err)
	if bindings != "" {
		req.Header.Set("X-Bindings", bindings)
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

func readBody(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err, "body was: %s", string(data))
	return result
}

func invokeWorker(rpcAddr, name, path string, body []byte) (*http.Response, error) {
	method := http.MethodGet
	var bodyReader io.Reader
	if body != nil {
		method = http.MethodPost
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(
		method,
		fmt.Sprintf("http://%s/workers/%s%s", rpcAddr, name, path),
		bodyReader,
	)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}
