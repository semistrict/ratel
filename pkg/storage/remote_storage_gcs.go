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
	"io"
	"strings"

	gcs "cloud.google.com/go/storage"
	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/pebble/objstorage/remote"
	"google.golang.org/api/iterator"
)

// GCSStorageConfig holds configuration for GCS-backed remote storage.
type GCSStorageConfig struct {
	Bucket string
	Prefix string
}

// GCSStorageFactory implements remote.StorageFactory for GCS.
type GCSStorageFactory struct {
	config GCSStorageConfig
}

var _ remote.StorageFactory = (*GCSStorageFactory)(nil)

// NewGCSStorageFactory creates a new GCSStorageFactory.
func NewGCSStorageFactory(config GCSStorageConfig) *GCSStorageFactory {
	return &GCSStorageFactory{config: config}
}

// CreateStorage implements remote.StorageFactory.
func (f *GCSStorageFactory) CreateStorage(locator remote.Locator) (remote.Storage, error) {
	ctx := context.Background()
	client, err := gcs.NewClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "creating GCS client")
	}
	return &GCSStorage{
		bucket: client.Bucket(f.config.Bucket),
		prefix: f.config.Prefix,
		client: client,
	}, nil
}

// GCSStorage implements remote.Storage backed by Google Cloud Storage.
type GCSStorage struct {
	bucket *gcs.BucketHandle
	prefix string
	client *gcs.Client
}

var _ remote.Storage = (*GCSStorage)(nil)

func (g *GCSStorage) objName(name string) string {
	return g.prefix + name
}

// ReadObject implements remote.Storage.
func (g *GCSStorage) ReadObject(
	ctx context.Context, objName string,
) (_ remote.ObjectReader, objSize int64, _ error) {
	attrs, err := g.bucket.Object(g.objName(objName)).Attrs(ctx)
	if err != nil {
		return nil, 0, errors.Wrapf(err, "attrs for %q", objName)
	}
	return &gcsObjectReader{g: g, name: objName}, attrs.Size, nil
}

// CreateObject implements remote.Storage.
func (g *GCSStorage) CreateObject(objName string) (io.WriteCloser, error) {
	ctx := context.Background()
	w := g.bucket.Object(g.objName(objName)).NewWriter(ctx)
	return w, nil
}

// List implements remote.Storage.
func (g *GCSStorage) List(prefix, delimiter string) ([]string, error) {
	fullPrefix := g.prefix + prefix
	q := &gcs.Query{Prefix: fullPrefix}
	if delimiter != "" {
		q.Delimiter = delimiter
	}
	ctx := context.Background()
	it := g.bucket.Objects(ctx, q)
	var result []string
	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, errors.Wrapf(err, "list prefix=%q", prefix)
		}
		if attrs.Prefix != "" {
			name := strings.TrimPrefix(attrs.Prefix, g.prefix)
			name = strings.TrimPrefix(name, prefix)
			result = append(result, name)
		} else {
			name := strings.TrimPrefix(attrs.Name, g.prefix)
			name = strings.TrimPrefix(name, prefix)
			result = append(result, name)
		}
	}
	return result, nil
}

// Delete implements remote.Storage.
func (g *GCSStorage) Delete(objName string) error {
	ctx := context.Background()
	err := g.bucket.Object(g.objName(objName)).Delete(ctx)
	return errors.Wrapf(err, "delete %q", objName)
}

// Size implements remote.Storage.
func (g *GCSStorage) Size(objName string) (int64, error) {
	ctx := context.Background()
	attrs, err := g.bucket.Object(g.objName(objName)).Attrs(ctx)
	if err != nil {
		return 0, errors.Wrapf(err, "attrs for %q", objName)
	}
	return attrs.Size, nil
}

// IsNotExistError implements remote.Storage.
func (g *GCSStorage) IsNotExistError(err error) bool {
	return errors.Is(err, gcs.ErrObjectNotExist)
}

// Close implements remote.Storage.
func (g *GCSStorage) Close() error {
	return g.client.Close()
}

// gcsObjectReader implements remote.ObjectReader.
type gcsObjectReader struct {
	g    *GCSStorage
	name string
}

// ReadAt implements remote.ObjectReader.
func (r *gcsObjectReader) ReadAt(ctx context.Context, p []byte, offset int64) error {
	obj := r.g.bucket.Object(r.g.objName(r.name))
	rc, err := obj.NewRangeReader(ctx, offset, int64(len(p)))
	if err != nil {
		return errors.Wrapf(err, "range read %q at %d", r.name, offset)
	}
	defer rc.Close()
	_, err = io.ReadFull(rc, p)
	return err
}

// Close implements remote.ObjectReader.
func (r *gcsObjectReader) Close() error {
	return nil
}
