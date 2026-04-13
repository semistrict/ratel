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
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed workerd_router.js
var routerScript string

// WorkerDef describes a deployed worker script.
type WorkerDef struct {
	Name       string
	Script     string
	CompatDate string
	DOClasses  []string // Durable Object class names exported by this worker
}

// generateWorkerdConfig writes a workerd capnp config file and the associated
// worker scripts to dir. It returns the path to the config file.
//
// The config contains:
//   - A router worker that dispatches incoming HTTP requests by X-Worker-Name header
//   - One service per deployed worker
//   - Durable Object namespace bindings backed by ratel storage (fd)
//   - An HTTP socket on listenPort
func generateWorkerdConfig(
	dir string, workers []WorkerDef, listenPort int, storageFd int,
) (configPath string, err error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	var cfg strings.Builder

	// Config preamble.
	fmt.Fprintf(&cfg, "using Workerd = import \"/workerd/workerd.capnp\";\n\n")
	fmt.Fprintf(&cfg, "const config :Workerd.Config = (\n")

	// Services section.
	fmt.Fprintf(&cfg, "  services = [\n")

	// Router service.
	fmt.Fprintf(&cfg, "    ( name = \"router\",\n")
	fmt.Fprintf(&cfg, "      worker = .routerWorker ),\n")

	// Per-worker services. Service names are strings — underscores are fine.
	for _, w := range workers {
		cid := workerCapnpID(w.Name)
		fmt.Fprintf(&cfg, "    ( name = \"%s\",\n", w.Name)
		fmt.Fprintf(&cfg, "      worker = .%s ),\n", cid)
	}

	fmt.Fprintf(&cfg, "  ],\n")

	// Sockets section.
	fmt.Fprintf(&cfg, "  sockets = [\n")
	fmt.Fprintf(&cfg, "    ( name = \"http\",\n")
	fmt.Fprintf(&cfg, "      address = \"localhost:%d\",\n", listenPort)
	fmt.Fprintf(&cfg, "      http = (),\n")
	fmt.Fprintf(&cfg, "      service = \"router\" ),\n")
	fmt.Fprintf(&cfg, "  ],\n")
	fmt.Fprintf(&cfg, ");\n\n")

	// Router worker definition.
	routerScriptPath := filepath.Join(dir, "router.js")
	if err := os.WriteFile(routerScriptPath, []byte(routerScript), 0644); err != nil {
		return "", err
	}

	fmt.Fprintf(&cfg, "const routerWorker :Workerd.Worker = (\n")
	fmt.Fprintf(&cfg, "  modules = [\n")
	fmt.Fprintf(&cfg, "    ( name = \"router\", esModule = embed \"router.js\" ),\n")
	fmt.Fprintf(&cfg, "  ],\n")
	fmt.Fprintf(&cfg, "  compatibilityDate = \"2024-01-01\",\n")

	// Router bindings to each worker service.
	if len(workers) > 0 {
		fmt.Fprintf(&cfg, "  bindings = [\n")
		for _, w := range workers {
			fmt.Fprintf(&cfg, "    ( name = \"%s\", service = \"%s\" ),\n", w.Name, w.Name)
		}
		fmt.Fprintf(&cfg, "  ],\n")
	}
	fmt.Fprintf(&cfg, ");\n\n")

	// Per-worker definitions.
	for _, w := range workers {
		cid := workerCapnpID(w.Name)
		scriptPath := filepath.Join(dir, fmt.Sprintf("worker_%s.js", w.Name))
		if err := os.WriteFile(scriptPath, []byte(w.Script), 0644); err != nil {
			return "", err
		}

		fmt.Fprintf(&cfg, "const %s :Workerd.Worker = (\n", cid)
		fmt.Fprintf(&cfg, "  modules = [\n")
		fmt.Fprintf(&cfg, "    ( name = \"worker\", esModule = embed \"worker_%s.js\" ),\n", w.Name)
		fmt.Fprintf(&cfg, "  ],\n")
		fmt.Fprintf(&cfg, "  compatibilityDate = \"%s\",\n", w.CompatDate)

		// Durable Object bindings and classes.
		if len(w.DOClasses) > 0 {
			fmt.Fprintf(&cfg, "  durableObjectNamespaces = [\n")
			for _, cls := range w.DOClasses {
				fmt.Fprintf(&cfg, "    ( className = \"%s\",\n", cls)
				fmt.Fprintf(&cfg, "      uniqueKey = \"%s/%s\" ),\n", w.Name, cls)
			}
			fmt.Fprintf(&cfg, "  ],\n")
			fmt.Fprintf(&cfg, "  durableObjectStorage = ( ratel = %d ),\n", storageFd)

			fmt.Fprintf(&cfg, "  bindings = [\n")
			for _, cls := range w.DOClasses {
				fmt.Fprintf(&cfg, "    ( name = \"%s\", durableObjectNamespace = \"%s\" ),\n", cls, cls)
			}
			fmt.Fprintf(&cfg, "  ],\n")
		}

		fmt.Fprintf(&cfg, ");\n\n")
	}

	configPath = filepath.Join(dir, "config.capnp")
	if err := os.WriteFile(configPath, []byte(cfg.String()), 0644); err != nil {
		return "", err
	}
	return configPath, nil
}

// workerCapnpID converts a worker name like "my-worker" to a camelCase capnp
// identifier like "workerMyWorker". Cap'n Proto requires camelCase names.
func workerCapnpID(name string) string {
	var b strings.Builder
	b.WriteString("worker")
	upper := true
	for _, c := range name {
		if c == '-' || c == '_' {
			upper = true
			continue
		}
		if upper {
			b.WriteRune(rune(strings.ToUpper(string(c))[0]))
			upper = false
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}
