// Copyright 2024 The Cockroach Authors.
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

	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/pebble/objstorage/remote"
)

// NodeRegistration describes a node registered in the cluster's shared storage.
type NodeRegistration struct {
	NodeID   int    `json:"node_id"`
	Addr     string `json:"addr"`      // RPC/KV address
	SQLAddr  string `json:"sql_addr"`  // PostgreSQL wire address
	HTTPAddr string `json:"http_addr"` // Admin UI address
}

// nodeFileName returns the object name for a node registration file.
func nodeFileName(nodeID int) string {
	return fmt.Sprintf("node-%d.json", nodeID)
}

// RegisterNode writes a node registration to the nodes/ storage.
func RegisterNode(ctx context.Context, store remote.Storage, reg NodeRegistration) error {
	data, err := json.Marshal(reg)
	if err != nil {
		return errors.Wrap(err, "marshaling node registration")
	}
	w, err := store.CreateObject(nodeFileName(reg.NodeID))
	if err != nil {
		return errors.Wrap(err, "creating node registration object")
	}
	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return errors.Wrap(err, "writing node registration")
	}
	return errors.Wrap(w.Close(), "closing node registration object")
}

// ListNodes reads all node registrations from the nodes/ storage.
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

// RemoveNode deletes a node registration from the nodes/ storage.
func RemoveNode(ctx context.Context, store remote.Storage, nodeID int) error {
	return errors.Wrapf(store.Delete(nodeFileName(nodeID)), "removing node %d", nodeID)
}

// ReadNodeRegistration reads a single node registration by ID.
func ReadNodeRegistration(ctx context.Context, store remote.Storage, nodeID int) (NodeRegistration, error) {
	reader, size, err := store.ReadObject(ctx, nodeFileName(nodeID))
	if err != nil {
		return NodeRegistration{}, errors.Wrapf(err, "reading node %d", nodeID)
	}
	buf := make([]byte, size)
	if err := reader.ReadAt(ctx, buf, 0); err != nil {
		_ = reader.Close()
		return NodeRegistration{}, errors.Wrapf(err, "reading node %d data", nodeID)
	}
	_ = reader.Close()

	var reg NodeRegistration
	if err := json.Unmarshal(buf, &reg); err != nil {
		return NodeRegistration{}, errors.Wrapf(err, "unmarshaling node %d", nodeID)
	}
	return reg, nil
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
