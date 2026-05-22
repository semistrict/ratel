// Copyright 2024 The Cockroach Authors.
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

package cli

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	gohex "encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/cli/clicfg"
	"github.com/cockroachdb/cockroach/pkg/cli/clierror"
	"github.com/cockroachdb/cockroach/pkg/cli/cliflags"
	"github.com/cockroachdb/cockroach/pkg/cli/clisqlcfg"
	"github.com/cockroachdb/cockroach/pkg/cli/clisqlclient"
	"github.com/cockroachdb/cockroach/pkg/cli/clisqlexec"
	"github.com/cockroachdb/cockroach/pkg/cli/clisqlshell"
	"github.com/cockroachdb/cockroach/pkg/cli/exit"
	"github.com/cockroachdb/cockroach/pkg/server"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/storage"
	"github.com/cockroachdb/cockroach/pkg/storage/enginepb"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/log/logcrash"
	"github.com/cockroachdb/cockroach/pkg/util/stop"
	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/pebble/objstorage/remote"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// autoNodeID generates a node ID from hostname plus a random suffix.
func autoNodeID() string {
	host, _ := os.Hostname()
	if host == "" {
		host = "node"
	}
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s-%s", host, gohex.EncodeToString(b[:]))
}

var ratelListenAddr string
var ratelHTTPAddr string
var ratelNoPassphrase bool
var ratelNodeID string
var ratelDeployCompatDate string
var ratelDeployConfig string
var ratelDeployDOClasses []string
var ratelDeployName string
var ratelSQLHost string
var ratelTLS bool

var ratelCmd = &cobra.Command{
	Use:   "ratel [command]",
	Short: "Ratel: CockroachDB with S3-native cluster identity",
	Long: `Ratel wraps CockroachDB with a simplified operational model where a single
storage URL (file:// or s3://) serves as the cluster identity. Node discovery,
TLS certificates, and shared storage are all derived from this URL.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

var ratelInitCmd = &cobra.Command{
	Use:   "init <storage-url>",
	Short: "Initialize a new cluster at the given storage URL",
	Long: `Initialize a new ratel cluster. This generates TLS certificates, uploads them
to the storage URL, bootstraps CockroachDB, registers the first node, and keeps
the server running.`,
	Args: cobra.ExactArgs(1),
	RunE: runRatelInit,
}

var ratelJoinCmd = &cobra.Command{
	Use:   "join <storage-url>",
	Short: "Start a node and join an existing cluster",
	Long: `Start a CockroachDB node that joins an existing ratel cluster. Peers are
discovered from the node registry in the storage URL. TLS certificates are
downloaded from the storage URL.`,
	Args: cobra.ExactArgs(1),
	RunE: runRatelJoin,
}

var ratelSQLCmd = &cobra.Command{
	Use:   "sql [storage-url | host:port]",
	Short: "Connect a SQL shell to a cluster",
	Long: `Connect to a running ratel cluster's SQL interface.

With a host:port argument, connect directly:
  ratel sql localhost:26257

With a storage URL, discover nodes from S3:
  ratel sql s3://bucket/path?endpoint=...`,
	Args: cobra.ExactArgs(1),
	RunE: runRatelSQL,
}

var ratelDeployCmd = &cobra.Command{
	Use:   "deploy <storage-url | host:port> <file.js>",
	Short: "Deploy a JavaScript worker to the cluster",
	Long: `Deploy a JavaScript worker file. The worker name is derived from the
filename (e.g. counter.js becomes "counter").

  ratel deploy s3://bucket/path worker.js
  ratel deploy localhost:5273 worker.js`,
	Args: cobra.ExactArgs(2),
	RunE: runRatelDeploy,
}

var ratelStartLocalCmd = &cobra.Command{
	Use:   "start-local",
	Short: "Start a single-node cluster for local development",
	Long: `Start a single-node Ratel instance with no S3, no TLS, and an in-memory store.
The node auto-initializes, listens on localhost, and is ready for SQL immediately.

Connect with:  ratel sql localhost:26257`,
	Args: cobra.NoArgs,
	RunE: runRatelStartLocal,
}

func init() {
	ratelCmd.AddCommand(ratelInitCmd, ratelJoinCmd, ratelSQLCmd, ratelDeployCmd, ratelStartLocalCmd)

	ratelStartLocalCmd.Flags().StringVar(&ratelListenAddr, "listen-addr", "localhost:26257",
		"Address to listen on for RPC and SQL connections")
	ratelStartLocalCmd.Flags().StringVar(&ratelHTTPAddr, "http-addr", "localhost:5273",
		"Address to listen on for the admin HTTP interface")

	for _, cmd := range []*cobra.Command{ratelInitCmd, ratelJoinCmd} {
		cmd.Flags().StringVar(&ratelListenAddr, "listen-addr", "localhost:26257",
			"Address to listen on for RPC and SQL connections")
		cmd.Flags().StringVar(&ratelHTTPAddr, "http-addr", "localhost:5273",
			"Address to listen on for the admin HTTP interface")
		cmd.Flags().BoolVar(&ratelTLS, "tls", false,
			"Enable application-level TLS (generates and manages certificates via S3)")
		cmd.Flags().BoolVar(&ratelNoPassphrase, "no-passphrase", false,
			"Do not encrypt the CA key (skip passphrase prompt, only with --tls)")
		cmd.Flags().StringVar(&ratelNodeID, "node-id", "",
			"Stable operator-assigned node identity (e.g. ratel-1); auto-generated if omitted")
	}

	// SQL-specific flags.
	ratelSQLCmd.Flags().VarP(&ratelSQLExecStmts, cliflags.Execute.Name, cliflags.Execute.Shorthand, cliflags.Execute.Description)
	ratelSQLCmd.Flags().StringVar(&ratelSQLHost, "host", "",
		"Override the node address to connect to (e.g. localhost:26257 when using fly proxy)")
	ratelSQLCmd.Flags().BoolVar(&ratelTLS, "tls", false,
		"Connect using TLS (download client certs from S3)")

	ratelDeployCmd.Flags().StringVar(&ratelDeployCompatDate, "compat-date", "2024-01-01",
		"Cloudflare Workers compatibility date for the deployed worker")
	ratelDeployCmd.Flags().StringVar(&ratelDeployConfig, "config", "",
		"Path to worker.jsonc config; defaults to worker.jsonc beside the JavaScript file when present")
	ratelDeployCmd.Flags().StringArrayVar(&ratelDeployDOClasses, "do-class", nil,
		"Durable Object class exported by this worker; repeat for multiple classes")
	ratelDeployCmd.Flags().StringVar(&ratelDeployName, "name", "",
		"Worker name to deploy; defaults to the JavaScript filename without extension")
}

// ratelSQLExecStmts holds -e statements for ratel sql.
var ratelSQLExecStmts clisqlshell.StatementsValue

// RatelMain is the entry point for the ratel binary.
func RatelMain() {
	if err := ratelCmd.Execute(); err != nil {
		clierror.OutputError(os.Stderr, err, true, false)
		exit.WithCode(exit.UnspecifiedError())
	}
}

// ratelAdvertiseHost returns the hostname this node should advertise and use
// in its TLS certificate. Derived from --listen-addr; if listening on 0.0.0.0,
// resolves the system hostname instead.
func ratelAdvertiseHost() string {
	host, _, _ := net.SplitHostPort(ratelListenAddr)
	if host == "0.0.0.0" || host == "" {
		// On Fly.io, use <machine_id>.vm.<app>.internal which is
		// resolvable by all machines in the app's private network.
		if machID := os.Getenv("FLY_MACHINE_ID"); machID != "" {
			if appName := os.Getenv("FLY_APP_NAME"); appName != "" {
				return fmt.Sprintf("%s.vm.%s.internal", machID, appName)
			}
		}
		if h, err := os.Hostname(); err == nil && h != "" {
			return h
		}
		return "localhost"
	}
	return host
}

// ratelPassphrase returns the CA key passphrase. It checks RATEL_PASSPHRASE
// env var first, then prompts interactively. Returns nil if --no-passphrase.
func ratelPassphrase(confirm bool) ([]byte, error) {
	if ratelNoPassphrase {
		return nil, nil
	}
	if env := os.Getenv("RATEL_PASSPHRASE"); env != "" {
		return []byte(env), nil
	}
	fmt.Fprint(os.Stderr, "Enter CA key passphrase: ")
	pass, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, errors.Wrap(err, "reading passphrase")
	}
	if len(pass) == 0 {
		return nil, errors.New("passphrase cannot be empty (use --no-passphrase to skip)")
	}
	if confirm {
		fmt.Fprint(os.Stderr, "Confirm passphrase: ")
		pass2, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return nil, errors.Wrap(err, "reading passphrase confirmation")
		}
		if string(pass) != string(pass2) {
			return nil, errors.New("passphrases do not match")
		}
	}
	return pass, nil
}

// ratelLocalDir returns a stable temp directory derived from the cluster UUID.
// Each cluster gets its own local store directory, preventing stale data
// conflicts when the same URL is reused for a new cluster.
func ratelLocalDir(clusterID string) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("ratel-%s", clusterID))
}

func runRatelStartLocal(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	storeDir, err := os.MkdirTemp("", "ratel-local-*")
	if err != nil {
		return errors.Wrap(err, "creating temp store directory")
	}

	fmt.Fprintf(os.Stderr, "Starting local single-node Ratel (store: %s)...\n", storeDir)
	return ratelStartServer(ctx, ratelServerOpts{
		clusterURL:     "file://" + storeDir,
		listenAddr:     ratelListenAddr,
		httpAddr:       ratelHTTPAddr,
		certsDir:       "", // insecure
		storeDir:       storeDir,
		joinList:       nil, // single-node
		autoInitialize: true,
		nodesStore:     nil, // no S3 node registry
		ratelNodeID:    "local",
	})
}

func runRatelInit(cmd *cobra.Command, args []string) error {
	clusterURL := args[0]
	ctx := context.Background()

	// Probe storage; offer to create the bucket if it does not exist.
	if err := storage.ProbeStorage(ctx, clusterURL); err != nil {
		if errors.Is(err, storage.ErrBucketNotFound) {
			bucket := storage.BucketName(clusterURL)
			fmt.Fprintf(os.Stderr, "Bucket %q does not exist. Create it? [y/N] ", bucket)
			var answer string
			fmt.Scanln(&answer)
			if answer != "y" && answer != "Y" {
				return errors.New("bucket does not exist")
			}
			if err := storage.CreateBucket(ctx, clusterURL); err != nil {
				return errors.Wrap(err, "creating bucket")
			}
			fmt.Fprintf(os.Stderr, "Created bucket %q\n", bucket)
		} else {
			return errors.Wrap(err, "probing storage")
		}
	}

	cs, err := storage.ClusterStorageFromURL(clusterURL)
	if err != nil {
		return err
	}

	if ratelNodeID == "" {
		ratelNodeID = autoNodeID()
		fmt.Fprintf(os.Stderr, "Auto-assigned node ID: %s\n", ratelNodeID)
	}

	// Check for duplicate running node with same --node-id.
	if err := checkNodeLiveness(ctx, cs.Nodes, ratelNodeID); err != nil {
		return err
	}

	// Check that cluster is not already initialized.
	nodes, err := storage.ListNodes(ctx, cs.Nodes)
	if err != nil {
		return errors.Wrap(err, "checking existing nodes")
	}
	if len(nodes) > 0 {
		return errors.Newf("cluster already initialized: found %d node(s) at %s", len(nodes), clusterURL)
	}

	clusterUUID, err := storage.WriteClusterID(ctx, cs.Metadata)
	if err != nil {
		return errors.Wrap(err, "writing cluster ID")
	}
	fmt.Fprintf(os.Stderr, "Cluster ID: %s\n", clusterUUID)

	ld := ratelLocalDir(clusterUUID.String())
	certsDir := ""
	storeDir := filepath.Join(ld, "store")

	if ratelTLS {
		// Get passphrase for CA key encryption.
		passphrase, err := ratelPassphrase(true /* confirm */)
		if err != nil {
			return err
		}

		// Generate and upload CA + client certs if not already present.
		exists, err := storage.CertsExist(ctx, cs.Certs)
		if err != nil {
			return err
		}
		if !exists {
			fmt.Fprintln(os.Stderr, "Generating CA and client certificates...")
			if err := storage.GenerateAndUploadCACerts(ctx, cs.Certs, passphrase); err != nil {
				return err
			}
		}

		// Download CA + client certs, then generate this node's cert locally.
		certsDir = filepath.Join(ld, "certs")
		if err := storage.DownloadCACerts(ctx, cs.Certs, certsDir, passphrase); err != nil {
			return err
		}
		hostname := ratelAdvertiseHost()
		if err := storage.GenerateNodeCert(certsDir, []string{hostname, "localhost"}); err != nil {
			return err
		}
	}

	// Configure and start the server.
	fmt.Fprintln(os.Stderr, "Starting CockroachDB node (init mode)...")
	return ratelStartServer(ctx, ratelServerOpts{
		clusterURL:     clusterURL,
		listenAddr:     ratelListenAddr,
		httpAddr:       ratelHTTPAddr,
		certsDir:       certsDir,
		storeDir:       storeDir,
		joinList:       nil, // no peers; we are bootstrapping
		autoInitialize: true,
		nodesStore:     cs.Nodes,
		ratelNodeID:    ratelNodeID,
	})
}

func runRatelJoin(cmd *cobra.Command, args []string) error {
	clusterURL := args[0]

	cs, err := storage.ClusterStorageFromURL(clusterURL)
	if err != nil {
		return err
	}

	ctx := context.Background()

	if ratelNodeID == "" {
		ratelNodeID = autoNodeID()
		fmt.Fprintf(os.Stderr, "Auto-assigned node ID: %s\n", ratelNodeID)
	}

	// Check for duplicate running node with same --node-id.
	if err := checkNodeLiveness(ctx, cs.Nodes, ratelNodeID); err != nil {
		return err
	}

	// Read cluster UUID from metadata storage.
	clusterUUID, err := storage.ReadClusterID(ctx, cs.Metadata)
	if err != nil {
		return errors.Wrap(err, "reading cluster ID (is the cluster initialized?)")
	}

	// Discover peers.
	nodes, err := storage.ListNodes(ctx, cs.Nodes)
	if err != nil {
		return errors.Wrap(err, "listing nodes")
	}
	if len(nodes) == 0 {
		return errors.New("cluster not initialized, run 'ratel init' first")
	}

	var joinList []string
	for _, n := range nodes {
		joinList = append(joinList, n.Addr)
	}

	ld := ratelLocalDir(clusterUUID.String())
	certsDir := ""
	storeDir := filepath.Join(ld, "store")

	if ratelTLS {
		// Get passphrase for CA key decryption.
		passphrase, err := ratelPassphrase(false /* confirm */)
		if err != nil {
			return err
		}

		certsDir = filepath.Join(ld, "certs")
		if err := storage.DownloadCACerts(ctx, cs.Certs, certsDir, passphrase); err != nil {
			return errors.Wrap(err, "downloading certs (is the cluster initialized?)")
		}
		hostname := ratelAdvertiseHost()
		if err := storage.GenerateNodeCert(certsDir, []string{hostname, "localhost"}); err != nil {
			return err
		}
	}

	fmt.Fprintf(os.Stderr, "Joining cluster with %d existing node(s)...\n", len(nodes))
	return ratelStartServer(ctx, ratelServerOpts{
		clusterURL:     clusterURL,
		listenAddr:     ratelListenAddr,
		httpAddr:       ratelHTTPAddr,
		certsDir:       certsDir,
		storeDir:       storeDir,
		joinList:       joinList,
		autoInitialize: false,
		nodesStore:     cs.Nodes,
		ratelNodeID:    ratelNodeID,
	})
}

type ratelServerOpts struct {
	clusterURL     string
	listenAddr     string
	httpAddr       string
	certsDir       string
	storeDir       string
	joinList       []string
	autoInitialize bool
	nodesStore     remote.Storage
	ratelNodeID    string
}

// checkNodeLiveness reads the node registration for the given ratel node ID
// and returns an error if the node appears to be already running (heartbeat
// within the last 60 seconds).
func checkNodeLiveness(ctx context.Context, store remote.Storage, nodeID string) error {
	reg, exists, err := storage.NodeRegistrationExists(ctx, store, nodeID)
	if err != nil {
		return errors.Wrapf(err, "checking liveness for node %s", nodeID)
	}
	if !exists {
		return nil
	}
	if reg.LastHeartbeat != nil && time.Since(*reg.LastHeartbeat) < 60*time.Second {
		return errors.Newf(
			"node %s appears to be already running (last heartbeat: %s, %s ago)",
			nodeID, reg.LastHeartbeat.Format(time.RFC3339), time.Since(*reg.LastHeartbeat).Round(time.Second))
	}
	return nil
}

func ratelStartServer(ctx context.Context, opts ratelServerOpts) error {
	// Build server config.
	st := cluster.MakeClusterSettings()
	logcrash.SetGlobalSettings(&st.SV)

	cfg := server.MakeConfig(ctx, st)
	cfg.Insecure = opts.certsDir == ""
	cfg.SSLCertsDir = opts.certsDir
	_, port, _ := net.SplitHostPort(opts.listenAddr)
	advertiseAddr := net.JoinHostPort(ratelAdvertiseHost(), port)
	cfg.Addr = opts.listenAddr
	cfg.AdvertiseAddr = advertiseAddr
	cfg.SQLAddr = opts.listenAddr
	cfg.SQLAdvertiseAddr = advertiseAddr
	cfg.HTTPAddr = opts.httpAddr
	cfg.AutoInitializeCluster = opts.autoInitialize
	cfg.JoinList = opts.joinList
	cfg.StorageEngine = enginepb.EngineTypePebble

	// Configure store with remote storage.
	if err := os.MkdirAll(opts.storeDir, 0755); err != nil {
		return errors.Wrapf(err, "creating store dir %s", opts.storeDir)
	}

	// Crash recovery: if a previous registration exists for this node-id,
	// recover its store_id so we download the right manifest bundle.
	var recoveryStoreID int32
	if opts.ratelNodeID != "" && opts.nodesStore != nil {
		reg, exists, err := storage.NodeRegistrationExists(ctx, opts.nodesStore, opts.ratelNodeID)
		if err != nil {
			return errors.Wrap(err, "checking for previous node registration")
		}
		if exists && reg.StoreID > 0 {
			recoveryStoreID = int32(reg.StoreID)
			fmt.Fprintf(os.Stderr, "Recovered store_id=%d from previous registration for node %s\n",
				recoveryStoreID, opts.ratelNodeID)
		}
	}

	storeSpec := base.StoreSpec{
		Path:            opts.storeDir,
		RecoveryStoreID: recoveryStoreID,
	}
	if opts.nodesStore != nil {
		storeSpec.RemoteStoragePath = opts.clusterURL
	}
	cfg.Stores = base.StoreSpecList{Specs: []base.StoreSpec{storeSpec}}

	// Initialize temp storage with the store path as the parent dir.
	cfg.TempStorageConfig = base.TempStorageConfigFromEnv(
		ctx, st, storeSpec, opts.storeDir, base.DefaultTempStorageMaxSizeBytes)
	tempDir := filepath.Join(opts.storeDir, "cockroach-temp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return errors.Wrapf(err, "creating temp dir %s", tempDir)
	}
	cfg.TempStorageConfig.Path = tempDir

	// InitNode parses bootstrap addresses from JoinList.
	if err := cfg.InitNode(ctx); err != nil {
		return errors.Wrap(err, "initializing node config")
	}

	// Set up stopper and signal handling.
	stopper := stop.NewStopper()
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, unix.SIGINT, unix.SIGTERM)

	// Start server in goroutine.
	var s *server.Server
	errChan := make(chan error, 1)

	go func() {
		defer log.Flush()

		if err := func() error {
			var err error
			s, err = server.NewServer(cfg, stopper)
			if err != nil {
				return errors.Wrap(err, "failed to create server")
			}

			if err := s.PreStart(ctx); err != nil {
				return errors.Wrap(err, "server pre-start failed")
			}

			if cfg.AutoInitializeCluster {
				if err := s.AcceptInternalClients(ctx); err != nil {
					return err
				}
				if err := s.RunInitialSQL(ctx, true, "", ""); err != nil {
					return err
				}
			}

			if err := s.AcceptClients(ctx); err != nil {
				return errors.Wrap(err, "accepting clients failed")
			}

			nodeID := int(s.NodeID())
			storeID := int(s.GetFirstStoreID())

			// Register the node using the real NodeID assigned by CockroachDB,
			// keyed by the operator-assigned ratel node ID.
			if opts.nodesStore != nil {
				reg := storage.NodeRegistration{
					NodeID:      nodeID,
					RatelNodeID: opts.ratelNodeID,
					StoreID:     storeID,
					Addr:        cfg.AdvertiseAddr,
					SQLAddr:     cfg.SQLAdvertiseAddr,
					HTTPAddr:    cfg.HTTPAddr,
				}
				if regErr := storage.RegisterNode(ctx, opts.nodesStore, reg); regErr != nil {
					return errors.Wrap(regErr, "registering node")
				}

				// Start background heartbeat goroutine.
				heartbeatCtx := context.Background()
				go runHeartbeat(heartbeatCtx, stopper.ShouldQuiesce(), opts.nodesStore, reg)
			}

			fmt.Fprintf(os.Stderr, "Node %d (store %d) is ready. SQL address: %s, HTTP address: %s\n",
				nodeID, storeID, cfg.SQLAdvertiseAddr, cfg.HTTPAddr)
			return nil
		}(); err != nil {
			errChan <- err
		}
	}()

	// Wait for shutdown.
	select {
	case err := <-errChan:
		return err
	case <-stopper.ShouldQuiesce():
		<-stopper.IsStopped()
		return nil
	case sig := <-signalCh:
		fmt.Fprintf(os.Stderr, "\nReceived signal %v, shutting down...\n", sig)
		go func() {
			drainCtx := context.Background()
			if s != nil {
				s.Drain(drainCtx, false)
			}
			stopper.Stop(drainCtx)
		}()
		<-stopper.IsStopped()
		return nil
	}
}

// runHeartbeat periodically updates the node registration's LastHeartbeat.
func runHeartbeat(ctx context.Context, quiesce <-chan struct{}, store remote.Storage, reg storage.NodeRegistration) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-quiesce:
			return
		case <-ticker.C:
			if err := storage.HeartbeatNode(ctx, store, reg); err != nil {
				fmt.Fprintf(os.Stderr, "heartbeat failed: %v\n", err)
			}
		}
	}
}

func runRatelDeploy(cmd *cobra.Command, args []string) error {
	target := args[0]
	jsFile := args[1]

	script, err := os.ReadFile(jsFile)
	if err != nil {
		return errors.Wrapf(err, "reading %s", jsFile)
	}

	base := filepath.Base(jsFile)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	compatDate := ratelDeployCompatDate
	doClasses := append([]string(nil), ratelDeployDOClasses...)

	cfg, err := readRatelWorkerConfig(jsFile, ratelDeployConfig)
	if err != nil {
		return err
	}
	if cfg.Name != "" {
		name = cfg.Name
	}
	if cfg.CompatibilityDate != "" {
		compatDate = cfg.CompatibilityDate
	}
	if len(cfg.DOClasses) > 0 {
		doClasses = cfg.DOClasses
	}
	if ratelDeployName != "" && ratelDeployFlagChanged(cmd, "name") {
		name = ratelDeployName
	}
	if ratelDeployCompatDate != "" && ratelDeployFlagChanged(cmd, "compat-date") {
		compatDate = ratelDeployCompatDate
	}
	if len(ratelDeployDOClasses) > 0 && ratelDeployFlagChanged(cmd, "do-class") {
		doClasses = ratelDeployDOClasses
	}
	if name == "" {
		return errors.Newf("cannot derive worker name from %q", jsFile)
	}

	var httpAddr string
	if strings.Contains(target, "://") {
		cs, err := storage.ClusterStorageFromURL(target)
		if err != nil {
			return err
		}
		ctx := context.Background()
		nodes, err := storage.ListNodes(ctx, cs.Nodes)
		if err != nil {
			return errors.Wrap(err, "listing nodes")
		}
		if len(nodes) == 0 {
			return errors.New("no nodes found; is the cluster running?")
		}
		httpAddr = nodes[0].HTTPAddr
	} else {
		httpAddr = target
	}

	if !strings.HasPrefix(httpAddr, "http") {
		httpAddr = "http://" + httpAddr
	}

	deployURL := fmt.Sprintf("%s/api/v2/workers/%s/", httpAddr, name)
	req, err := http.NewRequest("PUT", deployURL, bytes.NewReader(script))
	if err != nil {
		return errors.Wrap(err, "creating request")
	}
	req.Header.Set("Content-Type", "application/javascript")
	if compatDate != "" {
		req.Header.Set("X-Compat-Date", compatDate)
	}
	bindings := struct {
		DurableObjects []struct {
			ClassName string `json:"class_name"`
		} `json:"durable_objects,omitempty"`
		Assets []ratelWorkerAsset `json:"assets,omitempty"`
	}{}
	if len(doClasses) > 0 {
		for _, cls := range doClasses {
			bindings.DurableObjects = append(bindings.DurableObjects, struct {
				ClassName string `json:"class_name"`
			}{ClassName: cls})
		}
	}
	if cfg.AssetsDir != "" {
		assets, err := readRatelWorkerAssets(cfg.AssetsDir)
		if err != nil {
			return err
		}
		bindings.Assets = assets
	}
	if len(bindings.DurableObjects) > 0 || len(bindings.Assets) > 0 {
		bindingsJSON, err := json.Marshal(bindings)
		if err != nil {
			return errors.Wrap(err, "marshaling worker bindings")
		}
		req.Header.Set("X-Bindings", string(bindingsJSON))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrapf(err, "deploying to %s", deployURL)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return errors.Newf("deploy failed (%s): %s", resp.Status, string(body))
	}

	var result struct {
		Name    string `json:"name"`
		Version int64  `json:"version"`
	}
	if err := json.Unmarshal(body, &result); err == nil {
		fmt.Fprintf(os.Stderr, "Deployed %s v%d\n", result.Name, result.Version)
	} else {
		fmt.Fprintf(os.Stderr, "Deployed %s\n", name)
	}
	return nil
}

type ratelWorkerConfig struct {
	Name              string
	CompatibilityDate string
	DOClasses         []string
	AssetsDir         string
}

func ratelDeployFlagChanged(cmd *cobra.Command, name string) bool {
	return cmd == nil || cmd.Flags().Changed(name)
}

func readRatelWorkerConfig(jsFile, configPath string) (ratelWorkerConfig, error) {
	if configPath == "" {
		configPath = filepath.Join(filepath.Dir(jsFile), "worker.jsonc")
		if _, err := os.Stat(configPath); err != nil {
			if os.IsNotExist(err) {
				return ratelWorkerConfig{}, nil
			}
			return ratelWorkerConfig{}, errors.Wrapf(err, "checking %s", configPath)
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return ratelWorkerConfig{}, errors.Wrapf(err, "reading %s", configPath)
	}
	cfg, err := parseRatelWorkerJSONC(data)
	if err != nil {
		return ratelWorkerConfig{}, errors.Wrapf(err, "parsing %s", configPath)
	}
	if cfg.AssetsDir != "" && !filepath.IsAbs(cfg.AssetsDir) {
		cfg.AssetsDir = filepath.Join(filepath.Dir(configPath), cfg.AssetsDir)
	}
	return cfg, nil
}

func parseRatelWorkerJSONC(data []byte) (ratelWorkerConfig, error) {
	jsonData := stripJSONCTrailingCommas(stripJSONCComments(string(data)))
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(jsonData), &raw); err != nil {
		return ratelWorkerConfig{}, err
	}

	var cfg ratelWorkerConfig
	if err := unmarshalOptionalString(raw, "name", &cfg.Name); err != nil {
		return cfg, err
	}
	if err := unmarshalOptionalString(raw, "compatibility_date", &cfg.CompatibilityDate); err != nil {
		return cfg, err
	}
	if cfg.CompatibilityDate == "" {
		if err := unmarshalOptionalString(raw, "compat_date", &cfg.CompatibilityDate); err != nil {
			return cfg, err
		}
	}
	if err := unmarshalOptionalStringArray(raw, "do_classes", &cfg.DOClasses); err != nil {
		return cfg, err
	}
	if err := appendDurableObjectClasses(raw["durable_objects"], &cfg.DOClasses); err != nil {
		return cfg, err
	}
	if err := unmarshalAssets(raw["assets"], &cfg.AssetsDir); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func unmarshalAssets(raw json.RawMessage, dest *string) error {
	if len(raw) == 0 {
		return nil
	}
	var dir string
	if err := json.Unmarshal(raw, &dir); err == nil {
		*dest = dir
		return nil
	}
	var obj struct {
		Directory string `json:"directory"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return errors.Wrap(err, "assets must be a string or object")
	}
	*dest = obj.Directory
	return nil
}

type ratelWorkerAsset struct {
	Path        string `json:"path"`
	ContentType string `json:"content_type,omitempty"`
	DataBase64  string `json:"data_base64"`
}

func readRatelWorkerAssets(dir string) ([]ratelWorkerAsset, error) {
	var assets []ratelWorkerAsset
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		contentType := mime.TypeByExtension(filepath.Ext(path))
		if contentType == "" {
			contentType = http.DetectContentType(data)
		}
		assets = append(assets, ratelWorkerAsset{
			Path:        "/" + strings.TrimPrefix(rel, "/"),
			ContentType: contentType,
			DataBase64:  base64.StdEncoding.EncodeToString(data),
		})
		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "reading assets directory %s", dir)
	}
	if len(assets) == 0 {
		return nil, errors.Newf("assets directory %s is empty", dir)
	}
	return assets, nil
}

func unmarshalOptionalString(raw map[string]json.RawMessage, key string, dest *string) error {
	if len(raw[key]) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw[key], dest); err != nil {
		return errors.Wrapf(err, "%s must be a string", key)
	}
	return nil
}

func unmarshalOptionalStringArray(raw map[string]json.RawMessage, key string, dest *[]string) error {
	if len(raw[key]) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw[key], dest); err != nil {
		return errors.Wrapf(err, "%s must be an array of strings", key)
	}
	return nil
}

func appendDurableObjectClasses(raw json.RawMessage, dest *[]string) error {
	if len(raw) == 0 {
		return nil
	}

	var arr []struct {
		ClassName string `json:"class_name"`
	}
	if err := json.Unmarshal(raw, &arr); err == nil {
		for _, binding := range arr {
			if binding.ClassName != "" {
				*dest = append(*dest, binding.ClassName)
			}
		}
		return nil
	}

	var obj struct {
		Bindings []struct {
			ClassName string `json:"class_name"`
		} `json:"bindings"`
		Classes []string `json:"classes"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return errors.Wrap(err, "durable_objects must be an array or object")
	}
	for _, cls := range obj.Classes {
		if cls != "" {
			*dest = append(*dest, cls)
		}
	}
	for _, binding := range obj.Bindings {
		if binding.ClassName != "" {
			*dest = append(*dest, binding.ClassName)
		}
	}
	return nil
}

func stripJSONCComments(s string) string {
	var b strings.Builder
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inString {
			b.WriteByte(ch)
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			b.WriteByte(ch)
			continue
		}
		if ch == '/' && i+1 < len(s) {
			switch s[i+1] {
			case '/':
				for i < len(s) && s[i] != '\n' {
					i++
				}
				if i < len(s) {
					b.WriteByte(s[i])
				}
				continue
			case '*':
				i += 2
				for i+1 < len(s) && !(s[i] == '*' && s[i+1] == '/') {
					if s[i] == '\n' {
						b.WriteByte('\n')
					}
					i++
				}
				i++
				continue
			}
		}
		b.WriteByte(ch)
	}
	return b.String()
}

func stripJSONCTrailingCommas(s string) string {
	var b strings.Builder
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inString {
			b.WriteByte(ch)
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			b.WriteByte(ch)
			continue
		}
		if ch == ',' {
			j := i + 1
			for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\r' || s[j] == '\n') {
				j++
			}
			if j < len(s) && (s[j] == '}' || s[j] == ']') {
				continue
			}
		}
		b.WriteByte(ch)
	}
	return b.String()
}

func runRatelSQL(cmd *cobra.Command, args []string) error {
	arg := args[0]

	// Determine connection addresses: either direct host:port or discovered from S3.
	var addrs []string
	var certsDir string

	if strings.Contains(arg, "://") {
		// Storage URL — discover nodes from S3.
		cs, err := storage.ClusterStorageFromURL(arg)
		if err != nil {
			return err
		}
		ctx := context.Background()
		nodes, err := storage.ListNodes(ctx, cs.Nodes)
		if err != nil {
			return errors.Wrap(err, "listing nodes")
		}
		if len(nodes) == 0 {
			return errors.New("no nodes found; is the cluster running?")
		}
		if ratelSQLHost != "" {
			addrs = []string{ratelSQLHost}
		} else {
			for _, node := range nodes {
				addrs = append(addrs, node.SQLAddr)
			}
		}
		if ratelTLS {
			clusterUUID, err := storage.ReadClusterID(ctx, cs.Metadata)
			if err != nil {
				return errors.Wrap(err, "reading cluster ID")
			}
			ld := ratelLocalDir(clusterUUID.String())
			passphrase, err := ratelPassphrase(false /* confirm */)
			if err != nil {
				return err
			}
			certsDir = filepath.Join(ld, "certs")
			if err := storage.DownloadClientCerts(ctx, cs.Certs, certsDir, passphrase); err != nil {
				return err
			}
		}
	} else {
		// Direct host:port.
		addrs = []string{arg}
	}

	// Set up SQL shell config.
	cliCfg := &clicfg.Context{}
	sqlCfg := &clisqlcfg.Context{
		CliCtx:  cliCfg,
		ConnCtx: &clisqlclient.Context{CliCtx: cliCfg},
		ExecCtx: &clisqlexec.Context{CliCtx: cliCfg},
	}
	sqlCfg.LoadDefaults(os.Stdout, os.Stderr)
	sqlCfg.Database = "defaultdb"
	sqlCfg.User = "root"
	sqlCfg.ApplicationName = "$ ratel sql"
	sqlCfg.ConnectTimeout = 5
	sqlCfg.ShellCtx.ExecStmts = ratelSQLExecStmts

	closeFn, err := sqlCfg.Open(os.Stdin)
	if err != nil {
		return err
	}
	defer closeFn()

	if sqlCfg.CliCtx.IsInteractive {
		fmt.Print(`#
# Welcome to the Ratel SQL shell.
# All statements must be terminated by a semicolon.
# To exit, type: \q.
#
`)
	}

	var conn clisqlclient.Conn
	var lastErr error
	for _, addr := range addrs {
		var connURL string
		if ratelTLS {
			connURL = fmt.Sprintf(
				"postgresql://root@%s/defaultdb?sslmode=verify-full&sslrootcert=%s&sslcert=%s&sslkey=%s",
				addr,
				filepath.Join(certsDir, "ca.crt"),
				filepath.Join(certsDir, "client.root.crt"),
				filepath.Join(certsDir, "client.root.key"),
			)
		} else {
			connURL = fmt.Sprintf("postgresql://root@%s/defaultdb?sslmode=disable", addr)
		}
		conn, lastErr = sqlCfg.MakeConn(connURL)
		if lastErr == nil {
			break
		}
		fmt.Fprintf(os.Stderr, "node (%s): %v\n", addr, lastErr)
	}
	if conn == nil {
		return errors.Wrap(lastErr, "could not connect to any node")
	}
	defer func() { _ = conn.Close() }()

	return sqlCfg.Run(context.Background(), conn)
}
