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
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/semistrict/ratel/pkg/util/tracing"
	"go.opentelemetry.io/otel/attribute"
)

const (
	// maxConcurrentWorkerRequests limits in-flight worker invocations per node.
	maxConcurrentWorkerRequests = 256

	// workerRequestTimeout is the maximum time for a worker invocation.
	workerRequestTimeout = 30 * time.Second
)

// workerdProxy is an HTTP handler that serves on the cmux HTTP/1.x listener.
// It routes:
//   - /workers/<name>/... → reverse proxy to local workerd (sets X-Worker-Name)
//   - /workers/_health    → health check for the workerd sidecar
//   - /api/v2/workers/... → passes through to the API v2 handler
//
// For multi-node clusters, requests for DO-bearing workers are routed to a
// consistent node via rendezvous hashing (see workerRouter).
type workerdProxy struct {
	mux       *http.ServeMux
	apiServer *apiV2Server
	proxy     *httputil.ReverseProxy
	tracer    *tracing.Tracer
	sidecar   *WorkerdSidecar
	router    *workerRouter

	// sem limits concurrent worker invocations.
	sem chan struct{}
}

func newWorkerdProxy(
	apiServer *apiV2Server,
	workerdPort int,
	tracer *tracing.Tracer,
	sidecar *WorkerdSidecar,
	router *workerRouter,
) *workerdProxy {
	target, err := url.Parse(fmt.Sprintf("http://localhost:%d", workerdPort))
	if err != nil {
		panic(fmt.Sprintf("failed to parse workerd proxy target URL: %v", err))
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host

		path := req.URL.Path
		name, remainder := splitWorkerPath(path)
		if name != "" {
			req.Header.Set("X-Worker-Name", name)
			req.URL.Path = remainder
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
		}
	}

	wp := &workerdProxy{
		mux:       http.NewServeMux(),
		apiServer: apiServer,
		proxy:     proxy,
		tracer:    tracer,
		sidecar:   sidecar,
		router:    router,
		sem:       make(chan struct{}, maxConcurrentWorkerRequests),
	}
	wp.mux.HandleFunc("/workers/_health", wp.handleHealth)
	wp.mux.HandleFunc("/workers/", wp.handleWorkers)
	wp.mux.Handle("/api/v2/", apiServer)

	return wp
}

func (wp *workerdProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	wp.mux.ServeHTTP(w, r)
}

func (wp *workerdProxy) handleWorkers(w http.ResponseWriter, r *http.Request) {
	name, _ := splitWorkerPath(r.URL.Path)
	if name == "" {
		http.Error(w, "worker name required", http.StatusBadRequest)
		return
	}

	// Enforce request timeout.
	ctx, cancel := context.WithTimeout(r.Context(), workerRequestTimeout)
	defer cancel()

	ctx, span := wp.tracer.StartSpanCtx(ctx, "worker.invoke",
		tracing.WithServerSpanKind,
	)
	span.SetTag("worker.name", attribute.StringValue(name))
	defer span.Finish()

	// Multi-node routing: if this worker has DOs, route to the
	// consistent owner node.
	if wp.router != nil {
		hasDOs := wp.workerHasDOs(name)
		if wp.router.routeRequest(w, r.WithContext(ctx), name, hasDOs) {
			return
		}
	}

	// Acquire concurrency semaphore.
	select {
	case wp.sem <- struct{}{}:
		defer func() { <-wp.sem }()
	case <-ctx.Done():
		http.Error(w, "request timeout waiting for capacity", http.StatusServiceUnavailable)
		return
	}

	wp.proxy.ServeHTTP(w, r.WithContext(ctx))
}

// handleHealth returns 200 if the workerd sidecar is running, 503 otherwise.
func (wp *workerdProxy) handleHealth(w http.ResponseWriter, r *http.Request) {
	if wp.sidecar == nil || !wp.sidecar.IsRunning() {
		http.Error(w, "workerd not running", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

// workerHasDOs returns true if the named worker has Durable Object classes.
func (wp *workerdProxy) workerHasDOs(name string) bool {
	if wp.sidecar == nil {
		return false
	}
	return wp.sidecar.WorkerHasDOs(name)
}

// splitWorkerPath splits /workers/<name>/rest/of/path into (name, /rest/of/path).
func splitWorkerPath(path string) (name string, remainder string) {
	const prefix = "/workers/"
	if !strings.HasPrefix(path, prefix) {
		return "", path
	}
	rest := path[len(prefix):]
	slash := strings.IndexByte(rest, '/')
	if slash < 0 {
		return rest, "/"
	}
	return rest[:slash], rest[slash:]
}
