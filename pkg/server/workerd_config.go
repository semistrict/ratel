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
	"encoding/json"
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
	Name         string
	Script       string
	CompatDate   string
	DOClasses    []string // Durable Object class names exported by this worker
	Assets       []WorkerAsset
	AssetsConfig WorkerAssetsConfig
}

type WorkerAsset struct {
	Path        string
	ContentType string
	Content     []byte
}

type WorkerAssetsConfig struct {
	RunWorkerFirst       bool
	RunWorkerFirstRoutes []string
	NotFoundHandling     string
}

// generateWorkerdConfig writes a workerd capnp config file and the associated
// worker scripts to dir. It returns the path to the config file.
//
// The config contains:
//   - A router worker that dispatches incoming HTTP requests by X-Worker-Name header
//   - One service per deployed worker
//   - Durable Object namespace bindings backed by workerd local disk storage,
//     ratel storage when storageFd is non-negative, or in-memory storage when
//     storageFd is -1
//   - An HTTP socket on listenPort
func generateWorkerdConfig(
	dir string, workers []WorkerDef, listenPort int, storageFd int, sqlPort int,
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

	hasDurableObjects := false
	for _, w := range workers {
		if len(w.DOClasses) > 0 {
			hasDurableObjects = true
			break
		}
	}

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
	if hasDurableObjects && storageFd == -2 {
		doDir := filepath.Join(dir, "durable-objects")
		if err := os.MkdirAll(doDir, 0755); err != nil {
			return "", err
		}
		fmt.Fprintf(&cfg, "    ( name = \"durable-object-storage\",\n")
		fmt.Fprintf(&cfg, "      disk = ( path = %s, writable = true ) ),\n", strconv.Quote(doDir))
	}
	if hasDurableObjects && sqlPort > 0 {
		fmt.Fprintf(&cfg, "    ( name = \"ratel-sql\",\n")
		fmt.Fprintf(&cfg, "      external = ( address = \"localhost:%d\", http = () ) ),\n", sqlPort)
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
			if len(w.Assets) > 0 {
				fmt.Fprintf(&cfg, "    ( name = \"%s-assets\", service = \"%s-assets\" ),\n", w.Name, w.Name)
			}
		}
		if err := writeRouterManifest(dir, workers); err != nil {
			return "", err
		}
		fmt.Fprintf(&cfg, "    ( name = \"__RATEL_WORKERS\", text = embed \"workers.json\" ),\n")
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
				fmt.Fprintf(&cfg, "      uniqueKey = \"%s\",\n", durableObjectUniqueKey(w.Name, cls))
				fmt.Fprintf(&cfg, "      enableSql = true ),\n")
			}
			fmt.Fprintf(&cfg, "  ],\n")
			if storageFd >= 0 {
				fmt.Fprintf(&cfg, "  durableObjectStorage = ( ratel = %d ),\n", storageFd)
			} else if storageFd == -2 {
				fmt.Fprintf(&cfg, "  durableObjectStorage = ( localDisk = \"durable-object-storage\" ),\n")
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
			if sqlPort > 0 {
				fmt.Fprintf(&cfg, "    ( name = \"__RATEL_SQL\", service = \"ratel-sql\" ),\n")
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

func writeRouterManifest(dir string, workers []WorkerDef) error {
	type workerManifest struct {
		Assets               bool     `json:"assets"`
		RunWorkerFirst       bool     `json:"run_worker_first,omitempty"`
		RunWorkerFirstRoutes []string `json:"run_worker_first_routes,omitempty"`
		NotFoundHandling     string   `json:"not_found_handling,omitempty"`
	}
	manifest := make(map[string]workerManifest, len(workers))
	for _, w := range workers {
		manifest[w.Name] = workerManifest{
			Assets:               len(w.Assets) > 0,
			RunWorkerFirst:       w.AssetsConfig.RunWorkerFirst,
			RunWorkerFirstRoutes: w.AssetsConfig.RunWorkerFirstRoutes,
			NotFoundHandling:     w.AssetsConfig.NotFoundHandling,
		}
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "workers.json"), data, 0644)
}

func durableObjectUniqueKey(workerName, className string) string {
	return workerCapnpID(workerName) + "_" + className
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
	manifest.WriteString("const notFoundHandling = ")
	manifest.WriteString(strconv.Quote(w.AssetsConfig.NotFoundHandling))
	manifest.WriteString(";\n")
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
    let asset = manifest.get(path);
    let status = 200;
    if (!asset && notFoundHandling === "single-page-application") {
      asset = manifest.get("/index.html");
    }
    if (!asset && notFoundHandling === "404-page") {
      asset = manifest.get("/404.html");
      status = 404;
    }
    if (!asset) return new Response("not found", { status: 404 });
    const headers = new Headers();
    if (asset.contentType) headers.set("Content-Type", asset.contentType);
    headers.set("X-Ratel-Asset", "hit");
    return new Response(decodeBase64(asset.data), { status, headers });
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
