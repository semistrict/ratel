// Copyright 2023 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package storage

import (
	"context"
	"io"
	"sync/atomic"

	"github.com/cockroachdb/cockroach/pkg/cloud"
	"github.com/cockroachdb/pebble/objstorage/remote"
)

// externalStorageObjectReader implements remote.ObjectReader on top of a
// cloud.ExternalStorage, opening a new ranged-read stream for each ReadAt and
// recording bytes read against the parent Pebble metrics.
type externalStorageObjectReader struct {
	p       *Pebble
	es      cloud.ExternalStorage
	objName string
	objSize int64
}

var _ remote.ObjectReader = (*externalStorageObjectReader)(nil)

// ReadAt implements remote.ObjectReader. The contract requires a full read of
// len(p) bytes starting at offset; short reads must return an error.
func (r *externalStorageObjectReader) ReadAt(ctx context.Context, p []byte, offset int64) error {
	reader, _, err := r.es.ReadFileAt(ctx, r.objName, offset)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close(ctx) }()
	total := 0
	for total < len(p) {
		n, err := reader.Read(ctx, p[total:])
		total += n
		if err != nil {
			if err == io.EOF && total == len(p) {
				break
			}
			return err
		}
	}
	atomic.AddInt64(&r.p.sharedBytesRead, int64(total))
	return nil
}

// Close implements remote.ObjectReader.
func (r *externalStorageObjectReader) Close() error { return nil }

// externalStorageWriter wraps an io.WriteCloser returned by
// externalStorageWrapper and tracks metrics on bytes written to shared storage.
type externalStorageWriter struct {
	io.WriteCloser

	// Store a reference to the parent Pebble instance. Metrics around shared
	// storage reads/writes are stored there.
	p *Pebble
}

var _ io.WriteCloser = (*externalStorageWriter)(nil)

// Write implements the io.Writer interface.
func (e *externalStorageWriter) Write(p []byte) (n int, err error) {
	n, err = e.WriteCloser.Write(p)
	atomic.AddInt64(&e.p.sharedBytesWritten, int64(n))
	return n, err
}

// externalStorageWrapper wraps a cloud.ExternalStorage and implements the
// remote.Storage interface expected by Pebble. Also ensures reads and writes
// to shared cloud storage are tracked in store-specific metrics.
type externalStorageWrapper struct {
	p   *Pebble
	es  cloud.ExternalStorage
	ctx context.Context
}

var _ remote.Storage = (*externalStorageWrapper)(nil)

// Close implements remote.Storage.
func (e *externalStorageWrapper) Close() error {
	return e.es.Close()
}

// ReadObject implements remote.Storage. It returns an ObjectReader that opens
// a new range-read stream per ReadAt call and also reports the total object
// size (required by Pebble to plan reads).
func (e *externalStorageWrapper) ReadObject(
	ctx context.Context, objName string,
) (remote.ObjectReader, int64, error) {
	size, err := e.es.Size(ctx, objName)
	if err != nil {
		return nil, 0, err
	}
	return &externalStorageObjectReader{p: e.p, es: e.es, objName: objName, objSize: size}, size, nil
}

// CreateObject implements remote.Storage.
func (e *externalStorageWrapper) CreateObject(objName string) (io.WriteCloser, error) {
	writer, err := e.es.Writer(e.ctx, objName)
	if err != nil {
		return nil, err
	}
	return &externalStorageWriter{WriteCloser: writer, p: e.p}, nil
}

// List implements remote.Storage.
func (e *externalStorageWrapper) List(prefix, delimiter string) ([]string, error) {
	var directoryList []string
	err := e.es.List(e.ctx, prefix, delimiter, func(s string) error {
		directoryList = append(directoryList, s)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return directoryList, nil
}

// Delete implements remote.Storage.
func (e *externalStorageWrapper) Delete(objName string) error {
	return e.es.Delete(e.ctx, objName)
}

// Size implements remote.Storage.
func (e *externalStorageWrapper) Size(objName string) (int64, error) {
	return e.es.Size(e.ctx, objName)
}

// IsNotExistError implements remote.Storage. cloud.ExternalStorage does not
// expose a typed "not-found" error today; treat every error as possibly-exists
// so Pebble retries/falls through rather than silently skipping.
func (e *externalStorageWrapper) IsNotExistError(err error) bool {
	return false
}
