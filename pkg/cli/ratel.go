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
	"os"
	"os/signal"
	"path/filepath"

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
)

var ratelListenAddr string
var ratelHTTPAddr string

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
	Use:   "sql <storage-url>",
	Short: "Connect a SQL shell to a cluster",
	Long: `Connect to a running ratel cluster's SQL interface. A node is discovered from
the node registry in the storage URL, and client TLS certificates are downloaded
for authentication.`,
	Args: cobra.ExactArgs(1),
	RunE: runRatelSQL,
}

func init() {
	ratelCmd.AddCommand(ratelInitCmd, ratelJoinCmd, ratelSQLCmd)

	for _, cmd := range []*cobra.Command{ratelInitCmd, ratelJoinCmd} {
		cmd.Flags().StringVar(&ratelListenAddr, "listen-addr", "localhost:26257",
			"Address to listen on for RPC and SQL connections")
		cmd.Flags().StringVar(&ratelHTTPAddr, "http-addr", "localhost:8080",
			"Address to listen on for the admin HTTP interface")
	}

	// SQL-specific flags.
	ratelSQLCmd.Flags().VarP(&ratelSQLExecStmts, cliflags.Execute.Name, cliflags.Execute.Shorthand, cliflags.Execute.Description)
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

// ratelLocalDir returns a stable temp directory derived from the cluster URL.
func ratelLocalDir(clusterURL string) string {
	h := sha256.Sum256([]byte(clusterURL))
	return filepath.Join(os.TempDir(), fmt.Sprintf("ratel-%x", h[:8]))
}

func runRatelInit(cmd *cobra.Command, args []string) error {
	clusterURL := args[0]

	cs, err := storage.ClusterStorageFromURL(clusterURL)
	if err != nil {
		return err
	}

	ctx := context.Background()

	// Check that cluster is not already initialized.
	nodes, err := storage.ListNodes(ctx, cs.Nodes)
	if err != nil {
		return errors.Wrap(err, "checking existing nodes")
	}
	if len(nodes) > 0 {
		return errors.Newf("cluster already initialized: found %d node(s) at %s", len(nodes), clusterURL)
	}

	// Check if certs already exist; generate if not.
	exists, err := storage.CertsExist(ctx, cs.Certs)
	if err != nil {
		return err
	}
	if !exists {
		fmt.Fprintln(os.Stderr, "Generating TLS certificates...")
		if err := storage.GenerateAndUploadCerts(ctx, cs.Certs); err != nil {
			return err
		}
	}

	// Download certs to local dir.
	ld := ratelLocalDir(clusterURL)
	certsDir := filepath.Join(ld, "certs")
	storeDir := filepath.Join(ld, "store")
	if err := storage.DownloadCerts(ctx, cs.Certs, certsDir); err != nil {
		return err
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
	})
}

func runRatelJoin(cmd *cobra.Command, args []string) error {
	clusterURL := args[0]

	cs, err := storage.ClusterStorageFromURL(clusterURL)
	if err != nil {
		return err
	}

	ctx := context.Background()

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

	// Check certs exist.
	exists, err := storage.CertsExist(ctx, cs.Certs)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("no certificates found at storage URL; run 'ratel init' first")
	}

	// Download certs to local dir.
	ld := ratelLocalDir(clusterURL)
	certsDir := filepath.Join(ld, "certs")
	storeDir := filepath.Join(ld, "store")
	if err := storage.DownloadCerts(ctx, cs.Certs, certsDir); err != nil {
		return err
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
}

func ratelStartServer(ctx context.Context, opts ratelServerOpts) error {
	// Build server config.
	st := cluster.MakeClusterSettings()
	logcrash.SetGlobalSettings(&st.SV)

	cfg := server.MakeConfig(ctx, st)
	cfg.Insecure = false
	cfg.SSLCertsDir = opts.certsDir
	cfg.Addr = opts.listenAddr
	cfg.AdvertiseAddr = opts.listenAddr
	cfg.SQLAddr = opts.listenAddr
	cfg.SQLAdvertiseAddr = opts.listenAddr
	cfg.HTTPAddr = opts.httpAddr
	cfg.AutoInitializeCluster = opts.autoInitialize
	cfg.JoinList = opts.joinList
	cfg.StorageEngine = enginepb.EngineTypePebble

	// Configure store with remote storage.
	if err := os.MkdirAll(opts.storeDir, 0755); err != nil {
		return errors.Wrapf(err, "creating store dir %s", opts.storeDir)
	}
	storeSpec := base.StoreSpec{
		Path:              opts.storeDir,
		RemoteStoragePath: opts.clusterURL,
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

			// Register the node using the real NodeID assigned by CockroachDB.
			nodeID := int(s.NodeID())
			reg := storage.NodeRegistration{
				NodeID:   nodeID,
				Addr:     cfg.AdvertiseAddr,
				SQLAddr:  cfg.SQLAdvertiseAddr,
				HTTPAddr: cfg.HTTPAddr,
			}
			if regErr := storage.RegisterNode(ctx, opts.nodesStore, reg); regErr != nil {
				return errors.Wrap(regErr, "registering node")
			}

			fmt.Fprintf(os.Stderr, "Node %d is ready. SQL address: %s, HTTP address: %s\n",
				nodeID, cfg.SQLAdvertiseAddr, cfg.HTTPAddr)
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

func runRatelSQL(cmd *cobra.Command, args []string) error {
	clusterURL := args[0]

	cs, err := storage.ClusterStorageFromURL(clusterURL)
	if err != nil {
		return err
	}

	ctx := context.Background()

	// Discover nodes.
	nodes, err := storage.ListNodes(ctx, cs.Nodes)
	if err != nil {
		return errors.Wrap(err, "listing nodes")
	}
	if len(nodes) == 0 {
		return errors.New("no nodes found; is the cluster running?")
	}

	// Download client certs.
	ld := ratelLocalDir(clusterURL)
	certsDir := filepath.Join(ld, "certs")
	if err := storage.DownloadClientCerts(ctx, cs.Certs, certsDir); err != nil {
		return err
	}

	// Set up SQL shell config, following cockroach-sql pattern.
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

	// Try each registered node until one accepts a connection.
	var conn clisqlclient.Conn
	var lastErr error
	for _, node := range nodes {
		connURL := fmt.Sprintf(
			"postgresql://root@%s/defaultdb?sslmode=verify-full&sslrootcert=%s&sslcert=%s&sslkey=%s",
			node.SQLAddr,
			filepath.Join(certsDir, "ca.crt"),
			filepath.Join(certsDir, "client.root.crt"),
			filepath.Join(certsDir, "client.root.key"),
		)
		conn, lastErr = sqlCfg.MakeConn(connURL)
		if lastErr == nil {
			break
		}
		fmt.Fprintf(os.Stderr, "node %d (%s): %v\n", node.NodeID, node.SQLAddr, lastErr)
	}
	if conn == nil {
		return errors.Wrap(lastErr, "could not connect to any node")
	}
	defer func() { _ = conn.Close() }()

	return sqlCfg.Run(conn)
}
