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
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"capnproto.org/go/capnp/v3/rpc"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/server/actorstorage"
	"github.com/cockroachdb/cockroach/pkg/sql"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/stop"
	"github.com/cockroachdb/cockroach/pkg/util/tracing"
	"github.com/cockroachdb/errors"
)

// defaultWorkerdListenPort is the default port for workerd. In production this
// is used; in tests a random available port is picked instead.
const defaultWorkerdListenPort = 18787

const ratelWorkerdDOStorageEnv = "RATEL_WORKERD_DO_STORAGE"

// WorkerdSidecar manages the lifecycle of a workerd child process. It starts
// workerd on demand when workers exist in system.worker_versions, passes a
// socketpair fd for DO storage communication, and supports reloading the
// config when workers are deployed or updated.
type WorkerdSidecar struct {
	binaryPath string
	workDir    string
	listenPort int
	db         *kv.DB
	codec      keys.SQLCodec
	tracer     *tracing.Tracer
	stopper    *stop.Stopper

	// ie is set after SQL is initialized. Must be set before Start.
	ie *sql.InternalExecutor

	mu struct {
		sync.Mutex
		cmd        *exec.Cmd
		running    bool
		cancelMon  context.CancelFunc
		rpcConn    *rpc.Conn // capnp RPC connection on the socketpair
		workerDefs []WorkerDef
		waitDone   chan struct{} // closed when cmd.Wait() returns
	}
}

// WorkerdConfig holds configuration for the workerd sidecar.
type WorkerdConfig struct {
	// BinaryPath is the path to the workerd binary. If empty, looked up in PATH.
	BinaryPath string

	// WorkDir is the directory for workerd config and script files.
	WorkDir string
}

// NewWorkerdSidecar creates a new sidecar manager. Call Start to launch the
// workerd process.
func NewWorkerdSidecar(
	cfg WorkerdConfig, db *kv.DB, codec keys.SQLCodec, tracer *tracing.Tracer, stopper *stop.Stopper,
) (*WorkerdSidecar, error) {
	if cfg.BinaryPath == "" {
		cfg.BinaryPath = resolveWorkerdBinary()
		if cfg.BinaryPath == "" {
			return nil, errors.New("workerd binary not found: not embedded and not in PATH; set WorkerdConfig.BinaryPath")
		}
	}

	workDir := cfg.WorkDir
	if workDir == "" {
		var err error
		workDir, err = os.MkdirTemp("", "ratel-workerd-*")
		if err != nil {
			return nil, errors.Wrap(err, "creating workerd work directory")
		}
	}

	// Pick a free port for workerd to listen on.
	port, err := pickFreePort()
	if err != nil {
		return nil, errors.Wrap(err, "picking free port for workerd")
	}

	w := &WorkerdSidecar{
		binaryPath: cfg.BinaryPath,
		workDir:    workDir,
		listenPort: port,
		db:         db,
		codec:      codec,
		tracer:     tracer,
		stopper:    stopper,
	}

	// Always register a closer so stopProcess runs on shutdown, even if
	// the sidecar is started later via ReloadConfig.
	stopper.AddCloser(stop.CloserFn(func() {
		w.stopProcess()
	}))

	return w, nil
}

// SetInternalExecutor sets the internal executor used for querying
// system.worker_versions. Must be called before Start.
func (w *WorkerdSidecar) SetInternalExecutor(ie *sql.InternalExecutor) {
	w.ie = ie
}

// ListenPort returns the port the workerd sidecar listens on.
func (w *WorkerdSidecar) ListenPort() int {
	return w.listenPort
}

// IsRunning returns true if the workerd sidecar process is running.
func (w *WorkerdSidecar) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.mu.running
}

// WorkerHasDOs returns true if the named worker has Durable Object classes.
func (w *WorkerdSidecar) WorkerHasDOs(name string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, wd := range w.mu.workerDefs {
		if wd.Name == name {
			return len(wd.DOClasses) > 0
		}
	}
	return false
}

// Start launches the workerd process if workers exist. It is a no-op if no
// workers are deployed.
func (w *WorkerdSidecar) Start(ctx context.Context) error {
	workers, err := w.fetchWorkerDefs(ctx)
	if err != nil {
		return errors.Wrap(err, "fetching worker definitions")
	}
	if len(workers) == 0 {
		log.Infof(ctx, "no workers deployed; workerd sidecar not started")
		return nil
	}

	if err := w.startProcess(ctx, workers); err != nil {
		return err
	}

	return nil
}

// ReloadConfig fetches the latest worker definitions and restarts workerd
// with a new config. Called after deploying or updating a worker.
func (w *WorkerdSidecar) ReloadConfig(ctx context.Context) error {
	workers, err := w.fetchWorkerDefs(ctx)
	if err != nil {
		return errors.Wrap(err, "fetching worker definitions for reload")
	}
	if len(workers) == 0 {
		w.stopProcess()
		return nil
	}

	w.stopProcess()

	// Use a background context for the new process -- it should not be tied
	// to the HTTP request that triggered the reload.
	bgCtx := context.Background()
	return w.startProcess(bgCtx, workers)
}

func (w *WorkerdSidecar) startProcess(ctx context.Context, workers []WorkerDef) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var parentFile *os.File
	var childFile *os.File
	storageFd := -1
	useRatelStorage := os.Getenv(ratelWorkerdDOStorageEnv) != "in-memory"
	if useRatelStorage {
		// Create a Unix socketpair for DO storage communication.
		fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
		if err != nil {
			return errors.Wrap(err, "creating socketpair for DO storage")
		}
		parentFile = os.NewFile(uintptr(fds[0]), "ratel-storage-parent")
		childFile = os.NewFile(uintptr(fds[1]), "ratel-storage-child")

		// The child fd will be passed as fd 3 (ExtraFiles[0]).
		storageFd = 3
	}

	// Generate workerd config.
	configPath, err := generateWorkerdConfig(w.workDir, workers, w.listenPort, storageFd)
	if err != nil {
		if parentFile != nil {
			parentFile.Close()
		}
		if childFile != nil {
			childFile.Close()
		}
		return errors.Wrap(err, "generating workerd config")
	}

	cmd := exec.Command(w.binaryPath, "serve", configPath, "--verbose")
	if childFile != nil {
		cmd.ExtraFiles = []*os.File{childFile}
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		if parentFile != nil {
			parentFile.Close()
		}
		if childFile != nil {
			childFile.Close()
		}
		return errors.Wrap(err, "starting workerd sidecar")
	}

	var rpcConn *rpc.Conn
	if useRatelStorage {
		// Close the child end in the parent process -- workerd has it now.
		childFile.Close()

		// Wrap the parent end of the socketpair as a net.Conn for capnp RPC.
		parentConn, err := net.FileConn(parentFile)
		if err != nil {
			parentFile.Close()
			cmd.Process.Signal(os.Interrupt)
			cmd.Wait()
			return errors.Wrap(err, "creating conn from storage socketpair")
		}
		// FileConn dups the fd, so close our original handle.
		parentFile.Close()

		// Create the capnp RPC connection with the ActorStorage server as
		// the bootstrap capability. workerd will call getStage(actorId) to
		// get per-actor Stage capabilities.
		storageServer := &actorstorage.StorageServer{
			DB:     w.db,
			Codec:  w.codec,
			Tracer: w.tracer,
		}
		transport := rpc.NewStreamTransport(parentConn)
		rpcConn = rpc.NewConn(transport, &rpc.Options{
			BootstrapClient: actorstorage.NewClient(storageServer),
		})
	}

	// Start the monitor goroutine. It will exit when monCtx is cancelled.
	monCtx, monCancel := context.WithCancel(context.Background())
	waitDone := make(chan struct{})
	go func() {
		cmd.Wait()
		close(waitDone)
	}()

	if err := w.stopper.RunAsyncTask(ctx, "workerd-monitor", func(ctx context.Context) {
		w.monitor(monCtx, waitDone)
	}); err != nil {
		monCancel()
		if rpcConn != nil {
			rpcConn.Close()
		}
		cmd.Process.Signal(os.Interrupt)
		<-waitDone
		return err
	}

	w.mu.cmd = cmd
	w.mu.running = true
	w.mu.cancelMon = monCancel
	w.mu.rpcConn = rpcConn
	w.mu.workerDefs = workers
	w.mu.waitDone = waitDone
	log.Infof(ctx, "workerd sidecar started (pid %d) with %d workers", cmd.Process.Pid, len(workers))
	return nil
}

// gracefulStopTimeout is how long we wait after SIGTERM before SIGKILL.
const gracefulStopTimeout = 5 * time.Second

func (w *WorkerdSidecar) stopProcess() {
	w.mu.Lock()

	if !w.mu.running || w.mu.cmd == nil || w.mu.cmd.Process == nil {
		w.mu.Unlock()
		return
	}

	// Cancel the monitor so it doesn't try to restart.
	if w.mu.cancelMon != nil {
		w.mu.cancelMon()
		w.mu.cancelMon = nil
	}

	cmd := w.mu.cmd
	waitDone := w.mu.waitDone
	w.mu.running = false
	w.mu.cmd = nil
	w.mu.workerDefs = nil
	w.mu.waitDone = nil

	// Close the capnp RPC connection, which also closes the socketpair.
	if w.mu.rpcConn != nil {
		w.mu.rpcConn.Close()
		w.mu.rpcConn = nil
	}

	w.mu.Unlock()

	// Graceful shutdown: SIGTERM, wait up to gracefulStopTimeout, then SIGKILL.
	cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-waitDone:
		return
	case <-time.After(gracefulStopTimeout):
		log.Warningf(context.Background(), "workerd did not exit after %v; sending SIGKILL", gracefulStopTimeout)
		cmd.Process.Kill()
		<-waitDone
	}
}

func (w *WorkerdSidecar) monitor(ctx context.Context, waitDone <-chan struct{}) {
	select {
	case <-ctx.Done():
		return
	case <-w.stopper.ShouldQuiesce():
		return
	case <-waitDone:
		// Process exited.
	}

	// Check if this was an intentional stop.
	select {
	case <-ctx.Done():
		return
	case <-w.stopper.ShouldQuiesce():
		return
	default:
	}

	// Process exited unexpectedly. Restart with current config.
	log.Warningf(ctx, "workerd sidecar exited unexpectedly; restarting")

	workers, fetchErr := w.fetchWorkerDefs(ctx)
	if fetchErr != nil {
		log.Errorf(ctx, "failed to fetch workers for restart: %v", fetchErr)
		return
	}
	if len(workers) == 0 {
		return
	}
	if restartErr := w.startProcess(ctx, workers); restartErr != nil {
		log.Errorf(ctx, "failed to restart workerd sidecar: %v", restartErr)
	}
}

// workerBindings is the JSON structure stored in the bindings column.
type workerBindings struct {
	DurableObjects []doBinding `json:"durable_objects,omitempty"`
}

type doBinding struct {
	ClassName string `json:"class_name"`
}

// fetchWorkerDefs queries system.worker_versions for the latest version of
// each deployed worker, including bindings metadata.
func (w *WorkerdSidecar) fetchWorkerDefs(ctx context.Context) ([]WorkerDef, error) {
	if w.ie == nil {
		return nil, errors.New("internal executor not set")
	}

	rows, err := w.ie.QueryBufferedEx(
		ctx,
		"fetch-worker-defs",
		nil, // no txn
		sessiondata.InternalExecutorOverride{
			User:     username.RootUserName(),
			Database: "system",
		},
		`SELECT wv.worker_name, wv.id, wv.version, wv.script, wv.compat_date, wv.bindings
		 FROM system.worker_versions wv
		 INNER JOIN (
		   SELECT worker_name, max(version) AS max_version
		   FROM system.worker_versions
		   GROUP BY worker_name
		 ) latest ON wv.worker_name = latest.worker_name AND wv.version = latest.max_version
		 ORDER BY wv.worker_name`,
	)
	if err != nil {
		return nil, err
	}

	workers := make([]WorkerDef, 0, len(rows))
	workerIndexes := make(map[string]int, len(rows))
	type workerVersion struct {
		name string
		id   int64
	}
	versions := make([]workerVersion, 0, len(rows))
	for _, row := range rows {
		name := string(*row[0].(*tree.DString))
		versionID := int64(*row[1].(*tree.DInt))
		script := string(*row[3].(*tree.DBytes))
		compatDate := string(*row[4].(*tree.DString))

		var doClasses []string
		if row[5] != nil && row[5] != tree.DNull {
			bindingsStr := row[5].(*tree.DJSON).JSON.String()
			var b workerBindings
			if jsonErr := json.Unmarshal([]byte(bindingsStr), &b); jsonErr != nil {
				log.Warningf(ctx, "invalid bindings JSON for worker %s: %v", name, jsonErr)
			} else {
				for _, do := range b.DurableObjects {
					doClasses = append(doClasses, do.ClassName)
				}
			}
		}

		workerIndexes[name] = len(workers)
		versions = append(versions, workerVersion{name: name, id: versionID})
		workers = append(workers, WorkerDef{
			Name:       name,
			Script:     script,
			CompatDate: compatDate,
			DOClasses:  doClasses,
		})
	}
	for _, version := range versions {
		assetRows, err := w.ie.QueryBufferedEx(
			ctx,
			"fetch-worker-assets",
			nil, // no txn
			sessiondata.InternalExecutorOverride{
				User:     username.RootUserName(),
				Database: "system",
			},
			`SELECT wva.path, wva.content_type, wa.content
			 FROM system.worker_version_assets wva
			 INNER JOIN system.worker_assets wa ON wa.etag = wva.asset_etag
			 WHERE wva.worker_version_id = $1
			 ORDER BY wva.path`,
			version.id,
		)
		if err != nil {
			return nil, err
		}
		idx := workerIndexes[version.name]
		for _, assetRow := range assetRows {
			path := string(*assetRow[0].(*tree.DString))
			contentType := ""
			if assetRow[1] != nil && assetRow[1] != tree.DNull {
				contentType = string(*assetRow[1].(*tree.DString))
			}
			content := []byte(*assetRow[2].(*tree.DBytes))
			workers[idx].Assets = append(workers[idx].Assets, WorkerAsset{
				Path:        path,
				ContentType: contentType,
				Content:     content,
			})
		}
	}
	return workers, nil
}

// resolveWorkerdBinary returns the path to the workerd binary. It tries the
// embedded binary first (extracting to a cache directory if needed), then
// falls back to looking for workerd in PATH. Returns empty string if not found.
func resolveWorkerdBinary() string {
	if path, err := extractEmbeddedWorkerd(); err == nil {
		return path
	}
	if path, err := exec.LookPath("workerd"); err == nil {
		return path
	}
	return ""
}

// pickFreePort binds to port 0, reads the assigned port, and closes the
// listener. The port may be reused by the time workerd binds to it, but
// this is acceptable for the short window between pick and bind.
func pickFreePort() (int, error) {
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port, nil
}
