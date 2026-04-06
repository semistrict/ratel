// deploy/fly-cluster creates a globally distributed Ratel cluster
// on Fly.io Machines with Tigris object storage as the cluster identity.
//
// Ratel uses a single S3 URL as the cluster identity: certs, node
// discovery, and shared storage are all derived from it. Tigris
// provides S3-compatible storage on Fly.io with no egress fees.
//
// Usage:
//
//	export FLY_API_TOKEN=$(fly auth token)
//	go run ./deploy/fly-cluster [flags]
//
// Prerequisites:
//
//	fly storage create -n ratel-store         # create Tigris bucket
//	# note the AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, BUCKET_NAME
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const apiBase = "https://api.machines.dev"

var (
	flagApp     = flag.String("app", "semistrict-ratel", "Fly app name")
	flagOrg     = flag.String("org", "personal", "Fly org slug")
	flagRegions = flag.String("regions", "iad,lhr,sin", "Comma-separated Fly regions")
	flagImage   = flag.String("image", "registry.fly.io/semistrict-ratel:latest", "Docker image")
	flagCPUs    = flag.Int("cpus", 2, "CPUs per machine")
	flagMemory  = flag.Int("memory", 4096, "Memory in MB per machine")
	flagBucket  = flag.String("bucket", "", "Tigris bucket name (required)")
	flagKey     = flag.String("key", "", "AWS_ACCESS_KEY_ID for Tigris")
	flagSecret  = flag.String("secret", "", "AWS_SECRET_ACCESS_KEY for Tigris")
	flagDestroy = flag.Bool("destroy", false, "Tear down the cluster")
	flagStatus  = flag.Bool("status", false, "Show cluster status")
	flagSQL     = flag.Bool("sql", false, "Connect via ratel sql")
)

const tigrisEndpoint = "https://fly.storage.tigris.dev"

func storageURL() string {
	// Use app name as prefix so each deploy gets a clean namespace.
	return fmt.Sprintf("s3://%s/%s?endpoint=%s&region=auto",
		*flagBucket, *flagApp, tigrisEndpoint)
}

func main() {
	flag.Parse()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	token := os.Getenv("FLY_API_TOKEN")
	if token == "" {
		slog.Error("FLY_API_TOKEN not set. Run: export FLY_API_TOKEN=$(fly auth token)")
		os.Exit(1)
	}

	if !*flagDestroy && !*flagStatus {
		if *flagBucket == "" || *flagKey == "" || *flagSecret == "" {
			slog.Error("--bucket, --key, and --secret are required. Create a bucket first:\n  fly storage create -n ratel-store")
			os.Exit(1)
		}
	}

	c := &client{token: token, app: *flagApp}
	regions := strings.Split(*flagRegions, ",")

	switch {
	case *flagDestroy:
		destroy(c)
	case *flagStatus:
		status(c)
	case *flagSQL:
		connectSQL()
	default:
		create(c, regions)
	}
}

// --- Fly API client ---

type client struct {
	token string
	app   string
}

func (c *client) do(method, path string, body interface{}) ([]byte, int, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, apiBase+path, r)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return data, resp.StatusCode, nil
}

// --- Data types ---

type createAppReq struct {
	AppName string `json:"app_name"`
	OrgSlug string `json:"org_slug"`
}

type machineConfig struct {
	Image    string            `json:"image"`
	Env      map[string]string `json:"env,omitempty"`
	Guest    guestConfig       `json:"guest"`
	Services []service         `json:"services"`
	Init     *initConfig       `json:"init,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type initConfig struct {
	Cmd []string `json:"cmd,omitempty"`
}

type guestConfig struct {
	CPUs    int    `json:"cpus"`
	MemMB   int    `json:"memory_mb"`
	CPUKind string `json:"cpu_kind"`
}

type service struct {
	Protocol     string `json:"protocol"`
	InternalPort int    `json:"internal_port"`
	Ports        []port `json:"ports"`
}

type port struct {
	Port     int      `json:"port"`
	Handlers []string `json:"handlers"`
}

type createMachineReq struct {
	Name   string        `json:"name"`
	Region string        `json:"region"`
	Config machineConfig `json:"config"`
}

type machine struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	State     string `json:"state"`
	Region    string `json:"region"`
	PrivateIP string `json:"private_ip"`
}

// --- Create cluster ---

func create(c *client, regions []string) {
	slog.Info("creating cluster", "app", c.app, "regions", regions, "bucket", *flagBucket)

	// 1. Create app
	slog.Info("creating app")
	data, code, err := c.do("POST", "/v1/apps", createAppReq{
		AppName: c.app,
		OrgSlug: *flagOrg,
	})
	if err != nil {
		slog.Error("creating app", "error", err)
		os.Exit(1)
	}
	if code == 409 || code == 422 {
		slog.Info("app already exists", "app", c.app)
	} else if code >= 300 {
		slog.Error("creating app", "status", code, "body", string(data))
		os.Exit(1)
	} else {
		slog.Info("app created", "app", c.app)
	}

	url := storageURL()

	// 2. Create machines.
	// First node: "ratel init <url>" — initializes the cluster.
	// Remaining nodes: "ratel join <url>" — discover peers from S3.
	// Ratel handles TLS certs, node discovery, and storage via the S3 URL.
	var machines []machine
	for i, region := range regions {
		nodeID := fmt.Sprintf("node-%d", i)
		nodeName := fmt.Sprintf("%s-%s", c.app, nodeID)
		listenAddr := "0.0.0.0:26257"
		httpAddr := "0.0.0.0:8080"

		var cmd []string
		if i == 0 {
			cmd = []string{
				"init", url,
				"--listen-addr", listenAddr,
				"--http-addr", httpAddr,
				"--no-passphrase",
				"--node-id", nodeID,
			}
		} else {
			cmd = []string{
				"join", url,
				"--listen-addr", listenAddr,
				"--http-addr", httpAddr,
				"--no-passphrase",
				"--node-id", nodeID,
			}
		}

		slog.Info("creating machine", "name", nodeName, "region", region, "cmd", cmd[0])
		machReq := createMachineReq{
			Name:   nodeName,
			Region: region,
			Config: machineConfig{
				Image: *flagImage,
				Env: map[string]string{
					"AWS_ACCESS_KEY_ID":     *flagKey,
					"AWS_SECRET_ACCESS_KEY": *flagSecret,
					"AWS_REGION":            "auto",
				},
				Guest: guestConfig{
					CPUs:    *flagCPUs,
					MemMB:   *flagMemory,
					CPUKind: "shared",
				},
				Init: &initConfig{Cmd: cmd},
				Services: []service{
					{
						Protocol:     "tcp",
						InternalPort: 26257,
						Ports: []port{
							{Port: 26257, Handlers: []string{"tls"}},
						},
					},
					{
						Protocol:     "tcp",
						InternalPort: 8080,
						Ports: []port{
							{Port: 443, Handlers: []string{"tls", "http"}},
						},
					},
				},
				Metadata: map[string]string{
					"fly_process_group": "ratel",
					"ratel_node_id":     nodeID,
				},
			},
		}

		data, code, err = c.do("POST", fmt.Sprintf("/v1/apps/%s/machines", c.app), machReq)
		if err != nil {
			slog.Error("creating machine", "error", err)
			os.Exit(1)
		}
		if code >= 300 {
			slog.Error("creating machine", "status", code, "body", string(data))
			os.Exit(1)
		}
		var m machine
		if err := json.Unmarshal(data, &m); err != nil {
			slog.Error("parsing machine response", "error", err)
			os.Exit(1)
		}
		machines = append(machines, m)
		slog.Info("machine created", "id", m.ID, "region", region, "ip", m.PrivateIP)

		// Wait for init node to start before creating join nodes,
		// so the cluster is bootstrapped before peers try to connect.
		if i == 0 {
			slog.Info("waiting for init node to start...")
			if err := waitForState(c, m.ID, "started", 120*time.Second); err != nil {
				slog.Error("init node did not start", "id", m.ID, "error", err)
				os.Exit(1)
			}
			// Give it a few seconds to bootstrap.
			time.Sleep(5 * time.Second)
		}
	}

	// 3. Wait for remaining machines to start
	slog.Info("waiting for join nodes to start...")
	for _, m := range machines[1:] {
		if err := waitForState(c, m.ID, "started", 120*time.Second); err != nil {
			slog.Error("machine did not start", "id", m.ID, "error", err)
		}
	}

	// 4. Print connection info
	fmt.Fprintf(os.Stderr, "\nCluster ready!\n\n")
	fmt.Fprintf(os.Stderr, "Nodes:\n")
	for i, m := range machines {
		fmt.Fprintf(os.Stderr, "  [%d] %s  region=%-3s  ip=%s\n",
			i, m.ID, regions[i], m.PrivateIP)
	}
	fmt.Fprintf(os.Stderr, "\nConnect via ratel sql:\n")
	fmt.Fprintf(os.Stderr, "  go run ./deploy/fly-cluster -sql -bucket %s -key %s -secret <secret>\n\n",
		*flagBucket, *flagKey)
	fmt.Fprintf(os.Stderr, "Or via fly proxy:\n")
	fmt.Fprintf(os.Stderr, "  fly proxy 26257:26257 -a %s\n\n", c.app)
}

func waitForState(c *client, machineID, target string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, code, err := c.do("GET", fmt.Sprintf("/v1/apps/%s/machines/%s", c.app, machineID), nil)
		if err != nil {
			return err
		}
		if code >= 300 {
			return fmt.Errorf("status %d: %s", code, data)
		}
		var m machine
		if err := json.Unmarshal(data, &m); err != nil {
			return err
		}
		if m.State == target {
			slog.Info("machine ready", "id", machineID, "state", target)
			return nil
		}
		slog.Info("waiting", "id", machineID, "state", m.State, "want", target)
		time.Sleep(3 * time.Second)
	}
	return fmt.Errorf("timeout waiting for state %q", target)
}

// --- Status ---

func status(c *client) {
	data, code, err := c.do("GET", fmt.Sprintf("/v1/apps/%s/machines", c.app), nil)
	if err != nil {
		slog.Error("listing machines", "error", err)
		os.Exit(1)
	}
	if code >= 300 {
		slog.Error("listing machines", "status", code, "body", string(data))
		os.Exit(1)
	}
	var machines []machine
	if err := json.Unmarshal(data, &machines); err != nil {
		slog.Error("parsing machines", "error", err)
		os.Exit(1)
	}

	fmt.Printf("%-18s %-25s %-6s %-40s %s\n", "ID", "NAME", "REGION", "IP", "STATE")
	for _, m := range machines {
		fmt.Printf("%-18s %-25s %-6s %-40s %s\n", m.ID, m.Name, m.Region, m.PrivateIP, m.State)
	}
}

// --- Destroy ---

func destroy(c *client) {
	slog.Info("destroying cluster", "app", c.app)

	data, code, err := c.do("GET", fmt.Sprintf("/v1/apps/%s/machines", c.app), nil)
	if err != nil {
		slog.Error("listing machines", "error", err)
		os.Exit(1)
	}
	if code >= 300 {
		slog.Error("listing machines", "status", code, "body", string(data))
		os.Exit(1)
	}
	var machines []machine
	if err := json.Unmarshal(data, &machines); err != nil {
		slog.Error("parsing machines", "error", err)
		os.Exit(1)
	}

	for _, m := range machines {
		slog.Info("stopping machine", "id", m.ID)
		c.do("POST", fmt.Sprintf("/v1/apps/%s/machines/%s/stop", c.app, m.ID), nil)
	}
	for _, m := range machines {
		waitForState(c, m.ID, "stopped", 30*time.Second)
		slog.Info("destroying machine", "id", m.ID)
		_, code, err := c.do("DELETE", fmt.Sprintf("/v1/apps/%s/machines/%s?force=true", c.app, m.ID), nil)
		if err != nil {
			slog.Error("destroying machine", "id", m.ID, "error", err)
		} else if code >= 300 {
			slog.Error("destroying machine", "id", m.ID, "status", code)
		} else {
			slog.Info("machine destroyed", "id", m.ID)
		}
	}

	slog.Info("deleting app", "app", c.app)
	_, code, err = c.do("DELETE", fmt.Sprintf("/v1/apps/%s", c.app), nil)
	if err != nil {
		slog.Error("deleting app", "error", err)
	} else if code >= 300 {
		slog.Error("deleting app", "status", code)
	} else {
		slog.Info("app deleted")
	}
}

// --- SQL connect ---

func connectSQL() {
	url := storageURL()
	cmd := exec.Command("ratel", "sql", url)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		slog.Error("sql", "error", err)
		os.Exit(1)
	}
}
