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
	"net/url"
	"os"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/pebble/objstorage/remote"
	"github.com/cockroachdb/pebble/vfs"
)

// listableLocalFS wraps pebble's remote.LocalFS with a working List
// implementation. Pebble's built-in localFSStore stubs out List to return
// (nil, nil) — fine for pebble's own benchmarking use case, but ratel relies
// on List to discover nodes and cluster certs.
type listableLocalFS struct {
	remote.Storage
	dir string
}

func newListableLocalFS(dir string) remote.Storage {
	return &listableLocalFS{
		Storage: remote.NewLocalFS(dir, vfs.Default),
		dir:     dir,
	}
}

// List walks the backing directory, returning the file names. The prefix and
// delimiter arguments follow the pebble remote.Storage contract: names with
// the given prefix are returned, and delimiter truncates at the next occurrence
// after the prefix. We don't use delimiters today, so support is minimal.
func (l *listableLocalFS) List(prefix, delimiter string) ([]string, error) {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, errors.Wrapf(err, "listing %s", l.dir)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		if delimiter != "" {
			if idx := strings.Index(name[len(prefix):], delimiter); idx >= 0 {
				name = name[:len(prefix)+idx+len(delimiter)]
			}
		}
		out = append(out, name)
	}
	return out, nil
}


// LocalFSStorageFactory implements remote.StorageFactory using a local
// filesystem directory.
type LocalFSStorageFactory struct {
	dir string
}

var _ remote.StorageFactory = (*LocalFSStorageFactory)(nil)

// NewLocalFSStorageFactory creates a factory that produces local-filesystem
// backed remote.Storage instances rooted at dir.
func NewLocalFSStorageFactory(dir string) *LocalFSStorageFactory {
	return &LocalFSStorageFactory{dir: dir}
}

// CreateStorage implements remote.StorageFactory.
func (f *LocalFSStorageFactory) CreateStorage(locator remote.Locator) (remote.Storage, error) {
	if err := os.MkdirAll(f.dir, 0755); err != nil {
		return nil, errors.Wrapf(err, "creating local storage dir %s", f.dir)
	}
	return newListableLocalFS(f.dir), nil
}

// layoutVersion is prepended to all sub-paths in the storage URL so that
// incompatible layout changes can coexist or be detected.
const layoutVersion = "v1"

// ClusterStorage holds all remote.Storage instances for a ratel cluster URL.
type ClusterStorage struct {
	SSTableFactory remote.StorageFactory // sstables/
	Metadata       remote.Storage        // metadata/
	Nodes          remote.Storage        // discovery/
	Certs          remote.Storage        // certs/
}

// parseS3URL extracts S3StorageConfig from an s3:// URL.
//
// Format: s3://bucket/prefix/?endpoint=http://host:9000&region=us-east-1
//
// Query parameters:
//   - endpoint: S3 endpoint URL (required for S3-compatible stores like rustfs/minio)
//   - region: AWS region (defaults to "us-east-1")
//
// AWS credentials come from the environment (AWS_ACCESS_KEY_ID,
// AWS_SECRET_ACCESS_KEY) or the default credential chain.
func parseS3URL(u *url.URL) S3StorageConfig {
	prefix := strings.TrimPrefix(u.Path, "/")
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	region := u.Query().Get("region")
	if region == "" {
		region = "us-east-1"
	}
	return S3StorageConfig{
		Bucket:           u.Host,
		Prefix:           prefix,
		Region:           region,
		Endpoint:         u.Query().Get("endpoint"),
		S3ForcePathStyle: true,
	}
}

// newS3Storage creates an S3Storage instance with the given config and
// sub-prefix appended.
func newS3Storage(cfg S3StorageConfig, subPrefix string) (remote.Storage, error) {
	sub := cfg
	sub.Prefix = cfg.Prefix + subPrefix
	factory := NewS3StorageFactory(sub)
	return factory.CreateStorage("")
}

// RemoteStorageFromURL parses a storage URL and returns both a
// remote.StorageFactory (for SSTables) and a remote.Storage (for metadata).
// Supported schemes:
//
//	file:///path/to/dir — local filesystem
//	s3://bucket/prefix/?endpoint=...&region=... — S3-compatible storage
//
// SSTables go in <base>/sstables/ and metadata in <base>/metadata/.
func RemoteStorageFromURL(rawURL string) (remote.StorageFactory, remote.Storage, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "parsing remote storage URL %q", rawURL)
	}
	switch u.Scheme {
	case "file":
		basePath := u.Path + "/" + layoutVersion
		sstDir := basePath + "/sstables"
		metaDir := basePath + "/metadata"
		if err := os.MkdirAll(sstDir, 0755); err != nil {
			return nil, nil, errors.Wrapf(err, "creating sstables dir %s", sstDir)
		}
		if err := os.MkdirAll(metaDir, 0755); err != nil {
			return nil, nil, errors.Wrapf(err, "creating metadata dir %s", metaDir)
		}
		factory := NewLocalFSStorageFactory(sstDir)
		metaStore := newListableLocalFS(metaDir)
		return factory, metaStore, nil
	case "s3":
		cfg := parseS3URL(u)
		cfg.Prefix = cfg.Prefix + layoutVersion + "/"
		sstCfg := cfg
		sstCfg.Prefix = cfg.Prefix + "sstables/"
		factory := NewS3StorageFactory(sstCfg)
		metaStore, err := newS3Storage(cfg, "metadata/")
		if err != nil {
			return nil, nil, errors.Wrap(err, "creating S3 metadata storage")
		}
		return factory, metaStore, nil
	default:
		return nil, nil, errors.Errorf("unsupported remote storage scheme %q (use file:// or s3://)", u.Scheme)
	}
}

// ClusterStorageFromURL parses a storage URL and returns a ClusterStorage
// with separate remote.Storage instances for sstables, metadata, discovery,
// and certs. Supported schemes:
//
//	file:///path/to/dir — local filesystem
//	s3://bucket/prefix/?endpoint=...&region=... — S3-compatible storage
func ClusterStorageFromURL(rawURL string) (*ClusterStorage, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing cluster storage URL %q", rawURL)
	}
	switch u.Scheme {
	case "file":
		basePath := u.Path + "/" + layoutVersion
		dirs := []string{
			basePath + "/sstables",
			basePath + "/metadata",
			basePath + "/discovery",
			basePath + "/certs",
		}
		for _, d := range dirs {
			if err := os.MkdirAll(d, 0755); err != nil {
				return nil, errors.Wrapf(err, "creating dir %s", d)
			}
		}
		return &ClusterStorage{
			SSTableFactory: NewLocalFSStorageFactory(dirs[0]),
			Metadata:       newListableLocalFS(dirs[1]),
			Nodes:          newListableLocalFS(dirs[2]),
			Certs:          newListableLocalFS(dirs[3]),
		}, nil
	case "s3":
		cfg := parseS3URL(u)
		cfg.Prefix = cfg.Prefix + layoutVersion + "/"
		sstCfg := cfg
		sstCfg.Prefix = cfg.Prefix + "sstables/"
		metaStore, err := newS3Storage(cfg, "metadata/")
		if err != nil {
			return nil, errors.Wrap(err, "creating S3 metadata storage")
		}
		nodesStore, err := newS3Storage(cfg, "discovery/")
		if err != nil {
			return nil, errors.Wrap(err, "creating S3 nodes storage")
		}
		certsStore, err := newS3Storage(cfg, "certs/")
		if err != nil {
			return nil, errors.Wrap(err, "creating S3 certs storage")
		}
		return &ClusterStorage{
			SSTableFactory: NewS3StorageFactory(sstCfg),
			Metadata:       metaStore,
			Nodes:          nodesStore,
			Certs:          certsStore,
		}, nil
	default:
		return nil, errors.Errorf("unsupported cluster storage scheme %q (use file:// or s3://)", u.Scheme)
	}
}
