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
	"net/url"
	"os"

	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/pebble/objstorage/remote"
	"github.com/cockroachdb/pebble/vfs"
)

// LocalFSStorageFactory implements remote.StorageFactory using a local
// filesystem directory.
type LocalFSStorageFactory struct {
	dir string
}

var _ remote.StorageFactory = (*LocalFSStorageFactory)(nil)

// NewLocalFSStorageFactory creates a factory that produces local-filesystem
// backed remote.Storage instances. All objects are stored under dir.
func NewLocalFSStorageFactory(dir string) *LocalFSStorageFactory {
	return &LocalFSStorageFactory{dir: dir}
}

// CreateStorage implements remote.StorageFactory.
func (f *LocalFSStorageFactory) CreateStorage(locator remote.Locator) (remote.Storage, error) {
	if err := os.MkdirAll(f.dir, 0755); err != nil {
		return nil, errors.Wrapf(err, "creating local storage dir %s", f.dir)
	}
	return remote.NewLocalFS(f.dir, vfs.Default), nil
}

// RemoteStorageFromURL parses a storage URL and returns both a
// remote.StorageFactory (for SSTables) and a remote.Storage (for metadata).
// Supported schemes:
//
//	file:///path/to/dir — local filesystem
//
// For file:// URLs, SSTables go in <path>/sstables/ and metadata in
// <path>/metadata/.
func RemoteStorageFromURL(rawURL string) (remote.StorageFactory, remote.Storage, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "parsing remote storage URL %q", rawURL)
	}
	switch u.Scheme {
	case "file":
		basePath := u.Path
		sstDir := basePath + "/sstables"
		metaDir := basePath + "/metadata"
		if err := os.MkdirAll(sstDir, 0755); err != nil {
			return nil, nil, errors.Wrapf(err, "creating sstables dir %s", sstDir)
		}
		if err := os.MkdirAll(metaDir, 0755); err != nil {
			return nil, nil, errors.Wrapf(err, "creating metadata dir %s", metaDir)
		}
		factory := NewLocalFSStorageFactory(sstDir)
		metaStore := remote.NewLocalFS(metaDir, vfs.Default)
		return factory, metaStore, nil
	default:
		return nil, nil, errors.Errorf("unsupported remote storage scheme %q (use file:// or s3://)", u.Scheme)
	}
}
