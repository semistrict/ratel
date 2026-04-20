// Copyright 2026 The Ratel Authors.
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

// End-to-end smoke tests that exercise the built `ratel` binary: boot a node,
// connect with `ratel sql`, confirm cert material is encrypted at rest, and
// confirm a wrong passphrase is rejected.
//
// These tests locate the binary through Bazel runfiles when run under
// `bazel test`, and fall back to `../../../bin/ratel` for `go test` from the
// package directory.

package main_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/build/bazel"
	"github.com/stretchr/testify/require"
)

// ratelBinary returns an absolute path to the ratel binary under test.
func ratelBinary(t testing.TB) string {
	t.Helper()
	if bazel.BuiltWithBazel() {
		p, err := bazel.Runfile("pkg/cmd/ratel/ratel_/ratel")
		require.NoError(t, err)
		return p
	}
	// Fallback for `go test ./pkg/cmd/ratel` from repo root, which still
	// produces a binary under the repo's bin/ directory via `bazel build`.
	abs, err := filepath.Abs("../../../bin/ratel")
	require.NoError(t, err)
	return abs
}

// freeAddr returns host:port strings for the SQL and HTTP listeners. Tests
// serialize on the default ports to keep the smoke test simple; if that
// becomes an issue, swap in a net.Listen("tcp", ":0") trick per invocation.
const (
	smokeSQLAddr  = "127.0.0.1:26299"
	smokeHTTPAddr = "127.0.0.1:8099"
)

// startRatel boots `ratel init` in the background and returns a cleanup
// function that stops the node. The cluster URL points at a fresh temp dir.
func startRatel(t *testing.T, extraEnv []string, args ...string) (clusterURL string, stop func()) {
	t.Helper()
	dir := t.TempDir()
	clusterURL = "file://" + dir + "/cluster/"
	bin := ratelBinary(t)
	all := append([]string{
		"init", clusterURL,
		"--node-id", "r1",
		"--listen-addr", smokeSQLAddr,
		"--http-addr", smokeHTTPAddr,
	}, args...)
	cmd := exec.Command(bin, all...)
	cmd.Env = append(os.Environ(), extraEnv...)
	logFile, err := os.Create(filepath.Join(dir, "ratel.log"))
	require.NoError(t, err)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	require.NoError(t, cmd.Start())

	stop = func() {
		_ = cmd.Process.Signal(os.Interrupt)
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(15 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
		_ = logFile.Close()
		if t.Failed() {
			b, _ := os.ReadFile(logFile.Name())
			t.Logf("ratel log:\n%s", string(b))
		}
	}
	return clusterURL, stop
}

// waitForSQL polls `ratel sql -e 'SELECT 1'` until it succeeds or timeout.
func waitForSQL(t *testing.T, extraEnv []string, args ...string) {
	t.Helper()
	bin := ratelBinary(t)
	deadline := time.Now().Add(90 * time.Second)
	var lastOut []byte
	var lastErr error
	for time.Now().Before(deadline) {
		probe := append([]string{}, args...)
		probe = append(probe, "-e", "SELECT 1")
		cmd := exec.Command(bin, append([]string{"sql"}, probe...)...)
		cmd.Env = append(os.Environ(), extraEnv...)
		lastOut, lastErr = cmd.CombinedOutput()
		if lastErr == nil {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("SQL never came up. last error: %v\nlast output:\n%s", lastErr, lastOut)
}

// runSQL runs `ratel sql <args> -e <stmt>` once and returns combined output.
func runSQL(t *testing.T, extraEnv []string, sqlArgs []string, stmt string) ([]byte, error) {
	t.Helper()
	bin := ratelBinary(t)
	full := append([]string{"sql"}, sqlArgs...)
	full = append(full, "-e", stmt)
	cmd := exec.Command(bin, full...)
	cmd.Env = append(os.Environ(), extraEnv...)
	return cmd.CombinedOutput()
}

// TestRatelInsecureRoundTrip: `ratel init` + direct host:port SQL (no TLS).
// Verifies the Phase 3 default-insecure path still works.
func TestRatelInsecureRoundTrip(t *testing.T) {
	_, stop := startRatel(t, nil)
	defer stop()

	waitForSQL(t, nil, smokeSQLAddr)

	out, err := runSQL(t, nil, []string{smokeSQLAddr},
		"CREATE TABLE t (id INT PRIMARY KEY); INSERT INTO t VALUES (42); SELECT * FROM t;")
	require.NoError(t, err, "sql round-trip failed: %s", out)
	require.Contains(t, string(out), "42",
		"expected '42' in SQL output, got:\n%s", out)
}

// TestRatelTLSWithPassphrase: --tls with RATEL_PASSPHRASE env:
//  1. node boots, SQL shell connects via sslmode=verify-ca
//  2. ca.key on disk is encrypted at rest (magic bytes present)
//  3. wrong passphrase is rejected with a decryption error
func TestRatelTLSWithPassphrase(t *testing.T) {
	const pass = "correct horse battery staple"
	env := []string{"RATEL_PASSPHRASE=" + pass}

	clusterURL, stop := startRatel(t, env, "--tls")
	defer stop()

	waitForSQL(t, env, clusterURL, "--tls")

	out, err := runSQL(t, env, []string{clusterURL, "--tls"},
		"CREATE TABLE t (id INT PRIMARY KEY); INSERT INTO t VALUES (42); SELECT * FROM t;")
	require.NoError(t, err, "tls round-trip failed: %s", out)
	require.Contains(t, string(out), "42")

	// ca.key on disk must be encrypted.
	clusterDir := strings.TrimPrefix(clusterURL, "file://")
	caKey, err := os.ReadFile(filepath.Join(clusterDir, "v1", "certs", "ca.key"))
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(caKey), "RATELAES"),
		"ca.key should start with encryption magic bytes, got first 50: %q", head(caKey, 50))

	// Wrong passphrase should fail with a decryption error.
	wrongOut, err := runSQL(t,
		[]string{"RATEL_PASSPHRASE=wrong"},
		[]string{clusterURL, "--tls"},
		"SELECT 1")
	require.Error(t, err, "wrong passphrase should be rejected; got: %s", wrongOut)
	require.Contains(t, string(wrongOut), "decrypting",
		"wrong passphrase error should mention decrypting, got: %s", wrongOut)
}

func head(b []byte, n int) string {
	if len(b) < n {
		n = len(b)
	}
	return fmt.Sprintf("%x", b[:n])
}
