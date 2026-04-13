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
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/semistrict/ratel/pkg/util/tracing"
	"go.opentelemetry.io/otel/attribute"
)

// workerdProxy is an HTTP handler that serves on the cmux HTTP/1.x listener.
// It routes:
//   - /workers/<name>/... → reverse proxy to local workerd (sets X-Worker-Name)
//   - /api/v2/workers/... → passes through to the API v2 handler
type workerdProxy struct {
	mux       *http.ServeMux
	apiServer *apiV2Server
	proxy     *httputil.ReverseProxy
	tracer    *tracing.Tracer
}

func newWorkerdProxy(apiServer *apiV2Server, workerdPort int, tracer *tracing.Tracer) *workerdProxy {
	// localhost URL parse cannot fail, but handle it defensively.
	target, err := url.Parse(fmt.Sprintf("http://localhost:%d", workerdPort))
	if err != nil {
		panic(fmt.Sprintf("failed to parse workerd proxy target URL: %v", err))
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	// Override the Director to set X-Worker-Name and strip the /workers/<name> prefix.
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host

		// Extract worker name from path: /workers/<name>/... → name, remainder
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
	}
	wp.mux.HandleFunc("/workers/", wp.handleWorkers)
	wp.mux.Handle("/api/v2/", apiServer)

	return wp
}

func (wp *workerdProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	wp.mux.ServeHTTP(w, r)
}

func (wp *workerdProxy) handleWorkers(w http.ResponseWriter, r *http.Request) {
	name, _ := splitWorkerPath(r.URL.Path)
	ctx, span := wp.tracer.StartSpanCtx(r.Context(), "worker.invoke",
		tracing.WithServerSpanKind,
	)
	if name != "" {
		span.SetTag("worker.name", attribute.StringValue(name))
	}
	defer span.Finish()
	wp.proxy.ServeHTTP(w, r.WithContext(ctx))
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
