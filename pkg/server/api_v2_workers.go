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
	"net/http"
	"regexp"

	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/server/workers"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/gorilla/mux"
)

// validWorkerName matches alphanumeric, hyphen, underscore only.
var validWorkerName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// deployWorker handles PUT /api/v2/workers/{name}/.
//
// The request body is either JavaScript source or multipart/form-data with:
//   - script: worker JavaScript bytes
//   - metadata: JSON with compat_date, bindings, and asset metadata
//   - asset: repeated file parts matching metadata.assets order
//
// Raw JavaScript deploys pass compatibility date via X-Compat-Date and Durable
// Object bindings via X-Bindings, e.g.:
//
//	{"durable_objects": [{"class_name": "Counter"}]}
//
// Each call atomically inserts a new version row into system.worker_versions and
// links deduplicated asset rows through system.worker_version_assets.
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

	deploy, err := workers.ParseDeployRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	nextVersion, err := workers.InsertDeploy(ctx, a.db, a.admin.ie, name, deploy)
	if err != nil {
		apiV2InternalError(ctx, err, w)
		return
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
		`SELECT worker_name, max(version) AS latest_version FROM system.worker_versions GROUP BY worker_name ORDER BY worker_name`,
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
