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
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/pebble/objstorage/remote"
	"github.com/semistrict/ratel/pkg/base"
	"github.com/semistrict/ratel/pkg/cli/clicfg"
	"github.com/semistrict/ratel/pkg/cli/clierror"
	"github.com/semistrict/ratel/pkg/cli/cliflags"
	"github.com/semistrict/ratel/pkg/cli/clisqlcfg"
	"github.com/semistrict/ratel/pkg/cli/clisqlclient"
	"github.com/semistrict/ratel/pkg/cli/clisqlexec"
	"github.com/semistrict/ratel/pkg/cli/clisqlshell"
	"github.com/semistrict/ratel/pkg/cli/exit"
	"github.com/semistrict/ratel/pkg/server"
	"github.com/semistrict/ratel/pkg/settings/cluster"
	"github.com/semistrict/ratel/pkg/storage"
	"github.com/semistrict/ratel/pkg/storage/enginepb"
	"github.com/semistrict/ratel/pkg/util/log"
	"github.com/semistrict/ratel/pkg/util/log/logcrash"
	"github.com/semistrict/ratel/pkg/util/stop"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

var ratelListenAddr string
var ratelHTTPAddr string
var ratelNoPassphrase bool
var ratelNodeID string
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
	ratelCmd.AddCommand(ratelInitCmd, ratelJoinCmd, ratelSQLCmd, ratelStartLocalCmd)

	ratelStartLocalCmd.Flags().StringVar(&ratelListenAddr, "listen-addr", "localhost:26257",
		"Address to listen on for RPC and SQL connections")
	ratelStartLocalCmd.Flags().StringVar(&ratelHTTPAddr, "http-addr", "localhost:8080",
		"Address to listen on for the admin HTTP interface")

	for _, cmd := range []*cobra.Command{ratelInitCmd, ratelJoinCmd} {
		cmd.Flags().StringVar(&ratelListenAddr, "listen-addr", "localhost:26257",
			"Address to listen on for RPC and SQL connections")
		cmd.Flags().StringVar(&ratelHTTPAddr, "http-addr", "localhost:8080",
			"Address to listen on for the admin HTTP interface")
		cmd.Flags().BoolVar(&ratelTLS, "tls", false,
			"Enable application-level TLS (generates and manages certificates via S3)")
		cmd.Flags().BoolVar(&ratelNoPassphrase, "no-passphrase", false,
			"Do not encrypt the CA key (skip passphrase prompt, only with --tls)")
		cmd.Flags().StringVar(&ratelNodeID, "node-id", "",
			"Stable operator-assigned node identity (e.g. ratel-1)")
		_ = cmd.MarkFlagRequired("node-id")
	}

	// SQL-specific flags.
	ratelSQLCmd.Flags().VarP(&ratelSQLExecStmts, cliflags.Execute.Name, cliflags.Execute.Shorthand, cliflags.Execute.Description)
	ratelSQLCmd.Flags().StringVar(&ratelSQLHost, "host", "",
		"Override the node address to connect to (e.g. localhost:26257 when using fly proxy)")
	ratelSQLCmd.Flags().BoolVar(&ratelTLS, "tls", false,
		"Connect using TLS (download client certs from S3)")
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

// ratelLocalDir returns a stable temp directory derived from the cluster URL.
func ratelLocalDir(clusterURL string) string {
	h := sha256.Sum256([]byte(clusterURL))
	return filepath.Join(os.TempDir(), fmt.Sprintf("ratel-%x", h[:8]))
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
		certsDir:       "",   // insecure
		storeDir:       storeDir,
		joinList:       nil,  // single-node
		autoInitialize: true,
		nodesStore:     nil,  // no S3 node registry
		ratelNodeID:    "local",
	})
}

func runRatelInit(cmd *cobra.Command, args []string) error {
	clusterURL := args[0]

	cs, err := storage.ClusterStorageFromURL(clusterURL)
	if err != nil {
		return err
	}

	ctx := context.Background()

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

	ld := ratelLocalDir(clusterURL)
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

	// Check for duplicate running node with same --node-id.
	if err := checkNodeLiveness(ctx, cs.Nodes, ratelNodeID); err != nil {
		return err
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

	ld := ratelLocalDir(clusterURL)
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
				if err := runInitialSQL(ctx, s, true, "", ""); err != nil {
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
			ld := ratelLocalDir(arg)
			certsDir = filepath.Join(ld, "certs")
			if err := storage.DownloadClientCerts(ctx, cs.Certs, certsDir); err != nil {
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

	return sqlCfg.Run(conn)
}
