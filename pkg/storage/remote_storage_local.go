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
	"fmt"
	"net/url"
	"os"
	"strings"

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
// AWS credentials are read from the environment (AWS_ACCESS_KEY_ID,
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

// parseHTTPAsS3 treats an http(s) URL as an S3-compatible endpoint.
// The first path segment is the bucket, the rest is the prefix.
//
// Examples:
//
//	https://acct.r2.cloudflarestorage.com/mybucket/prefix/
//	https://fly.storage.tigris.dev/mybucket/prefix/
//	http://localhost:9000/mybucket/prefix/
func parseHTTPAsS3(u *url.URL) S3StorageConfig {
	// Split path into bucket + prefix.
	path := strings.TrimPrefix(u.Path, "/")
	bucket, prefix, _ := strings.Cut(path, "/")
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	region := u.Query().Get("region")
	if region == "" {
		region = "auto"
	}
	endpoint := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	// R2 uses virtual-hosted-style by default; most others use path-style.
	// Use path-style unless the bucket is embedded in the hostname.
	forcePathStyle := !strings.Contains(u.Host, bucket)
	return S3StorageConfig{
		Bucket:           bucket,
		Prefix:           prefix,
		Region:           region,
		Endpoint:         endpoint,
		S3ForcePathStyle: forcePathStyle,
	}
}

// clusterStorageFromS3 creates a ClusterStorage from an S3StorageConfig.
func clusterStorageFromS3(cfg S3StorageConfig) (*ClusterStorage, error) {
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
}

// parseGCSURL extracts GCSStorageConfig from a gcs:// or gs:// URL.
//
// Format: gcs://bucket/prefix/
func parseGCSURL(u *url.URL) GCSStorageConfig {
	prefix := strings.TrimPrefix(u.Path, "/")
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return GCSStorageConfig{
		Bucket: u.Host,
		Prefix: prefix,
	}
}

// clusterStorageFromGCS creates a ClusterStorage from a GCSStorageConfig.
func clusterStorageFromGCS(cfg GCSStorageConfig) (*ClusterStorage, error) {
	cfg.Prefix = cfg.Prefix + layoutVersion + "/"
	sstCfg := cfg
	sstCfg.Prefix = cfg.Prefix + "sstables/"
	metaStore, err := newGCSStorage(cfg, "metadata/")
	if err != nil {
		return nil, errors.Wrap(err, "creating GCS metadata storage")
	}
	nodesStore, err := newGCSStorage(cfg, "discovery/")
	if err != nil {
		return nil, errors.Wrap(err, "creating GCS discovery storage")
	}
	certsStore, err := newGCSStorage(cfg, "certs/")
	if err != nil {
		return nil, errors.Wrap(err, "creating GCS certs storage")
	}
	return &ClusterStorage{
		SSTableFactory: NewGCSStorageFactory(sstCfg),
		Metadata:       metaStore,
		Nodes:          nodesStore,
		Certs:          certsStore,
	}, nil
}

// newGCSStorage creates a GCSStorage instance with the given config and
// sub-prefix appended.
func newGCSStorage(cfg GCSStorageConfig, subPrefix string) (remote.Storage, error) {
	sub := cfg
	sub.Prefix = cfg.Prefix + subPrefix
	factory := NewGCSStorageFactory(sub)
	return factory.CreateStorage("")
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
		metaStore := remote.NewLocalFS(metaDir, vfs.Default)
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
// with separate remote.Storage instances for sstables, metadata, nodes, and
// certs. Supported schemes:
//
//	file:///path/to/dir — local filesystem
//	s3://bucket/prefix/?endpoint=...&region=... — S3-compatible storage
//	gcs://bucket/prefix/ — Google Cloud Storage (native auth)
//	https://endpoint/bucket/prefix/ — S3-compatible via HTTP URL (R2, Tigris, etc.)
//
// For https:// URLs, the first path segment is the bucket name and the
// remainder is the key prefix. The scheme + host form the S3 endpoint.
// Optional query parameters: region (default "auto").
func ClusterStorageFromURL(rawURL string) (*ClusterStorage, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing cluster storage URL %q", rawURL)
	}
	switch u.Scheme {
	case "http", "https":
		cfg := parseHTTPAsS3(u)
		return clusterStorageFromS3(cfg)
	case "gcs", "gs":
		cfg := parseGCSURL(u)
		return clusterStorageFromGCS(cfg)
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
			Metadata:       remote.NewLocalFS(dirs[1], vfs.Default),
			Nodes:          remote.NewLocalFS(dirs[2], vfs.Default),
			Certs:          remote.NewLocalFS(dirs[3], vfs.Default),
		}, nil
	case "s3":
		cfg := parseS3URL(u)
		return clusterStorageFromS3(cfg)
	default:
		return nil, errors.Errorf("unsupported cluster storage scheme %q (use file://, s3://, gcs://, or https://)", u.Scheme)
	}
}
