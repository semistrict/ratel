// deploy/fly-cluster creates a globally distributed Ratel cluster
// on Fly.io Machines with Tigris object storage for backups.
//
// Usage:
//
//	export FLY_API_TOKEN=$(fly auth token)
//	go run ./deploy/fly-cluster [flags]
//
// Flags:
//
//	-app        App name (default: ratel-cluster)
//	-org        Fly org slug (default: personal)
//	-regions    Comma-separated regions (default: iad,lhr,sin)
//	-image      Docker image (default: ghcr.io/semistrict/ratel:latest)
//	-cpus       CPUs per machine (default: 2)
//	-memory     Memory in MB per machine (default: 4096)
//	-destroy    Tear down the cluster
//	-status     Show cluster status
//	-sql        Connect via cockroach sql
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
	flagApp     = flag.String("app", "ratel-cluster", "Fly app name")
	flagOrg     = flag.String("org", "personal", "Fly org slug")
	flagRegions = flag.String("regions", "iad,lhr,sin", "Comma-separated Fly regions")
	flagImage   = flag.String("image", "ghcr.io/semistrict/ratel:latest", "Docker image")
	flagCPUs    = flag.Int("cpus", 2, "CPUs per machine")
	flagMemory  = flag.Int("memory", 4096, "Memory in MB per machine")
	flagDestroy = flag.Bool("destroy", false, "Tear down the cluster")
	flagStatus  = flag.Bool("status", false, "Show cluster status")
	flagSQL     = flag.Bool("sql", false, "Connect via cockroach sql")
)

func main() {
	flag.Parse()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	token := os.Getenv("FLY_API_TOKEN")
	if token == "" {
		slog.Error("FLY_API_TOKEN not set. Run: export FLY_API_TOKEN=$(fly auth token)")
		os.Exit(1)
	}

	c := &client{token: token, app: *flagApp}
	regions := strings.Split(*flagRegions, ",")

	switch {
	case *flagDestroy:
		destroy(c)
	case *flagStatus:
		status(c)
	case *flagSQL:
		connectSQL(c)
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
	Env      map[string]string `json:"env"`
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
	slog.Info("creating cluster", "app", c.app, "regions", regions)

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
	if code == 409 {
		slog.Info("app already exists", "app", c.app)
	} else if code >= 300 {
		slog.Error("creating app", "status", code, "body", string(data))
		os.Exit(1)
	} else {
		slog.Info("app created", "app", c.app)
	}

	// 2. Build join addresses using Fly internal DNS.
	// Fly machines get <name>.vm.<app>.internal as their DNS name.
	var joinAddrs []string
	for i := range regions {
		joinAddrs = append(joinAddrs, fmt.Sprintf("%s-node-%d.vm.%s.internal:26257", c.app, i, c.app))
	}
	joinStr := strings.Join(joinAddrs, ",")

	// 3. Create machines — no volumes, CockroachDB replicates data across
	// nodes so ephemeral storage is fine. Lost nodes re-replicate from peers.
	var machines []machine
	for i, region := range regions {
		nodeName := fmt.Sprintf("%s-node-%d", c.app, i)
		advertiseAddr := fmt.Sprintf("%s.vm.%s.internal:26257", nodeName, c.app)

		slog.Info("creating machine", "name", nodeName, "region", region)
		machReq := createMachineReq{
			Name:   nodeName,
			Region: region,
			Config: machineConfig{
				Image: *flagImage,
				Guest: guestConfig{
					CPUs:    *flagCPUs,
					MemMB:   *flagMemory,
					CPUKind: "shared",
				},
				Init: &initConfig{
					Cmd: []string{
						"/ratel", "start",
						"--insecure",
						"--advertise-addr", advertiseAddr,
						"--listen-addr", "0.0.0.0:26257",
						"--http-addr", "0.0.0.0:8080",
						"--join", joinStr,
						"--locality", fmt.Sprintf("region=%s", region),
					},
				},
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
					"ratel_node_index":  fmt.Sprintf("%d", i),
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
	}

	// 4. Wait for machines to start
	slog.Info("waiting for machines to start...")
	for _, m := range machines {
		if err := waitForState(c, m.ID, "started", 120*time.Second); err != nil {
			slog.Error("machine did not start", "id", m.ID, "error", err)
		}
	}

	// 5. Initialize cluster
	slog.Info("initializing cluster...")
	fmt.Fprintf(os.Stderr, "\nTo initialize the cluster, run:\n")
	fmt.Fprintf(os.Stderr, "  fly ssh console -a %s -s -C '/ratel init --insecure --host=%s'\n\n",
		c.app, joinAddrs[0])

	// 6. Print connection info
	fmt.Fprintf(os.Stderr, "Nodes:\n")
	for i, m := range machines {
		fmt.Fprintf(os.Stderr, "  [%d] %s  region=%-3s  ip=%s  state=%s\n",
			i, m.ID, regions[i], m.PrivateIP, m.State)
	}
	fmt.Fprintf(os.Stderr, "\nConnect:\n")
	fmt.Fprintf(os.Stderr, "  fly proxy 26257:26257 -a %s\n", c.app)
	fmt.Fprintf(os.Stderr, "  cockroach sql --insecure --host=localhost:26257\n\n")
	fmt.Fprintf(os.Stderr, "Admin UI:\n")
	fmt.Fprintf(os.Stderr, "  fly proxy 8080:8080 -a %s\n", c.app)
	fmt.Fprintf(os.Stderr, "  open http://localhost:8080\n\n")
	fmt.Fprintf(os.Stderr, "Tigris backup bucket:\n")
	fmt.Fprintf(os.Stderr, "  fly storage create -a %s -n %s-backups\n\n", c.app, c.app)
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

	fmt.Printf("%-18s %-20s %-6s %-40s %s\n", "ID", "NAME", "REGION", "IP", "STATE")
	for _, m := range machines {
		fmt.Printf("%-18s %-20s %-6s %-40s %s\n", m.ID, m.Name, m.Region, m.PrivateIP, m.State)
	}
}

// --- Destroy ---

func destroy(c *client) {
	slog.Info("destroying cluster", "app", c.app)

	// List all machines
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

	// Stop, then destroy each machine
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

	// Delete app
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

func connectSQL(c *client) {
	slog.Info("connecting via fly proxy...")
	cmd := exec.Command("fly", "proxy", "26257:26257", "-a", c.app)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	go func() {
		time.Sleep(2 * time.Second)
		sql := exec.Command("cockroach", "sql", "--insecure", "--host=localhost:26257")
		sql.Stdin = os.Stdin
		sql.Stdout = os.Stdout
		sql.Stderr = os.Stderr
		sql.Run()
		cmd.Process.Kill()
	}()
	cmd.Run()
}
