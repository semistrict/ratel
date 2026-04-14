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

package storage

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/pebble/objstorage/remote"
	"github.com/semistrict/ratel/pkg/util/uuid"
)

const clusterIDFile = "cluster-id"

// WriteClusterID generates a new cluster UUID and writes it to metadata storage.
func WriteClusterID(ctx context.Context, meta remote.Storage) (uuid.UUID, error) {
	id := uuid.MakeV4()
	w, err := meta.CreateObject(clusterIDFile)
	if err != nil {
		return uuid.UUID{}, errors.Wrap(err, "creating cluster-id object")
	}
	if _, err := w.Write([]byte(id.String())); err != nil {
		_ = w.Close()
		return uuid.UUID{}, errors.Wrap(err, "writing cluster-id")
	}
	if err := w.Close(); err != nil {
		return uuid.UUID{}, errors.Wrap(err, "closing cluster-id object")
	}
	return id, nil
}

// ReadClusterID reads the cluster UUID from metadata storage.
func ReadClusterID(ctx context.Context, meta remote.Storage) (uuid.UUID, error) {
	r, size, err := meta.ReadObject(ctx, clusterIDFile)
	if err != nil {
		return uuid.UUID{}, errors.Wrap(err, "reading cluster-id object")
	}
	defer r.Close()
	buf := make([]byte, size)
	if err := r.ReadAt(ctx, buf, 0); err != nil {
		return uuid.UUID{}, errors.Wrap(err, "reading cluster-id data")
	}
	id, err := uuid.FromString(string(buf))
	if err != nil {
		return uuid.UUID{}, errors.Wrap(err, "parsing cluster-id")
	}
	return id, nil
}
