// deploy/fly-cluster creates a globally distributed Ratel (CockroachDB)
// cluster on Fly.io Machines with Tigris object storage for backups.
//
// Usage:
//
//	export FLY_API_TOKEN=$(fly tokens deploy)
//	go run ./deploy/fly-cluster [flags]
//
// Flags:
//
//	-app        App name (default: ratel-cluster)
//	-org        Fly org slug (default: personal)
//	-regions    Comma-separated regions (default: iad,lhr,sin)
//	-image      Docker image for ratel (default: ghcr.io/semistrict/ratel:latest)
//	-cpus       CPUs per machine (default: 2)
//	-memory     Memory in MB per machine (default: 4096)
//	-disk       Disk size in GB per machine (default: 10)
//	-destroy    Tear down the cluster instead of creating it
//	-status     Show cluster status
//	-sql        Connect to the cluster via cockroach sql
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
	flagDisk    = flag.Int("disk", 10, "Disk size in GB per machine")
	flagDestroy = flag.Bool("destroy", false, "Tear down the cluster")
	flagStatus  = flag.Bool("status", false, "Show cluster status")
	flagSQL     = flag.Bool("sql", false, "Connect via cockroach sql")
)

func main() {
	flag.Parse()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	token := os.Getenv("FLY_API_TOKEN")
	if token == "" {
		slog.Error("FLY_API_TOKEN not set. Run: export FLY_API_TOKEN=$(fly tokens deploy)")
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
	Mounts   []mount           `json:"mounts,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
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

type mount struct {
	Volume string `json:"volume"`
	Path   string `json:"path"`
}

type createMachineReq struct {
	Name   string        `json:"name"`
	Region string        `json:"region"`
	Config machineConfig `json:"config"`
}

type createVolumeReq struct {
	Name   string `json:"name"`
	Region string `json:"region"`
	SizeGB int    `json:"size_gb"`
}

type machine struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	State     string `json:"state"`
	Region    string `json:"region"`
	PrivateIP string `json:"private_ip"`
}

type volume struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Region string `json:"region"`
	State  string `json:"state"`
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

	// 2. Create Tigris bucket for backups
	slog.Info("creating Tigris bucket (run manually if not done):")
	fmt.Fprintf(os.Stderr, "  fly storage create -a %s -n %s-backups\n", c.app, c.app)

	// 3. Create volumes and machines
	var joinAddrs []string
	for i := range regions {
		joinAddrs = append(joinAddrs, fmt.Sprintf("%s-node-%d.vm.%s.internal:26257", c.app, i, c.app))
	}
	joinStr := strings.Join(joinAddrs, ",")

	var machines []machine
	for i, region := range regions {
		nodeName := fmt.Sprintf("%s-node-%d", c.app, i)

		// Create volume
		slog.Info("creating volume", "name", nodeName, "region", region)
		data, code, err = c.do("POST", fmt.Sprintf("/v1/apps/%s/volumes", c.app), createVolumeReq{
			Name:   nodeName,
			Region: region,
			SizeGB: *flagDisk,
		})
		if err != nil {
			slog.Error("creating volume", "error", err)
			os.Exit(1)
		}
		if code >= 300 {
			slog.Error("creating volume", "status", code, "body", string(data))
			os.Exit(1)
		}
		var vol volume
		if err := json.Unmarshal(data, &vol); err != nil {
			slog.Error("parsing volume response", "error", err)
			os.Exit(1)
		}
		slog.Info("volume created", "id", vol.ID, "region", region)

		// Create machine
		slog.Info("creating machine", "name", nodeName, "region", region)
		machReq := createMachineReq{
			Name:   nodeName,
			Region: region,
			Config: machineConfig{
				Image: *flagImage,
				Env: map[string]string{
					"COCKROACH_CHANNEL": "fly",
				},
				Guest: guestConfig{
					CPUs:    *flagCPUs,
					MemMB:   *flagMemory,
					CPUKind: "shared",
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
				Mounts: []mount{
					{Volume: vol.ID, Path: "/cockroach/cockroach-data"},
				},
				Metadata: map[string]string{
					"fly_process_group": "cockroach",
					"ratel_node_index":  fmt.Sprintf("%d", i),
				},
			},
		}

		// Build the start command.
		// First node initializes the cluster; others join.
		cmd := []string{
			"/cockroach/cockroach", "start",
			"--insecure",
			"--store=/cockroach/cockroach-data",
			"--advertise-addr", fmt.Sprintf("%s.vm.%s.internal:26257", nodeName, c.app),
			"--listen-addr", "0.0.0.0:26257",
			"--http-addr", "0.0.0.0:8080",
			"--join", joinStr,
			"--locality", fmt.Sprintf("region=%s", region),
		}
		machReq.Config.Env["COCKROACH_ARGS"] = strings.Join(cmd[2:], " ")

		// Use the entrypoint from the image. Override CMD with our args.
		// For a custom image, we'd set config.cmd directly. For now,
		// pass args via env and use a wrapper entrypoint.

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

	// 5. Initialize cluster (only needed once)
	slog.Info("initializing cluster...")
	initNode := machines[0]
	fmt.Fprintf(os.Stderr, "\nTo initialize the cluster, run:\n")
	fmt.Fprintf(os.Stderr, "  fly ssh console -a %s -s -C '/cockroach/cockroach init --insecure --host=%s.vm.%s.internal:26257'\n\n",
		c.app, fmt.Sprintf("%s-node-0", c.app), c.app)

	// 6. Print connection info
	fmt.Fprintf(os.Stderr, "\nCluster ready!\n\n")
	fmt.Fprintf(os.Stderr, "Nodes:\n")
	for i, m := range machines {
		fmt.Fprintf(os.Stderr, "  [%d] %s  region=%-3s  ip=%s  state=%s\n",
			i, m.ID, regions[i], m.PrivateIP, m.State)
	}
	fmt.Fprintf(os.Stderr, "\nConnect via:\n")
	fmt.Fprintf(os.Stderr, "  fly proxy 26257:26257 -a %s\n", c.app)
	fmt.Fprintf(os.Stderr, "  cockroach sql --insecure --host=localhost:26257\n\n")
	fmt.Fprintf(os.Stderr, "Admin UI:\n")
	fmt.Fprintf(os.Stderr, "  fly proxy 8080:8080 -a %s\n", c.app)
	fmt.Fprintf(os.Stderr, "  open http://localhost:8080\n\n")

	_ = initNode
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

	// List and stop all machines
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

	// Wait for stopped, then destroy
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

	// Delete volumes
	data, code, err = c.do("GET", fmt.Sprintf("/v1/apps/%s/volumes", c.app), nil)
	if err == nil && code < 300 {
		var volumes []volume
		if json.Unmarshal(data, &volumes) == nil {
			for _, v := range volumes {
				slog.Info("deleting volume", "id", v.ID)
				c.do("DELETE", fmt.Sprintf("/v1/apps/%s/volumes/%s", c.app, v.ID), nil)
			}
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
	// Use fly proxy to tunnel to the cluster
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
