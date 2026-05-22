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
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
	Assets     []WorkerAsset
}

type WorkerAsset struct {
	Path        string
	ContentType string
	Content     []byte
}

// generateWorkerdConfig writes a workerd capnp config file and the associated
// worker scripts to dir. It returns the path to the config file.
//
// The config contains:
//   - A router worker that dispatches incoming HTTP requests by X-Worker-Name header
//   - One service per deployed worker
//   - Durable Object namespace bindings backed by ratel storage (fd), or
//     workerd's in-memory storage when storageFd is negative
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
		if len(w.Assets) > 0 {
			fmt.Fprintf(&cfg, "    ( name = \"%s-assets\",\n", w.Name)
			fmt.Fprintf(&cfg, "      worker = .%sAssets ),\n", cid)
		}
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
		if len(w.Assets) > 0 {
			if err := writeWorkerAssets(dir, w); err != nil {
				return "", err
			}
		}

		fmt.Fprintf(&cfg, "const %s :Workerd.Worker = (\n", cid)
		fmt.Fprintf(&cfg, "  modules = [\n")
		fmt.Fprintf(&cfg, "    ( name = \"worker\", esModule = embed \"worker_%s.js\" ),\n", w.Name)
		fmt.Fprintf(&cfg, "  ],\n")
		fmt.Fprintf(&cfg, "  compatibilityDate = \"%s\",\n", w.CompatDate)

		if len(w.DOClasses) > 0 {
			fmt.Fprintf(&cfg, "  durableObjectNamespaces = [\n")
			for _, cls := range w.DOClasses {
				fmt.Fprintf(&cfg, "    ( className = \"%s\",\n", cls)
				fmt.Fprintf(&cfg, "      uniqueKey = \"%s/%s\" ),\n", w.Name, cls)
			}
			fmt.Fprintf(&cfg, "  ],\n")
			if storageFd >= 0 {
				fmt.Fprintf(&cfg, "  durableObjectStorage = ( ratel = %d ),\n", storageFd)
			} else {
				fmt.Fprintf(&cfg, "  durableObjectStorage = ( inMemory = void ),\n")
			}
		}

		if len(w.DOClasses) > 0 || len(w.Assets) > 0 {
			fmt.Fprintf(&cfg, "  bindings = [\n")
		}
		if len(w.Assets) > 0 {
			fmt.Fprintf(&cfg, "    ( name = \"ASSETS\", service = \"%s-assets\" ),\n", w.Name)
		}
		if len(w.DOClasses) > 0 {
			for _, cls := range w.DOClasses {
				fmt.Fprintf(&cfg, "    ( name = \"%s\", durableObjectNamespace = \"%s\" ),\n", cls, cls)
			}
		}
		if len(w.DOClasses) > 0 || len(w.Assets) > 0 {
			fmt.Fprintf(&cfg, "  ],\n")
		}

		fmt.Fprintf(&cfg, ");\n\n")

		if len(w.Assets) > 0 {
			fmt.Fprintf(&cfg, "const %sAssets :Workerd.Worker = (\n", cid)
			fmt.Fprintf(&cfg, "  modules = [\n")
			fmt.Fprintf(&cfg, "    ( name = \"asset-worker\", esModule = embed \"assets_%s/asset-worker.js\" ),\n", w.Name)
			fmt.Fprintf(&cfg, "  ],\n")
			fmt.Fprintf(&cfg, "  compatibilityDate = \"%s\",\n", w.CompatDate)
			fmt.Fprintf(&cfg, ");\n\n")
		}
	}

	configPath = filepath.Join(dir, "config.capnp")
	if err := os.WriteFile(configPath, []byte(cfg.String()), 0644); err != nil {
		return "", err
	}
	return configPath, nil
}

func writeWorkerAssets(dir string, w WorkerDef) error {
	assetDir := filepath.Join(dir, "assets_"+w.Name)
	if err := os.MkdirAll(assetDir, 0755); err != nil {
		return err
	}

	var manifest strings.Builder
	manifest.WriteString("const manifest = new Map([\n")
	for _, asset := range w.Assets {
		path := strings.TrimPrefix(asset.Path, "/")
		if path == "" || strings.Contains(path, "..") {
			return fmt.Errorf("invalid asset path %q", asset.Path)
		}
		manifest.WriteString("  [")
		manifest.WriteString(strconv.Quote("/" + path))
		manifest.WriteString(", { contentType: ")
		manifest.WriteString(strconv.Quote(asset.ContentType))
		manifest.WriteString(", data: ")
		manifest.WriteString(strconv.Quote(base64.StdEncoding.EncodeToString(asset.Content)))
		manifest.WriteString(" }],\n")
	}
	manifest.WriteString("]);\n")
	manifest.WriteString(`
function decodeBase64(data) {
  const binary = atob(data);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
  return bytes;
}

export default {
  fetch(request) {
    const url = new URL(request.url);
    let path = url.pathname;
    if (path === "/" || path.endsWith("/")) path += "index.html";
    const asset = manifest.get(path);
    if (!asset) return new Response("not found", { status: 404 });
    const headers = new Headers();
    if (asset.contentType) headers.set("Content-Type", asset.contentType);
    return new Response(decodeBase64(asset.data), { headers });
  }
};
`)
	return os.WriteFile(filepath.Join(assetDir, "asset-worker.js"), []byte(manifest.String()), 0644)
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
