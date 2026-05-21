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
	"encoding/json"
	"io"
	"net/http"
	"regexp"

	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/gorilla/mux"
)

const maxWorkerScriptSize = 10 << 20 // 10 MiB

// validWorkerName matches alphanumeric, hyphen, underscore only.
var validWorkerName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// deployWorker handles PUT /api/v2/workers/{name}/.
//
// The request body is the JavaScript source. The compatibility date is passed
// via the X-Compat-Date header. Durable Object class bindings are declared via
// the X-Bindings header as a JSON object, e.g.:
//
//	{"durable_objects": [{"class_name": "Counter"}]}
//
// Each call atomically inserts a new version row into system.worker_scripts.
func (a *apiV2Server) deployWorker(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	name := vars["name"]
	if name == "" {
		http.Error(w, "worker name is required", http.StatusBadRequest)
		return
	}
	if !validWorkerName.MatchString(name) {
		http.Error(w, "worker name must match [a-zA-Z0-9_-]+", http.StatusBadRequest)
		return
	}

	compatDate := r.Header.Get("X-Compat-Date")
	if compatDate == "" {
		compatDate = "2024-01-01"
	}

	// Parse optional bindings header. We use interface{} because the internal
	// executor doesn't support *string — nil means SQL NULL, string means a value.
	var bindingsArg interface{}
	if bindingsStr := r.Header.Get("X-Bindings"); bindingsStr != "" {
		if !json.Valid([]byte(bindingsStr)) {
			http.Error(w, "X-Bindings header must be valid JSON", http.StatusBadRequest)
			return
		}
		bindingsArg = bindingsStr
	}

	script, err := io.ReadAll(io.LimitReader(r.Body, maxWorkerScriptSize+1))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(script) > maxWorkerScriptSize {
		http.Error(w, "worker script exceeds 10 MiB limit", http.StatusRequestEntityTooLarge)
		return
	}
	if len(script) == 0 {
		http.Error(w, "empty worker script", http.StatusBadRequest)
		return
	}

	// Atomic version increment: a single INSERT ... SELECT ensures no race
	// between concurrent deploys of the same worker name.
	ie := a.admin.ie
	row, err := ie.QueryRowEx(
		ctx,
		"deploy-worker-atomic",
		nil, // no txn — the INSERT ... SELECT is itself atomic
		sessiondata.InternalExecutorOverride{
			User:     username.RootUserName(),
			Database: "system",
		},
		`INSERT INTO system.worker_scripts (name, version, script, compat_date, bindings)
		 VALUES ($1, COALESCE((SELECT max(version) FROM system.worker_scripts WHERE name = $1), 0) + 1, $2, $3, $4)
		 RETURNING version`,
		name,
		script,
		compatDate,
		bindingsArg,
	)
	if err != nil {
		apiV2InternalError(ctx, err, w)
		return
	}

	var nextVersion int64
	if row != nil && row[0] != nil {
		if v, ok := row[0].(*tree.DInt); ok {
			nextVersion = int64(*v)
		}
	}

	// Trigger a workerd config reload so the new worker is available.
	if a.sidecar != nil {
		if reloadErr := a.sidecar.ReloadConfig(ctx); reloadErr != nil {
			log.Warningf(ctx, "failed to reload workerd config after deploy: %v", reloadErr)
		}
	}

	writeJSONResponse(ctx, w, http.StatusOK, map[string]interface{}{
		"name":    name,
		"version": nextVersion,
	})
}

// listWorkers handles GET /api/v2/workers/.
func (a *apiV2Server) listWorkers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ie := a.admin.ie

	rows, err := ie.QueryBufferedEx(
		ctx,
		"list-workers",
		nil,
		sessiondata.InternalExecutorOverride{
			User:     username.RootUserName(),
			Database: "system",
		},
		`SELECT name, max(version) AS latest_version FROM system.worker_scripts GROUP BY name ORDER BY name`,
	)
	if err != nil {
		apiV2InternalError(ctx, err, w)
		return
	}

	type workerInfo struct {
		Name    string `json:"name"`
		Version int64  `json:"latest_version"`
	}
	workers := make([]workerInfo, 0, len(rows))
	for _, row := range rows {
		name := string(*row[0].(*tree.DString))
		version := int64(*row[1].(*tree.DInt))
		workers = append(workers, workerInfo{Name: name, Version: version})
	}

	writeJSONResponse(ctx, w, http.StatusOK, map[string]interface{}{
		"workers": workers,
	})
}
