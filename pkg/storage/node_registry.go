// Copyright 2026 The Ratel Authors.
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

package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/pebble/objstorage/remote"
)

// NodeRegistration describes a node registered in the cluster's shared storage.
type NodeRegistration struct {
	NodeID        int        `json:"node_id"`
	RatelNodeID   string     `json:"ratel_node_id,omitempty"` // operator-assigned stable identity
	StoreID       int        `json:"store_id,omitempty"`      // CockroachDB-assigned store ID
	Addr          string     `json:"addr"`                    // RPC/KV address
	SQLAddr       string     `json:"sql_addr"`                // PostgreSQL wire address
	HTTPAddr      string     `json:"http_addr"`               // Admin UI address
	LastHeartbeat *time.Time `json:"last_heartbeat,omitempty"`
}

// nodeFileName returns the object name for a node registration file. When a
// RatelNodeID is set, the file is keyed by that stable identity; otherwise we
// fall back to the CockroachDB node ID.
func nodeFileName(reg NodeRegistration) string {
	if reg.RatelNodeID != "" {
		return fmt.Sprintf("node-%s.json", reg.RatelNodeID)
	}
	return fmt.Sprintf("node-%d.json", reg.NodeID)
}

// nodeFileNameByRatelID returns the object name for a ratel node ID.
func nodeFileNameByRatelID(ratelNodeID string) string {
	return fmt.Sprintf("node-%s.json", ratelNodeID)
}

// RegisterNode writes a node registration to the discovery/ storage.
func RegisterNode(ctx context.Context, store remote.Storage, reg NodeRegistration) error {
	data, err := json.Marshal(reg)
	if err != nil {
		return errors.Wrap(err, "marshaling node registration")
	}
	w, err := store.CreateObject(nodeFileName(reg))
	if err != nil {
		return errors.Wrap(err, "creating node registration object")
	}
	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return errors.Wrap(err, "writing node registration")
	}
	return errors.Wrap(w.Close(), "closing node registration object")
}

// ListNodes reads all node registrations from the discovery/ storage.
//
// Note: we pass an empty prefix and filter client-side because the S3
// implementation of List strips its prefix argument from returned keys and we
// need to work identically on local and S3 storage.
func ListNodes(ctx context.Context, store remote.Storage) ([]NodeRegistration, error) {
	names, err := store.List("", "")
	if err != nil {
		return nil, errors.Wrap(err, "listing node registrations")
	}
	sort.Strings(names)
	var nodes []NodeRegistration
	for _, name := range names {
		if !strings.HasPrefix(name, "node-") {
			continue
		}
		reader, size, err := store.ReadObject(ctx, name)
		if err != nil {
			return nil, errors.Wrapf(err, "reading node registration %s", name)
		}
		buf := make([]byte, size)
		if err := reader.ReadAt(ctx, buf, 0); err != nil {
			_ = reader.Close()
			return nil, errors.Wrapf(err, "reading node registration data %s", name)
		}
		_ = reader.Close()

		var reg NodeRegistration
		if err := json.Unmarshal(buf, &reg); err != nil {
			return nil, errors.Wrapf(err, "unmarshaling node registration %s", name)
		}
		nodes = append(nodes, reg)
	}
	return nodes, nil
}

// RemoveNode deletes a node registration from the discovery/ storage by ratel
// node ID.
func RemoveNode(ctx context.Context, store remote.Storage, ratelNodeID string) error {
	return errors.Wrapf(store.Delete(nodeFileNameByRatelID(ratelNodeID)), "removing node %s", ratelNodeID)
}

// ReadNodeRegistration reads a single node registration by ratel node ID.
func ReadNodeRegistration(
	ctx context.Context, store remote.Storage, ratelNodeID string,
) (NodeRegistration, error) {
	reader, size, err := store.ReadObject(ctx, nodeFileNameByRatelID(ratelNodeID))
	if err != nil {
		return NodeRegistration{}, errors.Wrapf(err, "reading node %s", ratelNodeID)
	}
	buf := make([]byte, size)
	if err := reader.ReadAt(ctx, buf, 0); err != nil {
		_ = reader.Close()
		return NodeRegistration{}, errors.Wrapf(err, "reading node %s data", ratelNodeID)
	}
	_ = reader.Close()

	var reg NodeRegistration
	if err := json.Unmarshal(buf, &reg); err != nil {
		return NodeRegistration{}, errors.Wrapf(err, "unmarshaling node %s", ratelNodeID)
	}
	return reg, nil
}

// NodeRegistrationExists returns true if a registration exists for the given
// ratel node ID, and the registration itself.
func NodeRegistrationExists(
	ctx context.Context, store remote.Storage, ratelNodeID string,
) (NodeRegistration, bool, error) {
	_, err := store.Size(nodeFileNameByRatelID(ratelNodeID))
	if err != nil {
		if store.IsNotExistError(err) {
			return NodeRegistration{}, false, nil
		}
		return NodeRegistration{}, false, errors.Wrapf(err, "checking node %s", ratelNodeID)
	}
	reg, err := ReadNodeRegistration(ctx, store, ratelNodeID)
	if err != nil {
		return NodeRegistration{}, false, err
	}
	return reg, true, nil
}

// HeartbeatNode updates a node registration's LastHeartbeat timestamp.
func HeartbeatNode(ctx context.Context, store remote.Storage, reg NodeRegistration) error {
	now := time.Now().UTC()
	reg.LastHeartbeat = &now
	return RegisterNode(ctx, store, reg)
}

// WriteObject is a helper that writes data to a remote.Storage object.
func WriteObject(store remote.Storage, name string, data []byte) error {
	w, err := store.CreateObject(name)
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return err
	}
	return w.Close()
}

// ReadObject is a helper that reads an entire object from remote.Storage.
func ReadObject(ctx context.Context, store remote.Storage, name string) ([]byte, error) {
	reader, size, err := store.ReadObject(ctx, name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()
	buf := make([]byte, size)
	if err := reader.ReadAt(ctx, buf, 0); err != nil && err != io.EOF {
		return nil, err
	}
	return buf, nil
}
