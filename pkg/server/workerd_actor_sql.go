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
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/server/actorstorage"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/errors"
)

type actorSQLRequest struct {
	Actor string        `json:"actor"`
	SQL   string        `json:"sql"`
	Args  []interface{} `json:"args"`
}

type actorSQLResponse struct {
	Columns []string                 `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
}

func (w *WorkerdSidecar) startActorSQLService(
	ctx context.Context, workers []WorkerDef,
) (*http.Server, net.Listener, error) {
	if w.ie == nil || !workersNeedActorSQL(workers) {
		return nil, nil, nil
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}
	srv := &http.Server{Handler: http.HandlerFunc(w.handleActorSQL)}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Warningf(context.Background(), "actor SQL service exited: %v", err)
		}
	}()
	return srv, ln, nil
}

func workersNeedActorSQL(workers []WorkerDef) bool {
	for _, worker := range workers {
		if len(worker.DOClasses) > 0 {
			return true
		}
	}
	return false
}

func (w *WorkerdSidecar) handleActorSQL(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, 1<<20))
	if err != nil {
		http.Error(rw, "reading request", http.StatusBadRequest)
		return
	}
	var sqlReq actorSQLRequest
	if err := json.Unmarshal(body, &sqlReq); err != nil {
		http.Error(rw, "invalid JSON", http.StatusBadRequest)
		return
	}
	if sqlReq.Actor == "" {
		http.Error(rw, "missing actor scope", http.StatusBadRequest)
		return
	}
	if err := actorstorage.ValidateActorSQL(sqlReq.SQL); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}
	if err := actorstorage.ValidateActorSQLScope(sqlReq.SQL); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	qargs := make([]interface{}, 0, len(sqlReq.Args)+1)
	qargs = append(qargs, sqlReq.Actor)
	qargs = append(qargs, sqlReq.Args...)
	rows, err := w.ie.QueryBufferedEx(
		req.Context(),
		"actor-sql-exec",
		nil,
		sessiondata.InternalExecutorOverride{
			User:     username.RootUserName(),
			Database: "system",
		},
		sqlReq.SQL,
		qargs...,
	)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	columns := actorSQLColumnNames(sqlReq.SQL)
	resp := actorSQLResponse{
		Columns: columns,
		Rows:    make([]map[string]interface{}, 0, len(rows)),
	}
	for _, row := range rows {
		result := make(map[string]interface{}, len(row))
		for i, datum := range row {
			col := fmt.Sprintf("column%d", i+1)
			if i < len(columns) {
				col = columns[i]
			}
			result[col] = datumToJSON(datum)
		}
		resp.Rows = append(resp.Rows, result)
	}
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(resp)
}

func actorSQLColumnNames(sql string) []string {
	upper := strings.ToUpper(sql)
	selectIdx := strings.Index(upper, "SELECT ")
	fromIdx := strings.Index(upper, " FROM ")
	if selectIdx < 0 || fromIdx <= selectIdx {
		return nil
	}
	fields := strings.Split(sql[selectIdx+len("SELECT "):fromIdx], ",")
	columns := make([]string, 0, len(fields))
	for _, field := range fields {
		parts := strings.Fields(strings.TrimSpace(field))
		if len(parts) == 0 {
			continue
		}
		name := parts[len(parts)-1]
		if len(parts) >= 3 && strings.EqualFold(parts[len(parts)-2], "AS") {
			name = parts[len(parts)-1]
		} else {
			if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
				name = name[dot+1:]
			}
		}
		columns = append(columns, strings.Trim(name, `"`))
	}
	return columns
}

func datumToJSON(d tree.Datum) interface{} {
	if d == nil || d == tree.DNull {
		return nil
	}
	switch t := d.(type) {
	case *tree.DString:
		return string(*t)
	case *tree.DBytes:
		return string(*t)
	case *tree.DInt:
		return int64(*t)
	case *tree.DBool:
		return bool(*t)
	case *tree.DFloat:
		return float64(*t)
	default:
		return tree.AsStringWithFlags(d, tree.FmtBareStrings)
	}
}
